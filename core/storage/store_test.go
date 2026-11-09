package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

var ctx = context.Background()

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "data #?", "kiko.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func execTest(t *testing.T, s *Store, statement string, args ...any) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, statement, args...); err != nil {
		t.Fatal(err)
	}
}
func seed(t *testing.T, s *Store) {
	t.Helper()
	for _, q := range []string{
		`INSERT INTO projects VALUES ('p','Project','Description',1)`,
		`INSERT INTO chats VALUES ('c','p','Planning','muse',1)`,
		`INSERT INTO messages(id,chat_id,role,content,tool_calls,created_at) VALUES ('m','c','assistant','Hello',NULL,1)`,
		`INSERT INTO documents VALUES ('d','c','Proposal',1,1)`,
		`INSERT INTO blocks VALUES ('b','d',NULL,1,'paragraph',NULL,'[{"text":"market research"}]','1234abcd',1,1,NULL)`,
		`INSERT INTO revisions VALUES ('d',1,'agent','[]',1)`,
		`INSERT INTO comments(id,document_id,parent_id,anchor_block,anchor_start,anchor_end,body,suggested_replies,author,resolved,created_at) VALUES ('q','d',NULL,'b',0,6,'Question','["Keep it"]','agent',0,1)`,
		`INSERT INTO nodes VALUES ('n','d','widget','<svg/>',NULL,1)`,
		`INSERT INTO tool_steps VALUES ('s','m','read','Read document','ok','{}',1,1)`,
		`INSERT INTO chat_view_state VALUES ('c','d','{"tab":"document"}')`,
		`UPDATE workspace_state SET project_id='p',chat_id='c' WHERE id=1`,
	} {
		execTest(t, s, q)
	}
}

func TestPersistenceAndDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kiko.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	prefs, err := s.Preferences(ctx)
	if err != nil || !reflect.DeepEqual(prefs, DefaultPreferences()) {
		t.Fatalf("defaults: %+v %v", prefs, err)
	}
	seed(t, s)
	prefs.Nickname = "  Haki  "
	prefs.Stage = "done"
	prefs.Theme = "system"
	if err = s.SavePreferences(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Preferences.Nickname != "Haki" || state.Preferences.Stage != "done" || state.Preferences.Theme != "system" || state.Selection.ChatID != "c" || len(state.Documents) != 1 {
		t.Fatalf("state lost: %+v", state)
	}
	for _, table := range []string{"projects", "chats", "messages", "documents", "blocks", "revisions", "comments", "nodes", "tool_steps", "chat_view_state"} {
		var n int
		if err = s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%s: %d %v", table, n, err)
		}
	}
}

func TestTransactionsAndMigrationRollback(t *testing.T) {
	s := openTest(t)
	sentinel := errors.New("rollback")
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO projects VALUES ('bad','Bad',NULL,1)")
		if err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic not propagated")
			}
		}()
		_ = s.Transaction(ctx, func(tx *sql.Tx) error { tx.Exec("INSERT INTO projects VALUES ('panic','Panic',NULL,1)"); panic("stop") })
	}()
	var n int
	s.db.QueryRow("SELECT count(*) FROM projects").Scan(&n)
	if n != 0 {
		t.Fatal("transaction leaked writes")
	}
	seed(t, s)
	err = migrate(ctx, s.db, append(append([]string{}, schemas...), "CREATE TABLE migration_probe(id INTEGER); INSERT INTO missing_table VALUES (1);"))
	if err == nil {
		t.Fatal("bad migration succeeded")
	}
	s.db.QueryRow("PRAGMA user_version").Scan(&n)
	if n != len(schemas) {
		t.Fatal("version advanced on failure")
	}
	if _, err = s.db.Exec("SELECT * FROM migration_probe"); err == nil {
		t.Fatal("DDL was not rolled back")
	}
	if err = migrate(ctx, s.db, append(append([]string{}, schemas...), "ALTER TABLE projects ADD COLUMN extra TEXT;")); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow("PRAGMA user_version").Scan(&n)
	if n != len(schemas)+1 {
		t.Fatal(n)
	}
	if err = migrate(ctx, s.db, []string{initialSchema}); !errors.Is(err, ErrSchemaTooNew) {
		t.Fatal(err)
	}
	var name string
	if err = s.db.QueryRow("SELECT name FROM projects WHERE id='p'").Scan(&name); err != nil || name != "Project" {
		t.Fatal("migration lost existing data")
	}
}

func TestForeignKeysAndOwnership(t *testing.T) {
	s := openTest(t)
	seed(t, s)
	execTest(t, s, `INSERT INTO chats VALUES ('other',NULL,'Other','muse',1)`)
	for _, q := range []string{
		`INSERT INTO chats VALUES ('bad','missing','Bad','muse',1)`,
		`INSERT INTO documents VALUES ('bad','missing','Bad',0,1)`,
		`UPDATE chat_view_state SET chat_id='other' WHERE chat_id='c'`,
		`UPDATE documents SET chat_id='other' WHERE id='d'`,
		`INSERT INTO blocks VALUES ('bad','d','missing',1,'paragraph',NULL,NULL,'1234abcd',1,1,NULL)`,
		`UPDATE comments SET anchor_start=-1 WHERE id='q'`,
		`UPDATE messages SET role='system' WHERE id='m'`,
		`DELETE FROM providers WHERE id='muse'`,
	} {
		if _, err := s.db.Exec(q); err == nil {
			t.Fatalf("invalid relationship accepted: %s", q)
		}
	}
	s.db.SetMaxOpenConns(2)
	a, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	for _, c := range []*sql.Conn{a, b} {
		var enabled int
		if err = c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
			t.Fatalf("connection missing FK enforcement: %v", err)
		}
	}
}

func TestCascadeAndDeletedView(t *testing.T) {
	s := openTest(t)
	seed(t, s)
	execTest(t, s, "DELETE FROM documents WHERE id='d'")
	var document sql.NullString
	if err := s.db.QueryRow("SELECT document_id FROM chat_view_state").Scan(&document); err != nil || document.Valid {
		t.Fatal("deleted document still selected")
	}
	if err := s.Delete(ctx, "project", "p"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"chats", "messages", "documents", "blocks", "revisions", "comments", "nodes", "tool_steps", "chat_view_state"} {
		var n int
		s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
		if n != 0 {
			t.Fatalf("%s retained orphan data", table)
		}
	}
	state, err := s.Snapshot(ctx)
	if err != nil || state.Selection != (Selection{}) {
		t.Fatalf("stale selection: %+v %v", state.Selection, err)
	}
}

func TestWorkspaceAndSearch(t *testing.T) {
	s := openTest(t)
	if err := s.CreateChat(ctx, ""); err != nil {
		t.Fatal(err)
	}
	state, _ := s.Snapshot(ctx)
	standalone := state.Selection.ChatID
	if err := s.CreateProject(ctx, "  Envi  ", "Secrets manager"); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Snapshot(ctx)
	project := state.Selection.ProjectID
	if err := s.CreateChat(ctx, project); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Snapshot(ctx)
	chat := state.Selection.ChatID
	if err := s.Rename(ctx, "chat", chat, "Plan 100%"); err != nil {
		t.Fatal(err)
	}
	results, err := s.Search(ctx, "%")
	if err != nil || len(results) != 1 || results[0].ID != chat {
		t.Fatalf("literal search: %+v %v", results, err)
	}
	if err = s.Select(ctx, "", chat); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Snapshot(ctx)
	if state.Selection.ProjectID != project {
		t.Fatal("chat project not restored")
	}
	if err = s.Select(ctx, project, standalone); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Snapshot(ctx)
	if state.Selection.ProjectID != "" {
		t.Fatal("standalone assigned a project")
	}
	if err = s.CreateChat(ctx, "missing"); err == nil {
		t.Fatal("unknown project accepted")
	}
	if err = s.Rename(ctx, "project", project, ""); err == nil {
		t.Fatal("empty name accepted")
	}
	if err = s.Delete(ctx, "anything", "x"); err == nil {
		t.Fatal("invalid entity accepted")
	}
	seed(t, s)
	results, err = s.Search(ctx, "market research")
	if err != nil || len(results) != 1 || results[0].ID != "d" {
		t.Fatalf("document search: %+v %v", results, err)
	}
}

func TestOpenErrorsAndConcurrentInitialization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.db")
	original := []byte("not a database")
	os.WriteFile(path, original, 0600)
	if _, err := Open(ctx, path); err == nil {
		t.Fatal("corrupt file accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Fatal("corrupt file overwritten")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Open(canceled, filepath.Join(dir, "cancel.db")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Open(ctx, ""); err == nil {
		t.Fatal("empty path accepted")
	}
	path = filepath.Join(dir, "shared.db")
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(ctx, path)
			if err != nil {
				t.Error(err)
				return
			}
			s.Close()
		}()
	}
	wg.Wait()
}

func TestPreferencesValidationAndNoSecretFields(t *testing.T) {
	s := openTest(t)
	for _, change := range []func(*Preferences){func(p *Preferences) { p.Stage = "invalid" }, func(p *Preferences) { p.Stage = "done" }, func(p *Preferences) { p.Theme = "brown" }, func(p *Preferences) { p.SidebarWidth = 1 }} {
		prefs := DefaultPreferences()
		change(&prefs)
		if err := s.SavePreferences(ctx, prefs); err == nil {
			t.Fatal("invalid preferences accepted")
		}
	}
	var secretColumns int
	if err := s.db.QueryRow("SELECT count(*) FROM pragma_table_info('providers') WHERE name LIKE '%key%' OR name LIKE '%secret%'").Scan(&secretColumns); err != nil || secretColumns != 0 {
		t.Fatal("provider schema includes secrets")
	}
	execTest(t, s, "INSERT INTO app_settings VALUES ('preferences','[]')")
	if _, err := s.Preferences(ctx); err == nil {
		t.Fatal("corrupt preferences silently reset")
	}
}

func TestFutureSchemaRefusedWithoutLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	seed(t, s)
	execTest(t, s, "PRAGMA user_version=99")
	s.Close()
	if opened, err := Open(ctx, path); !errors.Is(err, ErrSchemaTooNew) {
		if opened != nil {
			opened.Close()
		}
		t.Fatalf("future schema accepted: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var title string
	if err = db.QueryRow("SELECT title FROM documents WHERE id='d'").Scan(&title); err != nil || title != "Proposal" {
		t.Fatal("future database lost data")
	}
}
func TestSelectedDocumentAndResetPreferences(t *testing.T) {
	s := openTest(t)
	seed(t, s)
	if err := s.Select(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenDocument(ctx, "d"); err != nil {
		t.Fatal(err)
	}
	state, err := s.Snapshot(ctx)
	if err != nil || state.DocumentID != "d" || state.Selection.ChatID != "c" || state.Selection.ProjectID != "p" {
		t.Fatalf("document context not restored: %+v %v", state, err)
	}
	prefs := DefaultPreferences()
	prefs.Nickname = "Haki"
	prefs.Stage = "done"
	prefs.SidebarOpen = false
	prefs.SidebarWidth = 300
	if err = s.SavePreferences(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	execTest(t, s, "DELETE FROM app_settings")
	got, err := s.Preferences(ctx)
	if err != nil || !reflect.DeepEqual(got, DefaultPreferences()) {
		t.Fatal("cleared state did not return to onboarding")
	}
}
