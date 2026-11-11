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
func refreshOps(current document.Document, ops []document.Operation) ([]document.Operation, bool) {
	fresh := make([]document.Operation, len(ops))
	for i, op := range ops {
		if op.Target.Find == "" {
			for _, id := range op.Target.IDs {
				block := document.Find(current.Blocks, id)
				if block == nil {
					return nil, false
				}
				if want, ok := op.IfHash[id]; ok {
					if want != block.Hash {
						return nil, false
					}
				} else if block.Type != "pending" {
					return nil, false
				}
			}
		}
		op.IfRev = current.Rev
		fresh[i] = op
	}
	return fresh, true
}
func tryRebase(current document.Document, ops []document.Operation) (fresh []document.Operation, before, after string, ok bool) {
	fresh, ok = refreshOps(current, ops)
	if !ok {
		return nil, "", "", false
	}
	rebased, err := document.Apply(current, fresh)
	if err != nil {
		return nil, "", "", false
	}
	return fresh, document.Markdown(current.Blocks), document.Markdown(rebased.Blocks), true
}
func refreshSuggestions(ctx context.Context, tx *sql.Tx, current document.Document) error {
	rows, err := tx.QueryContext(ctx, "SELECT id,base_rev,ops FROM suggestions WHERE document_id=? AND status='pending'", current.ID)
	if err != nil {
		return err
	}
	type stale struct {
		id   string
		base int
		ops  string
	}
	pending := []stale{}
	for rows.Next() {
		var item stale
		if err = rows.Scan(&item.id, &item.base, &item.ops); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range pending {
		if item.base == current.Rev {
			continue
		}
		var ops []document.Operation
		if err = json.Unmarshal([]byte(item.ops), &ops); err != nil {
			return err
		}
		fresh, before, after, ok := tryRebase(current, ops)
		if !ok {
			if _, err = tx.ExecContext(ctx, "UPDATE suggestions SET status='rejected' WHERE id=?", item.id); err != nil {
				return err
			}
			continue
		}
		raw, _ := json.Marshal(fresh)
		if _, err = tx.ExecContext(ctx, "UPDATE suggestions SET base_rev=?,ops=?,before_markdown=?,after_markdown=? WHERE id=?", current.Rev, string(raw), before, after, item.id); err != nil {
			return err
		}
	}
	return nil
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
		var ops []document.Operation
		if err = json.Unmarshal([]byte(raw), &ops); err != nil {
			return err
		}
		if before.Rev != base {
			fresh, _, _, ok := tryRebase(before, ops)
			if !ok {
				if _, err = tx.ExecContext(ctx, "UPDATE suggestions SET status='rejected' WHERE id=?", id); err != nil {
					return err
				}
				return document.Refuse("stale_withdrawn", "That suggestion no longer applies, so it was withdrawn.", before, nil)
			}
			ops = fresh
			rebased, err := document.Apply(before, ops)
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(ops)
			if _, err = tx.ExecContext(ctx, "UPDATE suggestions SET base_rev=?,ops=?,before_markdown=?,after_markdown=? WHERE id=?", before.Rev, string(raw), document.Markdown(before.Blocks), document.Markdown(rebased.Blocks), id); err != nil {
				return err
			}
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
