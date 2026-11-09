package main

import (
	"errors"
	"github.com/zalando/go-keyring"
	"kiko/core"
)

type systemKeys struct{ account string }

func (k systemKeys) name() string {
	if k.account == "" {
		return "muse"
	}
	return k.account
}

func (k systemKeys) Get() (string, error) {
	key, err := keyring.Get("Kiko", k.name())
	if errors.Is(err, keyring.ErrNotFound) {
		return "", core.ErrKeyMissing
	}
	return key, err
}
func (k systemKeys) Set(key string) error {
	if err := keyring.Set("Kiko", k.name(), key); err != nil {
		return err
	}
	saved, err := keyring.Get("Kiko", k.name())
	if err != nil {
		return err
	}
	if saved != key {
		return errors.New("secure key verification failed")
	}
	return nil
}
func (k systemKeys) Delete() error {
	err := keyring.Delete("Kiko", k.name())
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
