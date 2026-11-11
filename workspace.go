package main

import (
	"kiko/core"
	"kiko/core/storage"
	"kiko/core/workspace"
)

func (a *App) ListWorkspaceDirectory(project, path string) ([]workspace.Entry, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.ListWorkspaceDirectory(a.ctx, project, path)
}
func (a *App) ReadWorkspaceEntry(project, path string) (core.WorkspaceEntry, error) {
	s, err := a.backend()
	if err != nil {
		return core.WorkspaceEntry{}, err
	}
	return s.ReadWorkspaceEntry(a.ctx, project, path)
}
func (a *App) SaveWorkspaceTextDraft(project, path, hash, content string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.SaveWorkspaceTextDraft(a.ctx, project, path, hash, content)
}
func (a *App) SaveWorkspaceText(project, chat, path, hash, content string) (core.WorkspaceEntry, error) {
	s, err := a.backend()
	if err != nil {
		return core.WorkspaceEntry{}, err
	}
	return s.SaveWorkspaceText(a.ctx, project, chat, path, hash, content)
}
func (a *App) DiscardWorkspaceTextDraft(project, path, hash string) error {
	s, err := a.backend()
	if err != nil {
		return err
	}
	return s.DiscardWorkspaceTextDraft(a.ctx, project, path, hash)
}
func (a *App) ImportWorkspaceFile(project, chat, path, hash string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.ImportWorkspaceFile(a.ctx, project, chat, path, hash) })
}
func (a *App) WorkspaceSource(id string) (*core.WorkspaceSource, error) {
	s, err := a.backend()
	if err != nil {
		return nil, err
	}
	return s.WorkspaceSource(a.ctx, id)
}
func (a *App) ReloadWorkspaceSource(id, hash string, rev int) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.ReloadWorkspaceSource(a.ctx, id, hash, rev) })
}
func (a *App) CreateArtifactAt(chat, kind, title, path string) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.CreateArtifactAt(a.ctx, chat, kind, title, path) })
}
func (a *App) SetArtifactPath(chat, id, path string) (storage.State, error) {
	return a.change(func(s *core.Service) error {
		if err := s.ReserveSourcePath(a.ctx, chat, id, path); err != nil {
			return err
		}
		_ = s.SyncWorkspace(a.ctx, storage.Run{ChatID: chat}, id)
		return nil
	})
}

func (a *App) SaveWorkspaceSourceVersion(id, hash string, rev int) (storage.State, error) {
	return a.change(func(s *core.Service) error { return s.SaveWorkspaceSourceVersion(a.ctx, id, hash, rev) })
}
