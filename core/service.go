package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"unicode/utf8"

	"kiko/core/provider"
	"kiko/core/storage"
)

var ErrKeyMissing = errors.New("no Muse key saved")

type Secrets interface {
	Get() (string, error)
	Set(string) error
	Delete() error
}
type ConnectionVerifier interface {
	Verify(context.Context, string, string) error
}

type Service struct {
	ProviderFactory func(storage.ProviderConfig, int) provider.Provider
	SecretFor       func(string) Secrets
	ResearchURL     string
	VisionURL       string
	Provider        provider.Provider
	runMu           sync.Mutex
	active          map[string]*activeRun
	closing         bool
	Store           *storage.Store
	Keys            Secrets
	Verifier        ConnectionVerifier
	mu              sync.Mutex
}

func (s *Service) State(ctx context.Context) (storage.State, error) {
	state, err := s.Store.Snapshot(ctx)
	if err != nil {
		return state, errors.New("Couldn’t read local data. Retry without resetting your data.")
	}
	if state.Preferences.Stage == "done" {
		secrets := s.providerSecrets(state.Provider.ID)
		var key string
		var err error
		if secrets != nil {
			key, err = secrets.Get()
		} else {
			err = ErrKeyMissing
		}
		state.Connected = state.Provider.ID == "ollama" || err == nil && key != ""
		if !state.Connected {
			if errors.Is(err, ErrKeyMissing) || err == nil {
				state.ConnectionIssue = "The saved provider key is missing. Open AI settings to reconnect."
			} else {
				state.ConnectionIssue = "Secure key storage is unavailable. Unlock your keychain and retry."
			}
		}
	}
	return state, nil
}

func (s *Service) SaveNickname(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 40 {
		return errors.New("Enter a nickname with 1–40 characters.")
	}
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.Nickname = name
	if prefs.Stage == "nickname" {
		prefs.Stage = "muse"
	}
	return s.Store.SavePreferences(ctx, prefs)
}

func (s *Service) Back(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.Stage = "nickname"
	return s.Store.SavePreferences(ctx, prefs)
}

func (s *Service) Connect(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	if prefs.Nickname == "" {
		return errors.New("Choose your nickname first.")
	}
	key = strings.TrimSpace(key)
	if err = s.Verifier.Verify(ctx, key, "muse-spark-1.3-contributor"); err != nil {
		return err
	}
	old, err := s.Keys.Get()
	if err != nil && !errors.Is(err, ErrKeyMissing) {
		return errors.New("Couldn’t open secure key storage. Unlock your keychain and retry.")
	}
	if err = s.Keys.Set(key); err != nil {
		var restore error
		if old != "" {
			restore = s.Keys.Set(old)
		} else {
			restore = s.Keys.Delete()
		}
		if restore != nil {
			return errors.New("Secure storage failed and key cleanup needs attention. Unlock your keychain and retry.")
		}
		return errors.New("Couldn’t save the key securely. Unlock your keychain and retry.")
	}
	prefs.Stage = "done"
	if err = s.Store.SavePreferences(ctx, prefs); err != nil {
		var restore error
		if old != "" {
			restore = s.Keys.Set(old)
		} else {
			restore = s.Keys.Delete()
		}
		if restore != nil {
			return errors.New("Local setup failed and secure key cleanup needs attention. Retry setup.")
		}
		return errors.New("Couldn’t finish saving setup. Your previous connection was restored.")
	}
	return nil
}

func (s *Service) Appearance(ctx context.Context, theme string, open bool, width int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.Theme = theme
	prefs.SidebarOpen = open
	prefs.SidebarWidth = width
	return s.Store.SavePreferences(ctx, prefs)
}
