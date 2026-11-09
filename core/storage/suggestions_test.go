package storage

import (
	"context"
	"testing"

	"kiko/core/document"
)

func TestSuggestionsDoNotWriteUntilAcceptedAndRefuseStaleContent(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "Original", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	op := document.Operation{Type: "replace", Target: document.Target{Find: "Original"}, Text: markdownPointer("Suggested")}
	proposal, err := s.ProposeDocument(ctx, doc.ID, []document.Operation{op})
	if err != nil {
		t.Fatal(err)
	}
	current, _ := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if current.Document.Rev != 1 || current.Markdown != "Original" || len(current.Suggestions) != 1 {
		t.Fatal(current)
	}
	accepted, err := s.AcceptSuggestion(ctx, proposal.ID)
	if err != nil || accepted.Rev != 2 || document.Plain(accepted.Blocks[0]) != "Suggested" {
		t.Fatal(err, accepted)
	}
	stale, err := s.ProposeDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Suggested"}, Text: markdownPointer("Later")}})
	if err != nil {
		t.Fatal(err)
	}
	accepted.Blocks[0].Content[0].Text = "User edits"
	if _, err = s.SaveDocument(ctx, doc.ID, 2, accepted.Blocks); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptSuggestion(ctx, stale.ID); err == nil {
		t.Fatal("stale suggestion overwrote user changes")
	}
	if err = s.RejectSuggestion(ctx, stale.ID); err != nil {
		t.Fatal(err)
	}
}
