package core

import (
	"context"
	"encoding/json"
	"errors"
	"kiko/core/attachment"
	"strings"

	"kiko/core/provider"
	"kiko/core/storage"
)

func attachmentTools() []provider.Tool {
	str := map[string]any{"type": "string"}
	number := map[string]any{"type": "integer"}
	definitions := []provider.ToolDefinition{
		{Name: "list_attachments", Description: "List files attached to this chat, including page/sheet counts.", Parameters: toolSchema(map[string]any{})},
		{Name: "read_attachment", Description: "Read extracted attachment text, optionally one 1-based page/sheet/slide.", Parameters: toolSchema(map[string]any{"attachment_id": str, "page": number}, "attachment_id")},
		{Name: "search_attachments", Description: "Search an exact phrase in this chat's extracted attachments.", Parameters: toolSchema(map[string]any{"query": str}, "query")},
		{Name: "describe_image", Description: "Ask the selected vision helper to interpret an attached image or PDF. This is a separate metered vision request; use only for visual content that extracted text misses.", Parameters: toolSchema(map[string]any{"attachment_id": str, "question": str}, "attachment_id", "question")},
	}
	result := []provider.Tool{}
	for _, definition := range definitions {
		result = append(result, provider.Tool{Type: "function", Function: definition})
	}
	return result
}
func (s *Service) attachmentTool(ctx context.Context, run storage.Run, call provider.ToolCall) (string, error) {
	var args struct {
		ID       string `json:"attachment_id"`
		Page     int    `json:"page"`
		Query    string `json:"query"`
		Question string `json:"question"`
	}
	if err := decodeArguments(call.Function.Arguments, &args); err != nil {
		return "", err
	}
	var value any
	var err error
	switch call.Function.Name {
	case "list_attachments":
		value, err = s.Store.Attachments(ctx, run.ChatID)
	case "read_attachment":
		value, err = s.Store.ReadAttachment(ctx, run.ChatID, args.ID, args.Page)
	case "search_attachments":
		value, err = s.Store.SearchAttachments(ctx, run.ChatID, args.Query)
	case "describe_image":
		var data []byte
		var mime string
		data, mime, err = s.Store.AttachmentData(ctx, run.ChatID, args.ID, true)
		if err != nil {
			return "", err
		}
		return s.describeMedia(ctx, mime, data, args.Question)
	default:
		return "", errors.New("unknown attachment tool")
	}
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(value)
	if len(raw) > 256*1024 {
		return "", errors.New("Extracted text is too large; use search or read one page at a time.")
	}
	return string(raw), nil
}

func (s *Service) AttachmentImage(ctx context.Context, chat, id string, page int) (provider.Image, error) {
	data, mime, err := s.Store.AttachmentData(ctx, chat, id, true)
	if err != nil {
		return provider.Image{}, errors.New("Attachment is unavailable in this chat.")
	}
	if mime == "application/pdf" {
		data, err = attachment.RenderPDFWorker(ctx, data, page)
		if err != nil {
			return provider.Image{}, err
		}
	} else if !strings.HasPrefix(mime, "image/") || page != 1 {
		return provider.Image{}, errors.New("Choose an image (page 1) or a valid PDF page. Office visual conversion is not available yet.")
	}
	image := provider.Image{MIME: "image/png", Data: data}
	return image, image.Validate()
}
