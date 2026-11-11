package muse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"kiko/core/provider"
)

type Client struct {
	MaxTokens   int
	HTTP        *http.Client
	BaseURL     string
	IdleTimeout time.Duration
	RetryBase   time.Duration
}

func streamError(kind, message string) error { return &provider.Error{Kind: kind, Message: message} }
func (c Client) Stream(ctx context.Context, input provider.Request, emit func(provider.Event) error) error {
	if strings.TrimSpace(input.Key) == "" || strings.ContainsAny(input.Key, "\r\n\x00") {
		return streamError("auth", "Reconnect Meta Muse to send messages.")
	}
	payload := map[string]any{"model": input.Model, "messages": input.Messages, "stream": true, "stream_options": map[string]bool{"include_usage": true}, "max_completion_tokens": 8192, "store": false}
	if c.MaxTokens > 0 {
		payload["max_completion_tokens"] = c.MaxTokens
	}
	messages := make([]map[string]any, 0, len(input.Messages))
	for _, message := range input.Messages {
		wire := map[string]any{"role": message.Role, "content": message.Content}
		if len(message.ToolCalls) > 0 {
			wire["tool_calls"] = message.ToolCalls
		}
		if message.ToolCallID != "" {
			wire["tool_call_id"] = message.ToolCallID
		}
		messages = append(messages, wire)
	}
	payload["messages"] = messages
	if len(input.Tools) > 0 {
		payload["tools"] = input.Tools
		payload["tool_choice"] = "auto"
		payload["parallel_tool_calls"] = false
	}
	body, err := json.Marshal(payload)
	if err != nil || len(body) > 8*1024*1024 {
		return streamError("input", "This conversation is too large to send. Start a new chat.")
	}
	client := http.Client{}
	if c.HTTP != nil {
		client = *c.HTTP
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	base := c.BaseURL
	if base == "" {
		base = "https://api.meta.ai/v1"
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	delay := c.RetryBase
	if delay <= 0 {
		delay = 500 * time.Millisecond
	}
	for attempt := 0; attempt < 3; attempt++ {
		requestCtx, stop := context.WithCancel(ctx)
		var idle atomic.Bool
		idleDuration := c.IdleTimeout
		if idleDuration <= 0 {
			idleDuration = 120 * time.Second
		}
		timer := time.AfterFunc(idleDuration, func() { idle.Store(true); stop() })
		req, err := http.NewRequestWithContext(requestCtx, "POST", strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			timer.Stop()
			stop()
			return streamError("request", "Couldn’t prepare this message.")
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(input.Key))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		response, requestErr := client.Do(req)
		received := false
		retryAfter := time.Duration(0)
		retryable := false
		if requestErr != nil {
			err = streamError("network", "Couldn’t reach Meta. Check your connection and retry.")
			retryable = true
		} else if response.StatusCode != 200 {
			retryAfter = parseRetryAfter(response.Header.Get("Retry-After"), time.Now())
			retryable = response.StatusCode == 429 || response.StatusCode >= 500
			kind := "provider"
			if response.StatusCode == 401 {
				kind = "auth"
			}
			if response.StatusCode == 402 {
				kind = "billing"
			}
			if response.StatusCode == 429 {
				kind = "rate_limit"
			}
			err = streamError(kind, statusError(response.StatusCode).Error())
			response.Body.Close()
		} else {
			reader := &activityReader{reader: response.Body, timer: timer, timeout: idleDuration}
			contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
			switch contentType {
			case "text/event-stream":
				received, err = readStreamTools(reader, emit, len(input.Tools) > 0)
			case "application/json":
				received, err = readCompletionTools(reader, emit, len(input.Tools) > 0)
			default:
				err = streamError("protocol", "Meta returned an unexpected response format.")
			}
			response.Body.Close()
			var problem *provider.Error
			retryable = errors.As(err, &problem) && (problem.Kind == "network" || problem.Kind == "interrupted")
		}
		timer.Stop()
		stop()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if idle.Load() && err != nil {
			err = streamError("timeout", "Meta stopped responding. Your partial reply is saved.")
			retryable = true
		}
		if err == nil {
			return nil
		}
		if received || !retryable || attempt == 2 {
			return err
		}
		wait := delay*time.Duration(1<<attempt) + time.Duration(rand.Float64()*float64(delay)/2)
		if retryAfter > wait {
			wait = retryAfter
		}
		pause := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			pause.Stop()
			return ctx.Err()
		case <-pause.C:
		}
	}
	return streamError("network", "Couldn’t complete this reply. Try again.")
}

type activityReader struct {
	reader  io.Reader
	timer   *time.Timer
	timeout time.Duration
}

func (r *activityReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.timer.Reset(r.timeout)
	}
	return n, err
}
func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 && seconds <= 86400 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return 0
}

type streamFrame struct {
	Error   json.RawMessage `json:"error"`
	Choices []struct {
		Index  int     `json:"index"`
		Finish *string `json:"finish_reason"`
		Delta  struct {
			Content   *string           `json:"content"`
			ToolCalls []json.RawMessage `json:"tool_calls"`
		} `json:"delta"`
		Message struct {
			Content   *string           `json:"content"`
			ToolCalls []json.RawMessage `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage json.RawMessage `json:"usage"`
}

func parseFrame(data []byte, complete bool, emit func(provider.Event) error) (bool, string, error) {
	return parseFrameTools(data, complete, emit, false)
}
func parseFrameTools(data []byte, complete bool, emit func(provider.Event) error, allowTools bool) (bool, string, error) {
	if !utf8.Valid(data) {
		return false, "", streamError("protocol", "Meta returned invalid text encoding.")
	}
	var frame streamFrame
	if json.Unmarshal(data, &frame) != nil {
		return false, "", streamError("protocol", "Meta returned an unreadable stream event.")
	}
	if len(frame.Error) > 0 && string(frame.Error) != "null" {
		return false, "", streamError("provider", "Meta reported a streaming error. Your partial reply is saved.")
	}
	received := false
	finish := ""
	for _, choice := range frame.Choices {
		if choice.Index != 0 {
			return received, "", streamError("protocol", "Meta returned an unsupported response choice.")
		}
		content := choice.Delta.Content
		tools := choice.Delta.ToolCalls
		if complete {
			content = choice.Message.Content
			tools = choice.Message.ToolCalls
		}
		if len(tools) > 0 {
			if !allowTools {
				return true, "", streamError("unsupported", "Unexpected tool response.")
			}
			received = true
			for i, raw := range tools {
				var call struct {
					Index    int               `json:"index"`
					ID       string            `json:"id"`
					Type     string            `json:"type"`
					Function provider.Function `json:"function"`
				}
				if json.Unmarshal(raw, &call) != nil {
					return true, "", streamError("protocol", "Invalid tool arguments.")
				}
				if complete {
					call.Index = i
				}
				if call.Type != "" && call.Type != "function" {
					return true, "", streamError("protocol", "Unknown tool type.")
				}
				if err := emit(provider.Event{Tool: &provider.ToolDelta{Index: call.Index, ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments}}); err != nil {
					return true, "", err
				}
			}
		}
		if content != nil && *content != "" {
			received = true
			if err := emit(provider.Event{Text: *content}); err != nil {
				return received, "", err
			}
		}
		if choice.Finish != nil && *choice.Finish != "" {
			finish = *choice.Finish
		}
	}
	if len(frame.Usage) > 0 && string(frame.Usage) != "null" {
		if err := emit(provider.Event{Usage: frame.Usage}); err != nil {
			return received, finish, err
		}
	}
	return received, finish, nil
}
func finishStream(finish string, emit func(provider.Event) error) error {
	return finishStreamTools(finish, emit, false)
}
func finishStreamTools(finish string, emit func(provider.Event) error, allowTools bool) error {
	switch finish {
	case "stop", "length", "content_filter":
		return emit(provider.Event{Finish: finish})
	case "tool_calls":
		if allowTools {
			return emit(provider.Event{Finish: finish})
		}
		return streamError("unsupported", "This build cannot execute model tools yet.")
	case "":
		return streamError("interrupted", "The connection ended before this reply finished. Your partial reply is saved.")
	default:
		return streamError("protocol", "Meta returned an unknown completion status.")
	}
}
func readStream(reader io.Reader, emit func(provider.Event) error) (bool, error) {
	return readStreamTools(reader, emit, false)
}
func readStreamTools(reader io.Reader, emit func(provider.Event) error, allowTools bool) (bool, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	var data strings.Builder
	received := false
	finish := ""
	done := false
	dispatch := func() error {
		payload := strings.TrimSuffix(data.String(), "\n")
		data.Reset()
		if payload == "" {
			return nil
		}
		if payload == "[DONE]" {
			done = true
			return nil
		}
		if done {
			return streamError("protocol", "Meta sent content after the end of the response.")
		}
		visible, terminal, err := parseFrameTools([]byte(payload), false, func(event provider.Event) error {
			if finish != "" && event.Text != "" {
				return streamError("protocol", "Meta sent text after finishing the response.")
			}
			return emit(event)
		}, allowTools)
		received = received || visible
		if terminal != "" {
			if finish != "" && terminal != finish {
				return streamError("protocol", "Meta returned conflicting completion statuses.")
			}
			finish = terminal
		}
		return err
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return received, err
			}
			if done {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			if data.Len()+len(value)+1 > 2*1024*1024 {
				return received, streamError("protocol", "A response event was too large.")
			}
			data.WriteString(value)
			data.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil && finish == "" {
		return received, streamError("network", "The response stream was interrupted. Your partial reply is saved.")
	}
	if data.Len() > 0 {
		if err := dispatch(); err != nil {
			return received, err
		}
	}
	return received, finishStreamTools(finish, emit, allowTools)
}
func readCompletion(reader io.Reader, emit func(provider.Event) error) (bool, error) {
	return readCompletionTools(reader, emit, false)
}
func readCompletionTools(reader io.Reader, emit func(provider.Event) error, allowTools bool) (bool, error) {
	data, err := io.ReadAll(io.LimitReader(reader, 4*1024*1024+1))
	if err != nil {
		return false, streamError("network", "The response was interrupted.")
	}
	if len(data) > 4*1024*1024 {
		return false, streamError("protocol", "The response was too large.")
	}
	received, finish, err := parseFrameTools(data, true, emit, allowTools)
	if err != nil {
		return received, err
	}
	return received, finishStreamTools(finish, emit, allowTools)
}
