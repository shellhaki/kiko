package main

import (
	"context"
	"encoding/json"
	"kiko/core"
	"kiko/core/vault"
	"kiko/core/webserver"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonalWebBindingsUseIsolatedPersistentWorkspace(t *testing.T) {
	directory := t.TempDir()
	secrets, err := vault.Open(filepath.Join(directory, "secrets.json"), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: context.Background(), dataPath: filepath.Join(directory, "web.db"), secretFor: func(id string) core.Secrets { return vaultSecrets{account: secrets.Account(id)} }}
	defer app.shutdown(context.Background())
	handler, err := webserver.New(webserver.Options{Token: "synthetic-web-login-token-at-least-32", Methods: webMethods(app)})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	request, _ := http.NewRequest("POST", server.URL+"/api/login", strings.NewReader(`{"token":"synthetic-web-login-token-at-least-32"}`))
	request.Header.Set("Origin", server.URL)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	cookie := response.Cookies()[0]
	rpc := func(method string, args string) map[string]json.RawMessage {
		t.Helper()
		request, _ := http.NewRequest("POST", server.URL+"/api/rpc", strings.NewReader(`{"method":"`+method+`","args":`+args+`}`))
		request.Header.Set("Origin", server.URL)
		request.AddCookie(cookie)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var value map[string]json.RawMessage
		if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&value) != nil || len(value["error"]) > 2 {
			t.Fatal(response.Status, value)
		}
		return value
	}
	state := rpc("Bootstrap", "[]")
	if !strings.Contains(string(state["result"]), "nickname") {
		t.Fatal(state)
	}
	rpc("SaveNickname", `["Web user"]`)
	state = rpc("Bootstrap", "[]")
	if !strings.Contains(string(state["result"]), "Web user") {
		t.Fatal(state)
	}
}
