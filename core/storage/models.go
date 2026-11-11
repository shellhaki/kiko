package storage

import (
	"context"
	"database/sql"
)

func (s *Store) DefaultModel(ctx context.Context) (string, error) {
	var model string
	err := s.db.QueryRowContext(ctx, "SELECT model FROM providers WHERE id='muse'").Scan(&model)
	return model, err
}

func (s *Store) ChatModel(ctx context.Context, chat string) (string, error) {
	if chat == "" {
		return s.DefaultModel(ctx)
	}
	var model string
	err := s.db.QueryRowContext(ctx, "SELECT model FROM chat_models WHERE chat_id=?", chat).Scan(&model)
	return model, err
}

func (s *Store) SetModel(ctx context.Context, model string) error {
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE chat_models SET model=? WHERE chat_id=(SELECT chat_id FROM workspace_state WHERE id=1)", model); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE providers SET model=? WHERE id='muse'", model)
		return err
	})
}
