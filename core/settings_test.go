package core

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"kiko/core/storage"
)

type deleteFailKeys struct {
	fakeSecrets
	failDelete bool
}

func (k *deleteFailKeys) Delete() error {
	if k.failDelete {
		return errors.New("fixture failure")
	}
	return k.fakeSecrets.Delete()
}
func TestResetIsRecoverableAndClearsOwnedData(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	keys := &deleteFailKeys{fakeSecrets: fakeSecrets{key: "fixture"}, failDelete: true}
	s.Keys = keys
	s.Store.CreateChat(ctx, "")
	state, _ := s.Store.Snapshot(ctx)
	chat := state.Selection.ChatID
	s.Store.CreateDocument(ctx, chat, "Plan", "Text", nil, "user")
	s.Store.SaveDraft(ctx, "", "", "Unsent")
	if err := s.Reset(ctx); err == nil {
		t.Fatal("failed key deletion accepted")
	}
	state, _ = s.Store.Snapshot(ctx)
	if !state.Preferences.ResetInProgress || len(state.Chats) != 1 || len(state.Documents) != 1 {
		t.Fatal("partial reset lost data", state)
	}
	keys.failDelete = false
	if err := s.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Store.Snapshot(ctx)
	if !reflect.DeepEqual(state.Preferences, storage.DefaultPreferences()) || len(state.Chats) != 0 || len(state.Documents) != 0 || state.Draft != "" || keys.key != "" {
		t.Fatal(state)
	}
}
func TestRemoveKeyKeepsDataAndModelValidation(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	s.Store.CreateChat(ctx, "")
	if err := s.RemoveKey(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := s.State(ctx)
	if err != nil || state.Connected || len(state.Chats) != 1 || state.Preferences.Stage != "done" {
		t.Fatal(err, state)
	}
	if s.SetModel(ctx, "unsupported") == nil {
		t.Fatal("unsupported model accepted")
	}
}
