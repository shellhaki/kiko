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

type revisionData struct {
	Before document.Document `json:"before"`
	After  document.Document `json:"after"`
}
type DocumentDraft struct {
	BaseRev int              `json:"baseRev"`
	Blocks  []document.Block `json:"blocks"`
}
type DocumentRead struct {
	Suggestions []Suggestion       `json:"suggestions"`
	Draft       *DocumentDraft     `json:"draft,omitempty"`
	View        json.RawMessage    `json:"view"`
	Document    document.Document  `json:"document"`
	Outline     []document.Outline `json:"outline"`
	Changed     []document.Block   `json:"changed"`
	Gone        []string           `json:"gone"`
	Markdown    string             `json:"markdown"`
}
type ReadOptions struct {
	AtRev    int    `json:"atRev"`
	SinceRev int    `json:"sinceRev"`
	ParentID string `json:"parentId"`
	Search   string `json:"search"`
}

func loadDocument(ctx context.Context, tx *sql.Tx, id string, rev int) (document.Document, error) {
	doc := document.Document{Blocks: []document.Block{}}
	if err := tx.QueryRowContext(ctx, "SELECT id,chat_id,title,rev FROM documents WHERE id=?", id).Scan(&doc.ID, &doc.ChatID, &doc.Title, &doc.Rev); err != nil {
		return doc, err
	}
	if rev > 0 {
		var data string
		if err := tx.QueryRowContext(ctx, "SELECT ops FROM revisions WHERE document_id=? AND rev=?", id, rev).Scan(&data); err != nil {
			return doc, err
		}
		var revision revisionData
		if err := json.Unmarshal([]byte(data), &revision); err != nil {
			return doc, err
		}
		return revision.After, nil
	}
	type record struct {
		block  document.Block
		parent string
	}
	records := []record{}
	rows, err := tx.QueryContext(ctx, "SELECT id,COALESCE(parent_id,''),type,COALESCE(attrs,'{}'),COALESCE(content,'[]'),h FROM blocks WHERE document_id=? AND gone_rev IS NULL ORDER BY position,id", id)
	if err != nil {
		return doc, err
	}
	defer rows.Close()
	for rows.Next() {
		var r record
		var attrs, content string
		if err = rows.Scan(&r.block.ID, &r.parent, &r.block.Type, &attrs, &content, &r.block.Hash); err != nil {
			return doc, err
		}
		if err = json.Unmarshal([]byte(attrs), &r.block.Attrs); err != nil {
			return doc, err
		}
		if err = json.Unmarshal([]byte(content), &r.block.Content); err != nil {
			return doc, err
		}
		records = append(records, r)
	}
	if err = rows.Err(); err != nil {
		return doc, err
	}
	var children func(string) []document.Block
	children = func(parent string) []document.Block {
		result := []document.Block{}
		for _, r := range records {
			if r.parent == parent {
				b := r.block
				b.Children = children(b.ID)
				result = append(result, b)
			}
		}
		return result
	}
	doc.Blocks = children("")
	return doc, nil
}
func writeDocument(ctx context.Context, tx *sql.Tx, before, after document.Document, author string) error {
	if author != "user" && author != "agent" {
		return errors.New("invalid revision author")
	}
	if err := document.Prepare(after.Blocks); err != nil {
		return err
	}
	after.Rev = before.Rev + 1
	data, err := json.Marshal(revisionData{before, after})
	if err != nil {
		return err
	}
	if len(data) > 16*1024*1024 {
		return errors.New("document revision is too large")
	}
	result, err := tx.ExecContext(ctx, "UPDATE documents SET rev=?,title=?,updated_at=? WHERE id=? AND rev=?", after.Rev, after.Title, time.Now().UnixMilli(), after.ID, before.Rev)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("document changed while saving")
	}
	old := map[string]document.Block{}
	for _, b := range document.Flatten(before.Blocks) {
		old[b.ID] = b
	}
	next := map[string]bool{}
	var save func([]document.Block, string) error
	save = func(blocks []document.Block, parent string) error {
		for i, b := range blocks {
			next[b.ID] = true
			attrs, _ := json.Marshal(b.Attrs)
			content, _ := json.Marshal(b.Content)
			changed := before.Rev + 1
			born := after.Rev
			if previous, ok := old[b.ID]; ok && previous.Hash == b.Hash {
				var previousParent string
				var position float64
				if err := tx.QueryRowContext(ctx, "SELECT COALESCE(parent_id,''),position,changed_rev,born_rev FROM blocks WHERE document_id=? AND id=?", after.ID, b.ID).Scan(&previousParent, &position, &changed, &born); err != nil {
					return err
				}
				if parent != previousParent || position != float64(i) {
					changed = after.Rev
				}
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO blocks(id,document_id,parent_id,position,type,attrs,content,h,born_rev,changed_rev) VALUES (?,?,NULLIF(?,''),?,?,?,?,?,?,?) ON CONFLICT(document_id,id) DO UPDATE SET parent_id=excluded.parent_id,position=excluded.position,type=excluded.type,attrs=excluded.attrs,content=excluded.content,h=excluded.h,changed_rev=excluded.changed_rev,gone_rev=NULL`, b.ID, after.ID, parent, i, b.Type, string(attrs), string(content), b.Hash, born, changed)
			if err != nil {
				return err
			}
			if b.Type == "embed" {
				result, err := tx.ExecContext(ctx, `INSERT INTO nodes(id,document_id,kind,code) VALUES (?,?,'svg',?) ON CONFLICT(id) DO UPDATE SET code=excluded.code WHERE nodes.document_id=excluded.document_id`, document.String(b.Attrs, "ref"), after.ID, document.String(b.Attrs, "svg"))
				if err != nil {
					return err
				}
				affected, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if affected != 1 {
					return errors.New("diagram reference belongs to another document")
				}
			}
			if err = save(b.Children, b.ID); err != nil {
				return err
			}
		}
		return nil
	}
	if err = save(after.Blocks, ""); err != nil {
		return err
	}
	for id := range old {
		if !next[id] {
			if _, err = tx.ExecContext(ctx, "UPDATE blocks SET gone_rev=? WHERE document_id=? AND id=?", after.Rev, after.ID, id); err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO revisions VALUES (?,?,?,?,?)", after.ID, after.Rev, author, string(data), time.Now().UnixMilli())
	return err
}
func (s *Store) CreateDocument(ctx context.Context, chat, title, markdown string, placeholders map[string]document.Placeholder, author string) (document.Document, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > 120 {
		return document.Document{}, errors.New("document title must contain 1–120 characters")
	}
	blocks, err := document.Parse(markdown, placeholders)
	if err != nil {
		return document.Document{}, err
	}
	doc := document.Document{ID: rand.Text(), ChatID: chat, Title: title, Blocks: blocks}
	err = s.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO documents(id,chat_id,title,updated_at) VALUES (?,?,?,?)", doc.ID, chat, title, time.Now().UnixMilli()); err != nil {
			return err
		}
		before := doc
		before.Blocks = []document.Block{}
		if err := writeDocument(ctx, tx, before, doc, author); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO chat_view_state(chat_id,document_id,state) VALUES (?,?,'{}') ON CONFLICT(chat_id) DO UPDATE SET document_id=excluded.document_id,state=json_remove(chat_view_state.state,'$.artifactId')`, chat, doc.ID)
		return err
	})
	if err == nil {
		doc.Rev = 1
	}
	return doc, err
}
func (s *Store) ReadDocument(ctx context.Context, id string, options ReadOptions) (DocumentRead, error) {
	result := DocumentRead{Outline: []document.Outline{}, Changed: []document.Block{}, Gone: []string{}}
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		doc, err := loadDocument(ctx, tx, id, options.AtRev)
		if err != nil {
			return err
		}
		result.Document = doc
		result.Suggestions, err = suggestions(ctx, tx, id)
		if err != nil {
			return err
		}
		result.View = json.RawMessage(`{}`)
		var view string
		if err = tx.QueryRowContext(ctx, "SELECT state FROM chat_view_state WHERE chat_id=?", doc.ChatID).Scan(&view); err == nil {
			result.View = json.RawMessage(view)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var draft DocumentDraft
		var raw string
		if err = tx.QueryRowContext(ctx, "SELECT base_rev,blocks FROM document_drafts WHERE document_id=?", id).Scan(&draft.BaseRev, &raw); err == nil {
			if err = json.Unmarshal([]byte(raw), &draft.Blocks); err != nil {
				return err
			}
			result.Draft = &draft
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		result.Markdown = document.Markdown(doc.Blocks)
		result.Outline = document.Outlines(doc.Blocks, "")
		if options.ParentID != "" {
			b := document.Find(doc.Blocks, options.ParentID)
			if b == nil {
				return document.Refuse("block_gone", "That block no longer exists.", doc, nil)
			}
			result.Document.Blocks = []document.Block{*b}
		}
		if options.Search != "" {
			result.Document.Blocks = []document.Block{}
			for _, b := range document.Flatten(doc.Blocks) {
				if len(b.Content) > 0 && strings.Contains(strings.ToLower(document.Plain(b)), strings.ToLower(options.Search)) {
					result.Document.Blocks = append(result.Document.Blocks, b)
				}
			}
		}
		if options.SinceRev > 0 {
			if options.SinceRev > doc.Rev {
				return errors.New("revision is in the future")
			}
			previous, err := loadDocument(ctx, tx, id, options.SinceRev)
			if err != nil {
				return err
			}
			old := map[string]document.Block{}
			for _, b := range document.Flatten(previous.Blocks) {
				old[b.ID] = b
			}
			oldOutlines := document.Outlines(previous.Blocks, "")
			newOutlines := document.Outlines(doc.Blocks, "")
			locations := map[string]int{}
			parents := map[string]string{}
			for i, o := range oldOutlines {
				locations[o.ID] = i
				parents[o.ID] = o.ParentID
			}
			for i, b := range document.Flatten(doc.Blocks) {
				if prior, ok := old[b.ID]; !ok || prior.Hash != b.Hash || locations[b.ID] != i || parents[b.ID] != newOutlines[i].ParentID {
					result.Changed = append(result.Changed, b)
				}
				delete(old, b.ID)
			}
			for id := range old {
				result.Gone = append(result.Gone, id)
			}
		}
		return nil
	})
	return result, err
}
func (s *Store) UpdateDocument(ctx context.Context, id string, ops []document.Operation, author string) (document.Document, error) {
	var result document.Document
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		before, err := loadDocument(ctx, tx, id, 0)
		if err != nil {
			return err
		}
		after, err := document.Apply(before, ops)
		if err != nil {
			return err
		}
		if err = protectAnchors(ctx, tx, before, after, ops); err != nil {
			return err
		}
		if err = writeDocument(ctx, tx, before, after, author); err != nil {
			return err
		}
		after.Rev = before.Rev + 1
		result = after
		return nil
	})
	return result, err
}
func protectAnchors(ctx context.Context, tx *sql.Tx, before, after document.Document, ops []document.Operation) error {
	return reconcileAnchors(ctx, tx, before, after, true)
}
func (s *Store) SaveDocument(ctx context.Context, id string, rev int, blocks []document.Block) (document.Document, error) {
	var result document.Document
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		before, err := loadDocument(ctx, tx, id, 0)
		if err != nil {
			return err
		}
		if before.Rev != rev {
			return document.Refuse("guard_mismatch", "This document changed while you were editing. Your draft is kept; copy it or reload the saved version.", before, nil)
		}
		after := before
		after.Blocks = blocks
		if err = reconcileAnchors(ctx, tx, before, after, false); err != nil {
			return err
		}
		if err = writeDocument(ctx, tx, before, after, "user"); err != nil {
			return err
		}
		after.Rev++
		data, _ := json.Marshal(blocks)
		if _, err = tx.ExecContext(ctx, "DELETE FROM document_drafts WHERE document_id=? AND blocks=?", id, string(data)); err != nil {
			return err
		}
		result = after
		return nil
	})
	return result, err
}

func (s *Store) SaveDocumentDraft(ctx context.Context, id string, rev int, blocks []document.Block) error {
	if err := document.Prepare(blocks); err != nil {
		return err
	}
	data, err := json.Marshal(blocks)
	if err != nil {
		return err
	}
	if len(data) > 8*1024*1024 {
		return errors.New("document draft is too large")
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO document_drafts VALUES (?,?,?,?) ON CONFLICT(document_id) DO UPDATE SET base_rev=excluded.base_rev,blocks=excluded.blocks,updated_at=excluded.updated_at`, id, rev, string(data), time.Now().UnixMilli())
	return err
}
func (s *Store) DiscardDocumentDraft(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM document_drafts WHERE document_id=?", id)
	return err
}
func (s *Store) SaveDocumentView(ctx context.Context, id string, state json.RawMessage) error {
	if len(state) > 4096 || !json.Valid(state) {
		return errors.New("invalid document view")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO chat_view_state(chat_id,document_id,state) SELECT chat_id,id,? FROM documents WHERE id=? ON CONFLICT(chat_id) DO UPDATE SET state=excluded.state`, string(state), id)
	return err
}

type Revision struct {
	Rev       int    `json:"rev"`
	Author    string `json:"author"`
	CreatedAt int64  `json:"createdAt"`
}

func (s *Store) DocumentHistory(ctx context.Context, id string) ([]Revision, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT rev,author,created_at FROM revisions WHERE document_id=? ORDER BY rev DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Revision{}
	for rows.Next() {
		var item Revision
		if err = rows.Scan(&item.Rev, &item.Author, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *Store) RestoreDocument(ctx context.Context, id string, expected, revision int) (document.Document, error) {
	var result document.Document
	err := s.Transaction(ctx, func(tx *sql.Tx) error {
		before, err := loadDocument(ctx, tx, id, 0)
		if err != nil {
			return err
		}
		if before.Rev != expected {
			return document.Refuse("guard_mismatch", "The document changed. Review the latest version before restoring.", before, nil)
		}
		old, err := loadDocument(ctx, tx, id, revision)
		if err != nil {
			return err
		}
		after := before
		after.Blocks = old.Blocks
		after.Title = old.Title
		if err = reconcileAnchors(ctx, tx, before, after, false); err != nil {
			return err
		}
		if err = writeDocument(ctx, tx, before, after, "user"); err != nil {
			return err
		}
		after.Rev++
		result = after
		return nil
	})
	return result, err
}
