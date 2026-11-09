package core

import (
	"context"
	"testing"

	"kiko/core/provider"
	"kiko/core/storage"
)

func TestExplicitProviderSelectionIsIsolatedAndBounded(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	keys := map[string]*fakeSecrets{}
	s.SecretFor = func(id string) Secrets {
		if keys[id] == nil {
			keys[id] = &fakeSecrets{}
		}
		return keys[id]
	}
	s.ProviderFactory = func(config storage.ProviderConfig, maxTokens int) provider.Provider {
		if config.ID != "openai" || maxTokens != 256 {
			t.Error("verification not bounded", config.ID, maxTokens)
		}
		return replyProvider(func(ctx context.Context, request provider.Request, emit func(provider.Event) error) error {
			if request.Key != "openai-fixture" {
				t.Error("wrong key")
			}
			emit(provider.Event{Text: "READY"})
			return emit(provider.Event{Finish: "stop"})
		})
	}
	if err := s.ConfigureProvider(ctx, "openai", "chosen-model", "openai-fixture", ""); err != nil {
		t.Fatal(err)
	}
	prefs, _ := s.Store.Preferences(ctx)
	if prefs.ProviderID != "openai" {
		t.Fatal(prefs)
	}
	if key, _ := s.Keys.Get(); key != "fixture-key" {
		t.Fatal("Muse key changed")
	}
	if err := s.Store.CreateChat(ctx, ""); err != nil {
		t.Fatal(err)
	}
	state, err := s.State(ctx)
	if err != nil || state.Provider.ID != "openai" || !state.Connected {
		t.Fatal(err, state.Provider)
	}
	if err := s.ConfigureProvider(ctx, "muse", "muse-spark-1.3", "fixture", ""); err == nil {
		t.Fatal("Standard Muse substitution accepted")
	}
	if err := s.ConfigureProvider(ctx, "ollama", "local", "", "http://remote.example/v1"); err == nil {
		t.Fatal("remote loopback override accepted")
	}
}

func TestDirectImagesRequireExplicitOptInForTheExactModel(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	config := storage.ProviderConfig{ID: "openai", Format: "openai-chat", BaseURL: ProviderEndpoints["openai"], Model: "fixture-vision"}
	if err := s.Store.SaveProvider(ctx, config); err != nil {
		t.Fatal(err)
	}
	prefs, _ := s.Store.Preferences(ctx)
	if len(prefs.ImageModels) != 0 {
		t.Fatal("image sharing enabled by default")
	}
	if err := s.SetDirectImages(ctx, "muse", true); err == nil {
		t.Fatal("Muse text adapter accepted image opt-in")
	}
	if err := s.SetDirectImages(ctx, "openai", true); err != nil {
		t.Fatal(err)
	}
	prefs, _ = s.Store.Preferences(ctx)
	if prefs.ImageModels["openai"] != "fixture-vision" {
		t.Fatal(prefs)
	}
	config.Model = "different-model"
	if err := s.Store.SaveProvider(ctx, config); err != nil {
		t.Fatal(err)
	}
	if prefs.ImageModels["openai"] == config.Model {
		t.Fatal("changing model retained image opt-in")
	}
	if err := s.SetDirectImages(ctx, "openai", false); err != nil {
		t.Fatal(err)
	}
	prefs, _ = s.Store.Preferences(ctx)
	if len(prefs.ImageModels) != 0 {
		t.Fatal("image opt-in not removed")
	}
}
