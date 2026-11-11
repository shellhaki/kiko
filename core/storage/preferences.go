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
	SuggestionMode  bool   `json:"suggestionMode"`
	TavilyEnabled   bool   `json:"tavilyEnabled"`
	Nickname        string `json:"nickname"`
	Stage           string `json:"stage"`
	Theme           string `json:"theme"`
	SidebarOpen     bool   `json:"sidebarOpen"`
	SidebarWidth    int    `json:"sidebarWidth"`
	ResetInProgress bool   `json:"resetInProgress"`
}

func DefaultPreferences() Preferences {
	return Preferences{Stage: "nickname", Theme: "dark", SidebarOpen: true, SidebarWidth: 260}
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
