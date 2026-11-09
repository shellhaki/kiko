package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kiko/core/provider"
)

func TestAnthropicStreamsTextAndToolArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "fixture" || r.Header.Get("anthropic-version") == "" {
			t.Error("missing auth headers")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["max_tokens"] != float64(256) || body["system"] != "Instructions" {
			t.Error(body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		stream := []string{`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"Hello "}}`, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`, `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call","name":"list_attachments","input":{}}}`, `{"type":"content_block_stop","index":1}`, `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}`, `{"type":"message_stop"}`}
		for _, data := range stream {
			for _, character := range "data: " + data + "\n\n" {
				w.Write([]byte(string(character)))
				w.(http.Flusher).Flush()
			}
		}
	}))
	defer server.Close()
	text, args, finish := "", "", ""
	err := (Client{BaseURL: server.URL, MaxTokens: 256}).Stream(context.Background(), provider.Request{Key: "fixture", Model: "test-model", Messages: []provider.Message{{Role: "developer", Content: "Instructions"}, {Role: "user", Content: "Hello"}}}, func(event provider.Event) error {
		text += event.Text
		finish += event.Finish
		if event.Tool != nil {
			if event.Tool.Index != 0 {
				t.Error("tool indices were not normalized")
			}
			args += event.Tool.Arguments
		}
		return nil
	})
	if err != nil || text != "Hello world" || args != "{}" || finish != "tool_calls" {
		t.Fatal(err, text, args, finish)
	}
}
func TestAnthropicDoesNotExposeRemoteErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401); w.Write([]byte("fixture-secret")) }))
	defer server.Close()
	err := (Client{BaseURL: server.URL}).Stream(context.Background(), provider.Request{Key: "fixture"}, func(provider.Event) error { return nil })
	if err == nil || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatal(err)
	}
}

func TestAnthropicImagesUseBase64Source(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			t.Error("invalid request")
		}
		raw := string(payload["messages"])
		for _, part := range []string{`"type":"image"`, `"type":"base64"`, `"media_type":"image/png"`, `"data":"cG5n"`} {
			if !strings.Contains(raw, part) {
				t.Error("missing image source", part)
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()
	err := (Client{BaseURL: server.URL}).Stream(context.Background(), provider.Request{Key: "synthetic", Model: "fixture", Messages: []provider.Message{{Role: "user", Content: "Inspect", Images: []provider.Image{{MIME: "image/png", Data: []byte("png")}}}}}, func(provider.Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
}
