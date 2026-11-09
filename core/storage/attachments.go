package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"kiko/core/attachment"
)

type Attachment struct {
	ID     string `json:"id"`
	ChatID string `json:"chatId"`
	Name   string `json:"name"`
	MIME   string `json:"mime"`
	Size   int    `json:"size"`
	Pages  int    `json:"pages"`
}
type AttachmentHit struct {
	AttachmentID string `json:"attachmentId"`
	Name         string `json:"name"`
	Page         int    `json:"page"`
	Text         string `json:"text"`
}

func (s *Store) AddAttachment(ctx context.Context, chat, name string, data []byte, parsed attachment.Extracted) (Attachment, error) {
	name = filepath.Base(strings.TrimSpace(name))
	result := Attachment{ID: rand.Text(), ChatID: chat, Name: name, MIME: parsed.MIME, Size: len(data), Pages: len(parsed.Pages)}
	if name == "" || len(name) > 200 || len(data) == 0 || len(data) > attachment.MaxFile {
		return result, errors.New("invalid attachment")
	}
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		var used int64
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(sum(size),0) FROM attachments").Scan(&used); err != nil {
			return err
		}
		if used+int64(len(data)) > 512*1024*1024 {
			return errors.New("attachment storage exceeds 512 MB; remove unused files")
		}
		pages, _ := json.Marshal(parsed.Pages)
		if _, err := tx.ExecContext(ctx, "INSERT INTO attachments VALUES (?,?,?,?,?,?,?,?,?)", result.ID, chat, name, result.MIME, len(data), string(pages), data, parsed.Thumbnail, time.Now().UnixMilli()); err != nil {
			return err
		}
		for i, page := range parsed.Pages {
			if _, err := tx.ExecContext(ctx, "INSERT INTO attachment_text VALUES (?,?,?,?,?)", result.ID, chat, i+1, name, page.Text); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}
func (s *Store) Attachments(ctx context.Context, chat string) ([]Attachment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,chat_id,name,mime,size,json_array_length(pages) FROM attachments WHERE chat_id=? ORDER BY created_at,id", chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Attachment{}
	for rows.Next() {
		var item Attachment
		if err = rows.Scan(&item.ID, &item.ChatID, &item.Name, &item.MIME, &item.Size, &item.Pages); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *Store) ReadAttachment(ctx context.Context, chat, id string, page int) ([]attachment.Page, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, "SELECT pages FROM attachments WHERE id=? AND chat_id=?", id, chat).Scan(&raw); err != nil {
		return nil, err
	}
	var pages []attachment.Page
	if err := json.Unmarshal([]byte(raw), &pages); err != nil {
		return nil, err
	}
	if page > 0 {
		if page > len(pages) {
			return nil, errors.New("page does not exist")
		}
		return pages[page-1 : page], nil
	}
	return pages, nil
}
func (s *Store) AttachmentData(ctx context.Context, chat, id string, thumbnail bool) ([]byte, string, error) {
	column := "data"
	if thumbnail {
		column = "COALESCE(thumbnail,data)"
	}
	var data []byte
	var mime string
	err := s.db.QueryRowContext(ctx, "SELECT "+column+",mime FROM attachments WHERE id=? AND chat_id=?", id, chat).Scan(&data, &mime)
	if thumbnail && strings.HasPrefix(mime, "image/") {
		mime = "image/png"
	}
	return data, mime, err
}
func (s *Store) RemoveAttachment(ctx context.Context, chat, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM attachments WHERE id=? AND chat_id=?", id, chat)
	return err
}
func (s *Store) SearchAttachments(ctx context.Context, chat, query string) ([]AttachmentHit, error) {
	result := []AttachmentHit{}
	if strings.TrimSpace(query) == "" || len(query) > 500 {
		return result, errors.New("enter a short search phrase")
	}
	phrase := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	rows, err := s.db.QueryContext(ctx, "SELECT attachment_id,name,page,snippet(attachment_text,4,'','', ' … ',40) FROM attachment_text WHERE attachment_text MATCH ? AND chat_id=? ORDER BY rank LIMIT 30", phrase, chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var hit AttachmentHit
		if err = rows.Scan(&hit.AttachmentID, &hit.Name, &hit.Page, &hit.Text); err != nil {
			return nil, err
		}
		result = append(result, hit)
	}
	return result, rows.Err()
}
