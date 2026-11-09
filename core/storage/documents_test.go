package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"kiko/core/document"
)

func markdownPointer(s string) *string { return &s }
func documentStore(t *testing.T) (*Store, string) {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "docs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	err = s.CreateChat(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s, state.Selection.ChatID
}
func TestDocumentRevisionsReadsAndReopen(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "# Goals\n\n- First\n  - Nested\n- Second\n\n| Name | Value |\n| --- | --- |\n| One | Two |", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil || loaded.Document.Rev != 1 || len(loaded.Document.Blocks) != 3 {
		t.Fatal(err, loaded)
	}
	if document.Markdown(loaded.Document.Blocks) != document.Markdown(doc.Blocks) {
		t.Fatal("stored structure changed")
	}
	originalIDs := []string{}
	for _, b := range document.Flatten(doc.Blocks) {
		originalIDs = append(originalIDs, b.ID)
	}
	changed, err := s.UpdateDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Nested"}, Text: markdownPointer("Edited nested")}}, "user")
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range document.Flatten(changed.Blocks) {
		if b.ID != originalIDs[i] {
			t.Fatal("text edit changed IDs")
		}
	}
	delta, err := s.ReadDocument(ctx, doc.ID, ReadOptions{SinceRev: 1})
	if err != nil || len(delta.Changed) < 1 {
		t.Fatal(err, delta)
	}
	old, err := s.ReadDocument(ctx, doc.ID, ReadOptions{AtRev: 1})
	if err != nil || old.Markdown != loaded.Markdown {
		t.Fatal(err, old)
	}
	view, err := s.ReadDocument(ctx, doc.ID, ReadOptions{ParentID: doc.Blocks[2].ID})
	if err != nil || len(view.Document.Blocks) != 1 || view.Document.Blocks[0].Type != "table" {
		t.Fatal(err, view)
	}
	hits, err := s.ReadDocument(ctx, doc.ID, ReadOptions{Search: "Edited nested"})
	if err != nil || len(hits.Document.Blocks) != 1 {
		t.Fatal(err, hits)
	}
	b := changed.Blocks[1]
	deleted, err := s.UpdateDocument(ctx, doc.ID, []document.Operation{{Type: "delete", Target: document.Target{IDs: []string{b.ID}}, IfRev: 2, IfHash: map[string]string{b.ID: b.Hash}}}, "agent")
	if err != nil {
		t.Fatal(err)
	}
	delta, err = s.ReadDocument(ctx, doc.ID, ReadOptions{SinceRev: 2})
	if err != nil || len(delta.Gone) < 2 || deleted.Rev != 3 {
		t.Fatal(err, delta)
	}
	old, err = s.ReadDocument(ctx, doc.ID, ReadOptions{AtRev: 2})
	if err != nil || len(old.Document.Blocks) != 3 {
		t.Fatal(err, old)
	}
	var before int
	if err = s.DB().QueryRow("SELECT count(*) FROM revisions WHERE document_id=?", doc.ID).Scan(&before); err != nil || before != 3 {
		t.Fatal(err, before)
	}
	var path string
	s.DB().QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path)
	s.Close()
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := reopened.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil || saved.Document.Rev != 3 {
		t.Fatal(err, saved)
	}
}
func TestDocumentAtomicRollbackAndConcurrentWriters(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Plan", "Text", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateDocument(ctx, doc.ID, []document.Operation{{Type: "insert", Markdown: markdownPointer("Inserted")}, {Type: "delete", Target: document.Target{IDs: []string{"gone"}}}}, "agent")
	if err == nil {
		t.Fatal("invalid operation accepted")
	}
	saved, _ := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if saved.Document.Rev != 1 || len(saved.Document.Blocks) != 1 {
		t.Fatal("partial update committed")
	}
	edited := document.Clone(doc)
	edited.Blocks[0].Content[0].Text = "User changed"
	if _, err = s.SaveDocument(ctx, doc.ID, 1, edited.Blocks); err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveDocument(ctx, doc.ID, 1, doc.Blocks)
	var refusal *document.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "guard_mismatch" {
		t.Fatal(err)
	}
	b := doc.Blocks[0]
	_, err = s.UpdateDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{IDs: []string{b.ID}}, IfRev: 1, IfHash: map[string]string{b.ID: b.Hash}, Markdown: markdownPointer("Agent overwrite")}}, "agent")
	if !errors.As(err, &refusal) || refusal.Code != "guard_mismatch" {
		t.Fatal(err)
	}
	s.DB().Exec("CREATE TRIGGER fail_revision BEFORE INSERT ON revisions BEGIN SELECT RAISE(ABORT,'fixture');END")
	if _, err = s.SaveDocument(ctx, doc.ID, 2, doc.Blocks); err == nil {
		t.Fatal("failed revision accepted")
	}
	saved, _ = s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if saved.Document.Rev != 2 || document.Plain(saved.Document.Blocks[0]) != "User changed" {
		t.Fatal("failed write was not rolled back")
	}
}
func TestDocumentOwnershipAndAnchorProtection(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	if _, err := s.CreateDocument(ctx, "missing", "Plan", "text", nil, "agent"); err == nil {
		t.Fatal("orphan document accepted")
	}
	doc, err := s.CreateDocument(ctx, chat, "Plan", "Important passage", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB().Exec("INSERT INTO comments(id,document_id,anchor_block,anchor_start,anchor_end,body,author,created_at) VALUES ('question',?,?,0,9,'Question','agent',1)", doc.ID, doc.Blocks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateDocument(ctx, doc.ID, []document.Operation{{Type: "replace", Target: document.Target{Find: "Important"}, Text: markdownPointer("Different")}}, "agent")
	var refusal *document.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "anchors_affected" {
		t.Fatal(err)
	}
	if err = s.Delete(ctx, "chat", chat); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadDocument(ctx, doc.ID, ReadOptions{}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
}

func TestDraftRecoveryHistoryAndRestore(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	doc, err := s.CreateDocument(ctx, chat, "Draft", "Original", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	blocks, _ := document.Parse("Edited draft", nil)
	if err = s.SaveDocumentDraft(ctx, doc.ID, 1, blocks); err != nil {
		t.Fatal(err)
	}
	read, err := s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if err != nil || read.Draft == nil || document.Plain(read.Draft.Blocks[0]) != "Edited draft" {
		t.Fatal(err, read)
	}
	if _, err = s.SaveDocument(ctx, doc.ID, 1, blocks); err != nil {
		t.Fatal(err)
	}
	read, _ = s.ReadDocument(ctx, doc.ID, ReadOptions{})
	if read.Draft != nil {
		t.Fatal("committed draft left behind")
	}
	restored, err := s.RestoreDocument(ctx, doc.ID, 2, 1)
	if err != nil || restored.Rev != 3 || document.Plain(restored.Blocks[0]) != "Original" {
		t.Fatal(err, restored)
	}
	history, err := s.DocumentHistory(ctx, doc.ID)
	if err != nil || len(history) != 3 {
		t.Fatal(err, history)
	}
	old, err := s.ReadDocument(ctx, doc.ID, ReadOptions{AtRev: 2})
	if err != nil || old.Markdown != "Edited draft" {
		t.Fatal(err, old)
	}
	if _, err = s.RestoreDocument(ctx, doc.ID, 2, 1); err == nil {
		t.Fatal("stale restore accepted")
	}
}
