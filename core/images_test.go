package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"kiko/core/attachment"
	"kiko/core/provider"
	"kiko/core/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAgentImageToolsRespectModelOptInChatScopeAndToolOrdering(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	if err := s.Store.CreateChat(ctx, ""); err != nil {
		t.Fatal(err)
	}
	state, _ := s.Store.Snapshot(ctx)
	chat := state.Selection.ChatID
	var picture bytes.Buffer
	png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 40, 20)))
	parsed, err := attachment.Extract("source.png", picture.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	file, err := s.Store.AddAttachment(ctx, chat, "source.png", picture.Bytes(), parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AttachmentImage(ctx, "another-chat", file.ID, 1); err == nil {
		t.Fatal("cross-chat image exposed")
	}
	config := storage.ProviderConfig{ID: "openai", Format: "openai-chat", BaseURL: ProviderEndpoints["openai"], Model: "fixture-vision"}
	if err = s.Store.SaveProvider(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.SelectProvider(ctx, "openai"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetDirectImages(ctx, "openai", true); err != nil {
		t.Fatal(err)
	}
	s.SecretFor = func(string) Secrets { return &fakeSecrets{key: "synthetic-image-key"} }
	var calls atomic.Int32
	s.Provider = replyProvider(func(_ context.Context, request provider.Request, emit func(provider.Event) error) error {
		switch calls.Add(1) {
		case 1:
			found := false
			for _, tool := range request.Tools {
				if tool.Function.Name == "view_page" {
					found = true
				}
			}
			if !found {
				t.Error("direct image tool missing after opt-in")
			}
			args, _ := json.Marshal(map[string]any{"attachment_id": file.ID, "page": 1})
			for index, id := range []string{"first", "second"} {
				emit(provider.Event{Tool: &provider.ToolDelta{Index: index, ID: id, Name: "view_page", Arguments: string(args)}})
			}
			return emit(provider.Event{Finish: "tool_calls"})
		case 2:
			size := len(request.Messages)
			if size < 5 {
				return errors.New("missing image tool context")
			}
			for index, role := range []string{"assistant", "tool", "tool", "user", "user"} {
				if request.Messages[size-5+index].Role != role {
					t.Errorf("invalid tool/image ordering: %v", request.Messages[size-5+index].Role)
				}
			}
			for _, message := range request.Messages[size-2:] {
				if len(message.Images) != 1 || message.Images[0].MIME != "image/png" || !bytes.HasPrefix(message.Images[0].Data, []byte("\x89PNG")) {
					t.Error("actual image was not returned to model")
				}
			}
		case 4:
			if err := s.SetDirectImages(ctx, "openai", false); err != nil {
				return err
			}
			args, _ := json.Marshal(map[string]any{"attachment_id": file.ID, "page": 1})
			emit(provider.Event{Tool: &provider.ToolDelta{Index: 0, ID: "revoked", Name: "view_page", Arguments: string(args)}})
			return emit(provider.Event{Finish: "tool_calls"})
		default:
			for _, tool := range request.Tools {
				if tool.Function.Name == "view_page" {
					t.Error("image tool remained enabled after model change")
				}
			}
			for _, message := range request.Messages {
				if len(message.Images) > 0 {
					t.Error("transient images leaked into a later text-only turn")
				}
			}
		}
		emit(provider.Event{Text: "Inspected the image."})
		return emit(provider.Event{Finish: "stop"})
	})
	events := make(chan storage.Message, 50)
	if _, err = s.Send(ctx, "", chat, "Inspect the picture", "images-one", func(message storage.Message) { events <- message }); err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.Status != "complete" || len(final.Steps) != 2 {
		t.Fatal(final)
	}
	for _, step := range final.Steps {
		if step.Status != "ok" || strings.Contains(string(step.Detail), base64.StdEncoding.EncodeToString(parsed.Thumbnail)) {
			t.Fatal("image bytes persisted in activity", step.Status)
		}
	}
	config.Model = "fixture-text"
	if err = s.Store.SaveProvider(ctx, config); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Send(ctx, "", chat, "Continue", "images-two", func(message storage.Message) { events <- message }); err != nil {
		t.Fatal(err)
	}
	if final = waitReply(t, events); final.Status != "complete" {
		t.Fatal(final)
	}
	if err = s.SetDirectImages(ctx, "openai", true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Send(ctx, "", chat, "Inspect again", "images-three", func(message storage.Message) { events <- message }); err != nil {
		t.Fatal(err)
	}
	final = waitReply(t, events)
	if final.Status != "error" || final.ErrorKind != "permission" || calls.Load() != 4 {
		t.Fatal("revocation allowed another model request", final, calls.Load())
	}
}

func TestVisionHelperHasASharedSixCallTurnLimit(t *testing.T) {
	s := chatService(t)
	ctx := context.Background()
	s.Store.CreateChat(ctx, "")
	state, _ := s.Store.Snapshot(ctx)
	var picture bytes.Buffer
	png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	parsed, _ := attachment.Extract("sample.png", picture.Bytes())
	file, err := s.Store.AddAttachment(ctx, state.Selection.ChatID, "sample.png", picture.Bytes(), parsed)
	if err != nil {
		t.Fatal(err)
	}
	var helperCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		helperCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"A small image"}]}]}`))
	}))
	defer server.Close()
	s.VisionURL = server.URL
	var turns atomic.Int32
	s.Provider = replyProvider(func(_ context.Context, request provider.Request, emit func(provider.Event) error) error {
		if turns.Add(1) == 1 {
			args, _ := json.Marshal(map[string]any{"attachment_id": file.ID, "question": "Describe this image"})
			for i := 0; i < 7; i++ {
				emit(provider.Event{Tool: &provider.ToolDelta{Index: i, ID: fmt.Sprintf("view-%d", i), Name: "describe_image", Arguments: string(args)}})
			}
			return emit(provider.Event{Finish: "tool_calls"})
		}
		if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "image_limit") {
			t.Error("image limit was not explained to the model")
		}
		emit(provider.Event{Text: "Inspected the source within the visual budget."})
		return emit(provider.Event{Finish: "stop"})
	})
	events := make(chan storage.Message, 80)
	if _, err = s.Send(ctx, "", state.Selection.ChatID, "Inspect the source", "vision-budget", func(message storage.Message) { events <- message }); err != nil {
		t.Fatal(err)
	}
	final := waitReply(t, events)
	if final.Status != "complete" || len(final.Steps) != 7 || final.Steps[6].Status != "refused" || helperCalls.Load() != 6 {
		t.Fatal("helper exceeded its turn budget", final.Status, len(final.Steps), helperCalls.Load())
	}
}
