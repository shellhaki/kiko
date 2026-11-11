package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kiko/core/deck"
	"kiko/core/document"
	"kiko/core/sheet"
	"kiko/core/storage"
	"kiko/core/workspace"
)

type WorkspaceSource struct {
	ID          string `json:"id"`
	ChatID      string `json:"chatId"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	Rev         int    `json:"rev"`
	Content     string `json:"content"`
	DiskContent string `json:"diskContent"`
	DiskHash    string `json:"diskHash"`
	Status      string `json:"status"`
	Issue       string `json:"issue"`
	Draft       bool   `json:"draft"`
	Running     bool   `json:"running"`
}

func (s *Service) sourceContent(ctx context.Context, id string) (WorkspaceSource, error) {
	source := WorkspaceSource{ID: id}
	read, err := s.Store.ReadDocument(ctx, id, storage.ReadOptions{})
	if err == nil {
		source.ChatID = read.Document.ChatID
		source.Title = read.Document.Title
		source.Kind = "document"
		source.Rev = read.Document.Rev
		source.Content = document.Markdown(read.Document.Blocks) + "\n"
		source.Draft = read.Draft != nil
		return source, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return source, err
	}
	record, err := s.Store.ReadArtifact(ctx, id)
	if err != nil {
		return source, err
	}
	source.ChatID = record.ChatID
	source.Title = record.Title
	source.Kind = record.Kind
	source.Rev = record.Rev
	source.Content = string(record.Payload) + "\n"
	source.Draft = record.Draft != nil
	return source, nil
}
func sourceExtension(kind string) string {
	if kind == "document" {
		return ".md"
	}
	return "." + kind + ".json"
}
func (s *Service) artifactPath(ctx context.Context, chat, kind, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	folder, err := s.Store.ChatFolder(ctx, chat)
	if err != nil {
		return "", errors.New("Select a folder before choosing a source file path.")
	}
	path, err = workspace.Path(path)
	if err != nil {
		return "", err
	}
	extension := sourceExtension(kind)
	if !strings.HasSuffix(strings.ToLower(path), extension) {
		return "", errors.New("Source path must end with " + extension + ".")
	}
	var existing string
	err = s.Store.DB().QueryRowContext(ctx, "SELECT path FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", folder.ProjectID, filepath.ToSlash(path)).Scan(&existing)
	if err == nil {
		return "", errors.New("This source path is already in use. Open the existing file or choose another name.")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return "", errors.New("Workspace folder is unavailable.")
	}
	defer root.Close()
	if _, err = root.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("This path already exists or is unavailable. Choose a new source file path.")
	}
	return filepath.ToSlash(path), nil
}
func (s *Service) ReserveSourcePath(ctx context.Context, chat, id, path string) error {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	source, err := s.sourceContent(ctx, id)
	if err != nil {
		return err
	}
	if source.ChatID != chat {
		return errors.New("Artifact belongs to another chat.")
	}
	path, err = s.artifactPath(ctx, chat, source.Kind, path)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	folder, err := s.Store.ChatFolder(ctx, chat)
	if err != nil {
		return err
	}
	err = s.Store.Transaction(ctx, func(tx *sql.Tx) error {
		var count int
		if e := tx.QueryRowContext(ctx, "SELECT count(*) FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", folder.ProjectID, path).Scan(&count); e != nil {
			return e
		}
		if count > 0 {
			return errors.New("Source path is already reserved.")
		}
		_, e := tx.ExecContext(ctx, "INSERT INTO workspace_files(project_id,chat_id,path,source_id,hash) VALUES (?,?,?,?,'')", folder.ProjectID, chat, path, id)
		return e
	})
	if err != nil {
		return errors.New("Couldn’t reserve this source path. Choose another unused path.")
	}
	return nil
}
func (s *Service) WorkspaceSource(ctx context.Context, id string) (*WorkspaceSource, error) {
	source, err := s.sourceContent(ctx, id)
	if err != nil {
		return nil, err
	}
	folder, err := s.Store.ChatFolder(ctx, source.ChatID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var baseline string
	err = s.Store.DB().QueryRowContext(ctx, "SELECT path,hash FROM workspace_files WHERE source_id=? AND project_id=?", id, folder.ProjectID).Scan(&source.Path, &baseline)
	if errors.Is(err, sql.ErrNoRows) {
		source.Status = "unlinked"
		return &source, nil
	}
	if err != nil {
		return nil, err
	}
	var running int
	if err = s.Store.DB().QueryRowContext(ctx, "SELECT count(*) FROM chat_runs WHERE chat_id=? AND status='running'", source.ChatID).Scan(&running); err != nil {
		return nil, err
	}
	source.Running = running > 0
	issueErr := s.Store.DB().QueryRowContext(ctx, "SELECT message FROM workspace_sync_errors WHERE source_id=?", id).Scan(&source.Issue)
	if issueErr != nil && !errors.Is(issueErr, sql.ErrNoRows) {
		return nil, issueErr
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		source.Status = "unavailable"
		source.Issue = "Workspace folder is unavailable."
		return &source, nil
	}
	defer root.Close()
	file, err := workspace.Read(root, source.Path)
	if err != nil {
		source.Status = "unavailable"
		source.Issue = err.Error()
		if _, e := root.Lstat(source.Path); errors.Is(e, os.ErrNotExist) {
			source.Status = "missing"
			if baseline == "" {
				source.Status = "pending"
			}
		}
		return &source, nil
	}
	source.DiskContent = file.Content
	source.DiskHash = file.Hash
	source.Status = "synced"
	if file.Hash != baseline {
		source.Status = "changed"
	} else if source.Issue != "" {
		source.Status = "pending"
	}
	return &source, nil
}
func (s *Service) ReloadWorkspaceSource(ctx context.Context, id, hash string, rev int) error {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	source, err := s.WorkspaceSource(ctx, id)
	if err != nil {
		return err
	}
	if source == nil || source.Path == "" {
		return errors.New("This artifact has no workspace source file.")
	}
	if source.Running {
		return errors.New("Stop the current reply before loading an external version.")
	}
	if source.Draft {
		return errors.New("Save or recover your current draft before loading an external version.")
	}
	if source.Rev != rev || source.DiskHash != hash || hash == "" {
		return errors.New("The document or file changed. Compare the latest versions again.")
	}
	if source.Kind == "document" {
		blocks, e := document.Parse(source.DiskContent, nil)
		if e != nil {
			return e
		}
		if _, err = s.Store.SaveDocument(ctx, id, rev, blocks); err != nil {
			return err
		}
	} else {
		raw := json.RawMessage(source.DiskContent)
		if source.Kind == "deck" {
			var slides deck.Deck
			if err = json.Unmarshal(raw, &slides); err != nil {
				return err
			}
			if err = s.validateSlideImages(ctx, source.ChatID, slides); err != nil {
				return err
			}
		}
		if _, err = s.Store.SaveArtifact(ctx, id, rev, raw, "user"); err != nil {
			return err
		}
	}
	folder, err := s.Store.ChatFolder(ctx, source.ChatID)
	if err != nil {
		return err
	}
	if err = s.Store.RecordWorkspaceFile(ctx, folder, source.ChatID, source.Path, id, hash); err != nil {
		return err
	}
	_, err = s.Store.DB().ExecContext(ctx, "DELETE FROM workspace_sync_errors WHERE source_id=?", id)
	return err
}

func (s *Service) CreateArtifactAt(ctx context.Context, chat, kind, title, path string) error {
	if kind != "document" && kind != "sheet" && kind != "deck" {
		return errors.New("Choose a document, spreadsheet or deck.")
	}
	path, err := s.artifactPath(ctx, chat, kind, path)
	if err != nil {
		return err
	}
	var id string
	if kind == "document" {
		doc, e := s.Store.CreateDocument(ctx, chat, title, "# "+title, nil, "user")
		if e != nil {
			return e
		}
		id = doc.ID
	} else {
		var raw []byte
		if kind == "sheet" {
			raw, _ = json.Marshal(sheet.Book{Tabs: []sheet.Tab{{Name: "Sheet1", Cells: map[string]sheet.Cell{}}}})
		} else {
			slides := deck.Deck{Slides: []deck.Slide{{Layout: "title", Title: title, Body: []string{}}}}
			if err = deck.Prepare(&slides); err != nil {
				return err
			}
			raw, _ = json.Marshal(slides)
		}
		record, e := s.Store.CreateArtifact(ctx, chat, kind, title, raw, "user")
		if e != nil {
			return e
		}
		id = record.ID
	}
	if path != "" {
		if err = s.ReserveSourcePath(ctx, chat, id, path); err != nil {
			return err
		}
	}
	_ = s.SyncWorkspace(ctx, storage.Run{ChatID: chat}, id)
	return nil
}

func (s *Service) SaveWorkspaceSourceVersion(ctx context.Context, id, hash string, rev int) error {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	source, err := s.WorkspaceSource(ctx, id)
	if err != nil {
		return err
	}
	if source == nil || source.Path == "" {
		return errors.New("Artifact has no workspace source file.")
	}
	if source.Running || source.Draft {
		return errors.New("Save the current draft and stop any reply before replacing a folder version.")
	}
	if source.Status == "unavailable" || source.Rev != rev || source.DiskHash != hash {
		return errors.New("The artifact or file changed. Compare the latest versions again.")
	}
	folder, err := s.Store.ChatFolder(ctx, source.ChatID)
	if err != nil {
		return err
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := workspace.Write(root, source.Path, source.Content, hash)
	if err != nil {
		return err
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err = s.Store.RecordWorkspaceFile(saveCtx, folder, source.ChatID, file.Path, id, file.Hash); err != nil {
		return err
	}
	_, err = s.Store.DB().ExecContext(saveCtx, "DELETE FROM workspace_sync_errors WHERE source_id=?", id)
	return err
}
