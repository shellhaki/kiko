package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type ProviderConfig struct {
	ID      string `json:"id"`
	Format  string `json:"format"`
	BaseURL string `json:"baseURL"`
	Model   string `json:"model"`
}

func (s *Store) Provider(ctx context.Context, id string) (ProviderConfig, error) {
	var p ProviderConfig
	err := s.db.QueryRowContext(ctx, "SELECT id,format,base_url,model FROM providers WHERE id=?", id).Scan(&p.ID, &p.Format, &p.BaseURL, &p.Model)
	return p, err
}
func (s *Store) ChatProvider(ctx context.Context, chat string) (ProviderConfig, error) {
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return ProviderConfig{}, err
	}
	id := prefs.ProviderID
	if id == "" {
		id = "muse"
	}
	if chat != "" {
		if err = s.db.QueryRowContext(ctx, "SELECT provider_id FROM chats WHERE id=?", chat).Scan(&id); err != nil {
			return ProviderConfig{}, err
		}
	}
	return s.Provider(ctx, id)
}
func (s *Store) Providers(ctx context.Context) ([]ProviderConfig, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,format,base_url,model FROM providers ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ProviderConfig{}
	for rows.Next() {
		var p ProviderConfig
		if err = rows.Scan(&p.ID, &p.Format, &p.BaseURL, &p.Model); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (s *Store) SaveProvider(ctx context.Context, p ProviderConfig) error {
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		var before string
		err := tx.QueryRowContext(ctx, "SELECT model FROM providers WHERE id=?", p.ID).Scan(&before)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO providers(id,format,base_url,model) VALUES (?,?,?,?) ON CONFLICT(id) DO UPDATE SET format=excluded.format,base_url=excluded.base_url,model=excluded.model`, p.ID, p.Format, p.BaseURL, p.Model); err != nil {
			return err
		}
		if before != "" && before != p.Model {
			if _, err = tx.ExecContext(ctx, "INSERT INTO app_settings VALUES ('image-models','{}') ON CONFLICT(key) DO NOTHING"); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, "UPDATE app_settings SET value=json_remove(value,?) WHERE key='image-models'", "$."+p.ID)
		}
		return err
	})
}
func (s *Store) SelectProvider(ctx context.Context, id string) error {
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return err
	}
	if _, err = s.Provider(ctx, id); err != nil {
		return err
	}
	prefs.ProviderID = id
	raw, _ := json.Marshal(prefs)
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO app_settings VALUES ('preferences',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(raw)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE chats SET provider_id=? WHERE id=(SELECT chat_id FROM workspace_state WHERE id=1)", id)
		return err
	})
}
func (s *Store) Run(ctx context.Context, id string) (Run, error) {
	if id == "" {
		return Run{}, errors.New("missing run")
	}
	return scanRun(s.db.QueryRowContext(ctx, runSQL+"WHERE r.id=?", id))
}
