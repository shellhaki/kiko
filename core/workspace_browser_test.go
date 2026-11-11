package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kiko/core/provider"
	"kiko/core/storage"
	"kiko/core/workspace"
)

func TestCustomArtifactPathsPersistAndRefuseCollisions(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	for _, test := range []struct{ tool, args, path string }{
		{"create_document", `{"name":"Plan","markdown":"# Plan","path":"/docs/plan.md"}`, "docs/plan.md"},
		{"create_sheet", `{"name":"Budget","tabs":[{"name":"Costs","cells":{}}],"path":"/docs/budget.sheet.json"}`, "docs/budget.sheet.json"},
		{"create_deck", `{"name":"Launch","slides":[{"layout":"title","title":"Launch"}],"path":"/docs/launch.deck.json"}`, "docs/launch.deck.json"},
	} {
		result, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: test.tool, Arguments: test.args}})
		if err != nil || strings.Contains(result, "workspace_error") {
			t.Fatal(result, err)
		}
		if _, err = os.Stat(filepath.Join(folder, test.path)); err != nil {
			t.Fatal(err)
		}
		entry, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, test.path)
		if err != nil || entry.SourceID == "" {
			t.Fatal(entry, err)
		}
		if err = s.SyncWorkspace(ctx, run, entry.SourceID); err != nil {
			t.Fatal(err)
		}
		if _, err = s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: test.tool, Arguments: test.args}}); err == nil {
			t.Fatal("reused source path")
		}
	}
	for _, path := range []string{"../bad.md", ".git/config.md", "docs/wrong.txt"} {
		if err := s.CreateArtifactAt(ctx, run.ChatID, "document", "Wrong", path); err == nil {
			t.Fatal("invalid source path", path)
		}
	}
	if err := s.CreateArtifactAt(ctx, run.ChatID, "document", "Manual", "docs/manual.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, "docs/manual.md")); err != nil {
		t.Fatal(err)
	}
}
func TestSourceReloadGuardsAndHistory(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	if err := s.CreateArtifactAt(ctx, run.ChatID, "document", "Plan", "docs/plan.md"); err != nil {
		t.Fatal(err)
	}
	entry, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, "docs/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	source, err := s.WorkspaceSource(ctx, entry.SourceID)
	if err != nil || source.Status != "synced" {
		t.Fatal(source, err)
	}
	if err := os.WriteFile(filepath.Join(folder, entry.Path), []byte("# External\n\nNew content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err = s.WorkspaceSource(ctx, entry.SourceID)
	if err != nil || source.Status != "changed" {
		t.Fatal(source, err)
	}
	if err = s.SyncWorkspace(ctx, run, entry.SourceID); err == nil {
		t.Fatal("external changes ignored")
	}
	if err = s.ReloadWorkspaceSource(ctx, entry.SourceID, source.DiskHash, source.Rev); err == nil {
		t.Fatal("reload during running reply")
	}
	if _, err = s.Store.DB().ExecContext(ctx, "UPDATE chat_runs SET status='complete' WHERE id=?", run.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.ReloadWorkspaceSource(ctx, entry.SourceID, "stale", source.Rev); err == nil {
		t.Fatal("stale hash accepted")
	}
	if err = s.ReloadWorkspaceSource(ctx, entry.SourceID, source.DiskHash, source.Rev+1); err == nil {
		t.Fatal("stale revision accepted")
	}
	read, err := s.Store.ReadDocument(ctx, entry.SourceID, storage.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Store.SaveDocumentDraft(ctx, entry.SourceID, read.Document.Rev, read.Document.Blocks); err != nil {
		t.Fatal(err)
	}
	if err = s.ReloadWorkspaceSource(ctx, entry.SourceID, source.DiskHash, source.Rev); err == nil {
		t.Fatal("draft overwritten")
	}
	if err = s.Store.DiscardDocumentDraft(ctx, entry.SourceID); err != nil {
		t.Fatal(err)
	}
	if err = s.ReloadWorkspaceSource(ctx, entry.SourceID, source.DiskHash, source.Rev); err != nil {
		t.Fatal(err)
	}
	read, err = s.Store.ReadDocument(ctx, entry.SourceID, storage.ReadOptions{})
	if err != nil || read.Document.Rev != 2 || !strings.Contains(read.Markdown, "New content") {
		t.Fatal(read, err)
	}
	before, err := s.Store.ReadDocument(ctx, entry.SourceID, storage.ReadOptions{AtRev: 1})
	if err != nil || !strings.Contains(before.Markdown, "Plan") {
		t.Fatal(before, err)
	}
	source, err = s.WorkspaceSource(ctx, entry.SourceID)
	if err != nil || source.Status != "synced" {
		t.Fatal(source, err)
	}
	if err = os.Remove(filepath.Join(folder, entry.Path)); err != nil {
		t.Fatal(err)
	}
	source, err = s.WorkspaceSource(ctx, entry.SourceID)
	if err != nil || source.Status != "missing" {
		t.Fatal(source, err)
	}
}
func TestWorkspaceBrowsingTextDraftsAndImports(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	if err := os.Mkdir(filepath.Join(folder, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "notes.txt"), []byte("Original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "docs/plan.md"), []byte("# Imported\n\nA local plan.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := s.ListWorkspaceDirectory(ctx, run.ProjectID, ".")
	if err != nil || len(entries) != 2 || !entries[0].Directory {
		t.Fatal(entries, err)
	}
	if _, err = s.ListWorkspaceDirectory(ctx, run.ProjectID, ".."); err == nil {
		t.Fatal("directory escape")
	}
	text, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveWorkspaceTextDraft(ctx, run.ProjectID, text.Path, text.Hash, "My draft"); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, text.Path)
	if err != nil || recovered.Draft == nil || recovered.Draft.Content != "My draft" {
		t.Fatal(recovered, err)
	}
	if err := os.WriteFile(filepath.Join(folder, "notes.txt"), []byte("External"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveWorkspaceText(ctx, run.ProjectID, run.ChatID, text.Path, text.Hash, "My draft"); err == nil {
		t.Fatal("external file overwritten")
	}
	if err = s.DiscardWorkspaceTextDraft(ctx, run.ProjectID, text.Path, "stale"); err == nil {
		t.Fatal("stale discard accepted")
	}
	current, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, text.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DiscardWorkspaceTextDraft(ctx, run.ProjectID, text.Path, current.Hash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveWorkspaceText(ctx, run.ProjectID, run.ChatID, text.Path, current.Hash, "Updated"); err != nil {
		t.Fatal(err)
	}
	imported, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, "docs/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ImportWorkspaceFile(ctx, run.ProjectID, run.ChatID, imported.Path, imported.Hash); err != nil {
		t.Fatal(err)
	}
	linked, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, imported.Path)
	if err != nil || linked.SourceID == "" || linked.Kind != "document" {
		t.Fatal(linked, err)
	}
	if _, err = s.SaveWorkspaceText(ctx, run.ProjectID, run.ChatID, linked.Path, linked.Hash, "Clobbered"); err == nil {
		t.Fatal("text editor overwrote artifact source")
	}
	if data, err := os.ReadFile(filepath.Join(folder, imported.Path)); err != nil || workspace.Hash(data) != imported.Hash {
		t.Fatal("import rewrote the file", err)
	}
	if err = s.ImportWorkspaceFile(ctx, run.ProjectID, "", imported.Path, imported.Hash); err != nil {
		t.Fatal(err)
	}
	state, _ := s.Store.Snapshot(ctx)
	if len(state.Documents) != 1 {
		t.Fatal("import duplicated linked document")
	}
	for _, kind := range []string{"sheet", "deck"} {
		payload := `{"tabs":[{"name":"Sheet1","cells":{}}]}`
		if kind == "deck" {
			payload = `{"slides":[{"id":"slide","layout":"title","title":"Imported"}]}`
		}
		path := "docs/imported." + kind + ".json"
		if err := os.WriteFile(filepath.Join(folder, path), []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
		if err = s.ImportWorkspaceFile(ctx, run.ProjectID, run.ChatID, path, workspace.Hash([]byte(payload))); err != nil {
			t.Fatal(kind, err)
		}
		entry, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, path)
		if err != nil || entry.Kind != kind {
			t.Fatal(entry, err)
		}
		record, err := s.Store.ReadArtifact(ctx, entry.SourceID)
		if err != nil || !json.Valid(record.Payload) {
			t.Fatal(record, err)
		}
	}
}

func TestArtifactLinkedFileRejectsDirectWrite(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	path := "plan.md"
	content := "# Plan\n"
	hash := workspace.Hash([]byte(content))
	if err := os.WriteFile(filepath.Join(folder, path), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportWorkspaceFile(ctx, run.ProjectID, run.ChatID, path, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := s.writeWorkspaceFile(ctx, run, path, "Clobbered", hash, ""); err == nil {
		t.Fatal("direct write clobbered linked artifact")
	}
	data, err := os.ReadFile(filepath.Join(folder, path))
	if err != nil || string(data) != content {
		t.Fatal(string(data), err)
	}
	entry, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, path)
	if err != nil || entry.SourceID == "" {
		t.Fatal(entry, err)
	}
}
func TestImportFromProjectHomePreservesDraft(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	text := "# Imported\n"
	path := "import.md"
	if err := os.WriteFile(filepath.Join(folder, path), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveDraft(ctx, run.ProjectID, "", "Keep this thought"); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportWorkspaceFile(ctx, run.ProjectID, "", path, workspace.Hash([]byte(text))); err != nil {
		t.Fatal(err)
	}
	state, err := s.Store.Snapshot(ctx)
	if err != nil || state.Draft != "Keep this thought" || state.Selection.ChatID == run.ChatID {
		t.Fatal(state, err)
	}
	home, err := s.Store.Conversation(ctx, run.ProjectID, "")
	if err != nil || home.Draft != "" {
		t.Fatal(home, err)
	}
}

func TestExplicitSourceResolutionChecksLatestVersions(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	if err := s.CreateArtifactAt(ctx, run.ChatID, "document", "Plan", "plan.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.DB().ExecContext(ctx, "UPDATE chat_runs SET status='complete' WHERE id=?", run.ID); err != nil {
		t.Fatal(err)
	}
	entry, err := s.ReadWorkspaceEntry(ctx, run.ProjectID, "plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(folder, "plan.md"), []byte("External"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := s.WorkspaceSource(ctx, entry.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveWorkspaceSourceVersion(ctx, entry.SourceID, "old", source.Rev); err == nil {
		t.Fatal("stale external version overwritten")
	}
	if err = s.SaveWorkspaceSourceVersion(ctx, entry.SourceID, source.DiskHash, source.Rev); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(folder, "plan.md"))
	if err != nil || string(data) != source.Content {
		t.Fatal(string(data), err)
	}
	if err = os.Remove(filepath.Join(folder, "plan.md")); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveWorkspaceSourceVersion(ctx, entry.SourceID, "", source.Rev); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(folder, "plan.md")); err != nil {
		t.Fatal(err)
	}
}
