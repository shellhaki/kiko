package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var ErrMissing = errors.New("secret is not configured")

type Vault struct {
	path string
	aead cipher.AEAD
	mu   sync.Mutex
}
type Account struct {
	vault *Vault
	name  string
}

func Open(path string, key []byte) (*Vault, error) {
	if len(key) != 32 {
		return nil, errors.New("web master key must contain 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	vault := &Vault{path: path, aead: aead}
	values, err := vault.load()
	if err != nil {
		return nil, err
	}
	for name, encoded := range values {
		sealed, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(sealed) < aead.NonceSize() {
			return nil, errors.New("encrypted vault is invalid")
		}
		if _, err = aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(name)); err != nil {
			return nil, errors.New("couldn’t unlock the vault; check the master key")
		}
	}
	return vault, nil
}
func (v *Vault) Account(name string) *Account { return &Account{vault: v, name: name} }
func (v *Vault) load() (map[string]string, error) {
	data, err := os.ReadFile(v.path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 1024*1024 {
		return nil, errors.New("vault exceeds size limit")
	}
	result := map[string]string{}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("vault file is invalid")
	}
	return result, nil
}
func (v *Vault) save(values map[string]string) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(v.path), ".vault-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), v.path)
}
func (a *Account) Get() (string, error) {
	v := a.vault
	v.mu.Lock()
	defer v.mu.Unlock()
	values, err := v.load()
	if err != nil {
		return "", err
	}
	encoded, ok := values[a.name]
	if !ok {
		return "", ErrMissing
	}
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(sealed) < v.aead.NonceSize() {
		return "", errors.New("encrypted secret is invalid")
	}
	nonce := sealed[:v.aead.NonceSize()]
	plain, err := v.aead.Open(nil, nonce, sealed[v.aead.NonceSize():], []byte(a.name))
	if err != nil {
		return "", errors.New("couldn’t unlock encrypted secret")
	}
	return string(plain), nil
}
func (a *Account) Set(value string) error {
	if len(value) > 8192 || a.name == "" || len(a.name) > 120 {
		return errors.New("invalid secret")
	}
	v := a.vault
	v.mu.Lock()
	defer v.mu.Unlock()
	values, err := v.load()
	if err != nil {
		return err
	}
	nonce := make([]byte, v.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	sealed := v.aead.Seal(nonce, nonce, []byte(value), []byte(a.name))
	values[a.name] = base64.StdEncoding.EncodeToString(sealed)
	return v.save(values)
}
func (a *Account) Delete() error {
	v := a.vault
	v.mu.Lock()
	defer v.mu.Unlock()
	values, err := v.load()
	if err != nil {
		return err
	}
	delete(values, a.name)
	return v.save(values)
}
