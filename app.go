package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"kiko/core/attachment"
	"kiko/core/deck"
	"kiko/core/sheet"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kiko/core"
	"kiko/core/document"
	exports "kiko/core/export"
	"kiko/core/muse"
	"kiko/core/storage"
	"kiko/core/workspace"
)

type App struct {
	pickFolder func(context.Context) (string, error)
	secretFor  func(string) core.Secrets
	onMessage  func(storage.Message)
	ctx        context.Context
	mu         sync.Mutex
	service    *core.Service
	dataPath   string
}

func (a *App) Info() core.AppInfo          { return core.AppInfo{Name: "Kiko", Version: core.Version} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.service != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.service.Close(ctx)
		a.service = nil
	}
}
func (a *App) backend() (*core.Service, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.service != nil {
		return a.service, nil
	}
	if a.ctx == nil {
		a.ctx = context.Background()
	}
	path := a.dataPath
	if path == "" {
		dir := os.Getenv("KIKO_DATA_DIR")
		if dir == "" {
			var err error
			dir, err = os.UserConfigDir()
			if err != nil {
				return nil, errors.New("Couldn’t locate your data folder.")
			}
			dir = filepath.Join(dir, "Kiko")
		}
		path = filepath.Join(dir, "kiko.db")
	}
	store, err := storage.Open(a.ctx, path)
	if err != nil {
		return nil, errors.New("Couldn’t open local storage. Retry without resetting your data.")
	}
	if err = store.RecoverRuns(a.ctx); err != nil {
		store.Close()
		return nil, errors.New("Couldn’t recover unfinished replies.")
	}
	if err = store.RecoverActivity(a.ctx); err != nil {
		store.Close()
		return nil, errors.New("Couldn’t recover unfinished activity.")
	}
	secrets := a.secretFor
	if secrets == nil {
		secrets = func(id string) core.Secrets { return systemKeys{account: id} }
	}
	a.service = &core.Service{Store: store, Keys: secrets("muse"), SecretFor: secrets, Verifier: muse.Verifier{}, Provider: muse.Client{}}
	return a.service, nil
}
func (a *App) Bootstrap() (storage.State, error) {
	s, err := a.backend()
	if err != nil {
		return storage.State{}, err
	}
	return s.State(a.ctx)
}
func (a *App) change(fn func(*core.Service) error) (storage.State, error) {
	s, err := a.backend()
	if err != nil {
		return storage.State{}, err
	}
	if err = fn(s); err != nil {
		return storage.State{}, err
	}
	return s.State(a.ctx)
}
func (a *App) SaveNickname(name string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.SaveNickname(a.ctx, name) })
}
func (a *App) Back() (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Back(a.ctx) })
}
func (a *App) ConnectMuse(key string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Connect(a.ctx, key) })
}
func (a *App) SaveAppearance(theme string, open bool, width int) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Appearance(a.ctx, theme, open, width) })
}
func (a *App) CreateProject(name, description string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Store.CreateProject(a.ctx, name, description) })
}
func (a *App) CreateChat(projectID string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Store.CreateChat(a.ctx, projectID) })
}
func (a *App) Select(projectID, chatID string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Store.Select(a.ctx, projectID, chatID) })
}
func (a *App) Rename(kind, id, name string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Store.Rename(a.ctx, kind, id, name) })
}
func (a *App) Delete(kind, id string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Delete(a.ctx, kind, id) })
}
func (a *App) Search(query string) ([]storage.SearchResult, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.Search(a.ctx, query)
}
func (a *App) OpenDocument(id string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Store.OpenDocument(a.ctx, id) })
}

func (a *App) CloseDocument() (storage.State, error) {
	return a.change(func(s *core.Service) error {
		_, err := s.Store.DB().ExecContext(a.ctx, "UPDATE chat_view_state SET document_id=NULL WHERE chat_id=(SELECT chat_id FROM workspace_state WHERE id=1)")
		return err
	})
}

func (a *App) OpenMuseGuide() {
	runtime.BrowserOpenURL(a.ctx, "https://dev.meta.ai/docs/authentication")
}

func (a *App) SendMessage(project, chat, text, requestID string) (storage.State, error) {
	s, err := a.backend()
	if err != nil {
		return storage.State{}, err
	}
	return s.Send(a.ctx, project, chat, text, requestID, func(message storage.Message) { a.emitMessage(message) })
}
func (a *App) RetryMessage(runID, requestID string) (storage.State, error) {
	s, err := a.backend()
	if err != nil {
		return storage.State{}, err
	}
	return s.Retry(a.ctx, runID, requestID, func(message storage.Message) { a.emitMessage(message) })
}
func (a *App) StopRun(id string, attempt int) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	s.Stop(id, attempt)
	return nil
}
func (a *App) Conversation(project, chat string) (storage.Conversation, error) {
	s, err := a.backend()
	if err != nil {
		return storage.Conversation{}, err
	}
	return s.Store.Conversation(a.ctx, project, chat)
}
func (a *App) SaveDraft(project, chat, content string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.SaveDraft(a.ctx, project, chat, content)
}
func (a *App) ReconnectMuse() (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Reconnect(a.ctx) })
}
func (a *App) OpenLink(value string) error {
	link, err := url.Parse(value)
	if err != nil || link.Host == "" || (link.Scheme != "https" && link.Scheme != "http") {
		return errors.New("This link cannot be opened.")
	}
	runtime.BrowserOpenURL(a.ctx, link.String())
	return nil
}

func (a *App) CreateDocument(chat, title, markdown string) (storage.State, error) {
	return a.change(func(s *core.Service) error {
		doc, err := s.Store.CreateDocument(a.ctx, chat, title, markdown, nil, "user")
		if err != nil {
			return err
		}
		_ = s.SyncWorkspace(a.ctx, storage.Run{ChatID: chat}, doc.ID)
		return nil
	})
}
func (a *App) ReadDocument(id string, options storage.ReadOptions) (storage.DocumentRead, error) {
	s, err := a.backend()
	if err != nil {
		return storage.DocumentRead{}, err
	}
	return s.Store.ReadDocument(a.ctx, id, options)
}
func (a *App) SaveDocument(id string, rev int, blocks []document.Block) (document.Document, error) {
	s, err := a.backend()
	if err != nil {
		return document.Document{}, err
	}
	doc, err := s.Store.SaveDocument(a.ctx, id, rev, blocks)
	if err == nil {
		_ = s.SyncWorkspace(a.ctx, storage.Run{ChatID: doc.ChatID}, id)
	}
	return doc, err
}
func (a *App) SaveDocumentDraft(id string, rev int, blocks []document.Block) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.SaveDocumentDraft(a.ctx, id, rev, blocks)
}
func (a *App) DiscardDocumentDraft(id string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.DiscardDocumentDraft(a.ctx, id)
}
func (a *App) SaveDocumentView(id string, state json.RawMessage) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.SaveDocumentView(a.ctx, id, state)
}

func (a *App) DocumentHistory(id string) ([]storage.Revision, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.DocumentHistory(a.ctx, id)
}
func (a *App) RestoreDocument(id string, expected, revision int) (document.Document, error) {
	s, err := a.backend()
	if err != nil {
		return document.Document{}, err
	}
	doc, err := s.Store.RestoreDocument(a.ctx, id, expected, revision)
	if err == nil {
		_ = s.SyncWorkspace(a.ctx, storage.Run{ChatID: doc.ChatID}, id)
	}
	return doc, err
}
func (a *App) ExportDocument(id string) (bool, error) { return a.ExportDocumentFormat(id, "md") }
func (a *App) ExportFormats() []string {
	formats := []string{"md", "docx"}
	if exports.TypstBinary() != "" {
		formats = append(formats, "pdf")
	}
	return formats
}
func (a *App) ExportDocumentFormat(id, format string) (bool, error) {
	if format != "md" && format != "docx" && format != "pdf" {
		return false, errors.New("Unsupported export format.")
	}
	s, err := a.backend()
	if err != nil {
		return false, err
	}
	read, err := s.Store.ReadDocument(a.ctx, id, storage.ReadOptions{})
	if err != nil {
		return false, err
	}
	if read.Draft != nil {
		return false, errors.New("Save the document draft before exporting.")
	}
	title := strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`/\\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, read.Document.Title)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{DefaultFilename: title + "." + format, Title: "Export document", Filters: []runtime.FileFilter{{DisplayName: strings.ToUpper(format), Pattern: "*." + format}}})
	if err != nil {
		return false, errors.New("Couldn’t open the save dialog.")
	}
	if path == "" {
		return false, nil
	}
	var data []byte
	switch format {
	case "md":
		data = []byte(document.Markdown(read.Document.Blocks) + "\n")
	case "docx":
		data, err = exports.Docx(read.Document)
	case "pdf":
		data, err = exports.PDF(a.ctx, read.Document)
	}
	if err != nil {
		return false, err
	}
	if err = exports.Write(path, data); err != nil {
		return false, errors.New("Couldn’t write the export. Choose another location.")
	}
	return true, nil
}

func (a *App) Questions(id string) ([]storage.Question, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.Questions(a.ctx, id)
}
func (a *App) ResolveQuestion(id string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.ResolveQuestion(a.ctx, id, true)
}
func (a *App) AnswerQuestion(id, text, requestID string) (storage.State, error) {
	s, err := a.backend()
	if err != nil {
		return storage.State{}, err
	}
	return s.AnswerQuestion(a.ctx, id, text, requestID, func(message storage.Message) { a.emitMessage(message) })
}

func (a *App) RemoveMuseKey() (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.RemoveKey(a.ctx) })
}
func (a *App) ResetData() (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Reset(a.ctx) })
}
func (a *App) SetModel(model string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.SetModel(a.ctx, model) })
}

func (a *App) CopyDocument(id string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	read, err := s.Store.ReadDocument(a.ctx, id, storage.ReadOptions{})
	if err != nil {
		return err
	}
	if read.Draft != nil {
		return errors.New("Save the document draft before copying.")
	}
	if err = runtime.ClipboardSetText(a.ctx, read.Markdown); err != nil {
		return errors.New("Couldn’t copy Markdown to the clipboard.")
	}
	return nil
}

func (a *App) ConfigureResearch(key string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.ConfigureResearch(a.ctx, key) })
}
func (a *App) RemoveResearch() (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.RemoveResearch(a.ctx) })
}

func (a *App) AttachFile(chat, name, encoded string) (storage.Attachment, error) {
	if len(encoded) > 28*1024*1024 {
		return storage.Attachment{}, errors.New("Attachment exceeds 20 MB.")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return storage.Attachment{}, errors.New("Invalid attachment data.")
	}
	var parsed attachment.Extracted
	if strings.EqualFold(filepath.Ext(name), ".pdf") {
		parsed.MIME = "application/pdf"
		parsed.Pages, err = attachment.ExtractPDFWorker(a.ctx, data)
	} else {
		parsed, err = attachment.Extract(name, data)
	}
	if err != nil {
		return storage.Attachment{}, err
	}
	s, err := a.backend()
	if err != nil {
		return storage.Attachment{}, err
	}
	return s.Store.AddAttachment(a.ctx, chat, name, data, parsed)
}
func (a *App) Attachments(chat string) ([]storage.Attachment, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.Attachments(a.ctx, chat)
}
func (a *App) ReadAttachment(chat, id string, page int) ([]attachment.Page, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.ReadAttachment(a.ctx, chat, id, page)
}
func (a *App) RemoveAttachment(chat, id string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.RemoveAttachment(a.ctx, chat, id)
}
func (a *App) AttachmentPreview(chat, id string) (string, error) {
	s, err := a.backend()
	if err != nil {
		return "", err
	}
	data, mime, err := s.Store.AttachmentData(a.ctx, chat, id, true)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(mime, "image/") {
		return "", nil
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (a *App) ValidateDiagram(code string) (string, error) { return document.SanitizeSVG(code) }
func (a *App) AttachmentBytes(chat, id string) (string, error) {
	s, err := a.backend()
	if err != nil {
		return "", err
	}
	data, _, err := s.Store.AttachmentData(a.ctx, chat, id, false)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

type ArtifactData struct {
	storage.Artifact
	Values map[string]map[string]sheet.Display `json:"values,omitempty"`
}

func artifactData(a storage.Artifact) (ArtifactData, error) {
	result := ArtifactData{Artifact: a}
	if a.Kind == "sheet" {
		var book sheet.Book
		if err := json.Unmarshal(a.Payload, &book); err != nil {
			return result, err
		}
		values, err := sheet.Calculate(book)
		result.Values = values
		return result, err
	}
	return result, nil
}
func (a *App) ReadArtifact(id string) (ArtifactData, error) {
	s, err := a.backend()
	if err != nil {
		return ArtifactData{}, err
	}
	value, err := s.Store.ReadArtifact(a.ctx, id)
	if err != nil {
		return ArtifactData{}, err
	}
	return artifactData(value)
}
func (a *App) SaveArtifact(id string, rev int, payload json.RawMessage) (ArtifactData, error) {
	s, err := a.backend()
	if err != nil {
		return ArtifactData{}, err
	}
	current, err := s.Store.ReadArtifact(a.ctx, id)
	if err != nil {
		return ArtifactData{}, err
	}
	if current.Kind == "deck" {
		var slides deck.Deck
		if err = json.Unmarshal(payload, &slides); err != nil {
			return ArtifactData{}, err
		}
		for _, slide := range slides.Slides {
			if slide.Layout == "image" {
				_, mime, err := s.Store.AttachmentData(a.ctx, current.ChatID, slide.ImageID, true)
				if err != nil || !strings.HasPrefix(mime, "image/") {
					return ArtifactData{}, errors.New("Choose an image attached to this chat.")
				}
			}
		}
	}
	value, err := s.Store.SaveArtifact(a.ctx, id, rev, payload, "user")
	if err != nil {
		return ArtifactData{}, err
	}
	_ = s.SyncWorkspace(a.ctx, storage.Run{ChatID: value.ChatID}, id)
	return artifactData(value)
}
func (a *App) SaveArtifactDraft(id string, rev int, payload json.RawMessage) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.SaveArtifactDraft(a.ctx, id, rev, payload)
}
func (a *App) OpenArtifact(id string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.Store.OpenArtifact(a.ctx, id) })
}
func (a *App) CloseArtifact() (storage.State, error) {
	return a.change(func(s *core.Service) error {
		_, err := s.Store.DB().ExecContext(a.ctx, "UPDATE chat_view_state SET state=json_remove(state,'$.artifactId') WHERE chat_id=(SELECT chat_id FROM workspace_state WHERE id=1)")
		return err
	})
}
func (a *App) CreateSheet(chat, title string) (storage.State, error) {
	return a.change(func(s *core.Service) error {
		payload, _ := json.Marshal(sheet.Book{Tabs: []sheet.Tab{{Name: "Sheet1", Cells: map[string]sheet.Cell{}}}})
		value, err := s.Store.CreateArtifact(a.ctx, chat, "sheet", title, payload, "user")
		if err != nil {
			return err
		}
		_ = s.SyncWorkspace(a.ctx, storage.Run{ChatID: chat}, value.ID)
		return nil
	})
}
func (a *App) ExportArtifact(id, format, tab string) (bool, error) {
	s, err := a.backend()
	if err != nil {
		return false, err
	}
	record, err := s.Store.ReadArtifact(a.ctx, id)
	if err != nil {
		return false, err
	}
	if record.Draft != nil {
		return false, errors.New("Save the artifact draft before exporting.")
	}
	var data []byte
	if record.Kind == "sheet" {
		if format != "xlsx" && format != "csv" {
			return false, errors.New("Unsupported spreadsheet export.")
		}
		var book sheet.Book
		if err = json.Unmarshal(record.Payload, &book); err != nil {
			return false, err
		}
		if format == "xlsx" {
			data, err = sheet.XLSX(book)
		} else {
			data, err = sheet.CSV(book, tab)
		}
	} else if record.Kind == "deck" {
		if format != "pptx" && format != "pdf" {
			return false, errors.New("Unsupported slide export.")
		}
		if format == "pptx" {
			return false, errors.New("Export slides from the editor preview.")
		}
		var slides deck.Deck
		if err = json.Unmarshal(record.Payload, &slides); err != nil {
			return false, err
		}
		resolve := func(id string) ([]byte, error) {
			data, mime, err := s.Store.AttachmentData(a.ctx, record.ChatID, id, true)
			if err != nil || !strings.HasPrefix(mime, "image/") {
				return nil, errors.New("Slide image is unavailable.")
			}
			return data, nil
		}
		data, err = deck.PDF(a.ctx, slides, resolve)
	} else {
		return false, errors.New("Unsupported artifact.")
	}
	if err != nil {
		return false, err
	}

	name := strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`/\\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, record.Title)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{DefaultFilename: name + "." + format, Title: "Export artifact", Filters: []runtime.FileFilter{{DisplayName: strings.ToUpper(format), Pattern: "*." + format}}})
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	if err = exports.Write(path, data); err != nil {
		return false, errors.New("Couldn’t write this export.")
	}
	return true, nil
}
func (a *App) ExportDeckPptx(id, encoded string) (bool, error) {
	s, err := a.backend()
	if err != nil {
		return false, err
	}
	record, err := s.Store.ReadArtifact(a.ctx, id)
	if err != nil {
		return false, err
	}
	if record.Kind != "deck" {
		return false, errors.New("Only slide decks export this way.")
	}
	if record.Draft != nil {
		return false, errors.New("Save the artifact draft before exporting.")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < 2 || data[0] != 'P' || data[1] != 'K' {
		return false, errors.New("The slide preview didn’t produce a presentation.")
	}
	if len(data) > 64*1024*1024 {
		return false, errors.New("This presentation is too large to export.")
	}
	name := strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`/\\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, record.Title)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{DefaultFilename: name + ".pptx", Title: "Export slides", Filters: []runtime.FileFilter{{DisplayName: "PPTX", Pattern: "*.pptx"}}})
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	if err = exports.Write(path, data); err != nil {
		return false, errors.New("Couldn’t write this export.")
	}
	return true, nil
}

func (a *App) CreateDeck(chat, title string) (storage.State, error) {
	return a.change(func(s *core.Service) error {
		slides := deck.Deck{Slides: []deck.Slide{{Layout: "title", Title: title, Body: []string{}}}}
		if err := deck.Prepare(&slides); err != nil {
			return err
		}
		data, _ := json.Marshal(slides)
		value, err := s.Store.CreateArtifact(a.ctx, chat, "deck", title, data, "user")
		if err != nil {
			return err
		}
		_ = s.SyncWorkspace(a.ctx, storage.Run{ChatID: chat}, value.ID)
		return nil
	})
}

func (a *App) emitMessage(message storage.Message) {
	if a.onMessage != nil {
		a.onMessage(message)
	} else {
		runtime.EventsEmit(a.ctx, "kiko:message", message)
	}
}

func (a *App) DiscardArtifactDraft(id string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.Store.DiscardArtifactDraft(a.ctx, id)
}

func (a *App) RenderAttachmentPage(chat, id string, page int) (string, error) {
	s, err := a.backend()
	if err != nil {
		return "", err
	}
	image, err := s.AttachmentImage(a.ctx, chat, id, page)
	if err != nil {
		return "", err
	}
	return "data:" + image.MIME + ";base64," + base64.StdEncoding.EncodeToString(image.Data), nil
}

func (a *App) ChooseFolder(chat string) (storage.State, error) {
	picker := a.pickFolder
	if picker == nil {
		picker = func(ctx context.Context) (string, error) {
			return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: "Choose workspace folder"})
		}
	}
	path, err := picker(a.ctx)
	if err != nil {
		return storage.State{}, errors.New("Couldn’t open the folder picker.")
	}
	if path == "" {
		return a.Bootstrap()
	}
	return a.change(func(s *core.Service) error { return s.UseFolder(a.ctx, path, chat) })
}
func (a *App) WorkspaceFiles(chat string) ([]storage.WorkspaceFile, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.WorkspaceFiles(a.ctx, chat)
}
func (a *App) FilePermissions(chat string) ([]storage.FilePermission, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.FilePermissions(a.ctx, chat)
}
func (a *App) ResolveFilePermission(id string, approved bool) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	status := "denied"
	if approved {
		status = "approved"
	}
	return s.Store.ResolveFilePermission(a.ctx, id, status)
}
func (a *App) ReadWorkspaceFile(chat, path string) (workspace.File, error) {
	s, err := a.backend()
	if err != nil {
		return workspace.File{}, err
	}
	folder, err := s.Store.ChatFolder(a.ctx, chat)
	if err != nil {
		return workspace.File{}, err
	}
	root, err := workspace.Open(folder.Root)
	if err != nil {
		return workspace.File{}, err
	}
	defer root.Close()
	return workspace.Read(root, path)
}

func (a *App) WorkspaceIssues(chat string) ([]storage.WorkspaceIssue, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.Store.WorkspaceIssues(a.ctx, chat)
}
func (a *App) RetryWorkspaceSync(chat, source string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.SyncWorkspace(a.ctx, storage.Run{ChatID: chat}, source)
}
