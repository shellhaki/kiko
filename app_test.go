package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestInfoBindingContract(t *testing.T) {
	data, err := json.Marshal((&App{}).Info())
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]string
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["name"] == "" || fields["version"] == "" || len(fields) != 2 {
		t.Fatalf("frontend requires name and version strings; received %s", data)
	}
}

func TestStorageStartupAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kiko.db")
	app := &App{dataPath: path}
	state, err := app.Bootstrap()
	if err != nil || state.Preferences.Stage != "nickname" {
		t.Fatal(err)
	}
	if _, err = app.SaveNickname("Haki"); err != nil {
		t.Fatal(err)
	}
	app.shutdown(context.Background())
	reopened := &App{dataPath: path}
	defer reopened.shutdown(context.Background())
	state, err = reopened.Bootstrap()
	if err != nil || state.Preferences.Nickname != "Haki" || state.Preferences.Stage != "muse" {
		t.Fatalf("setup was not resumed: %+v %v", state, err)
	}
}

func TestExportDeckPptxValidatesBeforeDialog(t *testing.T) {
	folder := t.TempDir()
	app := &App{dataPath: filepath.Join(t.TempDir(), "kiko.db"), pickFolder: func(context.Context) (string, error) { return folder, nil }}
	defer app.shutdown(context.Background())
	if _, err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	state, err := app.ChooseFolder("")
	if err != nil {
		t.Fatal(err)
	}
	state, err = app.CreateChat(state.Selection.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	chat := state.Selection.ChatID
	if _, err := app.CreateDeck(chat, "Slides"); err != nil {
		t.Fatal(err)
	}
	state, err = app.CreateSheet(chat, "Budget")
	if err != nil {
		t.Fatal(err)
	}
	var deckID, sheetID string
	for _, artifact := range state.Artifacts {
		if artifact.Kind == "deck" {
			deckID = artifact.ID
		} else if artifact.Kind == "sheet" {
			sheetID = artifact.ID
		}
	}
	if deckID == "" || sheetID == "" {
		t.Fatal("artifacts missing", state.Artifacts)
	}
	if _, err = app.ExportDeckPptx(sheetID, "UEs="); err == nil || err.Error() != "Only slide decks export this way." {
		t.Fatal("sheet accepted as deck", err)
	}
	if _, err = app.ExportDeckPptx(deckID, "!!!"); err == nil {
		t.Fatal("invalid encoding accepted")
	}
	if _, err = app.ExportDeckPptx(deckID, "aGVsbG8="); err == nil {
		t.Fatal("non-presentation bytes accepted")
	}
	if _, err = app.ExportArtifact(deckID, "pptx", ""); err == nil || err.Error() != "Export slides from the editor preview." {
		t.Fatal("server-side slide export still offered", err)
	}
}

func TestFolderPickerCreatesRootAndCancellationPreservesContext(t *testing.T) {
	folder := t.TempDir()
	canonical, err := filepath.EvalSymlinks(folder)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{dataPath: filepath.Join(t.TempDir(), "kiko.db"), pickFolder: func(context.Context) (string, error) { return folder, nil }}
	defer app.shutdown(context.Background())
	if _, err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	state, err := app.ChooseFolder("")
	if err != nil || len(state.Projects) != 1 || state.Projects[0].Root != canonical {
		t.Fatal(state, err)
	}
	project := state.Selection.ProjectID
	state, err = app.CreateChat(project)
	if err != nil {
		t.Fatal(err)
	}
	chat := state.Selection.ChatID
	app.pickFolder = func(context.Context) (string, error) { return "", nil }
	state, err = app.ChooseFolder(chat)
	if err != nil || state.Selection.ChatID != chat || state.Selection.ProjectID != project {
		t.Fatal(state, err)
	}
}
