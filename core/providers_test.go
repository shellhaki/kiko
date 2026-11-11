package core

import (
	"context"
	"testing"

	"kiko/core/muse"
	"kiko/core/provider"
	"kiko/core/storage"
)

func TestModelChoiceIsPerChatAndLimitedToTheMuseCatalog(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	if err := s.Store.CreateChat(ctx, ""); err != nil {
		t.Fatal(err)
	}
	first, _ := s.State(ctx)
	if first.Model != muse.DefaultModel || len(first.Models) != len(muse.Models) {
		t.Fatal("new chats start on the default model with the full catalog", first.Model, len(first.Models))
	}
	if err := s.SetModel(ctx, "muse-ember-1.0"); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.CreateChat(ctx, ""); err != nil {
		t.Fatal(err)
	}
	second, _ := s.State(ctx)
	if second.Model != "muse-ember-1.0" {
		t.Fatal("a new chat should start on the last chosen model", second.Model)
	}
	if err := s.SetModel(ctx, "muse-spark-1.3"); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.Select(ctx, "", first.Selection.ChatID); err != nil {
		t.Fatal(err)
	}
	back, _ := s.State(ctx)
	if back.Model != "muse-ember-1.0" {
		t.Fatal("changing one chat's model changed another", back.Model)
	}
	for _, bad := range []string{"", "gpt-4o", "muse-spark-9", "muse-spark-1.3-contributor "} {
		if err := s.SetModel(ctx, bad); err == nil {
			t.Fatal("unlisted model accepted:", bad)
		}
	}
}

func TestSendUsesTheModelChosenForThatChat(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	s.Store.CreateChat(ctx, "")
	if err := s.SetModel(ctx, "muse-wonder-1.0"); err != nil {
		t.Fatal(err)
	}
	state, _ := s.State(ctx)
	got := make(chan string, 1)
	s.Provider = replyProvider(func(_ context.Context, request provider.Request, emit func(provider.Event) error) error {
		got <- request.Model
		emit(provider.Event{Text: "Done"})
		return emit(provider.Event{Finish: "stop"})
	})
	events := make(chan storage.Message, 20)
	if _, err := s.Send(ctx, "", state.Selection.ChatID, "Hello", "model-choice", func(m storage.Message) { events <- m }); err != nil {
		t.Fatal(err)
	}
	waitReply(t, events)
	if model := <-got; model != "muse-wonder-1.0" {
		t.Fatal("request used the wrong model", model)
	}
}
