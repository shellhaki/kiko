package document

import (
	"encoding/json"
	"errors"
)

func ValidateScene(scene string) error {
	if scene == "" {
		return nil
	}
	var parsed struct {
		Type     string            `json:"type"`
		Elements []json.RawMessage `json:"elements"`
	}
	if len(scene) > 512*1024 || json.Unmarshal([]byte(scene), &parsed) != nil || parsed.Type != "excalidraw" || len(parsed.Elements) == 0 || len(parsed.Elements) > 2000 {
		return errors.New("diagram scene must be Excalidraw JSON under 512 KB with 1–2000 elements")
	}
	return nil
}
