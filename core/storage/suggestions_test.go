package storage

import (
	"context"
	"strings"
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
func userFindEdit(t *testing.T, s *Store, id, from, to string) {
	t.Helper()
	_, err := s.UpdateDocument(context.Background(), id, []document.Operation{{Type: "replace", Target: document.Target{Find: from}, Text: markdownPointer(to)}}, "user")
	if err != nil {
		t.Fatal(err)
	}
}
func TestAcceptSuggestionRebasesAcrossUnrelatedEdits(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "# Plan\n\nAlpha\n\nBeta", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := s.ProposeDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Beta"}, Text: markdownPointer("Gamma")}})
	if err != nil {
		t.Fatal(err)
	}
	userFindEdit(t, s, doc.ID, "Alpha", "Alpha!")
	accepted, err := s.AcceptSuggestion(ctx, proposal.ID)
	if err != nil {
		t.Fatal("healable suggestion refused", err)
	}
	markdown := document.Markdown(accepted.Blocks)
	if !strings.Contains(markdown, "Alpha") || !strings.Contains(markdown, "Gamma") {
		t.Fatal("accept lost edits", markdown)
	}
}
func TestAcceptSuggestionWithdrawsOnConflict(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "# Plan\n\nAlpha\n\nBeta", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := s.ProposeDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Beta"}, Text: markdownPointer("Gamma")}})
	if err != nil {
		t.Fatal(err)
	}
	userFindEdit(t, s, doc.ID, "Beta", "User words")
	if _, err = s.AcceptSuggestion(ctx, proposal.ID); err == nil {
		t.Fatal("conflicting suggestion applied")
	}
	read, err := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read.Markdown, "User words") || strings.Contains(read.Markdown, "Gamma") {
		t.Fatal("user content lost", read.Markdown)
	}
	if len(read.Suggestions) != 0 {
		t.Fatal("withdrawn suggestion still listed", read.Suggestions)
	}
}
func TestReadRebasesStaleSuggestion(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "# Plan\n\nAlpha\n\nBeta", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ProposeDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Beta"}, Text: markdownPointer("Gamma")}}); err != nil {
		t.Fatal(err)
	}
	userFindEdit(t, s, doc.ID, "Alpha", "Alpha!")
	read, err := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil || len(read.Suggestions) != 1 || read.Suggestions[0].BaseRev != read.Document.Rev {
		t.Fatal("stale suggestion not rebased on read", read.Suggestions, err)
	}
}
func TestReadRetiresConflictingSuggestion(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "# Plan\n\nAlpha\n\nBeta", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ProposeDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Beta"}, Text: markdownPointer("Gamma")}}); err != nil {
		t.Fatal(err)
	}
	userFindEdit(t, s, doc.ID, "Beta", "User words")
	read, err := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil || len(read.Suggestions) != 0 {
		t.Fatal("conflicting suggestion still listed", read.Suggestions, err)
	}
	if !strings.Contains(read.Markdown, "User words") {
		t.Fatal("user content lost", read.Markdown)
	}
}
func TestStaleFillAgainstUserTextIsWithdrawn(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "# Plan\n\n<?kiko block goals?>", map[string]document.Placeholder{"goals": {Type: "pending", Intent: "Goals"}}, "agent")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var pending string
	for _, block := range document.Flatten(loaded.Document.Blocks) {
		if block.Type == "pending" {
			pending = block.ID
		}
	}
	proposal, err := s.ProposeDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{IDs: []string{pending}}, Markdown: markdownPointer("## Goals\n\nAgent text.")}})
	if err != nil {
		t.Fatal(err)
	}
	blocks := loaded.Document.Blocks
	for i := range blocks {
		for j := range blocks[i].Children {
			if blocks[i].Children[j].Type == "pending" {
				blocks[i].Children[j].Type = "paragraph"
				blocks[i].Children[j].Content = []document.Inline{{Text: "User words."}}
			}
		}
		if blocks[i].Type == "pending" {
			blocks[i].Type = "paragraph"
			blocks[i].Content = []document.Inline{{Text: "User words."}}
		}
	}
	if _, err = s.SaveDocument(ctx, doc.ID, 1, blocks); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptSuggestion(ctx, proposal.ID); err == nil {
		t.Fatal("fill applied over user text")
	}
	read, err := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read.Markdown, "User words") || len(read.Suggestions) != 0 {
		t.Fatal("user text lost or proposal listed", read.Markdown, read.Suggestions)
	}
}
