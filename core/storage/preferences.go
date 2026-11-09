package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

type Preferences struct {
	ImageModels      map[string]string `json:"imageModels,omitempty"`
	SuggestionMode   bool              `json:"suggestionMode"`
	ProviderID       string            `json:"providerId"`
	VisionProviderID string            `json:"visionProviderId"`
	TavilyEnabled    bool              `json:"tavilyEnabled"`
	Nickname         string            `json:"nickname"`
	Stage            string            `json:"stage"`
	Theme            string            `json:"theme"`
	SidebarOpen      bool              `json:"sidebarOpen"`
	SidebarWidth     int               `json:"sidebarWidth"`
	ResetInProgress  bool              `json:"resetInProgress"`
}

func DefaultPreferences() Preferences {
	return Preferences{ProviderID: "muse", VisionProviderID: "muse", Stage: "nickname", Theme: "dark", SidebarOpen: true, SidebarWidth: 260}
}

func (s *Store) Preferences(ctx context.Context) (Preferences, error) {
	prefs := DefaultPreferences()
	var data string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM app_settings WHERE key='preferences'").Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		data = "{}"
	} else if err != nil {
		return prefs, err
	}
	if strings.TrimSpace(data) == "null" {
		return prefs, errors.New("invalid saved preferences")
	}
	if err = json.Unmarshal([]byte(data), &prefs); err != nil {
		return prefs, err
	}
	var imageModels string
	imageErr := s.db.QueryRowContext(ctx, "SELECT value FROM app_settings WHERE key='image-models'").Scan(&imageModels)
	if imageErr == nil {
		prefs.ImageModels = nil
		if err = json.Unmarshal([]byte(imageModels), &prefs.ImageModels); err != nil {
			return prefs, err
		}
	} else if !errors.Is(imageErr, sql.ErrNoRows) {
		return prefs, imageErr
	}
	return prefs, validatePreferences(prefs)
}

func (s *Store) SavePreferences(ctx context.Context, prefs Preferences) error {
	prefs.Nickname = strings.TrimSpace(prefs.Nickname)
	if err := validatePreferences(prefs); err != nil {
		return err
	}
	data, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO app_settings VALUES ('preferences', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(data))
	return err
}

func validatePreferences(prefs Preferences) error {
	if len(prefs.ImageModels) > 2 {
		return errors.New("invalid image model preferences")
	}
	for id, model := range prefs.ImageModels {
		if id != "openai" && id != "anthropic" || len(model) == 0 || len(model) > 150 || strings.ContainsAny(model, "\r\n\x00") {
			return errors.New("invalid image model preference")
		}
	}
	if utf8.RuneCountInString(prefs.Nickname) > 40 || (prefs.Stage != "nickname" && prefs.Nickname == "") {
		return errors.New("nickname must contain 1–40 characters")
	}
	if prefs.Stage != "nickname" && prefs.Stage != "muse" && prefs.Stage != "done" {
		return errors.New("invalid onboarding stage")
	}
	if prefs.Theme != "dark" && prefs.Theme != "light" && prefs.Theme != "system" {
		return errors.New("invalid theme")
	}
	if prefs.SidebarWidth < 220 || prefs.SidebarWidth > 320 {
		return errors.New("sidebar width must be 220–320")
	}
	return nil
}

func (s *Store) SetImageModel(ctx context.Context, id, model string) error {
	if id != "openai" && id != "anthropic" || len(model) > 150 || strings.ContainsAny(model, "\r\n\x00") {
		return errors.New("invalid image permission")
	}
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return err
	}
	initial, _ := json.Marshal(prefs.ImageModels)
	if prefs.ImageModels == nil {
		initial = []byte("{}")
	}
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO app_settings VALUES ('image-models',?) ON CONFLICT(key) DO NOTHING", string(initial)); err != nil {
			return err
		}
		if model == "" {
			_, err := tx.ExecContext(ctx, "UPDATE app_settings SET value=json_remove(value,?) WHERE key='image-models'", "$."+id)
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE app_settings SET value=json_set(value,?,?) WHERE key='image-models'", "$."+id, model)
		return err
	})
}
