package webserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const testToken = "synthetic-local-login-token-32-characters"

func TestAuthenticationOriginRPCAndLogout(t *testing.T) {
	calls := 0
	s, err := New(Options{Token: testToken, Assets: fstest.MapFS{"index.html": {Data: []byte("<html><head></head></html>")}}, Methods: map[string]any{"Echo": func(value string) (string, error) { calls++; return value, nil }}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s)
	defer server.Close()
	post := func(path, body, origin string, cookie *http.Cookie) *http.Response {
		t.Helper()
		request, _ := http.NewRequest("POST", server.URL+path, strings.NewReader(body))
		request.Header.Set("Origin", origin)
		if cookie != nil {
			request.AddCookie(cookie)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	response := post("/api/rpc", `{"method":"Echo","args":["test"]}`, server.URL, nil)
	response.Body.Close()
	if response.StatusCode != 401 || calls != 0 {
		t.Fatal("unauthenticated request executed")
	}
	response = post("/api/login", `{"token":"`+testToken+`"}`, "https://other.example", nil)
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("cross-origin login accepted")
	}
	response = post("/api/login", `{"token":"`+testToken+`"}`, server.URL, nil)
	response.Body.Close()
	if response.StatusCode != 204 || len(response.Cookies()) != 1 {
		t.Fatal(response.StatusCode)
	}
	cookie := response.Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie lacks protections")
	}
	response = post("/api/rpc", `{"method":"Echo","args":["hello"]}`, "https://other.example", cookie)
	response.Body.Close()
	if response.StatusCode != 403 || calls != 0 {
		t.Fatal("cross-origin RPC executed")
	}
	response = post("/api/rpc", `{"method":"Echo","args":["hello"]}`, server.URL, cookie)
	raw, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || calls != 1 || !bytes.Contains(raw, []byte(`"result":"hello"`)) {
		t.Fatal(response.StatusCode, string(raw))
	}
	response = post("/api/rpc", `{"method":"Unknown","args":[]}`, server.URL, cookie)
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("unlisted action exposed")
	}
	response = post("/api/logout", `{}`, server.URL, cookie)
	response.Body.Close()
	response = post("/api/rpc", `{"method":"Echo","args":["again"]}`, server.URL, cookie)
	response.Body.Close()
	if response.StatusCode != 401 || calls != 1 {
		t.Fatal("logged-out cookie reused")
	}
}
func TestLoginRateLimitAndSecretsNotServed(t *testing.T) {
	s, _ := New(Options{Token: testToken, Methods: map[string]any{}})
	for i := 0; i < 11; i++ {
		request := httptest.NewRequest("POST", "http://localhost/api/login", strings.NewReader(`{"token":"wrong"}`))
		request.Header.Set("Origin", "http://localhost")
		response := httptest.NewRecorder()
		s.ServeHTTP(response, request)
		if i == 10 && response.Code != 429 {
			t.Fatal(response.Code)
		}
	}
	request := httptest.NewRequest("GET", "http://localhost/api/session", nil)
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	var value map[string]any
	json.Unmarshal(response.Body.Bytes(), &value)
	if len(value) != 1 || value["authenticated"] != false {
		t.Fatal(value)
	}
}

func TestOriginValidationAndEventQueueBound(t *testing.T) {
	for _, origin := range []string{"https://host/path", "https://user@host", "file:///tmp", "https://host?token=x"} {
		if _, err := New(Options{Token: testToken, Origin: origin}); err == nil {
			t.Fatalf("accepted invalid origin %q", origin)
		}
	}
	s, err := New(Options{Token: testToken, Origin: "https://host"})
	if err != nil {
		t.Fatal(err)
	}
	listener := make(chan []byte, 2)
	s.listeners[listener] = true
	for i := 0; i < 100; i++ {
		s.Publish(i)
	}
	if len(listener) != 2 {
		t.Fatal("event queue not bounded")
	}
	<-listener
	if string(<-listener) != "99" {
		t.Fatal("latest update was lost")
	}
}
