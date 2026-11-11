package core

import (
	"bytes"
	"context"
	"encoding/json"
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
