package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Chat struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
}
type Document struct {
	ID        string `json:"id"`
	ChatID    string `json:"chatId"`
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
}
type Selection struct {
	ProjectID string `json:"projectId"`
	ChatID    string `json:"chatId"`
}
type State struct {
	Provider        ProviderConfig   `json:"provider"`
	Providers       []ProviderConfig `json:"providers"`
	Artifacts       []Artifact       `json:"artifacts"`
	ArtifactID      string           `json:"artifactId"`
	Messages        []Message        `json:"messages"`
	Draft           string           `json:"draft"`
	ConnectionIssue string           `json:"connectionIssue,omitempty"`
	DocumentID      string           `json:"documentId"`
	Preferences     Preferences      `json:"preferences"`
	Projects        []Project        `json:"projects"`
	Chats           []Chat           `json:"chats"`
	Documents       []Document       `json:"documents"`
	Selection       Selection        `json:"selection"`
	Connected       bool             `json:"connected"`
}
type SearchResult struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	ChatID    string `json:"chatId"`
	ProjectID string `json:"projectId"`
}

func (s *Store) Snapshot(ctx context.Context) (State, error) {
	state := State{Projects: []Project{}, Chats: []Chat{}, Documents: []Document{}}
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return state, err
	}
	state.Preferences = prefs
	err = s.Transaction(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT id,name,COALESCE(description,'') FROM projects ORDER BY created_at DESC,id")
		if err != nil {
			return err
		}
		for rows.Next() {
			var p Project
			if err = rows.Scan(&p.ID, &p.Name, &p.Description); err != nil {
				rows.Close()
				return err
			}
			state.Projects = append(state.Projects, p)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		rows, err = tx.QueryContext(ctx, "SELECT id,COALESCE(project_id,''),COALESCE(title,'New chat') FROM chats ORDER BY created_at DESC,id")
		if err != nil {
			return err
		}
		for rows.Next() {
			var c Chat
			if err = rows.Scan(&c.ID, &c.ProjectID, &c.Title); err != nil {
				rows.Close()
				return err
			}
			state.Chats = append(state.Chats, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		rows, err = tx.QueryContext(ctx, "SELECT d.id,d.chat_id,COALESCE(c.project_id,''),d.title FROM documents d JOIN chats c ON c.id=d.chat_id ORDER BY d.updated_at DESC,d.id")
		if err != nil {
			return err
		}
		for rows.Next() {
			var d Document
			if err = rows.Scan(&d.ID, &d.ChatID, &d.ProjectID, &d.Title); err != nil {
				rows.Close()
				return err
			}
			state.Documents = append(state.Documents, d)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(project_id,''),COALESCE(chat_id,'') FROM workspace_state WHERE id=1").Scan(&state.Selection.ProjectID, &state.Selection.ChatID); err != nil {
			return err
		}
		var id sql.NullString
		err = tx.QueryRowContext(ctx, "SELECT document_id,COALESCE(json_extract(state,'$.artifactId'),'') FROM chat_view_state WHERE chat_id=?", state.Selection.ChatID).Scan(&id, &state.ArtifactID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		state.DocumentID = id.String
		return err
	})
	if err == nil {
		var conversation Conversation
		conversation, err = s.Conversation(ctx, state.Selection.ProjectID, state.Selection.ChatID)
		state.Messages = conversation.Messages
		state.Draft = conversation.Draft
	}
	if err == nil {
		state.Artifacts, err = s.Artifacts(ctx, "")
		if err == nil {
			state.Provider, err = s.ChatProvider(ctx, state.Selection.ChatID)
		}
		if err == nil {
			state.Providers, err = s.Providers(ctx)
		}
	}
	return state, err
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return "", errors.New("name must contain 1–120 characters")
	}
	return name, nil
}

func (s *Store) CreateProject(ctx context.Context, name, description string) error {
	name, err := validName(name)
	if err != nil {
		return err
	}
	id := rand.Text()
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO projects VALUES (?,?,?,?)", id, name, strings.TrimSpace(description), time.Now().UnixMilli()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE workspace_state SET project_id=?,chat_id=NULL WHERE id=1", id)
		return err
	})
}

func (s *Store) CreateChat(ctx context.Context, projectID string) error {
	id := rand.Text()
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO chats VALUES (?,NULLIF(?,''),'New chat',(SELECT COALESCE(json_extract(value,'$.providerId'),'muse') FROM (SELECT COALESCE((SELECT value FROM app_settings WHERE key='preferences'),'{}') AS value)),?)", id, projectID, time.Now().UnixMilli()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE workspace_state SET project_id=NULLIF(?,''),chat_id=? WHERE id=1", projectID, id)
		return err
	})
}

func (s *Store) Select(ctx context.Context, projectID, chatID string) error {
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		if chatID != "" {
			if err := tx.QueryRowContext(ctx, "SELECT COALESCE(project_id,'') FROM chats WHERE id=?", chatID).Scan(&projectID); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, "UPDATE workspace_state SET project_id=NULLIF(?,''),chat_id=NULLIF(?,'') WHERE id=1", projectID, chatID)
		return err
	})
}

func (s *Store) Rename(ctx context.Context, kind, id, name string) error {
	table := map[string]string{"project": "projects", "chat": "chats"}[kind]
	if table == "" {
		return errors.New("invalid item type")
	}
	name, err := validName(name)
	if err != nil {
		return err
	}
	column := "name"
	if kind == "chat" {
		column = "title"
	}
	result, err := s.db.ExecContext(ctx, "UPDATE "+table+" SET "+column+"=? WHERE id=?", name, id)
	return affected(result, err)
}

func (s *Store) Delete(ctx context.Context, kind, id string) error {
	table := map[string]string{"project": "projects", "chat": "chats"}[kind]
	if table == "" {
		return errors.New("invalid item type")
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE id=?", id)
	return affected(result, err)
}

func affected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) Search(ctx context.Context, query string) ([]SearchResult, error) {
	results := []SearchResult{}
	query = strings.TrimSpace(query)
	if query == "" {
		return results, nil
	}
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query) + "%"
	rows, err := s.db.QueryContext(ctx, `SELECT 'chat',id,COALESCE(title,'New chat'),id,COALESCE(project_id,'') FROM chats WHERE title LIKE ? ESCAPE '\'
 UNION ALL SELECT 'document',d.id,d.title,d.chat_id,COALESCE(c.project_id,'') FROM documents d JOIN chats c ON c.id=d.chat_id
 WHERE d.title LIKE ? ESCAPE '\' OR EXISTS (SELECT 1 FROM blocks b WHERE b.document_id=d.id AND b.gone_rev IS NULL AND b.content LIKE ? ESCAPE '\')
 ORDER BY 3 LIMIT 50`, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r SearchResult
		if err = rows.Scan(&r.Kind, &r.ID, &r.Title, &r.ChatID, &r.ProjectID); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) OpenDocument(ctx context.Context, id string) error {
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		var chat, project string
		if err := tx.QueryRowContext(ctx, "SELECT d.chat_id,COALESCE(c.project_id,'') FROM documents d JOIN chats c ON c.id=d.chat_id WHERE d.id=?", id).Scan(&chat, &project); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE workspace_state SET chat_id=?,project_id=NULLIF(?,'') WHERE id=1", chat, project); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO chat_view_state VALUES (?,?,'{}') ON CONFLICT(chat_id) DO UPDATE SET document_id=excluded.document_id,state=json_remove(chat_view_state.state,'$.artifactId')`, chat, id)
		return err
	})
}
