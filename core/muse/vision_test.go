package muse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVisionBoundedResponsesWire(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer synthetic" {
			t.Error("wrong endpoint or authorization")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["store"] != false || body["max_output_tokens"] != float64(2048) || body["model"] != "synthetic-model" {
			t.Error("unbounded or invalid request")
		}
		raw, _ := json.Marshal(body["input"])
		if !strings.Contains(string(raw), `"type":"input_file"`) || !strings.Contains(string(raw), "data:application/pdf;base64,") {
			t.Error("PDF not sent as a Responses file")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Visible content"}]}]}`))
	}))
	defer server.Close()
	client := Vision{HTTP: server.Client(), BaseURL: server.URL}
	result, err := client.Describe(context.Background(), "synthetic", "synthetic-model", "application/pdf", []byte("synthetic pdf"), "Summarize")
	if err != nil || result != "Visible content" || calls != 1 {
		t.Fatal(result, err, calls)
	}
	if _, err = client.Describe(context.Background(), "synthetic", "model", "text/plain", []byte("text"), "Read"); err == nil || calls != 1 {
		t.Fatal("invalid input reached network")
	}
}
func TestVisionNeverRetriesOrLeaksErrorBody(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(429)
		w.Write([]byte("private synthetic key"))
	}))
	defer server.Close()
	_, err := (Vision{BaseURL: server.URL}).Describe(context.Background(), "synthetic", "model", "image/png", []byte("png"), "Read")
	if err == nil || strings.Contains(err.Error(), "private") || calls != 1 {
		t.Fatal(err, calls)
	}
}
