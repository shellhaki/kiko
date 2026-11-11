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
	"unicode/utf8"

	"kiko/core/document"
	"kiko/core/provider"
	"kiko/core/storage"
	"kiko/core/workspace"
)

func (s *Service) UseFolder(ctx context.Context, path, chat string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return errors.New("Choose an existing folder.")
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return errors.New("Choose an existing folder.")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return errors.New("Choose an existing folder.")
	}
	if err = s.Store.UseFolder(ctx, path, filepath.Base(path), chat); err != nil {
		return err
	}
	if chat != "" {
		docs, e := s.Store.ChatDocuments(ctx, chat)
		if e != nil {
			return e
		}
		for _, doc := range docs {
			_ = s.SyncWorkspace(ctx, storage.Run{ChatID: chat}, doc.ID)
		}
		artifacts, e := s.Store.Artifacts(ctx, chat)
		if e != nil {
			return e
		}
		for _, artifact := range artifacts {
			_ = s.SyncWorkspace(ctx, storage.Run{ChatID: chat}, artifact.ID)
		}
	}
	return nil
}
func fileTools() []provider.Tool {
	str := map[string]any{"type": "string"}
	definitions := []provider.ToolDefinition{
		{Name: "list_files", Description: "List entries inside the selected project folder. Paths are relative to its root.", Parameters: toolSchema(map[string]any{"path": str}, "path")},
		{Name: "read_file", Description: "Read a text file up to 1 MB inside the selected folder. Returns content and SHA-256 hash for guarded writes.", Parameters: toolSchema(map[string]any{"path": str}, "path")},
		{Name: "write_file", Description: "Create or update a text file inside the selected folder, including subdirectories. Use /docs/name.md for a root-relative path. Existing files require expected_hash from read_file. Writes apply directly. Do not claim completion until successful.", Parameters: toolSchema(map[string]any{"path": str, "content": str, "expected_hash": str}, "path", "content", "expected_hash")},
		{Name: "delete_file", Description: "Delete a text file inside the selected folder. Use /docs/name.md for a root-relative path. Requires expected_hash from read_file. Pauses for user approval; only delete what the user asked to delete. Do not claim completion until successful.", Parameters: toolSchema(map[string]any{"path": str, "expected_hash": str}, "path", "expected_hash")},
	}
	result := []provider.Tool{}
	for _, definition := range definitions {
		result = append(result, provider.Tool{Type: "function", Function: definition})
	}
	return result
}
func (s *Service) writeWorkspaceFile(ctx context.Context, run storage.Run, path, content, expected, source string) (workspace.File, error) {
	folder, err := s.Store.ChatFolder(ctx, run.ChatID)
	if err != nil {
		return workspace.File{}, errors.New("Select a project folder before writing files.")
	}
	if len(content) > workspace.MaxFileSize || !utf8.ValidString(content) || strings.ContainsRune(content, '\x00') {
		return workspace.File{}, errors.New("File content must be text up to 1 MB.")
	}
	path, err = workspace.Path(path)
	if err != nil {
		return workspace.File{}, err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	current, err := s.Store.ChatFolder(ctx, run.ChatID)
	if err != nil || current.Root != folder.Root {
		return workspace.File{}, errors.New("Workspace folder changed before saving.")
	}
	var owner string
	ownerErr := s.Store.DB().QueryRowContext(ctx, "SELECT source_id FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", folder.ProjectID, filepath.ToSlash(path)).Scan(&owner)
	if ownerErr != nil && !errors.Is(ownerErr, sql.ErrNoRows) {
		return workspace.File{}, ownerErr
	}
	if owner != "" && owner != source {
		return workspace.File{}, errors.New("This file is now linked to an editable artifact. Open that artifact before changing it.")
	}
	if source != "" {
		latest, e := s.sourceContent(ctx, source)
		if e != nil {
			return workspace.File{}, e
		}
		if latest.Content != content {
			return workspace.File{}, errors.New("The artifact changed while its file save was waiting. Save the current version instead.")
		}
	}
	if err = ctx.Err(); err != nil {
		return workspace.File{}, err
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return workspace.File{}, errors.New("The selected folder is unavailable. Reconnect it before writing.")
	}
	defer root.Close()
	file, err := workspace.Write(root, path, content, expected)
	if err != nil {
		return file, err
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = s.Store.RecordWorkspaceFile(saveCtx, folder, run.ChatID, file.Path, source, file.Hash)
	return file, err
}
func (s *Service) deleteWorkspaceFile(ctx context.Context, run storage.Run, path, expected string) (string, error) {
	folder, err := s.Store.ChatFolder(ctx, run.ChatID)
	if err != nil {
		return "", errors.New("Select a project folder before deleting files.")
	}
	path, err = workspace.Path(path)
	if err != nil || path == "." {
		return "", errors.New("Choose a file path inside the selected folder.")
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	current, err := s.Store.ChatFolder(ctx, run.ChatID)
	if err != nil || current.Root != folder.Root {
		return "", errors.New("Workspace folder changed before deleting.")
	}
	linked, err := s.workspaceFileOwner(ctx, folder.ProjectID, path)
	if err != nil {
		return "", err
	}
	if linked {
		return "", errors.New("This file is now linked to an editable artifact. Open that artifact before changing it.")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return "", errors.New("The selected folder is unavailable. Reconnect it before deleting.")
	}
	defer root.Close()
	checked, err := workspace.Read(root, path)
	if err != nil {
		if _, statErr := root.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
			if expected != "" {
				return "", errors.New("File was removed. Read it again before deleting.")
			}
			return "", errors.New("File is already gone. Nothing to delete.")
		}
		return "", err
	}
	if expected == "" || expected != checked.Hash {
		return "", errors.New("File changed since it was read. Read it again before deleting.")
	}
	if err = s.approveDestructive(ctx, run, filepath.ToSlash(path), "", "delete"); err != nil {
		return "", err
	}
	linked, err = s.workspaceFileOwner(ctx, folder.ProjectID, path)
	if err != nil {
		return "", err
	}
	if linked {
		return "", errors.New("This file is now linked to an editable artifact. Open that artifact before changing it.")
	}
	if err = workspace.Remove(root, path, checked.Hash); err != nil {
		return "", err
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = s.Store.DB().ExecContext(saveCtx, "DELETE FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", folder.ProjectID, filepath.ToSlash(path))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(path), nil
}
func (s *Service) workspaceFileOwner(ctx context.Context, project, path string) (bool, error) {
	var owner string
	err := s.Store.DB().QueryRowContext(ctx, "SELECT source_id FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", project, filepath.ToSlash(path)).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return owner != "", nil
}
func (s *Service) approveDestructive(ctx context.Context, run storage.Run, display, content, kind string) error {
	id, err := s.Store.RequestFilePermission(ctx, run.ChatID, run.ID, display, content, kind)
	if err != nil {
		return err
	}
	defer func() {
		cancel, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()
		_ = s.Store.ResolveFilePermission(cancel, id, "cancelled")
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := s.Store.FilePermissionStatus(ctx, id)
			if err != nil {
				return err
			}
			switch status {
			case "approved":
				return nil
			case "denied", "cancelled":
				return errors.New("The user declined this deletion. Do not retry it without a new instruction.")
			}
		}
	}
}
func (s *Service) fileTool(ctx context.Context, run storage.Run, call provider.ToolCall) (string, error) {
	var args struct {
		Path     string `json:"path"`
		Content  string `json:"content"`
		Expected string `json:"expected_hash"`
	}
	if err := decodeArguments(call.Function.Arguments, &args); err != nil {
		return "", err
	}
	folder, err := s.Store.ChatFolder(ctx, run.ChatID)
	if err != nil {
		return "", errors.New("Select a folder to use file tools.")
	}
	var value any
	if call.Function.Name == "delete_file" {
		var deleted string
		deleted, err = s.deleteWorkspaceFile(ctx, run, args.Path, args.Expected)
		value = map[string]string{"deleted": deleted}
	} else if call.Function.Name == "write_file" {
		var source string
		path, e := workspace.Path(args.Path)
		if e != nil {
			return "", e
		}
		if e = s.Store.DB().QueryRowContext(ctx, "SELECT source_id FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", folder.ProjectID, filepath.ToSlash(path)).Scan(&source); e != nil && !errors.Is(e, sql.ErrNoRows) {
			return "", e
		}
		if source != "" {
			return "", errors.New("This file belongs to an editable artifact. Use its document, spreadsheet or slide tools to keep the editor synchronized.")
		}
		value, err = s.writeWorkspaceFile(ctx, run, args.Path, args.Content, args.Expected, "")
	} else {
		root, e := workspace.Open(folder.Root)
		if e != nil {
			return "", errors.New("Workspace folder is unavailable.")
		}
		defer root.Close()
		if call.Function.Name == "read_file" {
			value, err = workspace.Read(root, args.Path)
		} else {
			value, err = workspace.List(root, args.Path)
		}
	}
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(value)
	return string(raw), err
}
func (s *Service) SyncWorkspace(ctx context.Context, run storage.Run, source string) (syncErr error) {
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if syncErr == nil {
			_, _ = s.Store.DB().ExecContext(cleanup, "DELETE FROM workspace_sync_errors WHERE source_id=?", source)
		} else {
			_, _ = s.Store.DB().ExecContext(cleanup, "INSERT INTO workspace_sync_errors VALUES (?,?,?) ON CONFLICT(source_id) DO UPDATE SET message=excluded.message", source, run.ChatID, syncErr.Error())
		}
	}()
	if _, err := s.Store.ChatFolder(ctx, run.ChatID); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	var title, content, extension string
	read, err := s.Store.ReadDocument(ctx, source, storage.ReadOptions{})
	if err == nil {
		if read.Document.ChatID != run.ChatID {
			return errors.New("Document belongs to another chat.")
		}
		title = read.Document.Title
		content = document.Markdown(read.Document.Blocks) + "\n"
		extension = ".md"
	} else {
		artifact, e := s.Store.ReadArtifact(ctx, source)
		if e != nil {
			return e
		}
		if artifact.ChatID != run.ChatID {
			return errors.New("Artifact belongs to another chat.")
		}
		title = artifact.Title
		content = string(artifact.Payload) + "\n"
		extension = "." + artifact.Kind + ".json"
	}
	var path, hash string
	err = s.Store.DB().QueryRowContext(ctx, "SELECT path,hash FROM workspace_files WHERE source_id=?", source).Scan(&path, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		title = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`/\\:*?"<>|`, r) || r < 32 {
				return '-'
			}
			return r
		}, title)
		title = strings.Trim(title, " .")
		if title == "" {
			title = "Untitled"
		}
		path = title + "-" + source + extension
		if err = s.ReserveSourcePath(ctx, run.ChatID, source, path); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if hash == workspace.Hash([]byte(content)) {
		folder, e := s.Store.ChatFolder(ctx, run.ChatID)
		if e != nil {
			return e
		}
		root, e := workspace.Open(folder.Root)
		if e != nil {
			return errors.New("Workspace folder is unavailable.")
		}
		defer root.Close()
		file, e := workspace.Read(root, path)
		if e != nil {
			return e
		}
		if file.Hash != hash {
			return errors.New("The source file changed outside Kiko. Compare it before saving.")
		}
		return nil
	}
	_, err = s.writeWorkspaceFile(ctx, run, path, content, hash, source)
	return err
}
