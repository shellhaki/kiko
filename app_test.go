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
