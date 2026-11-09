package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"kiko/core/storage"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSecrets struct {
	key      string
	fail     bool
	afterSet func()
}

func (k *fakeSecrets) Get() (string, error) {
	if k.key == "" {
		return "", ErrKeyMissing
	}
	return k.key, nil
}
func (k *fakeSecrets) Set(value string) error {
	if k.fail {
		return errors.New("fixture-key must not leak")
	}
	k.key = value
	if k.afterSet != nil {
		k.afterSet()
		k.afterSet = nil
	}
	return nil
}
func (k *fakeSecrets) Delete() error { k.key = ""; return nil }

type fakeVerifier struct {
	err   error
	calls int
}

func (v *fakeVerifier) Verify(context.Context, string, string) error { v.calls++; return v.err }
func newService(t *testing.T) (*Service, *fakeSecrets, *fakeVerifier) {
	t.Helper()
	s, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	keys := &fakeSecrets{}
	verify := &fakeVerifier{}
	return &Service{Store: s, Keys: keys, Verifier: verify}, keys, verify
}

func TestOnboardingAndReconnect(t *testing.T) {
	s, keys, verify := newService(t)
	ctx := context.Background()
	state, err := s.State(ctx)
	if err != nil || state.Preferences.Stage != "nickname" {
		t.Fatal(err, state)
	}
	if err = s.SaveNickname(ctx, "  Haki  "); err != nil {
		t.Fatal(err)
	}
	state, _ = s.State(ctx)
	if state.Preferences.Stage != "muse" || state.Preferences.Nickname != "Haki" {
		t.Fatal(state)
	}
	if err = s.Back(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = s.State(ctx)
	if state.Preferences.Nickname != "Haki" || state.Preferences.Stage != "nickname" {
		t.Fatal("Back lost nickname")
	}
	s.SaveNickname(ctx, "Haki")
	if err = s.Connect(ctx, "fixture-key"); err != nil {
		t.Fatal(err)
	}
	state, _ = s.State(ctx)
	if !state.Connected || state.Preferences.Stage != "done" || verify.calls != 1 {
		t.Fatal(state)
	}
	keys.key = ""
	state, _ = s.State(ctx)
	if state.Connected || state.Preferences.Stage != "done" {
		t.Fatal("missing key erased onboarding")
	}
	if err = s.SaveNickname(ctx, ""); err == nil {
		t.Fatal("empty nickname accepted")
	}
}

func TestFailedVerificationAndSecureStorageDoNotFinishSetup(t *testing.T) {
	s, keys, verify := newService(t)
	ctx := context.Background()
	s.SaveNickname(ctx, "Haki")
	verify.err = errors.New("offline")
	if err := s.Connect(ctx, "fixture-key"); err == nil {
		t.Fatal("verification failure accepted")
	}
	if keys.key != "" {
		t.Fatal("unverified key saved")
	}
	verify.err = nil
	keys.fail = true
	if err := s.Connect(ctx, "fixture-key"); err == nil || err.Error() == "fixture-key must not leak" {
		t.Fatal("secure storage failure mishandled")
	}
	state, _ := s.State(ctx)
	if state.Preferences.Stage == "done" {
		t.Fatal("incomplete setup marked done")
	}
}

func TestConnectionRollsBackKeyOnDatabaseFailure(t *testing.T) {
	s, keys, _ := newService(t)
	ctx := context.Background()
	s.SaveNickname(ctx, "Haki")
	keys.key = "old-fixture-key"
	keys.afterSet = func() { s.Store.Close() }
	if err := s.Connect(ctx, "new-fixture-key"); err == nil {
		t.Fatal("closed database accepted")
	}
	if keys.key != "old-fixture-key" {
		t.Fatal("previous key not restored")
	}
}

func TestSnapshotsAndPreferencesNeverContainSavedKey(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	s.SaveNickname(ctx, "Haki")
	secret := "synthetic-secret-unique-marker"
	if err := s.Connect(ctx, secret); err != nil {
		t.Fatal(err)
	}
	state, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(secret)) {
		t.Fatal("snapshot exposed a saved key")
	}
	var value string
	if err = s.Store.DB().QueryRow("SELECT value FROM app_settings WHERE key='preferences'").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(value, secret) {
		t.Fatal("key persisted outside secure store")
	}
	if err = s.Appearance(ctx, "light", false, 300); err != nil {
		t.Fatal(err)
	}
	state, _ = s.State(ctx)
	if state.Preferences.Theme != "light" || state.Preferences.SidebarOpen || state.Preferences.SidebarWidth != 300 {
		t.Fatal("appearance not persisted")
	}
}
