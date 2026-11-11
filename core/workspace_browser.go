package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"kiko/core/deck"
	"kiko/core/document"
	"kiko/core/sheet"
	"kiko/core/storage"
	"kiko/core/workspace"
)

type WorkspaceEntry struct {
	workspace.File
	SourceID string              `json:"sourceId"`
	ChatID   string              `json:"chatId"`
	Kind     string              `json:"kind"`
	Draft    *WorkspaceTextDraft `json:"draft,omitempty"`
}
type WorkspaceTextDraft struct {
	Content  string `json:"content"`
	BaseHash string `json:"baseHash"`
}

func (s *Service) projectFolder(ctx context.Context, project string) (storage.Folder, error) {
	folder := storage.Folder{ProjectID: project}
	err := s.Store.DB().QueryRowContext(ctx, "SELECT path FROM project_roots WHERE project_id=?", project).Scan(&folder.Root)
	if err != nil {
		return folder, errors.New("Select an available workspace folder.")
	}
	return folder, nil
}
func (s *Service) ListWorkspaceDirectory(ctx context.Context, project, path string) ([]workspace.Entry, error) {
	folder, err := s.projectFolder(ctx, project)
	if err != nil {
		return nil, err
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return nil, errors.New("Workspace folder is unavailable.")
	}
	defer root.Close()
	entries, err := workspace.List(root, path)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Directory != entries[j].Directory {
			return entries[i].Directory
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, err
}
func (s *Service) ReadWorkspaceEntry(ctx context.Context, project, path string) (WorkspaceEntry, error) {
	folder, err := s.projectFolder(ctx, project)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return WorkspaceEntry{}, errors.New("Workspace folder is unavailable.")
	}
	defer root.Close()
	file, err := workspace.Read(root, path)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	entry := WorkspaceEntry{File: file}
	err = s.Store.DB().QueryRowContext(ctx, "SELECT source_id,chat_id FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", project, file.Path).Scan(&entry.SourceID, &entry.ChatID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return entry, err
	}
	if entry.SourceID != "" {
		source, e := s.sourceContent(ctx, entry.SourceID)
		if e != nil {
			return entry, e
		}
		entry.Kind = source.Kind
	}
	var draft WorkspaceTextDraft
	err = s.Store.DB().QueryRowContext(ctx, "SELECT base_hash,content FROM workspace_text_drafts WHERE project_id=? AND path=?", project, file.Path).Scan(&draft.BaseHash, &draft.Content)
	if err == nil {
		entry.Draft = &draft
	} else if !errors.Is(err, sql.ErrNoRows) {
		return entry, err
	}
	return entry, nil
}
func (s *Service) SaveWorkspaceTextDraft(ctx context.Context, project, path, hash, content string) error {
	path, err := workspace.Path(path)
	if err != nil {
		return err
	}
	if len(content) > workspace.MaxFileSize || !utf8.ValidString(content) || strings.ContainsRune(content, '\x00') {
		return errors.New("Text drafts are limited to 1 MB.")
	}
	if _, err = s.projectFolder(ctx, project); err != nil {
		return err
	}
	_, err = s.Store.DB().ExecContext(ctx, `INSERT INTO workspace_text_drafts VALUES (?,?,?,?) ON CONFLICT(project_id,path) DO UPDATE SET base_hash=excluded.base_hash,content=excluded.content`, project, filepath.ToSlash(path), hash, content)
	return err
}
func (s *Service) SaveWorkspaceText(ctx context.Context, project, chat, path, hash, content string) (WorkspaceEntry, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	folder, err := s.projectFolder(ctx, project)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	path, err = workspace.Path(path)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	path = filepath.ToSlash(path)
	var source string
	err = s.Store.DB().QueryRowContext(ctx, "SELECT source_id FROM workspace_files WHERE project_id=? AND path=? COLLATE NOCASE", project, path).Scan(&source)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return WorkspaceEntry{}, err
	}
	if source != "" {
		return WorkspaceEntry{}, errors.New("Open the linked artifact to edit this source file.")
	}
	if chat != "" {
		current, e := s.Store.ChatFolder(ctx, chat)
		if e != nil || current.ProjectID != project {
			return WorkspaceEntry{}, errors.New("Chat belongs to another folder.")
		}
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return WorkspaceEntry{}, errors.New("Workspace folder is unavailable.")
	}
	defer root.Close()
	file, err := workspace.Write(root, path, content, hash)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if chat != "" {
		if err = s.Store.RecordWorkspaceFile(saveCtx, folder, chat, file.Path, "", file.Hash); err != nil {
			return WorkspaceEntry{}, err
		}
	}
	_, err = s.Store.DB().ExecContext(saveCtx, "DELETE FROM workspace_text_drafts WHERE project_id=? AND path=? AND content=?", project, file.Path, content)
	return WorkspaceEntry{File: file}, err
}
func (s *Service) DiscardWorkspaceTextDraft(ctx context.Context, project, path, hash string) error {
	entry, err := s.ReadWorkspaceEntry(ctx, project, path)
	if err != nil {
		return err
	}
	if entry.Hash != hash {
		return errors.New("File changed again. Compare its latest version.")
	}
	_, err = s.Store.DB().ExecContext(ctx, "DELETE FROM workspace_text_drafts WHERE project_id=? AND path=?", project, entry.Path)
	return err
}
func workspaceKind(path string) string {
	path = strings.ToLower(path)
	if strings.HasSuffix(path, ".sheet.json") {
		return "sheet"
	}
	if strings.HasSuffix(path, ".deck.json") {
		return "deck"
	}
	if strings.HasSuffix(path, ".md") {
		return "document"
	}
	return ""
}
func (s *Service) ImportWorkspaceFile(ctx context.Context, project, chat, path, hash string) error {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	entry, err := s.ReadWorkspaceEntry(ctx, project, path)
	if err != nil {
		return err
	}
	if entry.SourceID != "" {
		if entry.Kind == "document" {
			return s.Store.OpenDocument(ctx, entry.SourceID)
		}
		return s.Store.OpenArtifact(ctx, entry.SourceID)
	}
	if hash == "" || entry.Hash != hash {
		return errors.New("File changed. Open the latest version before importing.")
	}
	if entry.Draft != nil {
		return errors.New("Save your text draft before opening this file as an artifact.")
	}
	kind := workspaceKind(path)
	if kind == "" {
		return errors.New("Open Markdown, .sheet.json or .deck.json as an editable artifact.")
	}
	title := filepath.Base(path)
	title = strings.TrimSuffix(title, sourceExtension(kind))
	runes := []rune(title)
	if len(runes) > 120 {
		title = string(runes[:120])
	}
	if kind == "document" {
		if _, err = document.Parse(entry.Content, nil); err != nil {
			return err
		}
	} else if kind == "sheet" {
		var book sheet.Book
		if err = json.Unmarshal([]byte(entry.Content), &book); err != nil {
			return err
		}
		if err = sheet.Validate(book); err != nil {
			return err
		}
	} else {
		var slides deck.Deck
		if err = json.Unmarshal([]byte(entry.Content), &slides); err != nil {
			return err
		}
		if err = deck.Prepare(&slides); err != nil {
			return err
		}
	}
	homeChat := chat == ""
	var homeDraft string
	if homeChat {
		conversation, e := s.Store.Conversation(ctx, project, "")
		if e != nil {
			return e
		}
		homeDraft = conversation.Draft
	}
	if kind == "deck" && chat == "" {
		var slides deck.Deck
		_ = json.Unmarshal([]byte(entry.Content), &slides)
		for _, slide := range slides.Slides {
			if slide.Layout == "image" {
				return errors.New("Open this deck in a chat with its image attachments.")
			}
		}
	}
	if chat == "" {
		if err = s.Store.CreateChat(ctx, project); err != nil {
			return err
		}
		state, e := s.Store.Snapshot(ctx)
		if e != nil {
			return e
		}
		chat = state.Selection.ChatID
	} else {
		folder, e := s.Store.ChatFolder(ctx, chat)
		if e != nil || folder.ProjectID != project {
			return errors.New("Chat belongs to another workspace.")
		}
	}
	var id string
	if kind == "document" {
		doc, e := s.Store.CreateDocument(ctx, chat, title, entry.Content, nil, "user")
		if e != nil {
			return e
		}
		id = doc.ID
	} else {
		if kind == "deck" {
			var slides deck.Deck
			_ = json.Unmarshal([]byte(entry.Content), &slides)
			if err = s.validateSlideImages(ctx, chat, slides); err != nil {
				return err
			}
		}
		payload := json.RawMessage(entry.Content)
		if kind == "deck" {
			var slides deck.Deck
			if err = json.Unmarshal(payload, &slides); err != nil {
				return err
			}
			if err = deck.Prepare(&slides); err != nil {
				return err
			}
			payload, _ = json.Marshal(slides)
		}
		record, e := s.Store.CreateArtifact(ctx, chat, kind, title, payload, "user")
		if e != nil {
			return e
		}
		id = record.ID
	}
	folder, err := s.Store.ChatFolder(ctx, chat)
	if err != nil {
		return err
	}
	if err = s.Store.RecordWorkspaceFile(ctx, folder, chat, entry.Path, id, hash); err != nil {
		return err
	}
	if homeChat {
		if err = s.Store.SaveDraft(ctx, "", chat, homeDraft); err != nil {
			return err
		}
		return s.Store.SaveDraft(ctx, project, "", "")
	}
	return nil
}
