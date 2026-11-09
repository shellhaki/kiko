package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"kiko/core/document"
)

type Question struct {
	ID         string   `json:"id"`
	DocumentID string   `json:"documentId"`
	BlockID    string   `json:"blockId"`
	Start      int      `json:"start"`
	End        int      `json:"end"`
	Quote      string   `json:"quote"`
	Body       string   `json:"body"`
	Suggested  []string `json:"suggested"`
	Resolved   bool     `json:"resolved"`
}

func (s *Store) AddQuestion(ctx context.Context, id, anchor, body string, suggested []string) (Question, error) {
	var question Question
	if strings.TrimSpace(body) == "" || len(body) > 4000 || strings.TrimSpace(anchor) == "" || len(anchor) > 4000 || len(suggested) > 5 {
		return question, errors.New("invalid question or anchor")
	}
	for _, reply := range suggested {
		if len(reply) > 500 {
			return question, errors.New("suggested reply too long")
		}
	}
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		doc, err := loadDocument(ctx, tx, id, 0)
		if err != nil {
			return err
		}
		matches := []Question{}
		for _, b := range document.Flatten(doc.Blocks) {
			if len(b.Content) == 0 {
				continue
			}
			value := document.Plain(b)
			offset := 0
			for {
				position := strings.Index(value[offset:], anchor)
				if position < 0 {
					break
				}
				start := utf8.RuneCountInString(value[:offset+position])
				matches = append(matches, Question{ID: rand.Text(), DocumentID: id, BlockID: b.ID, Start: start, End: start + utf8.RuneCountInString(anchor), Quote: anchor, Body: body, Suggested: suggested})
				offset += position + len(anchor)
			}
		}
		if len(matches) != 1 {
			return document.Refuse("anchor_ambiguous", "Quote one unique passage from the current document.", doc, nil)
		}
		question = matches[0]
		replies, _ := json.Marshal(suggested)
		_, err = tx.ExecContext(ctx, `INSERT INTO comments(id,document_id,anchor_block,anchor_start,anchor_end,body,suggested_replies,author,created_at,anchor_text) VALUES (?,?,?,?,?,?,?,'agent',?,?)`, question.ID, id, question.BlockID, question.Start, question.End, body, string(replies), time.Now().UnixMilli(), anchor)
		return err
	})
	return question, err
}
func (s *Store) Questions(ctx context.Context, id string) ([]Question, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,document_id,COALESCE(anchor_block,''),COALESCE(anchor_start,0),COALESCE(anchor_end,0),anchor_text,body,COALESCE(suggested_replies,'[]'),resolved FROM comments WHERE document_id=? AND parent_id IS NULL ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	questions := []Question{}
	for rows.Next() {
		var q Question
		var replies string
		if err = rows.Scan(&q.ID, &q.DocumentID, &q.BlockID, &q.Start, &q.End, &q.Quote, &q.Body, &replies, &q.Resolved); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(replies), &q.Suggested); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}
func (s *Store) ResolveQuestion(ctx context.Context, id string, resolved bool) error {
	_, err := s.db.ExecContext(ctx, "UPDATE comments SET resolved=? WHERE id=?", resolved, id)
	return err
}
func (s *Store) QuestionContext(ctx context.Context, id string) (string, string, string, error) {
	var chat, quote, body string
	err := s.db.QueryRowContext(ctx, "SELECT d.chat_id,c.anchor_text,c.body FROM comments c JOIN documents d ON d.id=c.document_id WHERE c.id=? AND c.parent_id IS NULL", id).Scan(&chat, &quote, &body)
	return chat, quote, body, err
}
func reconcileAnchors(ctx context.Context, tx *sql.Tx, before, after document.Document, protect bool) error {
	rows, err := tx.QueryContext(ctx, "SELECT id,anchor_block,anchor_start,anchor_end FROM comments WHERE document_id=? AND resolved=0 AND anchor_block IS NOT NULL", before.ID)
	if err != nil {
		return err
	}
	type anchor struct {
		id, block  string
		start, end int
	}
	anchors := []anchor{}
	for rows.Next() {
		var a anchor
		if err = rows.Scan(&a.id, &a.block, &a.start, &a.end); err != nil {
			rows.Close()
			return err
		}
		anchors = append(anchors, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range anchors {
		previous := document.Find(before.Blocks, a.block)
		next := document.Find(after.Blocks, a.block)
		if previous == nil {
			continue
		}
		old := []rune(document.Plain(*previous))
		if a.start < 0 || a.end > len(old) || a.end <= a.start {
			continue
		}
		quote := string(old[a.start:a.end])
		newStart := -1
		if next != nil {
			value := document.Plain(*next)
			index := strings.Index(value, quote)
			if index >= 0 && strings.Count(value, quote) == 1 {
				newStart = utf8.RuneCountInString(value[:index])
			} else if string(old) == value {
				newStart = a.start
			}
		}
		if newStart < 0 {
			if protect {
				return document.Refuse("anchors_affected", "This change would remove an open question’s passage. Ask the user to answer or resolve the question first.", before, previous)
			}
			if _, err = tx.ExecContext(ctx, "UPDATE comments SET anchor_block=NULL,anchor_start=NULL,anchor_end=NULL WHERE id=?", a.id); err != nil {
				return err
			}
		} else if _, err = tx.ExecContext(ctx, "UPDATE comments SET anchor_start=?,anchor_end=? WHERE id=?", newStart, newStart+len([]rune(quote)), a.id); err != nil {
			return err
		}
	}
	return nil
}
