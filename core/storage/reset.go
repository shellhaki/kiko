package storage

import (
	"context"
	"database/sql"
)

func (s *Store) ResetData(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
		return err
	}
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		for _, query := range []string{"DELETE FROM projects", "DELETE FROM chats", "DELETE FROM drafts", "DELETE FROM app_settings", "DELETE FROM providers WHERE id!='muse'", "UPDATE providers SET model='muse-spark-1.3-contributor' WHERE id='muse'", "UPDATE workspace_state SET project_id=NULL,chat_id=NULL WHERE id=1"} {
			if _, err := tx.ExecContext(ctx, query); err != nil {
				return err
			}
		}
		return nil
	})
}
