package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiko/core/document"
	"kiko/core/provider"
	"kiko/core/storage"
	"kiko/core/workspace"
)

func deleteDocRun(t *testing.T) (*Service, storage.Run) {
	t.Helper()
	s := chatService(t)
	ctx := context.Background()
	run, _, err := s.Store.StartRun(ctx, "", "", "Delete", "doc-test")
	if err != nil {
		t.Fatal(err)
	}
	return s, run
}
func waitDestructiveRequest(t *testing.T, s *Service, run storage.Run, kind string) storage.FilePermission {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for {
		requests, err := s.Store.FilePermissions(ctx, run.ChatID)
		if err != nil {
			t.Fatal(err)
		}
		for _, request := range requests {
			if request.Kind == kind {
				return request
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("destructive approval request never appeared")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func folderRun(t *testing.T) (*Service, storage.Run, string) {
	t.Helper()
	s := chatService(t)
	ctx := context.Background()
	folder := t.TempDir()
	if err := s.UseFolder(ctx, folder, ""); err != nil {
		t.Fatal(err)
	}
	state, err := s.Store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := s.Store.StartRun(ctx, state.Selection.ProjectID, "", "File request", "folder-test")
	if err != nil {
		t.Fatal(err)
	}
	return s, run, folder
}
func TestWorkspaceWriteAppliesDirectly(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	if _, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "write_file", Arguments: `{"path":"/docs/plan.md","content":"Hello","expected_hash":""}`}}); err != nil {
		t.Fatal(err)
	}
	requests, err := s.Store.FilePermissions(ctx, run.ChatID)
	if err != nil || len(requests) != 0 {
		t.Fatal("direct write created a permission request", requests, err)
	}
	files, err := s.Store.WorkspaceFiles(ctx, run.ChatID)
	if err != nil || len(files) != 1 || files[0].Path != "docs/plan.md" {
		t.Fatal(files, err)
	}
	if data, err := os.ReadFile(filepath.Join(folder, "docs/plan.md")); err != nil || string(data) != "Hello" {
		t.Fatal(string(data), err)
	}
}
func writeDeletableFile(t *testing.T, s *Service, run storage.Run) string {
	t.Helper()
	ctx := context.Background()
	if _, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "write_file", Arguments: `{"path":"/docs/old.md","content":"Bye","expected_hash":""}`}}); err != nil {
		t.Fatal(err)
	}
	files, err := s.Store.WorkspaceFiles(ctx, run.ChatID)
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	return files[0].Hash
}
func waitDeleteRequest(t *testing.T, s *Service, run storage.Run) storage.FilePermission {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for {
		requests, err := s.Store.FilePermissions(ctx, run.ChatID)
		if err != nil {
			t.Fatal(err)
		}
		for _, request := range requests {
			if request.Kind == "delete" {
				return request
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("delete approval request never appeared")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func TestWorkspaceDeleteAsksApprovalThenDeletes(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	hash := writeDeletableFile(t, s, run)
	type result struct {
		output string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_file", Arguments: `{"path":"/docs/old.md","expected_hash":"` + hash + `"}`}})
		done <- result{output, err}
	}()
	request := waitDeleteRequest(t, s, run)
	if request.Path != "docs/old.md" {
		t.Fatal(request)
	}
	if err := s.Store.ResolveFilePermission(ctx, request.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	select {
	case finished := <-done:
		if finished.err != nil {
			t.Fatal(finished.err)
		}
		if !strings.Contains(finished.output, "docs/old.md") {
			t.Fatal(finished.output)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("approved delete never finished")
	}
	if _, err := os.Stat(filepath.Join(folder, "docs/old.md")); !os.IsNotExist(err) {
		t.Fatal("deleted file still on disk")
	}
	files, err := s.Store.WorkspaceFiles(ctx, run.ChatID)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
}
func TestWorkspaceDeleteDeniedKeepsFile(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	hash := writeDeletableFile(t, s, run)
	done := make(chan error, 1)
	go func() {
		_, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_file", Arguments: `{"path":"/docs/old.md","expected_hash":"` + hash + `"}`}})
		done <- err
	}()
	request := waitDeleteRequest(t, s, run)
	if err := s.Store.ResolveFilePermission(ctx, request.ID, "denied"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "declined") {
			t.Fatal("expected decline error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("denied delete never finished")
	}
	if data, err := os.ReadFile(filepath.Join(folder, "docs/old.md")); err != nil || string(data) != "Bye" {
		t.Fatal(string(data), err)
	}
}
func TestWorkspaceDeleteRefusesStaleHash(t *testing.T) {
	s, run, _ := folderRun(t)
	ctx := context.Background()
	writeDeletableFile(t, s, run)
	if _, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_file", Arguments: `{"path":"/docs/old.md","expected_hash":"stale"}`}}); err == nil || !strings.Contains(err.Error(), "Read it again") {
		t.Fatal("expected stale hash refusal", err)
	}
	requests, err := s.Store.FilePermissions(ctx, run.ChatID)
	if err != nil || len(requests) != 0 {
		t.Fatal("refused delete created a permission request", requests, err)
	}
}
func TestWorkspaceDeleteRefusesMissingAndTraversal(t *testing.T) {
	s, run, _ := folderRun(t)
	ctx := context.Background()
	if _, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_file", Arguments: `{"path":"/docs/gone.md","expected_hash":"abc"}`}}); err == nil || !strings.Contains(err.Error(), "removed") {
		t.Fatal("expected missing file refusal", err)
	}
	if _, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_file", Arguments: `{"path":"/docs/gone.md","expected_hash":""}`}}); err == nil || !strings.Contains(err.Error(), "already gone") {
		t.Fatal("expected already-gone refusal", err)
	}
	if _, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_file", Arguments: `{"path":"/../evil.md","expected_hash":"abc"}`}}); err == nil {
		t.Fatal("expected traversal refusal")
	}
	requests, err := s.Store.FilePermissions(ctx, run.ChatID)
	if err != nil || len(requests) != 0 {
		t.Fatal("refused delete created a permission request", requests, err)
	}
}
func TestWorkspaceDeleteRefusesArtifactFile(t *testing.T) {
	s, run, _ := folderRun(t)
	ctx := context.Background()
	doc, err := s.Store.CreateDocument(ctx, run.ChatID, "Plan", "# Plan\n\nHello", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SyncWorkspace(ctx, run, doc.ID); err != nil {
		t.Fatal(err)
	}
	files, err := s.Store.WorkspaceFiles(ctx, run.ChatID)
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	call := provider.ToolCall{Function: provider.Function{Name: "delete_file", Arguments: `{"path":"/` + files[0].Path + `","expected_hash":"` + files[0].Hash + `"}`}}
	if _, err = s.executeTool(ctx, run, call); err == nil || !strings.Contains(err.Error(), "artifact") {
		t.Fatal("expected artifact refusal", err)
	}
	requests, err := s.Store.FilePermissions(ctx, run.ChatID)
	if err != nil || len(requests) != 0 {
		t.Fatal("refused delete created a permission request", requests, err)
	}
}
func TestAgentStreamsFileWritesInOrder(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	big := strings.Repeat("A", 6000)
	var snapshots [][]storage.Activity
	changed := func() error {
		steps, err := s.Store.Activities(ctx, run.AssistantID)
		if err != nil {
			return err
		}
		snapshots = append(snapshots, append([]storage.Activity{}, steps...))
		return nil
	}
	client := replyProvider(func(_ context.Context, r provider.Request, emit func(provider.Event) error) error {
		if len(r.Messages) > 0 && r.Messages[len(r.Messages)-1].Role == "tool" {
			emit(provider.Event{Text: "Files ready."})
			return emit(provider.Event{Finish: "stop"})
		}
		emit(provider.Event{Tool: &provider.ToolDelta{Index: 0, ID: "w0", Name: "write_file", Arguments: `{"path":"/docs/a.md",`}})
		emit(provider.Event{Tool: &provider.ToolDelta{Index: 0, Arguments: `"content":"` + big}})
		emit(provider.Event{Tool: &provider.ToolDelta{Index: 0, Arguments: `","expected_hash":""}`}})
		emit(provider.Event{Tool: &provider.ToolDelta{Index: 1, ID: "w1", Name: "write_file", Arguments: `{"path":"/docs/b.md","content":"Hi B","expected_hash":""}`}})
		return emit(provider.Event{Finish: "tool_calls"})
	})
	if err := s.agentStream(ctx, client, run, provider.Request{Model: "fixture"}, func(provider.Event) error { return nil }, changed); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(folder, "docs/a.md")); err != nil || string(data) != big {
		t.Fatal("first file wrong", len(data), err)
	}
	if data, err := os.ReadFile(filepath.Join(folder, "docs/b.md")); err != nil || string(data) != "Hi B" {
		t.Fatal("second file wrong", string(data), err)
	}
	steps, err := s.Store.Activities(ctx, run.AssistantID)
	if err != nil || len(steps) != 2 || steps[0].Label != "Wrote docs/a.md" || steps[0].Status != "ok" || steps[1].Label != "Wrote docs/b.md" || steps[1].Status != "ok" {
		t.Fatal("files did not finish in order", steps, err)
	}
	sawInitial, sawProgress := false, false
	for _, snapshot := range snapshots {
		for _, step := range snapshot {
			if step.Tool != "write_file" || step.Status != "running" {
				continue
			}
			if step.Label == "Writing docs/a.md" {
				sawInitial = true
			}
			if strings.HasPrefix(step.Label, "Writing docs/a.md") && strings.Contains(step.Label, "KB") {
				sawProgress = true
			}
		}
	}
	if !sawInitial || !sawProgress {
		t.Fatal("no live write progress observed", snapshots)
	}
}
func TestDocumentDeleteAsksApprovalThenDeletes(t *testing.T) {
	s, run := deleteDocRun(t)
	ctx := context.Background()
	doc, err := s.Store.CreateDocument(ctx, run.ChatID, "Old Plan", "# Old Plan\n\nBye", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		output string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_document", Arguments: `{"doc_id":"` + doc.ID + `"}`}})
		done <- result{output, err}
	}()
	request := waitDestructiveRequest(t, s, run, "document")
	if request.Path != "Old Plan" {
		t.Fatal(request)
	}
	if err := s.Store.ResolveFilePermission(ctx, request.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	select {
	case finished := <-done:
		if finished.err != nil {
			t.Fatal(finished.err)
		}
		if !strings.Contains(finished.output, "Old Plan") {
			t.Fatal(finished.output)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("approved document delete never finished")
	}
	if _, err = s.Store.ReadDocument(ctx, doc.ID, storage.ReadOptions{}); err == nil {
		t.Fatal("deleted document still readable")
	}
	var view sql.NullString
	if err = s.Store.DB().QueryRowContext(ctx, "SELECT document_id FROM chat_view_state WHERE chat_id=?", run.ChatID).Scan(&view); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if view.Valid {
		t.Fatal("deleted document still selected")
	}
}
func TestDocumentDeleteDeniedKeepsDocument(t *testing.T) {
	s, run := deleteDocRun(t)
	ctx := context.Background()
	doc, err := s.Store.CreateDocument(ctx, run.ChatID, "Keep Me", "# Keep Me\n\nHi", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_document", Arguments: `{"doc_id":"` + doc.ID + `"}`}})
		done <- err
	}()
	request := waitDestructiveRequest(t, s, run, "document")
	if err := s.Store.ResolveFilePermission(ctx, request.ID, "denied"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "declined") {
			t.Fatal("expected decline error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("denied document delete never finished")
	}
	if _, err = s.Store.ReadDocument(ctx, doc.ID, storage.ReadOptions{}); err != nil {
		t.Fatal("denied delete removed the document", err)
	}
}
func TestDocumentDeleteRefusesForeignAndMissing(t *testing.T) {
	s, run := deleteDocRun(t)
	ctx := context.Background()
	doc, err := s.Store.CreateDocument(ctx, run.ChatID, "Mine", "# Mine", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_document", Arguments: `{"doc_id":"missing"}`}}); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatal("expected missing refusal", err)
	}
	other, _, err := s.Store.StartRun(ctx, "", "", "Delete", "doc-test-other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.executeTool(ctx, other, provider.ToolCall{Function: provider.Function{Name: "delete_document", Arguments: `{"doc_id":"` + doc.ID + `"}`}}); err == nil || !strings.Contains(err.Error(), "another chat") {
		t.Fatal("expected foreign refusal", err)
	}
	requests, err := s.Store.FilePermissions(ctx, run.ChatID)
	if err != nil || len(requests) != 0 {
		t.Fatal("refused delete created a permission request", requests, err)
	}
}
func TestArtifactDeleteAsksApprovalThenDeletes(t *testing.T) {
	s, run := deleteDocRun(t)
	ctx := context.Background()
	output, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "create_sheet", Arguments: `{"name":"Budget","tabs":[{"name":"T1","cells":{}}]}`}})
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal([]byte(output), &created); err != nil || created.ID == "" {
		t.Fatal(output, err)
	}
	type result struct {
		output string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "delete_artifact", Arguments: `{"artifact_id":"` + created.ID + `"}`}})
		done <- result{output, err}
	}()
	request := waitDestructiveRequest(t, s, run, "sheet")
	if request.Path != "Budget" {
		t.Fatal(request)
	}
	if err := s.Store.ResolveFilePermission(ctx, request.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	select {
	case finished := <-done:
		if finished.err != nil {
			t.Fatal(finished.err)
		}
		if !strings.Contains(finished.output, "Budget") {
			t.Fatal(finished.output)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("approved artifact delete never finished")
	}
	if _, err = s.Store.ReadArtifact(ctx, created.ID); err == nil {
		t.Fatal("deleted artifact still readable")
	}
}
func TestWorkspaceSyncProtectsExternalEdits(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	doc, err := s.Store.CreateDocument(ctx, run.ChatID, "Plan", "# Plan\n\nHello", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SyncWorkspace(ctx, run, doc.ID); err != nil {
		t.Fatal(err)
	}
	files, err := s.Store.WorkspaceFiles(ctx, run.ChatID)
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	if _, err = os.Stat(filepath.Join(folder, files[0].Path)); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(folder, files[0].Path), []byte("external edit"), 0600); err != nil {
		t.Fatal(err)
	}
	read, err := s.Store.ReadDocument(ctx, doc.ID, storage.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	blocks := read.Document.Blocks
	blocks[0].Content = []document.Inline{{Text: "Changed"}}
	if _, err = s.Store.SaveDocument(ctx, doc.ID, doc.Rev, blocks); err != nil {
		t.Fatal(err)
	}
	if err = s.SyncWorkspace(ctx, run, doc.ID); err == nil {
		t.Fatal("overwrote external edit")
	}
	if data, err := os.ReadFile(filepath.Join(folder, files[0].Path)); err != nil || string(data) != "external edit" {
		t.Fatal(string(data), err)
	}
	if err = s.UseFolder(ctx, t.TempDir(), run.ChatID); err == nil {
		t.Fatal("moved a chat with files")
	}
}

func TestAgentUsesFileToolsOnlyInSelectedFolder(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprint(selected), func(t *testing.T) {
			s := chatService(t)
			ctx := context.Background()
			folder := t.TempDir()
			project, chat := "", ""
			if selected {
				if err := s.UseFolder(ctx, folder, ""); err != nil {
					t.Fatal(err)
				}
				state, _ := s.Store.Snapshot(ctx)
				project = state.Selection.ProjectID
				if err := s.Store.CreateChat(ctx, project); err != nil {
					t.Fatal(err)
				}
				state, _ = s.Store.Snapshot(ctx)
				chat = state.Selection.ChatID
			}
			calls := 0
			s.Provider = replyProvider(func(_ context.Context, request provider.Request, emit func(provider.Event) error) error {
				calls++
				hasFileTools := false
				for _, tool := range request.Tools {
					if tool.Function.Name == "write_file" {
						hasFileTools = true
					}
				}
				if hasFileTools != selected {
					t.Error("incorrect file tool availability")
				}
				if selected && calls == 1 {
					_ = emit(provider.Event{Tool: &provider.ToolDelta{ID: "write-plan", Name: "write_file", Arguments: `{"path":"/docs/plan.md","content":"A real file","expected_hash":""}`}})
					return emit(provider.Event{Finish: "tool_calls"})
				}
				if selected && calls == 2 {
					result := request.Messages[len(request.Messages)-1]
					var file workspace.File
					if result.Role != "tool" || json.Unmarshal([]byte(result.Content), &file) != nil || file.Path != "docs/plan.md" {
						t.Error("file result missing")
					}
				}
				_ = emit(provider.Event{Text: "Done"})
				return emit(provider.Event{Finish: "stop"})
			})
			events := make(chan storage.Message, 30)
			if _, err := s.Send(ctx, project, chat, "Create the plan", "agent-folder-test", func(message storage.Message) { events <- message }); err != nil {
				t.Fatal(err)
			}
			if final := waitReply(t, events); final.Status != "complete" {
				t.Fatal(final)
			}
			if selected {
				if data, err := os.ReadFile(filepath.Join(folder, "docs/plan.md")); err != nil || string(data) != "A real file" {
					t.Fatal(string(data), err)
				}
			}
		})
	}
}

func TestCreatedArtifactToolsSaveSourceCopies(t *testing.T) {
	s, run, folder := folderRun(t)
	ctx := context.Background()
	for _, test := range []struct{ name, args, suffix string }{
		{"create_document", `{"name":"Plan","markdown":"# Plan"}`, ".md"},
		{"create_sheet", `{"name":"Budget","tabs":[{"name":"Costs","cells":{"A1":{"value":"Build"}}}]}`, ".sheet.json"},
		{"create_deck", `{"name":"Launch","slides":[{"layout":"title","title":"Launch","body":[]}]}`, ".deck.json"},
	} {
		if result, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: test.name, Arguments: test.args}}); err != nil || strings.Contains(result, "workspace_error") {
			t.Fatal(result, err)
		}
		files, err := s.Store.WorkspaceFiles(ctx, run.ChatID)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, file := range files {
			if strings.HasSuffix(file.Path, test.suffix) {
				data, err := os.ReadFile(filepath.Join(folder, file.Path))
				if err != nil || len(data) == 0 || file.SourceID == "" {
					t.Fatal(file, err)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("missing source copy", test.name, files)
		}
	}
	files, _ := s.Store.WorkspaceFiles(ctx, run.ChatID)
	for _, file := range files {
		raw, _ := json.Marshal(map[string]string{"path": "./" + file.Path, "content": "clobbered", "expected_hash": file.Hash})
		if _, err := s.executeTool(ctx, run, provider.ToolCall{Function: provider.Function{Name: "write_file", Arguments: string(raw)}}); err == nil {
			t.Fatal("generic file tool overwrote editor source")
		}
	}
}
