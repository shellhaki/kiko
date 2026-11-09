package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"kiko/core/deck"
	"kiko/core/sheet"
)

type ArtifactDraft struct {
	BaseRev int             `json:"baseRev"`
	Payload json.RawMessage `json:"payload"`
}
type Artifact struct {
	ID      string          `json:"id"`
	ChatID  string          `json:"chatId"`
	Kind    string          `json:"kind"`
	Title   string          `json:"title"`
	Rev     int             `json:"rev"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Draft   *ArtifactDraft  `json:"draft,omitempty"`
}

func validateArtifact(kind string, raw json.RawMessage) error {
	if len(raw) > 4*1024*1024 || !json.Valid(raw) {
		return errors.New("artifact payload must be valid JSON up to 4 MB")
	}
	if kind == "sheet" {
		var book sheet.Book
		if err := json.Unmarshal(raw, &book); err != nil {
			return err
		}
		return sheet.Validate(book)
	}
	if kind == "deck" {
		var slides deck.Deck
		if err := json.Unmarshal(raw, &slides); err != nil {
			return err
		}
		for _, slide := range slides.Slides {
			if slide.ID == "" {
				return errors.New("slides require stable IDs")
			}
		}
		return deck.Prepare(&slides)
	}
	return errors.New("unsupported artifact kind")
}
func scanArtifact(row scanner) (Artifact, error) {
	var a Artifact
	var raw, draft string
	err := row.Scan(&a.ID, &a.ChatID, &a.Kind, &a.Title, &a.Rev, &raw, &draft)
	a.Payload = json.RawMessage(raw)
	if err == nil && draft != "" {
		err = json.Unmarshal([]byte(draft), &a.Draft)
	}
	return a, err
}

const artifactSQL = "SELECT id,chat_id,kind,title,rev,payload,COALESCE(draft,'') FROM artifacts "

func (s *Store) CreateArtifact(ctx context.Context, chat, kind, title string, raw json.RawMessage, author string) (Artifact, error) {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > 120 || author != "user" && author != "agent" {
		return Artifact{}, errors.New("invalid artifact title or author")
	}
	if err := validateArtifact(kind, raw); err != nil {
		return Artifact{}, err
	}
	a := Artifact{ID: rand.Text(), ChatID: chat, Kind: kind, Title: title, Rev: 1, Payload: raw}
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO artifacts(id,chat_id,kind,title,rev,payload,updated_at) VALUES (?,?,?,?,1,?,?)", a.ID, chat, kind, title, string(raw), time.Now().UnixMilli()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO artifact_revisions VALUES (?,1,?,?,?)", a.ID, author, string(raw), time.Now().UnixMilli()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO chat_view_state VALUES (?,NULL,json_object('artifactId',?)) ON CONFLICT(chat_id) DO UPDATE SET document_id=NULL,state=json_set(chat_view_state.state,'$.artifactId',?)`, chat, a.ID, a.ID)
		return err
	})
	return a, err
}
func (s *Store) ReadArtifact(ctx context.Context, id string) (Artifact, error) {
	return scanArtifact(s.db.QueryRowContext(ctx, artifactSQL+"WHERE id=?", id))
}
func (s *Store) Artifacts(ctx context.Context, chat string) ([]Artifact, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,chat_id,kind,title,rev FROM artifacts WHERE ?='' OR chat_id=? ORDER BY updated_at,id", chat, chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Artifact{}
	for rows.Next() {
		var item Artifact
		if err = rows.Scan(&item.ID, &item.ChatID, &item.Kind, &item.Title, &item.Rev); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *Store) SaveArtifact(ctx context.Context, id string, expected int, raw json.RawMessage, author string) (Artifact, error) {
	var result Artifact
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		before, err := scanArtifact(tx.QueryRowContext(ctx, artifactSQL+"WHERE id=?", id))
		if err != nil {
			return err
		}
		if before.Rev != expected {
			return errors.New("This artifact changed. Your draft is kept; reload or copy it before trying again.")
		}
		if author != "user" && author != "agent" {
			return errors.New("invalid author")
		}
		if err = validateArtifact(before.Kind, raw); err != nil {
			return err
		}
		next := before
		next.Rev++
		next.Payload = raw
		next.Draft = nil
		if _, err = tx.ExecContext(ctx, "UPDATE artifacts SET payload=?,rev=?,draft=CASE WHEN json_extract(draft,'$.payload')=json(?) THEN NULL ELSE draft END,updated_at=? WHERE id=?", string(raw), next.Rev, string(raw), time.Now().UnixMilli(), id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO artifact_revisions VALUES (?,?,?,?,?)", id, next.Rev, author, string(raw), time.Now().UnixMilli()); err != nil {
			return err
		}
		result = next
		return nil
	})
	return result, err
}
func (s *Store) SaveArtifactDraft(ctx context.Context, id string, rev int, raw json.RawMessage) error {
	current, err := s.ReadArtifact(ctx, id)
	if err != nil {
		return err
	}
	if err = validateArtifact(current.Kind, raw); err != nil {
		return err
	}
	draft, _ := json.Marshal(ArtifactDraft{BaseRev: rev, Payload: raw})
	_, err = s.db.ExecContext(ctx, "UPDATE artifacts SET draft=? WHERE id=?", string(draft), id)
	return err
}
func (s *Store) OpenArtifact(ctx context.Context, id string) error {
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		var chat, project string
		if err := tx.QueryRowContext(ctx, "SELECT a.chat_id,COALESCE(c.project_id,'') FROM artifacts a JOIN chats c ON c.id=a.chat_id WHERE a.id=?", id).Scan(&chat, &project); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE workspace_state SET chat_id=?,project_id=NULLIF(?,'') WHERE id=1", chat, project); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO chat_view_state VALUES (?,NULL,json_object('artifactId',?)) ON CONFLICT(chat_id) DO UPDATE SET document_id=NULL,state=json_set(chat_view_state.state,'$.artifactId',?)`, chat, id, id)
		return err
	})
}

func (s *Store) DiscardArtifactDraft(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE artifacts SET draft=NULL WHERE id=?", id)
	return err
}
