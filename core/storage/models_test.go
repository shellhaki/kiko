package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMuseOnlyMigrationMovesOldProvidersAndChatsToMuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "older.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	old.SetMaxOpenConns(1)
	if err = migrate(ctx, old, schemas[:11]); err != nil {
		t.Fatal(err)
	}
	if _, err = old.Exec(`INSERT INTO providers VALUES ('openai','openai-chat','https://api.openai.com/v1','gpt-fixture',0);
		INSERT INTO chats VALUES ('on-openai',NULL,'Old','openai',1),('on-muse',NULL,'Muse','muse',2);
		INSERT INTO app_settings VALUES ('image-models','{"openai":"gpt-fixture"}')`); err != nil {
		t.Fatal(err)
	}
	old.Close()
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var providers, others int
	s.DB().QueryRow("SELECT count(*) FROM providers").Scan(&providers)
	s.DB().QueryRow("SELECT count(*) FROM chats WHERE provider_id!='muse'").Scan(&others)
	var images int
	s.DB().QueryRow("SELECT count(*) FROM app_settings WHERE key='image-models'").Scan(&images)
	if providers != 1 || others != 0 || images != 0 {
		t.Fatal("old provider state survived", providers, others, images)
	}
	for _, chat := range []string{"on-openai", "on-muse"} {
		if model, err := s.ChatModel(ctx, chat); err != nil || model != "muse-spark-1.3-contributor" {
			t.Fatal("existing chat did not get the default model", chat, model, err)
		}
	}
}

func TestNewChatsGetTheDefaultModelAndSetModelTargetsTheSelectedChat(t *testing.T) {
	s := openTest(t)
	if err := s.CreateChat(ctx, ""); err != nil {
		t.Fatal(err)
	}
	state, _ := s.Snapshot(ctx)
	chat := state.Selection.ChatID
	if model, _ := s.ChatModel(ctx, chat); model != "muse-spark-1.3-contributor" || state.Model != model || len(state.Models) == 0 {
		t.Fatal("new chat model", model, state.Model)
	}
	if err := s.SetModel(ctx, "muse-ember-1.0"); err != nil {
		t.Fatal(err)
	}
	if model, _ := s.ChatModel(ctx, chat); model != "muse-ember-1.0" {
		t.Fatal("selected chat did not change", model)
	}
	if fallback, _ := s.DefaultModel(ctx); fallback != "muse-ember-1.0" {
		t.Fatal("default for new chats did not follow", fallback)
	}
	if _, err := s.DB().Exec("DELETE FROM chats WHERE id=?", chat); err != nil {
		t.Fatal(err)
	}
	var left int
	s.DB().QueryRow("SELECT count(*) FROM chat_models WHERE chat_id=?", chat).Scan(&left)
	if left != 0 {
		t.Fatal("deleting a chat left its model row")
	}
}
