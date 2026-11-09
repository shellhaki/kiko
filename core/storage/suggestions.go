package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"kiko/core/document"
)

type Suggestion struct {
	ID         string `json:"id"`
	DocumentID string `json:"documentId"`
	BaseRev    int    `json:"baseRev"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Status     string `json:"status"`
}

func (s *Store) ProposeDocument(ctx context.Context, id string, ops []document.Operation) (Suggestion, error) {
	var result Suggestion
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		before, err := loadDocument(ctx, tx, id, 0)
		if err != nil {
			return err
		}
		after, err := document.Apply(before, ops)
		if err != nil {
			return err
		}
		result = Suggestion{ID: rand.Text(), DocumentID: id, BaseRev: before.Rev, Before: document.Markdown(before.Blocks), After: document.Markdown(after.Blocks), Status: "pending"}
		raw, _ := json.Marshal(ops)
		var pending int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM suggestions WHERE document_id=? AND status='pending'", id).Scan(&pending); err != nil {
			return err
		}
		if pending >= 50 {
			return errors.New("review existing suggestions before creating more")
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO suggestions VALUES (?,?,?,?,?,?,'pending',?)", result.ID, id, before.Rev, string(raw), result.Before, result.After, time.Now().UnixMilli())
		return err
	})
	return result, err
}
func suggestions(ctx context.Context, db queryRows, id string) ([]Suggestion, error) {
	rows, err := db.QueryContext(ctx, "SELECT id,document_id,base_rev,before_markdown,after_markdown,status FROM suggestions WHERE document_id=? AND status='pending' ORDER BY created_at,id", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Suggestion{}
	for rows.Next() {
		var item Suggestion
		if err = rows.Scan(&item.ID, &item.DocumentID, &item.BaseRev, &item.Before, &item.After, &item.Status); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *Store) AcceptSuggestion(ctx context.Context, id string) (document.Document, error) {
	var result document.Document
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		var documentID, raw string
		var base int
		if err := tx.QueryRowContext(ctx, "SELECT document_id,base_rev,ops FROM suggestions WHERE id=? AND status='pending'", id).Scan(&documentID, &base, &raw); err != nil {
			return err
		}
		before, err := loadDocument(ctx, tx, documentID, 0)
		if err != nil {
			return err
		}
		if before.Rev != base {
			return document.Refuse("guard_mismatch", "This suggestion is based on an older revision. Keep your edits and ask for a fresh suggestion.", before, nil)
		}
		var ops []document.Operation
		if err = json.Unmarshal([]byte(raw), &ops); err != nil {
			return err
		}
		after, err := document.Apply(before, ops)
		if err != nil {
			return err
		}
		if err = protectAnchors(ctx, tx, before, after, ops); err != nil {
			return err
		}
		if err = writeDocument(ctx, tx, before, after, "user"); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE suggestions SET status='accepted' WHERE id=?", id); err != nil {
			return err
		}
		after.Rev++
		result = after
		return nil
	})
	return result, err
}
func (s *Store) RejectSuggestion(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE suggestions SET status='rejected' WHERE id=? AND status='pending'", id)
	return err
}
