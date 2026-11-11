package core

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"kiko/core/document"
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
func TestDocumentUpdateAppliesDirectlyWithoutReview(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	s.Store.CreateChat(ctx, "")
	state, _ := s.Store.Snapshot(ctx)
	chat := state.Selection.ChatID
	doc, err := s.Store.CreateDocument(ctx, chat, "Plan", "# Plan\n\nDraft", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefs.SuggestionMode = true
	if err = s.Store.SavePreferences(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"doc_id": doc.ID, "ops": []any{map[string]any{"type": "replace", "target": map[string]any{"find": "Draft"}, "text": "Done"}}})
	output, err := s.executeTool(ctx, storage.Run{ChatID: chat}, provider.ToolCall{Function: provider.Function{Name: "update", Arguments: string(args)}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "suggestion_id") {
		t.Fatal("update created a review proposal", output)
	}
	read, err := s.Store.ReadDocument(ctx, doc.ID, storage.ReadOptions{})
	if err != nil || read.Document.Rev != 2 || !strings.Contains(read.Markdown, "Done") || len(read.Suggestions) != 0 {
		t.Fatal("update did not apply directly", read, err)
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
func TestGuideSlidesTopicAddsDesignRules(t *testing.T) {
	s := chatService(t)
	slides, err := s.executeTool(context.Background(), storage.Run{}, provider.ToolCall{Function: provider.Function{Name: "guide", Arguments: `{"topics":["slides"]}`}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(slides, "Slides:") || !strings.Contains(slides, "16:9") {
		t.Fatal("slide design rules missing", slides)
	}
	editing, err := s.executeTool(context.Background(), storage.Run{}, provider.ToolCall{Function: provider.Function{Name: "guide", Arguments: `{"topics":["editing"]}`}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(editing, "Slides:") {
		t.Fatal("slide rules leaked into editing topic")
	}
}

func TestCreateDiagramToolDrawsEditableExcalidrawScene(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	s.Store.CreateChat(ctx, "")
	state, _ := s.Store.Snapshot(ctx)
	chat := state.Selection.ChatID
	doc, err := s.Store.CreateDocument(ctx, chat, "Plan", "# Plan", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	call := func(arguments string) (string, error) {
		return s.executeTool(ctx, storage.Run{ChatID: chat}, provider.ToolCall{Function: provider.Function{Name: "create_diagram", Arguments: arguments}})
	}
	good := `{"doc_id":"` + doc.ID + `","caption":"Flow","elements":[{"type":"rectangle","id":"a","x":0,"y":0,"width":160,"height":60,"label":{"text":"Client"}},{"type":"rectangle","id":"b","x":300,"y":0,"width":160,"height":60,"label":{"text":"Server"}},{"type":"arrow","start":{"id":"a"},"end":{"id":"b"}}]}`
	output, err := call(good)
	if err != nil || !strings.Contains(output, "Diagram") {
		t.Fatal("diagram was not created", output, err)
	}
	read, err := s.Store.ReadDocument(ctx, doc.ID, storage.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	attrs := read.Document.Blocks[len(read.Document.Blocks)-1].Attrs
	if !strings.Contains(document.String(attrs, "scene"), `"skeleton":true`) || !strings.Contains(document.String(attrs, "svg"), "Server") {
		t.Fatal("saved diagram lacks scene or SVG", attrs)
	}
	for _, bad := range []string{`{"doc_id":"` + doc.ID + `","caption":"x","code":"<svg/>"}`, `{"doc_id":"` + doc.ID + `","caption":"x","elements":[{"type":"arrow","start":{"id":"missing"},"end":{"id":"missing"}}]}`} {
		if _, err = call(bad); err == nil {
			t.Fatal("bad diagram accepted", bad)
		}
	}
}
