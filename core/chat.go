package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"kiko/core/provider"
	"kiko/core/storage"
)

type activeRun struct {
	client  provider.Provider
	initial storage.Message
	run     storage.Run
	cancel  context.CancelFunc
	done    chan struct{}
}

func (s *Service) sendKey(ctx context.Context, chat string) (string, error) {
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return "", errors.New("Couldn’t read local setup.")
	}
	if prefs.Stage != "done" || prefs.ResetInProgress {
		return "", errors.New("Finish Meta Muse setup before sending.")
	}
	key, err := s.Keys.Get()
	if err != nil || key == "" {
		return "", &provider.Error{Kind: "auth", Message: "Reconnect Meta Muse to send messages. Your draft is saved."}
	}
	if s.Provider == nil {
		return "", errors.New("The model provider is unavailable.")
	}
	return key, nil
}
func (s *Service) Send(ctx context.Context, project, chat, text, requestID string, emit func(storage.Message)) (storage.State, error) {
	key, err := s.sendKey(ctx, chat)
	if err != nil {
		return storage.State{}, err
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.closing {
		return storage.State{}, errors.New("Kiko is closing.")
	}
	run, created, err := s.Store.StartRun(ctx, project, chat, text, requestID)
	if err != nil {
		return storage.State{}, err
	}
	return s.startReply(ctx, run, created, key, emit)
}
func (s *Service) Retry(ctx context.Context, runID, requestID string, emit func(storage.Message)) (storage.State, error) {
	runInfo, err := s.Store.Run(ctx, runID)
	if err != nil {
		return storage.State{}, err
	}
	key, err := s.sendKey(ctx, runInfo.ChatID)
	if err != nil {
		return storage.State{}, err
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.closing {
		return storage.State{}, errors.New("Kiko is closing.")
	}
	run, created, err := s.Store.RetryRun(ctx, runID, requestID)
	if err != nil {
		return storage.State{}, err
	}
	return s.startReply(ctx, run, created, key, emit)
}
func (s *Service) startReply(ctx context.Context, run storage.Run, created bool, key string, emit func(storage.Message)) (storage.State, error) {
	state, err := s.Store.Snapshot(ctx)
	if err != nil {
		return state, err
	}
	state.Connected = true
	if !created {
		return state, nil
	}
	history, err := s.Store.History(ctx, run)
	if err != nil {
		_, _ = s.Store.UpdateRun(ctx, run, "", "error", "Couldn’t load the conversation. Retry this reply.", "storage", nil)
		return state, err
	}
	docs, _ := s.Store.ChatDocuments(ctx, run.ChatID)
	contextDocs, _ := json.Marshal(docs)
	messages := []provider.Message{{Role: "developer", Content: documentInstructions + "\nDocuments in this chat (read current content before later edits): " + string(contextDocs)}}
	for _, m := range history {
		if m.Role == "assistant" {
			steps, err := s.Store.Activities(ctx, m.ID)
			if err != nil {
				return state, err
			}
			for _, step := range steps {
				if step.Status != "ok" && step.Status != "refused" {
					continue
				}
				var detail struct {
					Call   provider.ToolCall `json:"call"`
					Result string            `json:"result"`
				}
				if json.Unmarshal(step.Detail, &detail) == nil && detail.Call.ID != "" {
					messages = append(messages, provider.Message{Role: "assistant", ToolCalls: []provider.ToolCall{detail.Call}}, provider.Message{Role: "tool", ToolCallID: detail.Call.ID, Content: detail.Result})
				}
			}
		}
		content := m.Content
		if m.Role == "assistant" && m.Status != "complete" {
			content = "[This earlier reply ended before completion.]\n" + content
		}
		messages = append(messages, provider.Message{Role: m.Role, Content: content})
	}
	client := s.Provider
	runCtx, cancel := context.WithCancel(ctx)
	active := &activeRun{client: client, run: run, cancel: cancel, done: make(chan struct{})}
	for _, message := range state.Messages {
		if message.ID == run.AssistantID {
			active.initial = message
		}
	}
	if s.active == nil {
		s.active = make(map[string]*activeRun)
	}
	s.active[run.ChatID] = active
	go s.generate(runCtx, active, provider.Request{Key: key, Model: run.Model, Messages: messages}, emit)
	return state, nil
}
func (s *Service) generate(ctx context.Context, active *activeRun, request provider.Request, emit func(storage.Message)) {
	defer func() {
		active.cancel()
		s.runMu.Lock()
		if s.active[active.run.ChatID] == active {
			delete(s.active, active.run.ChatID)
		}
		close(active.done)
		s.runMu.Unlock()
	}()
	var text strings.Builder
	var usage json.RawMessage
	finish := ""
	lastSaved := time.Time{}
	lastMessage := active.initial
	publish := func(content, status, detail, kind string) error {
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		message, err := s.Store.UpdateRun(saveCtx, active.run, content, status, detail, kind, usage)
		if err == nil {
			lastMessage = message
			if emit != nil {
				emit(message)
			}
		}
		return err
	}
	err := s.agentStream(ctx, active.client, active.run, request, func(event provider.Event) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if event.Text != "" {
			if text.Len()+len(event.Text) > 4*1024*1024 {
				return &provider.Error{Kind: "limit", Message: "The reply exceeded this app’s size limit."}
			}
			text.WriteString(event.Text)
		}
		if len(event.Usage) > 0 {
			usage = event.Usage
		}
		if event.Finish != "" {
			finish = event.Finish
		}
		if event.Text != "" && time.Since(lastSaved) >= 50*time.Millisecond {
			if err := publish(text.String(), "running", "", ""); err != nil {
				return &provider.Error{Kind: "storage", Message: "Couldn’t save the reply. Your sent message is still in the conversation."}
			}
			lastSaved = time.Now()
		}
		return nil
	}, func() error { return publish(text.String(), "running", "", "") })
	status, detail, kind := "complete", "", ""
	switch {
	case errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled):
		status = "stopped"
	case err != nil:
		status = "error"
		kind = "provider"
		detail = "Couldn’t complete this reply. Retry when you’re ready."
		var problem *provider.Error
		if errors.As(err, &problem) {
			kind = problem.Kind
			detail = problem.Message
			if kind == "interrupted" {
				status = "interrupted"
			}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			kind = "timeout"
			detail = "The reply took too long. Your partial response is saved."
		}
	case finish == "length":
		status = "truncated"
		detail = "The reply reached its output limit. You can retry or ask Muse to continue."
	case finish == "content_filter":
		status = "filtered"
		detail = "Meta could not complete this response. Try adjusting your request."
	case finish != "stop":
		status = "interrupted"
		kind = "interrupted"
		detail = "The response ended before completion. Your partial reply is saved."
	case strings.TrimSpace(text.String()) == "":
		status = "error"
		kind = "empty"
		detail = "Meta returned an empty reply. Try again."
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = s.Store.FinishOpenActivity(cleanupCtx, active.run.AssistantID)
	cleanupCancel()
	if err := publish(text.String(), status, detail, kind); err != nil && !errors.Is(err, storage.ErrStaleRun) && emit != nil {
		lastMessage.Content = text.String()
		lastMessage.Status = "error"
		lastMessage.Error = "Couldn’t save the final reply. Keep this window open and copy any text you need."
		lastMessage.ErrorKind = "storage"
		lastMessage.Sequence++
		emit(lastMessage)
	}
}
func (s *Service) Stop(runID string, attempt int) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	for _, active := range s.active {
		if active.run.ID == runID && active.run.Attempt == attempt {
			active.cancel()
		}
	}
}
func (s *Service) Delete(ctx context.Context, kind, id string) error {
	s.runMu.Lock()
	for _, active := range s.active {
		if (kind == "chat" && active.run.ChatID == id) || (kind == "project" && active.run.ProjectID == id) {
			active.cancel()
		}
	}
	s.runMu.Unlock()
	return s.Store.Delete(ctx, kind, id)
}
func (s *Service) Close(ctx context.Context) error {
	s.runMu.Lock()
	s.closing = true
	pending := []*activeRun{}
	for _, active := range s.active {
		active.cancel()
		pending = append(pending, active)
	}
	s.runMu.Unlock()
	for _, active := range pending {
		select {
		case <-active.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.Store.Close()
}
func (s *Service) Reconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.Stage = "muse"
	return s.Store.SavePreferences(ctx, prefs)
}

func (s *Service) AnswerQuestion(ctx context.Context, id, text, requestID string, emit func(storage.Message)) (storage.State, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 4000 {
		return storage.State{}, errors.New("Enter a short answer.")
	}
	chat, quote, body, err := s.Store.QuestionContext(ctx, id)
	if err != nil {
		return storage.State{}, errors.New("This question is unavailable.")
	}
	if err = s.Store.ResolveQuestion(ctx, id, true); err != nil {
		return storage.State{}, err
	}
	state, err := s.Send(ctx, "", chat, "Regarding the passage “"+quote+"” and your question “"+body+"”: "+text, requestID, emit)
	if err != nil {
		_ = s.Store.ResolveQuestion(ctx, id, false)
	}
	return state, err
}
