package storage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestFolderOwnershipModesAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kiko.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	if err = store.UseFolder(ctx, folder, "Project", ""); err != nil {
		t.Fatal(err)
	}
	state, err := store.Snapshot(ctx)
	if err != nil || len(state.Projects) != 1 || state.Projects[0].Root != folder {
		t.Fatal(state, err)
	}
	project := state.Selection.ProjectID
	if err = store.CreateChat(ctx, project); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Snapshot(ctx)
	chat := state.Selection.ChatID
	if err = store.SetFileMode(ctx, chat, "full"); err != nil {
		t.Fatal(err)
	}
	if err = store.SetFileMode(ctx, chat, "anything"); err == nil {
		t.Fatal("invalid mode accepted")
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	saved, err := store.ChatFolder(ctx, chat)
	if err != nil || saved.Root != folder || saved.Mode != "full" {
		t.Fatal(saved, err)
	}
	if err = store.UseFolder(ctx, folder, "Same folder", chat); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Snapshot(ctx)
	saved, _ = store.ChatFolder(ctx, chat)
	if len(state.Projects) != 1 || saved.Mode != "ask" {
		t.Fatal("duplicate project or stale grant", state, saved)
	}
	if err = store.RecordWorkspaceFile(ctx, saved, chat, "docs/plan.md", "", "hash"); err != nil {
		t.Fatal(err)
	}
	if err = store.UseFolder(ctx, t.TempDir(), "Other", chat); err == nil {
		t.Fatal("reassigned chat with existing files")
	}
	state, _ = store.Snapshot(ctx)
	if len(state.Projects) != 1 {
		t.Fatal("failed reassignment left a project")
	}
	if err = store.Delete(ctx, "project", project); err != nil {
		t.Fatal(err)
	}
	files, err := store.WorkspaceFiles(ctx, chat)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
}
func TestActivityExposesOutputIdentityWithoutToolContent(t *testing.T) {
	store := openTest(t)
	run, _, err := store.StartRun(ctx, "", "", "Plan", "activity")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ tool, result, status, want string }{
		{"create_document", "Document doc-id · Plan · rev 1\nsecret data", "ok", "doc-id"},
		{"create_sheet", `{"id":"sheet-id","secret":"hidden"}`, "ok", "sheet-id"},
		{"create_deck", `{"id":"deck-id"}`, "ok", "deck-id"},
		{"create_document", "Document secret-id · Plan", "refused", ""},
		{"read", `{"id":"secret-id"}`, "ok", ""},
	} {
		id, err := store.StartActivity(ctx, run.AssistantID, test.tool, "Working")
		if err != nil {
			t.Fatal(err)
		}
		if err = store.FinishActivity(ctx, id, test.status, map[string]any{"result": test.result, "private": "key-material"}); err != nil {
			t.Fatal(err)
		}
		steps, err := store.Activities(ctx, run.AssistantID)
		if err != nil {
			t.Fatal(err)
		}
		step := steps[len(steps)-1]
		if step.OutputID != test.want {
			t.Fatal(step, test.want)
		}
		raw, _ := json.Marshal(step)
		if strings.Contains(string(raw), "key-material") || strings.Contains(string(raw), "hidden") || strings.Contains(string(raw), "secret data") {
			t.Fatal("raw details exposed", string(raw))
		}
	}
}
func TestUpdateActivityRefreshesRunningLabelOnly(t *testing.T) {
	store := openTest(t)
	ctx := context.Background()
	run, _, err := store.StartRun(ctx, "", "", "Plan", "activity")
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.StartActivity(ctx, run.AssistantID, "write_file", "Writing docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.UpdateActivity(ctx, id, "Writing docs/a.md · 5 KB"); err != nil {
		t.Fatal(err)
	}
	steps, err := store.Activities(ctx, run.AssistantID)
	if err != nil || len(steps) != 1 || steps[0].Label != "Writing docs/a.md · 5 KB" || steps[0].Status != "running" {
		t.Fatal(steps, err)
	}
	if err = store.FinishActivity(ctx, id, "ok", map[string]any{"result": "{}"}); err != nil {
		t.Fatal(err)
	}
	if err = store.UpdateActivity(ctx, id, "Writing docs/a.md · 6 KB"); err != nil {
		t.Fatal(err)
	}
	steps, err = store.Activities(ctx, run.AssistantID)
	if err != nil || steps[0].Label != "Writing docs/a.md · 5 KB" {
		t.Fatal("finished label changed", steps)
	}
}
