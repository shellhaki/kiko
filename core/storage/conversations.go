package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrStaleRun = errors.New("reply attempt is no longer active")

type Attempt struct {
	Content string `json:"content"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}
type Message struct {
	Steps     []Activity `json:"steps,omitempty"`
	ID        string     `json:"id"`
	ChatID    string     `json:"chatId"`
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Ordinal   int        `json:"ordinal"`
	RunID     string     `json:"runId"`
	Status    string     `json:"status"`
	Error     string     `json:"error"`
	ErrorKind string     `json:"errorKind"`
	Sequence  int        `json:"sequence"`
	Attempt   int        `json:"attempt"`
	Previous  []Attempt  `json:"previous"`
}
type Conversation struct {
	Messages []Message `json:"messages"`
	Draft    string    `json:"draft"`
}
type Run struct {
	ID, ChatID, ProjectID, UserID, AssistantID, Model, Status, ProviderID string
	Attempt                                                               int
}

const messageSQL = `SELECT m.id,m.chat_id,m.role,COALESCE(m.content,''),COALESCE(m.ordinal,0),COALESCE(r.id,''),COALESCE(r.status,'complete'),COALESCE(r.error,''),COALESCE(r.error_kind,''),COALESCE(r.sequence,0),COALESCE(r.attempt,0),COALESCE(r.previous_attempts,'[]') FROM messages m LEFT JOIN chat_runs r ON r.assistant_message_id=m.id `
const runSQL = `SELECT r.id,r.chat_id,COALESCE(c.project_id,''),r.user_message_id,r.assistant_message_id,r.model,r.status,r.attempt,c.provider_id FROM chat_runs r JOIN chats c ON c.id=r.chat_id `

type scanner interface{ Scan(...any) error }

func scanRun(row scanner) (Run, error) {
	var r Run
	err := row.Scan(&r.ID, &r.ChatID, &r.ProjectID, &r.UserID, &r.AssistantID, &r.Model, &r.Status, &r.Attempt, &r.ProviderID)
	return r, err
}
func scanMessage(row scanner) (Message, error) {
	m := Message{Previous: []Attempt{}}
	var previous string
	err := row.Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.Ordinal, &m.RunID, &m.Status, &m.Error, &m.ErrorKind, &m.Sequence, &m.Attempt, &previous)
	if err == nil {
		err = json.Unmarshal([]byte(previous), &m.Previous)
	}
	return m, err
}
func draftScope(project, chat string) string {
	if chat != "" {
		return "chat:" + chat
	}
	if project != "" {
		return "project:" + project
	}
	return "home"
}

func (s *Store) Conversation(ctx context.Context, project, chat string) (Conversation, error) {
	result := Conversation{Messages: []Message{}}
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, messageSQL+"WHERE m.chat_id=? ORDER BY COALESCE(m.ordinal,0),m.created_at,m.rowid", chat)
		if err != nil {
			return err
		}
		for rows.Next() {
			m, err := scanMessage(rows)
			if err != nil {
				rows.Close()
				return err
			}
			result.Messages = append(result.Messages, m)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for i := range result.Messages {
			steps, err := activity(ctx, tx, result.Messages[i].ID)
			if err != nil {
				return err
			}
			result.Messages[i].Steps = steps
		}
		err = tx.QueryRowContext(ctx, "SELECT content FROM drafts WHERE scope=?", draftScope(project, chat)).Scan(&result.Draft)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})
	return result, err
}
func (s *Store) SaveDraft(ctx context.Context, project, chat, content string) error {
	if len(content) > 128*1024 {
		return errors.New("draft is too long")
	}
	if chat != "" {
		project = ""
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO drafts VALUES (?,NULLIF(?,''),NULLIF(?,''),?,?) ON CONFLICT(scope) DO UPDATE SET content=excluded.content,updated_at=excluded.updated_at`, draftScope(project, chat), chat, project, content, time.Now().UnixMilli())
	return err
}
func (s *Store) RecoverRuns(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chat_runs SET status='interrupted',error='Kiko closed before this reply finished. Your partial reply is saved.',error_kind='interrupted',sequence=sequence+1,updated_at=? WHERE status='running'`, time.Now().UnixMilli())
	return err
}
func (s *Store) StartRun(ctx context.Context, project, chat, text, requestID string) (Run, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 128*1024 {
		return Run{}, false, errors.New("enter a message shorter than 128 KB")
	}
	if requestID == "" || len(requestID) > 120 {
		return Run{}, false, errors.New("invalid request identifier")
	}
	var run Run
	created := false
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		existing, err := scanRun(tx.QueryRowContext(ctx, runSQL+"JOIN chat_requests q ON q.run_id=r.id WHERE q.id=?", requestID))
		if err == nil {
			var original string
			if err = tx.QueryRowContext(ctx, "SELECT content FROM messages WHERE id=?", existing.UserID).Scan(&original); err != nil {
				return err
			}
			if original != text || (chat != "" && chat != existing.ChatID) {
				return errors.New("request identifier already used")
			}
			run = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		scope := draftScope(project, chat)
		if chat == "" {
			chat = rand.Text()
			if _, err = tx.ExecContext(ctx, "INSERT INTO chats VALUES (?,NULLIF(?,''),'New chat',(SELECT COALESCE(json_extract(value,'$.providerId'),'muse') FROM (SELECT COALESCE((SELECT value FROM app_settings WHERE key='preferences'),'{}') AS value)),?)", chat, project, time.Now().UnixMilli()); err != nil {
				return err
			}
		}
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(c.project_id,''),p.model,c.provider_id FROM chats c JOIN providers p ON p.id=c.provider_id WHERE c.id=?`, chat).Scan(&project, &run.Model, &run.ProviderID); err != nil {
			return err
		}
		var active int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM chat_runs WHERE chat_id=? AND status='running'", chat).Scan(&active); err != nil {
			return err
		}
		if active > 0 {
			return errors.New("this chat already has a reply in progress")
		}
		var ordinal int
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(max(ordinal),0) FROM messages WHERE chat_id=?", chat).Scan(&ordinal); err != nil {
			return err
		}
		run.ID = rand.Text()
		run.ChatID = chat
		run.ProjectID = project
		run.UserID = rand.Text()
		run.AssistantID = rand.Text()
		run.Attempt = 1
		run.Status = "running"
		now := time.Now().UnixMilli()
		if _, err = tx.ExecContext(ctx, `INSERT INTO messages(id,chat_id,role,content,created_at,ordinal) VALUES (?,?,'user',?,?,?),(?,?,'assistant','',?,?)`, run.UserID, chat, text, now, ordinal+1, run.AssistantID, chat, now, ordinal+2); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO chat_runs(id,chat_id,user_message_id,assistant_message_id,model,status,created_at,updated_at) VALUES (?,?,?,?,?,'running',?,?)`, run.ID, chat, run.UserID, run.AssistantID, run.Model, now, now); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO chat_requests VALUES (?,?,1)", requestID, run.ID); err != nil {
			return err
		}
		title := strings.Join(strings.Fields(text), " ")
		runes := []rune(title)
		if len(runes) > 60 {
			title = string(runes[:60]) + "…"
		}
		if _, err = tx.ExecContext(ctx, "UPDATE chats SET title=? WHERE id=? AND title='New chat'", title, chat); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE workspace_state SET project_id=NULLIF(?,''),chat_id=? WHERE id=1", project, chat); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM drafts WHERE scope=?", scope); err != nil {
			return err
		}
		created = true
		return nil
	})
	return run, created, err
}
func (s *Store) RetryRun(ctx context.Context, id, requestID string) (Run, bool, error) {
	if requestID == "" || len(requestID) > 120 {
		return Run{}, false, errors.New("invalid request identifier")
	}
	var run Run
	created := false
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		existing, err := scanRun(tx.QueryRowContext(ctx, runSQL+"JOIN chat_requests q ON q.run_id=r.id WHERE q.id=?", requestID))
		if err == nil {
			if existing.ID != id {
				return errors.New("request identifier already used")
			}
			run = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		run, err = scanRun(tx.QueryRowContext(ctx, runSQL+"WHERE r.id=?", id))
		if err != nil {
			return err
		}
		if run.Status == "running" || run.Status == "complete" {
			return errors.New("this reply cannot be retried")
		}
		var latest, userOrdinal int
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(max(ordinal),0) FROM messages WHERE chat_id=? AND role='user'", run.ChatID).Scan(&latest); err != nil {
			return err
		}
		if err = tx.QueryRowContext(ctx, "SELECT ordinal FROM messages WHERE id=?", run.UserID).Scan(&userOrdinal); err != nil {
			return err
		}
		if latest != userOrdinal {
			return errors.New("only the latest reply can be retried; send a new message instead")
		}
		m, err := scanMessage(tx.QueryRowContext(ctx, messageSQL+"WHERE m.id=?", run.AssistantID))
		if err != nil {
			return err
		}
		previous := append(m.Previous, Attempt{Content: m.Content, Status: m.Status, Error: m.Error})
		data, err := json.Marshal(previous)
		if err != nil {
			return err
		}
		if err = tx.QueryRowContext(ctx, "SELECT p.model FROM providers p JOIN chats c ON c.provider_id=p.id WHERE c.id=?", run.ChatID).Scan(&run.Model); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE chat_runs SET status='running',model=?,error='',error_kind='',attempt=attempt+1,sequence=sequence+1,previous_attempts=?,usage=NULL,updated_at=? WHERE id=?`, run.Model, string(data), time.Now().UnixMilli(), id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE messages SET content='' WHERE id=?", run.AssistantID); err != nil {
			return err
		}
		run.Attempt++
		run.Status = "running"
		if _, err = tx.ExecContext(ctx, "INSERT INTO chat_requests VALUES (?,?,?)", requestID, id, run.Attempt); err != nil {
			return err
		}
		created = true
		return nil
	})
	return run, created, err
}
func (s *Store) History(ctx context.Context, run Run) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, messageSQL+"WHERE m.chat_id=? AND COALESCE(m.ordinal,0)<(SELECT ordinal FROM messages WHERE id=?) ORDER BY COALESCE(m.ordinal,0),m.created_at,m.rowid", run.ChatID, run.AssistantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		if m.Role == "tool" {
			continue
		}
		if strings.TrimSpace(m.Content) != "" {
			result = append(result, m)
		}
	}
	return result, rows.Err()
}
func (s *Store) UpdateRun(ctx context.Context, run Run, content, status, detail, kind string, usage json.RawMessage) (Message, error) {
	if !utf8.ValidString(content) {
		return Message{}, errors.New("invalid reply encoding")
	}
	var message Message
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		var savedUsage any
		if len(usage) > 0 {
			savedUsage = string(usage)
		}
		result, err := tx.ExecContext(ctx, `UPDATE chat_runs SET status=?,error=?,error_kind=?,sequence=sequence+1,usage=COALESCE(?,usage),updated_at=? WHERE id=? AND attempt=? AND status='running'`, status, detail, kind, savedUsage, time.Now().UnixMilli(), run.ID, run.Attempt)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrStaleRun
		}
		if _, err = tx.ExecContext(ctx, "UPDATE messages SET content=? WHERE id=?", content, run.AssistantID); err != nil {
			return err
		}
		message, err = scanMessage(tx.QueryRowContext(ctx, messageSQL+"WHERE m.id=?", run.AssistantID))
		if err == nil {
			message.Steps, err = activity(ctx, tx, message.ID)
		}
		return err
	})
	return message, err
}
