package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"
)

type Folder struct {
	ProjectID string `json:"projectId"`
	Root      string `json:"root"`
	Mode      string `json:"mode"`
}
type FilePermission struct {
	ID      string `json:"id"`
	ChatID  string `json:"chatId"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Kind    string `json:"kind"`
}
type WorkspaceFile struct {
	ChatID   string `json:"chatId"`
	Path     string `json:"path"`
	SourceID string `json:"sourceId"`
	Hash     string `json:"hash"`
}

func (s *Store) UseFolder(ctx context.Context, path, name, chat string) error {
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		var project string
		err := tx.QueryRowContext(ctx, "SELECT project_id FROM project_roots WHERE path=?", path).Scan(&project)
		if errors.Is(err, sql.ErrNoRows) {
			project = rand.Text()
			if _, err = tx.ExecContext(ctx, "INSERT INTO projects(id,name,description,created_at) VALUES (?,?,'',?)", project, name, time.Now().UnixMilli()); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO project_roots VALUES (?,?)", project, path)
		}
		if err != nil {
			return err
		}
		if chat != "" {
			var running int
			if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM chat_runs WHERE chat_id=? AND status='running'", chat).Scan(&running); err != nil {
				return err
			}
			if running > 0 {
				return errors.New("Stop the current reply before changing its folder.")
			}
			var owned int
			if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM workspace_files WHERE chat_id=?", chat).Scan(&owned); err != nil {
				return err
			}
			if owned > 0 {
				var current string
				if err = tx.QueryRowContext(ctx, "SELECT COALESCE(project_id,'') FROM chats WHERE id=?", chat).Scan(&current); err != nil {
					return err
				}
				if current != project {
					return errors.New("Start a new chat to use a different folder. This chat already has files.")
				}
			}
			result, e := tx.ExecContext(ctx, "UPDATE chats SET project_id=? WHERE id=?", project, chat)
			if e = affected(result, e); e != nil {
				return e
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO chat_permissions VALUES (?,'ask') ON CONFLICT(chat_id) DO UPDATE SET mode='ask'", chat); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE workspace_state SET project_id=?,chat_id=NULLIF(?,'') WHERE id=1", project, chat)
		return err
	})
}
func (s *Store) ChatFolder(ctx context.Context, chat string) (Folder, error) {
	var folder Folder
	err := s.db.QueryRowContext(ctx, `SELECT r.project_id,r.path,COALESCE(p.mode,'ask') FROM chats c JOIN project_roots r ON r.project_id=c.project_id LEFT JOIN chat_permissions p ON p.chat_id=c.id WHERE c.id=?`, chat).Scan(&folder.ProjectID, &folder.Root, &folder.Mode)
	return folder, err
}
func (s *Store) SetFileMode(ctx context.Context, chat, mode string) error {
	if mode != "ask" && mode != "full" {
		return errors.New("invalid permission mode")
	}
	if _, err := s.ChatFolder(ctx, chat); err != nil {
		return errors.New("Select a folder first.")
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO chat_permissions VALUES (?,?) ON CONFLICT(chat_id) DO UPDATE SET mode=excluded.mode", chat, mode)
	return err
}
func (s *Store) RequestFilePermission(ctx context.Context, chat, run, path, content, kind string) (string, error) {
	id := rand.Text()
	_, err := s.db.ExecContext(ctx, "INSERT INTO file_permissions(id,chat_id,run_id,path,content,kind) VALUES (?,?,?,?,?,?)", id, chat, run, path, content, kind)
	return id, err
}
func (s *Store) FilePermissionStatus(ctx context.Context, id string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, "SELECT status FROM file_permissions WHERE id=?", id).Scan(&status)
	return status, err
}
func (s *Store) ResolveFilePermission(ctx context.Context, id, status string) error {
	if status != "approved" && status != "denied" && status != "cancelled" {
		return errors.New("invalid permission decision")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE file_permissions SET status=? WHERE id=? AND status='pending' AND EXISTS(SELECT 1 FROM chat_runs r WHERE r.id=file_permissions.run_id AND r.status='running')`, status, id)
	return affected(result, err)
}
func (s *Store) FilePermissions(ctx context.Context, chat string) ([]FilePermission, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.chat_id,p.path,p.content,p.kind FROM file_permissions p JOIN chat_runs r ON r.id=p.run_id WHERE p.chat_id=? AND p.status='pending' AND r.status='running' ORDER BY p.rowid`, chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []FilePermission{}
	for rows.Next() {
		var p FilePermission
		if err = rows.Scan(&p.ID, &p.ChatID, &p.Path, &p.Content, &p.Kind); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (s *Store) WorkspaceFiles(ctx context.Context, chat string) ([]WorkspaceFile, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT chat_id,path,source_id,hash FROM workspace_files WHERE chat_id=? ORDER BY path", chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkspaceFile{}
	for rows.Next() {
		var f WorkspaceFile
		if err = rows.Scan(&f.ChatID, &f.Path, &f.SourceID, &f.Hash); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}
func (s *Store) RecordWorkspaceFile(ctx context.Context, folder Folder, chat, path, source, hash string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspace_files VALUES (?,?,?,?,?) ON CONFLICT(project_id,path) DO UPDATE SET chat_id=excluded.chat_id,source_id=excluded.source_id,hash=excluded.hash`, folder.ProjectID, chat, path, source, hash)
	return err
}

type WorkspaceIssue struct {
	SourceID string `json:"sourceId"`
	Message  string `json:"message"`
}

func (s *Store) WorkspaceIssues(ctx context.Context, chat string) ([]WorkspaceIssue, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT source_id,message FROM workspace_sync_errors WHERE chat_id=?", chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkspaceIssue{}
	for rows.Next() {
		var issue WorkspaceIssue
		if err = rows.Scan(&issue.SourceID, &issue.Message); err != nil {
			return nil, err
		}
		result = append(result, issue)
	}
	return result, rows.Err()
}
