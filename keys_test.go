package main

import (
	"errors"
	"github.com/zalando/go-keyring"
	"kiko/core"
	"testing"
)

func TestSecureKeyAdapter(t *testing.T) {
	keyring.MockInit()
	keys := systemKeys{}
	if _, err := keys.Get(); !errors.Is(err, core.ErrKeyMissing) {
		t.Fatal(err)
	}
	if err := keys.Set("synthetic-fixture-key"); err != nil {
		t.Fatal(err)
	}
	if value, err := keys.Get(); err != nil || value != "synthetic-fixture-key" {
		t.Fatal("key round trip failed")
	}
	if err := keys.Delete(); err != nil {
		t.Fatal(err)
	}
	if err := keys.Delete(); err != nil {
		t.Fatal("delete should be idempotent")
	}
}
