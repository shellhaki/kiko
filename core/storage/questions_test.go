package storage

import (
	"context"
	"testing"

	"kiko/core/document"
)

func TestQuestionAnchorsFollowSafeEditsAndKeepDeletedPassages(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "你好 Important words after.", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.AddQuestion(ctx, doc.ID, "Important words", "Keep this?", []string{"Yes", "No"})
	if err != nil || q.Start != 3 {
		t.Fatal(err, q)
	}
	updated, err := s.UpdateDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "你好"}, Text: markdownPointer("Longer prefix")}}, "agent")
	if err != nil {
		t.Fatal(err)
	}
	questions, _ := s.Questions(ctx, doc.ID)
	if questions[0].Start != 14 {
		t.Fatal(questions)
	}
	if _, err = s.UpdateDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Important words"}, Text: markdownPointer("Changed")}}, "agent"); err == nil {
		t.Fatal("anchor passage overwritten")
	}
	updated.Blocks[0].Content = []document.Inline{{Text: "User removed the passage"}}
	if _, err = s.SaveDocument(ctx, doc.ID, updated.Rev, updated.Blocks); err != nil {
		t.Fatal(err)
	}
	questions, _ = s.Questions(ctx, doc.ID)
	if questions[0].BlockID != "" || questions[0].Quote != "Important words" || questions[0].Resolved {
		t.Fatal("question or quote lost", questions)
	}
	if err = s.ResolveQuestion(ctx, q.ID, true); err != nil {
		t.Fatal(err)
	}
	questions, _ = s.Questions(ctx, doc.ID)
	if !questions[0].Resolved {
		t.Fatal(questions)
	}
}
