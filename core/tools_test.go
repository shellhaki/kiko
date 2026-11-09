package core

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"kiko/core/provider"
	"kiko/core/storage"
)

func TestAgentCreatesAndFillsDocumentWithSavedActivity(t *testing.T) {
	s := chatService(t)
	events := make(chan storage.Message, 50)
	var calls atomic.Int32
	s.Provider = replyProvider(func(_ context.Context, r provider.Request, emit func(provider.Event) error) error {
		switch calls.Add(1) {
		case 1:
			if len(r.Tools) < 4 {
				t.Error("document tools missing")
			}
			args := `{"name":"Plan","markdown":"# Plan\n\n<?kiko block goals?>","blocks":{"goals":{"type":"pending","intent":"Goals"}}}`
			emit(provider.Event{Tool: &provider.ToolDelta{Index: 0, ID: "create", Name: "create_document", Arguments: args}})
			return emit(provider.Event{Finish: "tool_calls"})
		case 2:
			result := r.Messages[len(r.Messages)-1]
			if result.Role != "tool" || result.ToolCallID != "create" {
				t.Error("tool result missing")
			}
			fields := strings.Fields(result.Content)
			docID := fields[1]
			pending := ""
			for _, line := range strings.Split(result.Content, "\n") {
				if strings.HasPrefix(line, "pending ") {
					pending = strings.Fields(line)[1]
				}
			}
			args, _ := json.Marshal(map[string]any{"doc_id": docID, "ops": []any{map[string]any{"type": "replace", "target": map[string]any{"ids": []string{pending}}, "markdown": "## Goals\n\nShip a reliable app."}}})
			emit(provider.Event{Tool: &provider.ToolDelta{ID: "fill", Name: "update", Arguments: string(args)}})
			return emit(provider.Event{Finish: "tool_calls"})
		default:
			if calls.Load() == 4 {
				seen := 0
				for _, message := range r.Messages {
					if message.Role == "tool" {
						seen++
					}
				}
				if seen < 2 {
					t.Error("saved tool results missing from later turn")
				}
			}
			emit(provider.Event{Text: "Your plan is ready."})
			return emit(provider.Event{Finish: "stop"})
		}
	})
	state, err := s.Send(context.Background(), "", "", "Write a plan", "one", func(m storage.Message) { events <- m })
	if err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.Status != "complete" || len(final.Steps) != 2 || final.Steps[1].Status != "ok" {
		t.Fatal(final)
	}
	docs, err := s.Store.ChatDocuments(context.Background(), state.Selection.ChatID)
	if err != nil || len(docs) != 1 || docs[0].Rev != 2 {
		t.Fatal(err, docs)
	}
	if _, err = s.Send(context.Background(), "", state.Selection.ChatID, "Continue", "later", func(m storage.Message) { events <- m }); err != nil {
		t.Fatal(err)
	}
	if later := waitReply(t, events); later.Status != "complete" {
		t.Fatal(later)
	}
}
func TestToolScopeMalformedArgumentsAndStepLimit(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	s.Store.CreateChat(ctx, "")
	state, _ := s.Store.Snapshot(ctx)
	chat := state.Selection.ChatID
	doc, err := s.Store.CreateDocument(ctx, chat, "Private", "Text", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"doc_id": doc.ID, "payload": "view"})
	_, err = s.executeTool(ctx, storage.Run{ChatID: "other"}, provider.ToolCall{Function: provider.Function{Name: "read", Arguments: string(args)}})
	if err == nil {
		t.Fatal("cross-chat read accepted")
	}
	if decodeArguments(`{"topics":[]} {}`, &struct {
		Topics []string `json:"topics"`
	}{}) == nil {
		t.Fatal("trailing JSON accepted")
	}
	s.Provider = replyProvider(func(_ context.Context, _ provider.Request, emit func(provider.Event) error) error {
		emit(provider.Event{Tool: &provider.ToolDelta{ID: "guide", Name: "guide", Arguments: `{"topics":["editing"]}`}})
		return emit(provider.Event{Finish: "tool_calls"})
	})
	events := make(chan storage.Message, 100)
	_, err = s.Send(ctx, "", chat, "Loop", "loop", func(m storage.Message) { events <- m })
	if err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.ErrorKind != "limit" || len(final.Steps) != 30 {
		t.Fatal(final.ErrorKind, len(final.Steps))
	}
}
func TestIncompleteToolsNeverExecute(t *testing.T) {
	s := chatService(t)
	events := make(chan storage.Message, 10)
	s.Provider = replyProvider(func(_ context.Context, _ provider.Request, emit func(provider.Event) error) error {
		emit(provider.Event{Tool: &provider.ToolDelta{ID: "partial", Name: "create_document", Arguments: `{"name":"`}})
		return emit(provider.Event{Finish: "length"})
	})
	state, err := s.Send(context.Background(), "", "", "Plan", "partial", func(m storage.Message) { events <- m })
	if err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.Status != "interrupted" {
		t.Fatal(final)
	}
	docs, _ := s.Store.ChatDocuments(context.Background(), state.Selection.ChatID)
	if len(docs) > 0 {
		t.Fatal("incomplete tool executed")
	}
}
