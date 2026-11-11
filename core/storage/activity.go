package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type Activity struct {
	OutputID string          `json:"outputId,omitempty"`
	ID       string          `json:"id"`
	Tool     string          `json:"tool"`
	Label    string          `json:"label"`
	Status   string          `json:"status"`
	Detail   json.RawMessage `json:"-"`
}
type queryRows interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func activity(ctx context.Context, db queryRows, messageID string) ([]Activity, error) {
	rows, err := db.QueryContext(ctx, "SELECT id,tool,label,status,COALESCE(detail,'{}') FROM tool_steps WHERE message_id=? ORDER BY created_at,rowid", messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Activity{}
	for rows.Next() {
		var step Activity
		var raw string
		if err = rows.Scan(&step.ID, &step.Tool, &step.Label, &step.Status, &raw); err != nil {
			return nil, err
		}
		step.Detail = json.RawMessage(raw)
		if step.Status == "ok" {
			var detail struct {
				Result string `json:"result"`
			}
			_ = json.Unmarshal(step.Detail, &detail)
			switch step.Tool {
			case "create_document":
				fields := strings.Fields(detail.Result)
				if len(fields) > 1 && fields[0] == "Document" {
					step.OutputID = fields[1]
				}
			case "create_sheet", "create_deck":
				var result struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal([]byte(detail.Result), &result)
				step.OutputID = result.ID
			}
		}
		result = append(result, step)
	}
	return result, rows.Err()
}
func (s *Store) StartActivity(ctx context.Context, message, tool, label string) (string, error) {
	id := rand.Text()
	_, err := s.db.ExecContext(ctx, "INSERT INTO tool_steps(id,message_id,tool,label,status,created_at) VALUES (?,?,?,?,'running',?)", id, message, tool, label, time.Now().UnixMilli())
	return id, err
}
func (s *Store) UpdateActivity(ctx context.Context, id, label string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE tool_steps SET label=? WHERE id=? AND status='running'", label, id)
	return err
}
func (s *Store) FinishActivity(ctx context.Context, id, status string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE tool_steps SET status=?,detail=?,elapsed_ms=max(0,?-created_at) WHERE id=? AND status='running'", status, string(data), time.Now().UnixMilli(), id)
	return err
}
func (s *Store) ChatDocuments(ctx context.Context, chat string) ([]struct {
	ID    string
	Title string
	Rev   int
}, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,title,rev FROM documents WHERE chat_id=? ORDER BY updated_at", chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []struct {
		ID    string
		Title string
		Rev   int
	}{}
	for rows.Next() {
		var d struct {
			ID    string
			Title string
			Rev   int
		}
		if err = rows.Scan(&d.ID, &d.Title, &d.Rev); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s *Store) RecoverActivity(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "UPDATE tool_steps SET status='cancelled' WHERE status='running'")
	if err == nil {
		_, err = s.db.ExecContext(ctx, "UPDATE file_permissions SET status='cancelled' WHERE status='pending'")
	}
	return err
}

func (s *Store) FinishOpenActivity(ctx context.Context, messageID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE tool_steps SET status='cancelled' WHERE message_id=? AND status='running'", messageID)
	return err
}

func (s *Store) Activities(ctx context.Context, messageID string) ([]Activity, error) {
	return activity(ctx, s.db, messageID)
}
