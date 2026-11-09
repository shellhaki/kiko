package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"kiko/core/anthropic"
	"kiko/core/muse"
	"kiko/core/provider"
	"kiko/core/storage"
)

var ProviderEndpoints = map[string]string{"muse": "https://api.meta.ai/v1", "openai": "https://api.openai.com/v1", "openrouter": "https://openrouter.ai/api/v1", "groq": "https://api.groq.com/openai/v1", "gemini": "https://generativelanguage.googleapis.com/v1beta/openai", "ollama": "http://127.0.0.1:11434/v1", "anthropic": "https://api.anthropic.com/v1"}

func NewProvider(config storage.ProviderConfig, maxTokens int) provider.Provider {
	if config.Format == "anthropic-messages" {
		return anthropic.Client{BaseURL: config.BaseURL, MaxTokens: maxTokens}
	}
	return muse.Client{ImageInputs: config.ID == "openai", BaseURL: config.BaseURL, Compatibility: config.ID != "muse" && config.ID != "openai", MaxTokens: maxTokens}
}
func (s *Service) providerSecrets(id string) Secrets {
	if id == "muse" || id == "" {
		return s.Keys
	}
	if s.SecretFor != nil {
		return s.SecretFor(id)
	}
	return nil
}
func (s *Service) ConfigureProvider(ctx context.Context, id, model, key, endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	base, ok := ProviderEndpoints[id]
	if !ok {
		return errors.New("Unsupported provider.")
	}
	model = strings.TrimSpace(model)
	key = strings.TrimSpace(key)
	if model == "" || len(model) > 150 || strings.ContainsAny(model+key, "\r\n\x00") {
		return errors.New("Enter a valid model ID and API key.")
	}
	if id == "muse" && model != "muse-spark-1.3-contributor" {
		return errors.New("Muse stays on the Contributor model for this demo.")
	}
	if id == "ollama" && endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" && parsed.Hostname() != "::1") {
			return errors.New("Local Ollama must use a loopback URL.")
		}
		base = strings.TrimRight(endpoint, "/")
	}
	if id == "ollama" {
		key = "ollama"
	}
	if key == "" {
		return errors.New("Enter this provider’s API key.")
	}
	config := storage.ProviderConfig{ID: id, Model: model, BaseURL: base, Format: "openai-chat"}
	if id == "anthropic" {
		config.Format = "anthropic-messages"
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := NewProvider(config, 256)
	if s.ProviderFactory != nil {
		client = s.ProviderFactory(config, 256)
	}
	visible := ""
	finish := ""
	if err := client.Stream(verifyCtx, provider.Request{Key: key, Model: model, Messages: []provider.Message{{Role: "user", Content: "Reply briefly with READY."}}}, func(event provider.Event) error { visible += event.Text; finish = event.Finish; return nil }); err != nil {
		return err
	}
	if strings.TrimSpace(visible) == "" || finish != "stop" {
		return errors.New("Connection check did not produce a complete reply.")
	}
	secrets := s.providerSecrets(id)
	if secrets == nil {
		return errors.New("Secure provider storage is unavailable.")
	}
	old, oldErr := secrets.Get()
	if oldErr != nil && !errors.Is(oldErr, ErrKeyMissing) {
		return errors.New("Couldn’t read secure provider storage.")
	}
	if err := secrets.Set(key); err != nil {
		return errors.New("Couldn’t securely save this key.")
	}
	if err := s.Store.SaveProvider(ctx, config); err != nil {
		if old != "" {
			_ = secrets.Set(old)
		} else {
			_ = secrets.Delete()
		}
		return errors.New("Couldn’t save provider configuration. The previous key was restored.")
	}
	return s.Store.SelectProvider(ctx, id)
}
func (s *Service) SelectProvider(ctx context.Context, id string) error {
	config, err := s.Store.Provider(ctx, id)
	if err != nil {
		return errors.New("Connect this provider before selecting it.")
	}
	if config.ID != "ollama" {
		secrets := s.providerSecrets(id)
		if secrets == nil {
			return errors.New("Provider key is unavailable.")
		}
		if _, err = secrets.Get(); err != nil {
			return errors.New("Reconnect this provider in Settings.")
		}
	}
	return s.Store.SelectProvider(ctx, id)
}

func (s *Service) SetVisionProvider(ctx context.Context, id string) error {
	if id != "muse" && id != "openai" {
		return errors.New("The vision helper currently uses a connected Muse or OpenAI Responses model.")
	}
	if _, err := s.Store.Provider(ctx, id); err != nil {
		return errors.New("Connect that provider first.")
	}
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.VisionProviderID = id
	return s.Store.SavePreferences(ctx, prefs)
}
func (s *Service) describeMedia(ctx context.Context, mime string, data []byte, question string) (string, error) {
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return "", err
	}
	id := prefs.VisionProviderID
	if id == "" {
		id = "muse"
	}
	config, err := s.Store.Provider(ctx, id)
	if err != nil {
		return "", err
	}
	secrets := s.providerSecrets(id)
	if secrets == nil {
		return "", errors.New("Vision helper key storage is unavailable.")
	}
	key, err := secrets.Get()
	if err != nil {
		return "", errors.New("Reconnect the selected vision helper.")
	}
	base := config.BaseURL
	if s.VisionURL != "" {
		base = s.VisionURL
	}
	return (muse.Vision{BaseURL: base}).Describe(ctx, key, config.Model, mime, data, question)
}

func (s *Service) RemoveProviderKey(ctx context.Context, id string) error {
	if id == "muse" {
		return s.RemoveKey(ctx)
	}
	if _, ok := ProviderEndpoints[id]; !ok {
		return errors.New("Unknown provider.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.resumeSends()
	if err := s.cancelRuns(ctx); err != nil {
		return err
	}
	secrets := s.providerSecrets(id)
	if secrets == nil {
		return errors.New("Secure provider storage is unavailable.")
	}
	if err := secrets.Delete(); err != nil {
		return errors.New("Couldn’t remove this provider key.")
	}
	return nil
}

func (s *Service) SetDirectImages(ctx context.Context, id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !provider.DirectImages(id) {
		return errors.New("Direct image input currently supports configured OpenAI and Anthropic models.")
	}
	config, err := s.Store.Provider(ctx, id)
	if err != nil {
		return errors.New("Connect this provider first.")
	}
	model := ""
	if enabled {
		model = config.Model
	}
	return s.Store.SetImageModel(ctx, id, model)
}
