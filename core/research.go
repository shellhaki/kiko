package core

import (
	"context"
	"errors"
	"strings"

	"kiko/core/provider"
)

func (s *Service) researchKey(ctx context.Context) (string, error) {
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return "", err
	}
	if !prefs.TavilyEnabled || s.SecretFor == nil {
		return "", errors.New("Tavily is not connected.")
	}
	return s.SecretFor("tavily").Get()
}
func (s *Service) ConfigureResearch(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 1024 || strings.ContainsAny(key, "\r\n\x00") {
		return errors.New("Enter a valid Tavily API key.")
	}
	if s.SecretFor == nil {
		return errors.New("Secure plugin storage is unavailable.")
	}
	if err := s.SecretFor("tavily").Set(key); err != nil {
		return errors.New("Couldn’t save the Tavily key securely.")
	}
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.TavilyEnabled = true
	return s.Store.SavePreferences(ctx, prefs)
}
func (s *Service) RemoveResearch(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SecretFor != nil {
		if err := s.SecretFor("tavily").Delete(); err != nil {
			return errors.New("Couldn’t remove the Tavily key.")
		}
	}
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.TavilyEnabled = false
	return s.Store.SavePreferences(ctx, prefs)
}
func researchTools() []provider.Tool {
	str := map[string]any{"type": "string"}
	return []provider.Tool{{Type: "function", Function: provider.ToolDefinition{Name: "web_search", Description: "Search the public web with Tavily Basic. Costs credits; use focused queries.", Parameters: toolSchema(map[string]any{"query": str}, "query")}}, {Type: "function", Function: provider.ToolDefinition{Name: "web_fetch", Description: "Read up to five source pages as Markdown before citing specific facts.", Parameters: toolSchema(map[string]any{"urls": map[string]any{"type": "array", "items": str}}, "urls")}}}
}
