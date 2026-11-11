package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"kiko/core"
	"kiko/core/deck"
	"kiko/core/document"
	exports "kiko/core/export"
	"kiko/core/sheet"
	"kiko/core/storage"
	"kiko/core/vault"
	"kiko/core/webserver"
)

type vaultSecrets struct{ account *vault.Account }

func (v vaultSecrets) Get() (string, error) {
	value, err := v.account.Get()
	if errors.Is(err, vault.ErrMissing) {
		return "", core.ErrKeyMissing
	}
	return value, err
}
func (v vaultSecrets) Set(value string) error { return v.account.Set(value) }
func (v vaultSecrets) Delete() error          { return v.account.Delete() }
func webMethods(a *App) map[string]any {
	return map[string]any{
		"ListWorkspaceDirectory":     a.ListWorkspaceDirectory,
		"ReadWorkspaceEntry":         a.ReadWorkspaceEntry,
		"SaveWorkspaceTextDraft":     a.SaveWorkspaceTextDraft,
		"SaveWorkspaceText":          a.SaveWorkspaceText,
		"DiscardWorkspaceTextDraft":  a.DiscardWorkspaceTextDraft,
		"ImportWorkspaceFile":        a.ImportWorkspaceFile,
		"SaveWorkspaceSourceVersion": a.SaveWorkspaceSourceVersion,
		"WorkspaceSource":            a.WorkspaceSource,
		"ReloadWorkspaceSource":      a.ReloadWorkspaceSource,
		"CreateArtifactAt":           a.CreateArtifactAt,
		"SetArtifactPath":            a.SetArtifactPath,
		"WorkspaceIssues":            a.WorkspaceIssues,
		"RetryWorkspaceSync":         a.RetryWorkspaceSync,
		"Bootstrap":                  a.Bootstrap,
		"WorkspaceFiles":             a.WorkspaceFiles,
		"FilePermissions":            a.FilePermissions,
		"ResolveFilePermission":      a.ResolveFilePermission,
		"ReadWorkspaceFile":          a.ReadWorkspaceFile,
		"SaveNickname":               a.SaveNickname,
		"Back":                       a.Back,
		"ConnectMuse":                a.ConnectMuse,
		"SaveAppearance":             a.SaveAppearance,
		"CreateProject":              a.CreateProject,
		"CreateChat":                 a.CreateChat,
		"Select":                     a.Select,
		"Rename":                     a.Rename,
		"Delete":                     a.Delete,
		"Search":                     a.Search,
		"OpenDocument":               a.OpenDocument,
		"CloseDocument":              a.CloseDocument,
		"SendMessage":                a.SendMessage,
		"RetryMessage":               a.RetryMessage,
		"StopRun":                    a.StopRun,
		"Conversation":               a.Conversation,
		"SaveDraft":                  a.SaveDraft,
		"ReconnectMuse":              a.ReconnectMuse,
		"CreateDocument":             a.CreateDocument,
		"ReadDocument":               a.ReadDocument,
		"SaveDocument":               a.SaveDocument,
		"SaveDocumentDraft":          a.SaveDocumentDraft,
		"DiscardDocumentDraft":       a.DiscardDocumentDraft,
		"SaveDocumentView":           a.SaveDocumentView,
		"DocumentHistory":            a.DocumentHistory,
		"RestoreDocument":            a.RestoreDocument,
		"Questions":                  a.Questions,
		"ResolveQuestion":            a.ResolveQuestion,
		"AnswerQuestion":             a.AnswerQuestion,
		"RemoveMuseKey":              a.RemoveMuseKey,
		"ResetData":                  a.ResetData,
		"SetModel":                   a.SetModel,
		"ExportFormats":              a.ExportFormats,
		"ConfigureResearch":          a.ConfigureResearch,
		"RemoveResearch":             a.RemoveResearch,
		"AttachFile":                 a.AttachFile,
		"Attachments":                a.Attachments,
		"ReadAttachment":             a.ReadAttachment,
		"RemoveAttachment":           a.RemoveAttachment,
		"AttachmentPreview":          a.AttachmentPreview,
		"AttachmentBytes":            a.AttachmentBytes,
		"ValidateDiagram":            a.ValidateDiagram,
		"ReadArtifact":               a.ReadArtifact,
		"SaveArtifact":               a.SaveArtifact,
		"DiscardArtifactDraft":       a.DiscardArtifactDraft,
		"SaveArtifactDraft":          a.SaveArtifactDraft,
		"OpenArtifact":               a.OpenArtifact,
		"CloseArtifact":              a.CloseArtifact,
		"CreateSheet":                a.CreateSheet,
		"CreateDeck":                 a.CreateDeck,
		"RenderAttachmentPage":       a.RenderAttachmentPage,
	}
}
func webDownload(ctx context.Context, a *App, kind, id, format, tab string) (webserver.Download, error) {
	s, err := a.backend()
	if err != nil {
		return webserver.Download{}, err
	}
	result := webserver.Download{}
	if kind == "document" {
		read, err := s.Store.ReadDocument(ctx, id, storage.ReadOptions{})
		if err != nil {
			return result, err
		}
		if read.Draft != nil {
			return result, errors.New("Save the draft before exporting.")
		}
		result.Name = read.Document.Title + "." + format
		switch format {
		case "md":
			result.MIME = "text/markdown; charset=utf-8"
			result.Data = []byte(document.Markdown(read.Document.Blocks) + "\n")
		case "docx":
			result.MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
			result.Data, err = exports.Docx(read.Document)
		case "pdf":
			result.MIME = "application/pdf"
			result.Data, err = exports.PDF(ctx, read.Document)
		default:
			return result, errors.New("Unsupported format.")
		}
		return result, err
	}
	if kind != "artifact" {
		return result, errors.New("Unknown export kind.")
	}
	record, err := s.Store.ReadArtifact(ctx, id)
	if err != nil {
		return result, err
	}
	if record.Draft != nil {
		return result, errors.New("Save the draft before exporting.")
	}
	result.Name = record.Title + "." + format
	if record.Kind == "sheet" {
		var book sheet.Book
		if err = json.Unmarshal(record.Payload, &book); err != nil {
			return result, err
		}
		switch format {
		case "xlsx":
			result.MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			result.Data, err = sheet.XLSX(book)
		case "csv":
			result.MIME = "text/csv; charset=utf-8"
			result.Data, err = sheet.CSV(book, tab)
		default:
			return result, errors.New("Unsupported sheet export.")
		}
	} else {
		var slides deck.Deck
		if err = json.Unmarshal(record.Payload, &slides); err != nil {
			return result, err
		}
		resolve := func(imageID string) ([]byte, error) {
			data, mime, err := s.Store.AttachmentData(ctx, record.ChatID, imageID, true)
			if err != nil || !strings.HasPrefix(mime, "image/") {
				return nil, errors.New("Image is unavailable.")
			}
			return data, nil
		}
		switch format {
		case "pptx":
			return result, errors.New("Export slides from the editor preview.")
		case "pdf":
			result.MIME = "application/pdf"
			result.Data, err = deck.PDF(ctx, slides, resolve)
		default:
			return result, errors.New("Unsupported deck export.")
		}
	}
	return result, err
}
func serveWeb(assets fs.FS) error {
	address := os.Getenv("KIKO_WEB_ADDRESS")
	if address == "" {
		address = "127.0.0.1:34116"
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("KIKO_WEB_ADDRESS must be host:port")
	}
	loopback := host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
	if !loopback && os.Getenv("KIKO_WEB_ALLOW_REMOTE") != "1" {
		return errors.New("remote binding requires explicit KIKO_WEB_ALLOW_REMOTE=1 and a TLS reverse proxy")
	}
	token := os.Getenv("KIKO_WEB_TOKEN")
	rawKey := os.Getenv("KIKO_WEB_MASTER_KEY")
	master, err := base64.StdEncoding.DecodeString(rawKey)
	if err != nil || len(master) != 32 {
		master, err = hex.DecodeString(rawKey)
	}
	if err != nil || len(master) != 32 {
		return errors.New("set KIKO_WEB_MASTER_KEY to a 32-byte base64 or hex key; no plaintext fallback is available")
	}
	dir := os.Getenv("KIKO_WEB_DATA_DIR")
	if dir == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(config, "Kiko-Web")
	}
	secrets, err := vault.Open(filepath.Join(dir, "secrets.json"), master)
	if err != nil {
		return err
	}
	frontend, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app := &App{ctx: ctx, dataPath: filepath.Join(dir, "kiko.db"), secretFor: func(id string) core.Secrets { return vaultSecrets{account: secrets.Account(id)} }}
	server, err := webserver.New(webserver.Options{Token: token, Origin: os.Getenv("KIKO_WEB_ORIGIN"), Secure: os.Getenv("KIKO_WEB_SECURE") == "1", Assets: frontend, Methods: webMethods(app), Export: func(ctx context.Context, kind, id, format, tab string) (webserver.Download, error) {
		return webDownload(ctx, app, kind, id, format, tab)
	}})
	if err != nil {
		return err
	}
	if !loopback && (os.Getenv("KIKO_WEB_ORIGIN") == "" || os.Getenv("KIKO_WEB_SECURE") != "1") {
		return errors.New("remote web mode requires KIKO_WEB_ORIGIN and KIKO_WEB_SECURE=1 behind HTTPS")
	}
	app.onMessage = func(message storage.Message) { server.Publish(message) }
	httpServer := &http.Server{Addr: address, Handler: server, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = httpServer.Shutdown(shutdown)
		_ = httpServer.Close()
	}()
	defer app.shutdown(context.Background())
	fmt.Printf("Kiko personal web workspace: http://%s\n", address)
	if err = httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	cancel()
	<-stopped
	return nil
}
