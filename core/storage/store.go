package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var initialSchema string

//go:embed migrations/002_conversations.sql
var conversationSchema string

//go:embed migrations/003_document_drafts.sql
var documentDraftSchema string

//go:embed migrations/004_question_quotes.sql
var questionSchema string

//go:embed migrations/005_contributor_model.sql
var contributorSchema string

//go:embed migrations/006_attachments.sql
var attachmentSchema string

//go:embed migrations/007_artifacts.sql
var artifactSchema string

//go:embed migrations/008_suggestions.sql
var suggestionSchema string

var schemas = []string{initialSchema, conversationSchema, documentDraftSchema, questionSchema, contributorSchema, attachmentSchema, artifactSchema, suggestionSchema}

var ErrSchemaTooNew = errors.New("database requires a newer Kiko version")

type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	file, err := os.OpenFile(absolute, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open database file: %w", err)
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	filePath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}
	uri := url.URL{Scheme: "file", Path: filePath}
	query := url.Values{"_foreign_keys": {"on"}, "_busy_timeout": {"5000"}, "_txlock": {"immediate"}}
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = db.PingContext(ctx); err == nil {
		err = migrate(ctx, db, schemas)
	}
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize storage: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) DB() *sql.DB  { return s.db }

func (s *Store) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	return transaction(ctx, s.db, fn)
}

func transaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func migrate(ctx context.Context, db *sql.DB, scripts []string) error {
	return transaction(ctx, db, func(tx *sql.Tx) error {
		var version int
		if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
			return err
		}
		if version > len(scripts) {
			return ErrSchemaTooNew
		}
		if version < 0 {
			return errors.New("invalid database schema version")
		}
		for i := version; i < len(scripts); i++ {
			if _, err := tx.ExecContext(ctx, scripts[i]); err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
				return err
			}
		}
		return nil
	})
}
