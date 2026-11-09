package storage

import (
	"context"
	"testing"
)

func TestImagePermissionCannotBeRestoredByStalePreferences(t *testing.T) {
	s, _ := documentStore(t)
	ctx := context.Background()
	prefs, _ := s.Preferences(ctx)
	if err := s.SavePreferences(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	if err := s.SetImageModel(ctx, "openai", "model-one"); err != nil {
		t.Fatal(err)
	}
	stale, _ := s.Preferences(ctx)
	if err := s.SetImageModel(ctx, "openai", ""); err != nil {
		t.Fatal(err)
	}
	stale.Theme = "light"
	if err := s.SavePreferences(ctx, stale); err != nil {
		t.Fatal(err)
	}
	current, err := s.Preferences(ctx)
	if err != nil || len(current.ImageModels) != 0 || current.Theme != "light" {
		t.Fatal("stale settings restored image sharing", current, err)
	}
	if err = s.SetImageModel(ctx, "anthropic", "another-model"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetImageModel(ctx, "openai", "model-two"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetImageModel(ctx, "openai", ""); err != nil {
		t.Fatal(err)
	}
	current, _ = s.Preferences(ctx)
	if len(current.ImageModels) != 1 || current.ImageModels["anthropic"] != "another-model" {
		t.Fatal("permission changed another provider", current.ImageModels)
	}
	if err = s.ResetData(ctx); err != nil {
		t.Fatal(err)
	}
	current, _ = s.Preferences(ctx)
	if len(current.ImageModels) != 0 {
		t.Fatal("reset retained image permissions")
	}
}
func TestChangingBackToAnOldModelDoesNotRestoreConsent(t *testing.T) {
	s, _ := documentStore(t)
	ctx := context.Background()
	prefs, _ := s.Preferences(ctx)
	s.SavePreferences(ctx, prefs)
	p := ProviderConfig{ID: "openai", Format: "openai-chat", BaseURL: "https://api.openai.com/v1", Model: "one"}
	if err := s.SaveProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetImageModel(ctx, "openai", "one"); err != nil {
		t.Fatal(err)
	}
	p.Model = "two"
	if err := s.SaveProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.Model = "one"
	if err := s.SaveProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	current, err := s.Preferences(ctx)
	if err != nil || len(current.ImageModels) != 0 {
		t.Fatal("returning to a previous model restored image permission", current, err)
	}
}
