package core

import (
	"context"
	"testing"
)

func TestResearchKeysStaySeparateAndResetCleansThem(t *testing.T) {
	s := chatService(t)
	plugin := &fakeSecrets{}
	s.SecretFor = func(id string) Secrets {
		if id != "tavily" {
			t.Fatal("wrong secret namespace", id)
		}
		return plugin
	}
	ctx := context.Background()
	if _, err := s.researchKey(ctx); err == nil {
		t.Fatal("unconfigured research appeared available")
	}
	if err := s.ConfigureResearch(ctx, "synthetic-tavily-key"); err != nil {
		t.Fatal(err)
	}
	if key, err := s.researchKey(ctx); err != nil || key != "synthetic-tavily-key" {
		t.Fatal(err)
	}
	if key, _ := s.Keys.Get(); key != "fixture-key" {
		t.Fatal("model key was replaced")
	}
	if err := s.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if plugin.key != "" {
		t.Fatal("reset retained plugin key")
	}
}
