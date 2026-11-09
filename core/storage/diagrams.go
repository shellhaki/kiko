package storage

import (
	"context"
	"crypto/rand"
	"database/sql"

	"kiko/core/document"
)

func (s *Store) CreateDiagram(ctx context.Context, id, code, caption, after string) (document.Document, error) {
	sanitized, err := document.SanitizeSVG(code)
	if err != nil {
		return document.Document{}, err
	}
	var result document.Document
	err = s.Transaction(ctx, func(tx *sql.Tx) error {
		before, err := loadDocument(ctx, tx, id, 0)
		if err != nil {
			return err
		}
		next := document.Clone(before)
		index := len(next.Blocks)
		if after != "" {
			index = -1
			for i, b := range next.Blocks {
				if b.ID == after {
					index = i + 1
					break
				}
			}
			if index < 0 {
				return document.Refuse("block_gone", "The diagram destination no longer exists.", before, nil)
			}
		}
		block := document.Block{ID: rand.Text() + ".1", Type: "embed", Attrs: map[string]any{"ref": rand.Text(), "svg": sanitized, "caption": caption}}
		next.Blocks = append(append(append([]document.Block{}, next.Blocks[:index]...), block), next.Blocks[index:]...)
		if err = writeDocument(ctx, tx, before, next, "agent"); err != nil {
			return err
		}
		next.Rev++
		result = next
		return nil
	})
	return result, err
}

func (s *Store) Diagram(ctx context.Context, chat, nodeID string) (string, error) {
	var code string
	err := s.db.QueryRowContext(ctx, "SELECT n.code FROM nodes n JOIN documents d ON d.id=n.document_id WHERE n.id=? AND d.chat_id=? AND n.kind='svg'", nodeID, chat).Scan(&code)
	return code, err
}
