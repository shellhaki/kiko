package muse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyRequestContract(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture-not-a-real-key" {
			t.Error("missing Bearer auth")
		}
		if r.URL.Path == "/models" {
			w.Write([]byte(`{"data":[{"id":"muse-spark-1.3-contributor"}]}`))
			return
		}
		if r.URL.Path != "/chat/completions" || r.Method != "POST" {
			t.Error("wrong endpoint")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["model"] != "muse-spark-1.3-contributor" || body["store"] != false || body["stream"] != false || body["max_completion_tokens"] != float64(256) || body["reasoning_effort"] != "minimal" {
			t.Error("wrong readiness request")
		}
		if _, ok := body["tools"]; ok {
			t.Error("readiness invoked tools")
		}
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"Ready!"}}]}`))
	}))
	defer server.Close()
	if err := (Verifier{BaseURL: server.URL}).Verify(context.Background(), " fixture-not-a-real-key ", "muse-spark-1.3-contributor"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestVerifyFailureCases(t *testing.T) {
	for _, test := range []struct {
		name                string
		status              int
		catalog, completion string
	}{
		{"invalid key", 401, "", ""}, {"billing", 402, "", ""}, {"access", 403, "", ""}, {"rate limit", 429, "", ""}, {"offline service", 503, "", ""},
		{"missing model", 200, `{"data":[]}`, ""},
		{"bad JSON", 200, `not json`, ""},
		{"truncated", 200, `{"data":[{"id":"muse-spark-1.3-contributor"}]}`, `{"choices":[{"finish_reason":"length","message":{"content":"ready"}}]}`},
		{"empty", 200, `{"data":[{"id":"muse-spark-1.3-contributor"}]}`, `{"choices":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(test.status)
				if r.URL.Path == "/models" {
					w.Write([]byte(test.catalog))
				} else {
					w.Write([]byte(test.completion))
				}
			}))
			defer server.Close()
			err := (Verifier{BaseURL: server.URL}).Verify(context.Background(), "fixture-key", "muse-spark-1.3-contributor")
			if err == nil {
				t.Fatal("failure accepted")
			}
			if strings.Contains(err.Error(), "fixture-key") {
				t.Fatal("key leaked")
			}
			if requests > 2 {
				t.Fatal("automatic retry during verification")
			}
		})
	}
}

func TestVerifyRejectsRedirectAndCancelledContext(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls++ }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	verifier := Verifier{BaseURL: redirect.URL}
	if err := verifier.Verify(context.Background(), "fixture-key", "muse-spark-1.3-contributor"); err == nil {
		t.Fatal("redirect accepted")
	}
	if targetCalls != 0 {
		t.Fatal("key sent to redirected server")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := verifier.Verify(ctx, "fixture-key", "muse-spark-1.3-contributor"); err == nil {
		t.Fatal("cancel ignored")
	}
	if err := verifier.Verify(context.Background(), "bad\nkey", "muse-spark-1.3-contributor"); err == nil {
		t.Fatal("header injection allowed")
	}
}
