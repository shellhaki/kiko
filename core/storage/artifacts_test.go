package storage

import (
	"context"
	"encoding/json"
	"testing"

	"kiko/core/deck"
	"kiko/core/sheet"
)

func TestArtifactsKeepDraftsGuardRevisionsAndRestoreSelection(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	book := sheet.Book{Tabs: []sheet.Tab{{Name: "Budget", Cells: map[string]sheet.Cell{"A1": {Value: 10}, "A2": {Formula: "=A1*2"}}}}}
	raw, _ := json.Marshal(book)
	a, err := s.CreateArtifact(ctx, chat, "sheet", "Budget", raw, "user")
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.Snapshot(ctx)
	if err != nil || state.ArtifactID != a.ID || state.DocumentID != "" || len(state.Artifacts) != 1 {
		t.Fatal(err, state.ArtifactID)
	}
	book.Tabs[0].Cells["A1"] = sheet.Cell{Value: 20}
	changed, _ := json.Marshal(book)
	if err = s.SaveArtifactDraft(ctx, a.ID, 1, changed); err != nil {
		t.Fatal(err)
	}
	saved, err := s.SaveArtifact(ctx, a.ID, 1, changed, "user")
	if err != nil || saved.Rev != 2 {
		t.Fatal(err, saved)
	}
	read, _ := s.ReadArtifact(ctx, a.ID)
	if read.Draft != nil {
		t.Fatal("committed draft retained")
	}
	if _, err = s.SaveArtifact(ctx, a.ID, 1, raw, "agent"); err == nil {
		t.Fatal("stale artifact write accepted")
	}
	doc, err := s.CreateDocument(ctx, chat, "Doc", "Text", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = s.Snapshot(ctx)
	if state.ArtifactID != "" || state.DocumentID != doc.ID {
		t.Fatal("document did not replace artifact selection")
	}
	if err = s.OpenArtifact(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Snapshot(ctx)
	if state.DocumentID != "" || state.ArtifactID != a.ID {
		t.Fatal("artifact selection was not restored")
	}
	slides := deck.Deck{Slides: []deck.Slide{{Layout: "title", Title: "Plan"}}}
	deck.Prepare(&slides)
	payload, _ := json.Marshal(slides)
	if _, err = s.CreateArtifact(ctx, chat, "deck", "Slides", payload, "agent"); err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(ctx, "chat", chat); err != nil {
		t.Fatal(err)
	}
	var count int
	s.DB().QueryRow("SELECT count(*) FROM artifacts").Scan(&count)
	if count != 0 {
		t.Fatal("orphan artifacts remain")
	}
	s.DB().QueryRow("SELECT count(*) FROM artifact_revisions").Scan(&count)
	if count != 0 {
		t.Fatal("orphan revisions remain")
	}
}

func TestDiscardArtifactDraftRetainsSavedRevision(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	payload := json.RawMessage(`{"tabs":[{"name":"Sheet1","cells":{"A1":{"value":10}}}]}`)
	saved, err := s.CreateArtifact(ctx, chat, "sheet", "Sheet", payload, "user")
	if err != nil {
		t.Fatal(err)
	}
	draft := json.RawMessage(`{"tabs":[{"name":"Sheet1","cells":{"A1":{"value":99}}}]}`)
	if err = s.SaveArtifactDraft(ctx, saved.ID, 1, draft); err != nil {
		t.Fatal(err)
	}
	if err = s.DiscardArtifactDraft(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	read, err := s.ReadArtifact(ctx, saved.ID)
	if err != nil || read.Draft != nil || read.Rev != 1 || string(read.Payload) != string(payload) {
		t.Fatal(read, err)
	}
}
