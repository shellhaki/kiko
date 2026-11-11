package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestTurnPersistenceIdempotencyAndRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.SaveDraft(ctx, "", "", "Hello"); err != nil {
		t.Fatal(err)
	}
	run, created, err := s.StartRun(ctx, "", "", "Hello", "request-1")
	if err != nil || !created {
		t.Fatal(err)
	}
	again, created, err := s.StartRun(ctx, "", run.ChatID, "Hello", "request-1")
	if err != nil || created || again.ID != run.ID {
		t.Fatal("send not idempotent")
	}
	if _, _, err = s.StartRun(ctx, "", run.ChatID, "Second", "request-2"); err == nil {
		t.Fatal("overlapping run accepted")
	}
	first, err := s.UpdateRun(ctx, run, "Partial", "running", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := s.UpdateRun(ctx, run, "Partial", "stopped", "", "", nil)
	if err != nil || stopped.Sequence <= first.Sequence {
		t.Fatal(err)
	}
	retry, created, err := s.RetryRun(ctx, run.ID, "retry-1")
	if err != nil || !created || retry.Attempt != 2 {
		t.Fatal(err)
	}
	again, created, err = s.RetryRun(ctx, run.ID, "retry-1")
	if err != nil || created || again.ID != run.ID {
		t.Fatal("retry not idempotent")
	}
	if _, err = s.UpdateRun(ctx, run, "late old output", "complete", "", "", nil); !errors.Is(err, ErrStaleRun) {
		t.Fatal("old attempt overwrote retry")
	}
	final, err := s.UpdateRun(ctx, retry, "New answer", "complete", "", "", json.RawMessage(`{"total_tokens":10}`))
	if err != nil || len(final.Previous) != 1 || final.Previous[0].Content != "Partial" {
		t.Fatal("partial attempt was lost", err)
	}
	s.Close()
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	convo, err := s.Conversation(ctx, "", run.ChatID)
	if err != nil || len(convo.Messages) != 2 || convo.Messages[0].Content != "Hello" || convo.Messages[1].Content != "New answer" {
		t.Fatal("reopen duplicated/lost messages", err)
	}
	home, err := s.Conversation(ctx, "", "")
	if err != nil || home.Draft != "" {
		t.Fatal("sent draft resurrected")
	}
}
func TestDraftScopeAndCrashRecovery(t *testing.T) {
	s := openTest(t)
	s.CreateProject(ctx, "Envi", "")
	state, _ := s.Snapshot(ctx)
	project := state.Selection.ProjectID
	s.CreateChat(ctx, project)
	state, _ = s.Snapshot(ctx)
	chat := state.Selection.ChatID
	s.SaveDraft(ctx, project, "", "project draft")
	s.SaveDraft(ctx, project, chat, "chat draft")
	s.SaveDraft(ctx, "", "", "home draft")
	for _, tc := range []struct{ p, c, want string }{{project, "", "project draft"}, {project, chat, "chat draft"}, {"", "", "home draft"}} {
		data, err := s.Conversation(ctx, tc.p, tc.c)
		if err != nil || data.Draft != tc.want {
			t.Fatal("draft mixed across scopes", err)
		}
	}
	run, _, err := s.StartRun(ctx, project, chat, "Hello", "start")
	if err != nil {
		t.Fatal(err)
	}
	s.UpdateRun(ctx, run, "Saved fragment", "running", "", "", nil)
	if err = s.RecoverRuns(ctx); err != nil {
		t.Fatal(err)
	}
	data, _ := s.Conversation(ctx, project, chat)
	if data.Messages[1].Status != "interrupted" || data.Messages[1].Content != "Saved fragment" {
		t.Fatal("crash recovery lost partial reply")
	}
	if err = s.Delete(ctx, "project", project); err != nil {
		t.Fatal(err)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM chat_runs").Scan(&count)
	if count != 0 {
		t.Fatal("run survived deleted project")
	}
	s.db.QueryRow("SELECT count(*) FROM drafts WHERE scope!='home'").Scan(&count)
	if count != 0 {
		t.Fatal("draft survived deleted project")
	}
}
func TestVersionOneUpgradePreservesConversation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(initialSchema); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO chats VALUES ('c',NULL,'Old chat','muse',1);INSERT INTO messages VALUES ('u','c','user','Original question',NULL,1);INSERT INTO messages VALUES ('a','c','assistant','Original answer',NULL,1);PRAGMA user_version=1;`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	data, err := s.Conversation(ctx, "", "c")
	if err != nil || len(data.Messages) != 2 || data.Messages[0].Ordinal != 1 || data.Messages[0].Content != "Original question" || data.Messages[1].Ordinal != 2 {
		t.Fatalf("upgrade damaged history: %+v %v", data, err)
	}
}

func TestContributorMigrationAndRetryPreserveExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contributor.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	var model string
	if err = s.DB().QueryRow("SELECT model FROM providers WHERE id='muse'").Scan(&model); err != nil || model != "muse-spark-1.3-contributor" {
		t.Fatal(err, model)
	}
	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	legacy.SetMaxOpenConns(1)
	if err = migrate(ctx, legacy, []string{initialSchema, conversationSchema, documentDraftSchema, questionSchema}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = &Store{db: legacy}
	path = legacyPath
	if _, err = s.DB().Exec("UPDATE providers SET model='muse-spark-1.3' WHERE id='muse'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().Exec(`INSERT INTO chats VALUES ('old-chat',NULL,'Old chat','muse',1);
		INSERT INTO messages(id,chat_id,role,content,created_at,ordinal) VALUES ('q','old-chat','user','Keep my chat',1,1),('a','old-chat','assistant','Saved partial',1,2);
		INSERT INTO chat_runs(id,chat_id,user_message_id,assistant_message_id,model,status,error,error_kind,created_at,updated_at) VALUES ('old-run','old-chat','q','a','muse-spark-1.3','error','Try again','provider',1,1);
		INSERT INTO chat_requests VALUES ('old-request','old-run',1)`); err != nil {
		t.Fatal(err)
	}
	run := Run{ID: "old-run", ChatID: "old-chat"}
	if _, err = s.DB().Exec("PRAGMA user_version=4"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.DB().QueryRow("SELECT model FROM providers WHERE id='muse'").Scan(&model); err != nil || model != "muse-spark-1.3-contributor" {
		t.Fatal(err, model)
	}
	if err = s.DB().QueryRow("SELECT model FROM chat_runs WHERE id=?", run.ID).Scan(&model); err != nil || model != "muse-spark-1.3" {
		t.Fatal("historical model changed", err, model)
	}
	if err = s.DB().QueryRow("SELECT model FROM chat_models WHERE chat_id=?", run.ChatID).Scan(&model); err != nil || model != "muse-spark-1.3-contributor" {
		t.Fatal("existing chat lost its model", err, model)
	}
	conversation, err := s.Conversation(ctx, "", run.ChatID)
	if err != nil || len(conversation.Messages) != 2 || conversation.Messages[1].Content != "Saved partial" {
		t.Fatal(err, conversation)
	}
	retry, created, err := s.RetryRun(ctx, run.ID, "contributor-retry")
	if err != nil || !created || retry.Model != "muse-spark-1.3-contributor" {
		t.Fatal(err, retry)
	}
	if err = s.DB().QueryRow("SELECT model FROM chat_runs WHERE id=?", run.ID).Scan(&model); err != nil || model != retry.Model {
		t.Fatal(err, model)
	}
	if err = s.ResetData(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.DB().QueryRow("SELECT model FROM providers WHERE id='muse'").Scan(&model); err != nil || model != "muse-spark-1.3-contributor" {
		t.Fatal(err, model)
	}
}
