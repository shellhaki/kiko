package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"kiko/core/provider"
)

type Client struct {
	HTTP      *http.Client
	BaseURL   string
	MaxTokens int
}

func (c Client) Stream(ctx context.Context, request provider.Request, emit func(provider.Event) error) error {
	if request.Key == "" || strings.ContainsAny(request.Key, "\r\n\x00") {
		return &provider.Error{Kind: "auth", Message: "Reconnect Anthropic in Settings."}
	}
	messages := []map[string]any{}
	system := []string{}
	for _, message := range request.Messages {
		if len(message.Images) > 0 && message.Role != "user" {
			return errors.New("images require a user message")
		}
		if message.Role == "developer" || message.Role == "system" {
			system = append(system, message.Content)
			continue
		}
		role := message.Role
		content := []map[string]any{}
		if role == "tool" {
			role = "user"
			content = append(content, map[string]any{"type": "tool_result", "tool_use_id": message.ToolCallID, "content": message.Content})
		} else {
			if message.Content != "" {
				content = append(content, map[string]any{"type": "text", "text": message.Content})
			}
			for _, image := range message.Images {
				if err := image.Validate(); err != nil {
					return err
				}
				content = append(content, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": image.MIME, "data": base64.StdEncoding.EncodeToString(image.Data)}})
			}
			for _, call := range message.ToolCalls {
				var input any
				if json.Unmarshal([]byte(call.Function.Arguments), &input) != nil {
					return errors.New("invalid saved tool arguments")
				}
				content = append(content, map[string]any{"type": "tool_use", "id": call.ID, "name": call.Function.Name, "input": input})
			}
		}
		if len(content) == 0 {
			continue
		}
		if len(messages) > 0 && messages[len(messages)-1]["role"] == role {
			prior := messages[len(messages)-1]["content"].([]map[string]any)
			messages[len(messages)-1]["content"] = append(prior, content...)
		} else {
			messages = append(messages, map[string]any{"role": role, "content": content})
		}
	}
	maxTokens := c.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	payload := map[string]any{"model": request.Model, "system": strings.Join(system, "\n\n"), "messages": messages, "max_tokens": maxTokens, "stream": true}
	if len(request.Tools) > 0 {
		tools := []map[string]any{}
		for _, tool := range request.Tools {
			tools = append(tools, map[string]any{"name": tool.Function.Name, "description": tool.Function.Description, "input_schema": tool.Function.Parameters})
		}
		payload["tools"] = tools
	}
	data, _ := json.Marshal(payload)
	if len(data) > 8*1024*1024 {
		return errors.New("conversation exceeds request limit")
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/messages", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", request.Key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	client := http.Client{}
	if c.HTTP != nil {
		client = *c.HTTP
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return &provider.Error{Kind: "network", Message: "Couldn’t reach Anthropic. Check your connection."}
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		kind := "provider"
		if response.StatusCode == 401 || response.StatusCode == 403 {
			kind = "auth"
		}
		return &provider.Error{Kind: kind, Message: "Anthropic rejected this request. Check the model, key, account access, or usage limit."}
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	var frame strings.Builder
	finish := ""
	indexes := map[int]int{}
	arguments := map[int]bool{}
	nextIndex := 0
	dispatch := func() (bool, error) {
		raw := strings.TrimSpace(frame.String())
		frame.Reset()
		if raw == "" {
			return false, nil
		}
		var event struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Block struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Text  string          `json:"text"`
				Input json.RawMessage `json:"input"`
			} `json:"content_block"`
			Delta struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Partial string `json:"partial_json"`
				Stop    string `json:"stop_reason"`
			} `json:"delta"`
			Usage json.RawMessage `json:"usage"`
		}
		if json.Unmarshal([]byte(raw), &event) != nil {
			return false, errors.New("unreadable Anthropic event")
		}
		switch event.Type {
		case "error":
			return false, &provider.Error{Kind: "provider", Message: "Anthropic interrupted this reply. Your partial response is saved."}
		case "content_block_start":
			if event.Block.Type == "tool_use" {
				indexes[event.Index] = nextIndex
				nextIndex++
				if err := emit(provider.Event{Tool: &provider.ToolDelta{Index: indexes[event.Index], ID: event.Block.ID, Name: event.Block.Name}}); err != nil {
					return false, err
				}
				if len(event.Block.Input) > 0 && string(event.Block.Input) != "{}" {
					arguments[event.Index] = true
					return false, emit(provider.Event{Tool: &provider.ToolDelta{Index: indexes[event.Index], Arguments: string(event.Block.Input)}})
				}
			} else if event.Block.Type == "text" && event.Block.Text != "" {
				return false, emit(provider.Event{Text: event.Block.Text})
			}
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				return false, emit(provider.Event{Text: event.Delta.Text})
			}
			if event.Delta.Type == "input_json_delta" {
				index, ok := indexes[event.Index]
				if !ok {
					return false, errors.New("tool delta without start")
				}
				arguments[event.Index] = true
				return false, emit(provider.Event{Tool: &provider.ToolDelta{Index: index, Arguments: event.Delta.Partial}})
			}
		case "content_block_stop":
			if index, ok := indexes[event.Index]; ok && !arguments[event.Index] {
				return false, emit(provider.Event{Tool: &provider.ToolDelta{Index: index, Arguments: "{}"}})
			}
		case "message_delta":
			switch event.Delta.Stop {
			case "end_turn", "stop_sequence":
				finish = "stop"
			case "tool_use":
				finish = "tool_calls"
			case "max_tokens":
				finish = "length"
			case "refusal":
				finish = "content_filter"
			case "":
			default:
				return false, errors.New("unsupported Anthropic completion status")
			}
			if len(event.Usage) > 0 {
				if err := emit(provider.Event{Usage: event.Usage}); err != nil {
					return false, err
				}
			}
		case "message_stop":
			if finish == "" {
				return false, errors.New("missing Anthropic completion status")
			}
			return true, emit(provider.Event{Finish: finish})
		}
		return false, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := dispatch()
			if err != nil || done {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if frame.Len()+len(line) > 2*1024*1024 {
				return errors.New("Anthropic event exceeds limit")
			}
			frame.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			frame.WriteByte('\n')
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return &provider.Error{Kind: "interrupted", Message: "The Anthropic stream ended before completion."}
}
