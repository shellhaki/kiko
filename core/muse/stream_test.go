package muse

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiko/core/provider"
)

type byteReader struct {
	data []byte
	step int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.step, len(r.data))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}
func TestStreamFixturesAtEveryChunkBoundary(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "fixtures", "muse", "text-stream.sse"))
	if err != nil {
		t.Fatal(err)
	}
	for size := 1; size <= len(data); size++ {
		var text, finish string
		var usage map[string]int
		visible, err := readStream(&byteReader{data: append([]byte{}, data...), step: size}, func(event provider.Event) error {
			text += event.Text
			if event.Finish != "" {
				finish = event.Finish
			}
			if len(event.Usage) > 0 {
				json.Unmarshal(event.Usage, &usage)
			}
			return nil
		})
		if err != nil || !visible || text != "Hello, Kiko — ready." || finish != "stop" || usage["total_tokens"] != 15 {
			t.Fatalf("chunk %d: %q %q %v", size, text, finish, err)
		}
	}
}
func TestSSEFramingAndTerminalCases(t *testing.T) {
	cases := []struct {
		name, input, finish string
		failure             bool
	}{
		{"CRLF comments multiline", ": heartbeat\r\nevent: chunk\r\ndata: {\"choices\":\r\ndata: [{\"index\":0,\"delta\":{\"content\":\"ready\"},\"finish_reason\":\"stop\"}]}\r\n\r\ndata: [DONE]\r\n\r\n", "stop", false},
		{"terminal EOF", `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`, "stop", false},
		{"length", `data: {"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}`, "length", false},
		{"filtered", `data: {"choices":[{"index":0,"delta":{},"finish_reason":"content_filter"}]}`, "content_filter", false},
		{"interrupted", "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", "", true},
		{"bad JSON", "data: {bad}\n\n", "", true},
		{"unknown terminal", "data: {\"choices\":[{\"finish_reason\":\"unexpected\"}]}\n\n", "", true},
		{"tools rejected", "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0}]}}]}\n\n", "", true},
		{"remote error", "data: {\"error\":{\"message\":\"synthetic-key\"}}\n\n", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			finish := ""
			_, err := readStream(strings.NewReader(tc.input), func(e provider.Event) error { finish = e.Finish; return nil })
			if (err != nil) != tc.failure || (!tc.failure && finish != tc.finish) {
				t.Fatalf("finish=%q err=%v", finish, err)
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-key") {
				t.Fatal("remote error leaked raw content")
			}
		})
	}
	_, err := readStream(strings.NewReader("data: "+strings.Repeat("x", 2*1024*1024)+"\n\n"), func(provider.Event) error { return nil })
	if err == nil {
		t.Fatal("oversized event accepted")
	}
}
func TestHTTPStreamingRetryAndRequestShape(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("missing auth")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true || body["store"] != false || body["max_completion_tokens"] != float64(8192) {
			t.Error("incorrect streaming options")
		}
		if _, exists := body["tools"]; exists {
			t.Error("plain chat sent tools")
		}
		if _, exists := body["key"]; exists {
			t.Error("key included in body")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		w.(http.Flusher).Flush()
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	text := ""
	err := (Client{BaseURL: server.URL, RetryBase: time.Millisecond}).Stream(context.Background(), provider.Request{Key: "fixture-key", Model: "muse-spark-1.3-contributor", Messages: []provider.Message{{Role: "user", Content: "Hello"}}}, func(e provider.Event) error { text += e.Text; return nil })
	if err != nil || text != "Hello" || calls.Load() != 2 {
		t.Fatalf("%q %d %v", text, calls.Load(), err)
	}
}
func TestNoReplayAfterPartialOrAuthFailure(t *testing.T) {
	for _, status := range []int{200, 401, 402, 403, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if status != 200 {
					w.WriteHeader(status)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
			}))
			defer server.Close()
			err := (Client{BaseURL: server.URL, RetryBase: time.Millisecond}).Stream(context.Background(), provider.Request{Key: "fixture-key"}, func(provider.Event) error { return nil })
			if err == nil || calls.Load() != 1 {
				t.Fatalf("replayed interrupted/client-error response: %d %v", calls.Load(), err)
			}
		})
	}
}
func TestCancellationIdleAndRetryWait(t *testing.T) {
	for _, tc := range []struct {
		name   string
		idle   time.Duration
		status int
	}{{"inflight", time.Second, 200}, {"idle", 10 * time.Millisecond, 200}, {"backoff", time.Second, 429}} {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan struct{}, 3)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				started <- struct{}{}
				if tc.status == 429 {
					w.Header().Set("Retry-After", "60")
					w.WriteHeader(429)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- (Client{BaseURL: server.URL, IdleTimeout: tc.idle, RetryBase: time.Millisecond}).Stream(ctx, provider.Request{Key: "fixture-key"}, func(provider.Event) error { return nil })
			}()
			<-started
			if tc.name != "idle" {
				cancel()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("expected interrupted run")
				}
				if tc.name != "idle" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation/idle timeout did not finish")
			}
			if tc.name == "idle" && calls.Load() != 3 {
				t.Fatalf("retry bound wrong: %d", calls.Load())
			}
		})
	}
	now := time.Now()
	if parseRetryAfter("3", now) != 3*time.Second || parseRetryAfter(now.Add(5*time.Second).UTC().Format(http.TimeFormat), now) < 4*time.Second {
		t.Fatal("Retry-After not respected")
	}
}
func TestJSONFallbackAndRedirect(t *testing.T) {
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalled = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer server.Close()
	err := (Client{BaseURL: server.URL}).Stream(context.Background(), provider.Request{Key: "fixture-key"}, func(provider.Event) error { return nil })
	if err == nil || targetCalled {
		t.Fatal("redirect followed")
	}
	text, finish := "", ""
	_, err = readCompletion(strings.NewReader(`{"choices":[{"message":{"content":"Ready"},"finish_reason":"stop"}]}`), func(e provider.Event) error { text += e.Text; finish = e.Finish; return nil })
	if err != nil || text != "Ready" || finish != "stop" {
		t.Fatal(err)
	}
}

func TestRejectsTextAfterTerminal(t *testing.T) {
	stream := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"first\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"late\"}}]}\n\n"
	text := ""
	_, err := readStream(strings.NewReader(stream), func(e provider.Event) error { text += e.Text; return nil })
	if err == nil || text != "first" {
		t.Fatal(text, err)
	}
}

func TestToolFixtureAssemblyAcrossByteBoundaries(t *testing.T) {
	data, err := os.ReadFile("../../docs/fixtures/muse/tool-stream.sse")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{1, 2, 3, 17, len(data)} {
		calls := map[int]provider.ToolDelta{}
		finish := ""
		visible, err := readStreamTools(&byteReader{data: append([]byte{}, data...), step: size}, func(e provider.Event) error {
			if e.Tool != nil {
				old := calls[e.Tool.Index]
				old.ID += e.Tool.ID
				old.Name += e.Tool.Name
				old.Arguments += e.Tool.Arguments
				calls[e.Tool.Index] = old
			}
			finish += e.Finish
			return nil
		}, true)
		if err != nil || !visible || finish != "tool_calls" || len(calls) == 0 {
			t.Fatal(err, finish, calls)
		}
		for _, call := range calls {
			if call.ID == "" || call.Name == "" || !json.Valid([]byte(call.Arguments)) {
				t.Fatal(call)
			}
		}
	}
}
