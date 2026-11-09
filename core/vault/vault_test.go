package vault

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVaultEncryptsSecretsAndBindsThemToAccounts(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	path := filepath.Join(t.TempDir(), "secrets.json")
	v, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.Account("muse").Set("synthetic-sensitive-key"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte("synthetic-sensitive-key")) {
		t.Fatal("plaintext secret on disk")
	}
	copy, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := copy.Account("muse").Get(); err != nil || value != "synthetic-sensitive-key" {
		t.Fatal(err)
	}
	var values map[string]string
	json.Unmarshal(raw, &values)
	values["other"] = values["muse"]
	swapped, _ := json.Marshal(values)
	os.WriteFile(path, swapped, 0600)
	if _, err = copy.Account("other").Get(); err == nil {
		t.Fatal("account ciphertext swapping accepted")
	}
	wrong := make([]byte, 32)
	rand.Read(wrong)
	if _, err := Open(path, wrong); err == nil {
		t.Fatal("wrong master key accepted")
	}
}
