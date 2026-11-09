package muse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type Verifier struct {
	Client  *http.Client
	BaseURL string
}

func (v Verifier) Verify(ctx context.Context, key, model string) error {
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, "\r\n\x00") {
		return errors.New("Enter a valid API key.")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := http.Client{Timeout: 30 * time.Second}
	if v.Client != nil {
		client = *v.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	base := v.BaseURL
	if base == "" {
		base = "https://api.meta.ai/v1"
	}
	request := func(method, path string, body any, dest any) error {
		var reader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return err
			}
			reader = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, reader)
		if err != nil {
			return errors.New("Couldn’t prepare the connection check.")
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return errors.New("Connection check timed out or was cancelled. Try again.")
			}
			return errors.New("Couldn’t reach Meta. Check your connection and try again.")
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return statusError(response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
		if err != nil || len(data) > 1024*1024 {
			return errors.New("Meta returned an unreadable response. Try again.")
		}
		if json.Unmarshal(data, dest) != nil {
			return errors.New("Meta returned an unexpected response. Try again.")
		}
		return nil
	}
	var catalog struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := request("GET", "/models", nil, &catalog); err != nil {
		return err
	}
	found := false
	for _, m := range catalog.Data {
		if m.ID == model {
			found = true
		}
	}
	if !found {
		return errors.New("Your Meta account does not list Muse Spark 1.3 Contributor. Check model access in the Meta dashboard.")
	}
	body := map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Reply with the single word: ready"}}, "max_completion_tokens": 256, "reasoning_effort": "minimal", "stream": false, "store": false}
	var result struct {
		Choices []struct {
			Finish  string `json:"finish_reason"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := request("POST", "/chat/completions", body, &result); err != nil {
		return err
	}
	if len(result.Choices) != 1 || result.Choices[0].Finish != "stop" || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return errors.New("The connection check did not complete. Try again.")
	}
	return nil
}

func statusError(status int) error {
	switch status {
	case 401:
		return errors.New("Meta rejected this key. Check it and try again.")
	case 402:
		return errors.New("Your Meta account needs available credits or billing setup.")
	case 403:
		return errors.New("This key does not have access to the selected model.")
	case 404:
		return errors.New("The selected Muse model is unavailable.")
	case 429:
		return errors.New("Meta is rate limiting requests. Wait a moment and retry.")
	default:
		return errors.New("Meta could not complete the connection check. Try again later.")
	}
}
