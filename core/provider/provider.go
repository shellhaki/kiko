package provider

import (
	"context"
	"encoding/json"
)

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}
type ToolDelta struct {
	Index     int    `json:"index"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}
type Tool struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
type Request struct {
	Key      string
	Model    string
	Messages []Message
	Tools    []Tool
}
type Event struct {
	Text   string
	Finish string
	Usage  json.RawMessage
	Tool   *ToolDelta
}
type Provider interface {
	Stream(context.Context, Request, func(Event) error) error
}
type Error struct {
	Kind    string
	Message string
}

func (e *Error) Error() string { return e.Message }
