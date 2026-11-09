package core

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiko/core/provider"
	"kiko/core/storage"
)

type replyProvider func(context.Context, provider.Request, func(provider.Event) error) error

func (p replyProvider) Stream(ctx context.Context, r provider.Request, emit func(provider.Event) error) error {
	return p(ctx, r, emit)
}
func chatService(t *testing.T) *Service {
	t.Helper()
	s, _, _ := newService(t)
	if err := s.SaveNickname(context.Background(), "Haki"); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(context.Background(), "fixture-key"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Close(ctx)
	})
	return s
}
func waitReply(t *testing.T, events <-chan storage.Message) storage.Message {
	t.Helper()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case m := <-events:
			if m.Status != "running" {
				return m
			}
		case <-timeout.C:
			t.Fatal("reply did not finish")
			return storage.Message{}
		}
	}
}
func TestChatStreamsPersistsAndReplaysHistory(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	events := make(chan storage.Message, 20)
	var calls atomic.Int32
	s.Provider = replyProvider(func(_ context.Context, r provider.Request, emit func(provider.Event) error) error {
		n := calls.Add(1)
		if r.Key != "fixture-key" || r.Model != "muse-spark-1.3-contributor" || len(r.Messages) != int(n)*2 {
			return errors.New("incorrect request history")
		}
		if n == 2 && r.Messages[2].Content != "Hello 世界" {
			return errors.New("saved reply missing from history")
		}
		if err := emit(provider.Event{Text: "Hello "}); err != nil {
			return err
		}
		if err := emit(provider.Event{Text: "世界"}); err != nil {
			return err
		}
		return emit(provider.Event{Finish: "stop"})
	})
	emit := func(m storage.Message) { events <- m }
	first, err := s.Send(ctx, "", "", "Hi", "send-one", emit)
	if err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.Status != "complete" || final.Content != "Hello 世界" {
		t.Fatal(final)
	}
	if _, err = s.Send(ctx, "", "", "Hi", "send-one", emit); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate request started provider twice")
	}
	second, err := s.Send(ctx, "", first.Selection.ChatID, "Again", "send-two", emit)
	if err != nil {
		t.Fatal(err)
	}
	final = waitReply(t, events)
	saved, err := s.Store.Conversation(ctx, "", second.Selection.ChatID)
	if err != nil || len(saved.Messages) != 4 || final.Status != "complete" {
		t.Fatal(err, saved, final)
	}
}
func TestStopRetryAndLateCallbacksAreIsolated(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	events := make(chan storage.Message, 30)
	started := make(chan struct{})
	late := make(chan error, 1)
	var calls atomic.Int32
	s.Provider = replyProvider(func(ctx context.Context, _ provider.Request, emit func(provider.Event) error) error {
		if calls.Add(1) == 1 {
			if err := emit(provider.Event{Text: "Partial"}); err != nil {
				return err
			}
			close(started)
			<-ctx.Done()
			late <- emit(provider.Event{Text: " stale"})
			return ctx.Err()
		}
		if err := emit(provider.Event{Text: "Replacement"}); err != nil {
			return err
		}
		return emit(provider.Event{Finish: "stop"})
	})
	state, err := s.Send(ctx, "", "", "Hi", "one", func(m storage.Message) { events <- m })
	if err != nil {
		t.Fatal(err)
	}
	<-started
	running := state.Messages[1]
	if err = s.Store.SaveDraft(ctx, "", running.ChatID, "Next question"); err != nil {
		t.Fatal(err)
	}
	s.Stop(running.RunID, running.Attempt)
	stopped := waitReply(t, events)
	if stopped.Status != "stopped" || stopped.Content != "Partial" || !errors.Is(<-late, context.Canceled) {
		t.Fatal(stopped)
	}
	if _, err = s.Retry(ctx, running.RunID, "retry", func(m storage.Message) { events <- m }); err != nil {
		t.Fatal(err)
	}
	s.Stop(running.RunID, running.Attempt)
	final := waitReply(t, events)
	if final.Status != "complete" || final.Content != "Replacement" || len(final.Previous) != 1 || final.Previous[0].Content != "Partial" {
		t.Fatal(final)
	}
	saved, err := s.Store.Conversation(ctx, "", running.ChatID)
	if err != nil || len(saved.Messages) != 2 || saved.Draft != "Next question" {
		t.Fatal(err, saved)
	}
}
func TestChatFailureDoesNotLeakSecretsOrLoseDraft(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	events := make(chan storage.Message, 10)
	s.Provider = replyProvider(func(_ context.Context, _ provider.Request, emit func(provider.Event) error) error {
		if err := emit(provider.Event{Text: "Saved partial"}); err != nil {
			return err
		}
		return errors.New("fixture-key private response")
	})
	state, err := s.Send(ctx, "", "", "Hi", "one", func(m storage.Message) { events <- m })
	if err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.Status != "error" || final.Content != "Saved partial" || strings.Contains(final.Error, "fixture-key") {
		t.Fatal(final)
	}
	s.Keys = &fakeSecrets{}
	if err = s.Store.SaveDraft(ctx, "", state.Selection.ChatID, "Unsent"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Send(ctx, "", state.Selection.ChatID, "Unsent", "two", nil); err == nil {
		t.Fatal("missing key accepted")
	}
	saved, _ := s.Store.Conversation(ctx, "", state.Selection.ChatID)
	if len(saved.Messages) != 2 || saved.Draft != "Unsent" {
		t.Fatal(saved)
	}
}
func TestReplySaveFailureIsVisible(t *testing.T) {
	s := chatService(t)
	events := make(chan storage.Message, 10)
	s.Provider = replyProvider(func(_ context.Context, _ provider.Request, emit func(provider.Event) error) error {
		s.Store.Close()
		return emit(provider.Event{Text: "Copy this text"})
	})
	if _, err := s.Send(context.Background(), "", "", "Hi", "one", func(m storage.Message) { events <- m }); err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.ErrorKind != "storage" || final.Content != "Copy this text" || final.ID == "" {
		t.Fatal(final)
	}
}
func TestDeleteDuringReplyAndShutdown(t *testing.T) {
	s := chatService(t)
	started := make(chan struct{})
	done := make(chan struct{})
	s.Provider = replyProvider(func(ctx context.Context, _ provider.Request, _ func(provider.Event) error) error {
		close(started)
		<-ctx.Done()
		close(done)
		return ctx.Err()
	})
	state, err := s.Send(context.Background(), "", "", "Hi", "one", nil)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err = s.Delete(context.Background(), "chat", state.Selection.ChatID); err != nil {
		t.Fatal(err)
	}
	<-done
	if err = s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
