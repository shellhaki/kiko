package research

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBoundedResearchRequestsAndSafeErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing authorization")
		}
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if r.URL.Path == "/search" && (payload["search_depth"] != "basic" || payload["max_results"] != float64(5) || payload["auto_parameters"] != false) {
			t.Error(payload)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"title":"Source","url":"https://example.com","content":"Evidence"}],"usage":{"credits":1}}`))
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL}
	result, err := client.Call(context.Background(), "fixture", "web_search", "test", nil)
	if err != nil || len(result.Results) != 1 {
		t.Fatal(err, result)
	}
	if _, err = client.Call(context.Background(), "fixture", "web_fetch", "", []string{"http://insecure.test"}); err == nil || calls != 1 {
		t.Fatal("invalid URL was sent")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("fixture secret body"))
	}))
	defer bad.Close()
	_, err = (Client{BaseURL: bad.URL}).Call(context.Background(), "fixture", "web_search", "test", nil)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
}
