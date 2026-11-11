package document

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type Mark struct {
	Type  string `json:"type"`
	Href  string `json:"href,omitempty"`
	Title string `json:"title,omitempty"`
}
type Inline struct {
	Text  string `json:"text"`
	Marks []Mark `json:"marks,omitempty"`
}
type Block struct {
	ID       string         `json:"id"`
	Hash     string         `json:"h"`
	Type     string         `json:"type"`
	Attrs    map[string]any `json:"attrs,omitempty"`
	Content  []Inline       `json:"content,omitempty"`
	Children []Block        `json:"children,omitempty"`
}
type Document struct {
	ID     string  `json:"id"`
	ChatID string  `json:"chatId"`
	Title  string  `json:"title"`
	Rev    int     `json:"rev"`
	Blocks []Block `json:"blocks"`
}

func Plain(b Block) string {
	var text strings.Builder
	for _, i := range b.Content {
		text.WriteString(i.Text)
	}
	for _, child := range b.Children {
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(Plain(child))
	}
	if b.Type == "embed" {
		text.WriteString("Diagram " + String(b.Attrs, "ref") + ": " + String(b.Attrs, "caption"))
	}
	if b.Type == "pending" {
		text.WriteString(String(b.Attrs, "intent"))
	}
	return text.String()
}
func String(attrs map[string]any, key string) string { value, _ := attrs[key].(string); return value }
func Number(attrs map[string]any, key string, fallback int) int {
	switch n := attrs[key].(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return fallback
}
func SafeLink(value string) bool {
	u, err := url.Parse(value)
	return err == nil && !strings.ContainsAny(value, "\x00\r\n") && ((u.Scheme == "http" || u.Scheme == "https") && u.Host != "" || u.Scheme == "mailto" || u.Scheme == "" && !strings.HasPrefix(value, "//"))
}
func Hash(b Block) string {
	children := make([]string, len(b.Children))
	for i, c := range b.Children {
		children[i] = Hash(c)
	}
	data, _ := json.Marshal(struct {
		Type     string
		Attrs    map[string]any
		Content  []Inline
		Children []string
	}{b.Type, b.Attrs, b.Content, children})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:4])
}
func Prepare(blocks []Block) error {
	seen := map[string]bool{}
	diagramRefs := map[string]bool{}
	prefix := rand.Text()
	counter := 0
	var walk func([]Block, int, string) error
	walk = func(list []Block, depth int, parent string) error {
		if depth > 32 {
			return errors.New("document nesting exceeds 32 levels")
		}
		for i := range list {
			b := &list[i]
			counter++
			if counter > 10000 {
				return errors.New("document exceeds 10000 blocks")
			}
			if b.ID == "" {
				b.ID = fmt.Sprintf("%s.%d", prefix, counter)
			}
			if len(b.ID) > 120 || seen[b.ID] {
				return errors.New("duplicate or invalid block identifier")
			}
			seen[b.ID] = true
			valid := false
			switch b.Type {
			case "embed":
				reference := String(b.Attrs, "ref")
				valid = parent == "" && reference != "" && len(reference) <= 120 && !diagramRefs[reference] && len(String(b.Attrs, "caption")) <= 2000 && len(b.Content) == 0
				diagramRefs[reference] = true
				if _, err := SanitizeSVG(String(b.Attrs, "svg")); err != nil {
					return err
				}
				if err := ValidateScene(String(b.Attrs, "scene")); err != nil {
					return err
				}
			case "heading":
				valid = (parent == "" || parent == "listItem" || parent == "cell") && Number(b.Attrs, "level", 0) >= 1 && Number(b.Attrs, "level", 0) <= 6
			case "paragraph", "code", "pending":
				valid = parent == "" || parent == "listItem" || parent == "cell"
			case "list":
				valid = parent == "" || parent == "listItem"
			case "listItem":
				valid = parent == "list"
			case "table":
				valid = parent == ""
			case "row":
				valid = parent == "table"
			case "cell":
				valid = parent == "row"
			}
			if !valid {
				return fmt.Errorf("unsupported %s in %s", b.Type, parent)
			}
			if len(b.Children) > 0 && b.Type != "list" && b.Type != "listItem" && b.Type != "table" && b.Type != "row" && b.Type != "cell" {
				return errors.New("text block cannot contain child blocks")
			}
			if len(b.Content) > 0 && (b.Type == "list" || b.Type == "listItem" || b.Type == "table" || b.Type == "row" || b.Type == "pending") {
				return errors.New("container cannot contain inline text")
			}
			for _, inline := range b.Content {
				for _, mark := range inline.Marks {
					switch mark.Type {
					case "bold", "italic", "code", "strike":
					case "link":
						if !SafeLink(mark.Href) {
							return errors.New("unsafe link")
						}
					default:
						return errors.New("unsupported text formatting")
					}
				}
			}
			if b.Type == "pending" && strings.TrimSpace(String(b.Attrs, "intent")) == "" {
				return errors.New("pending block requires intent")
			}
			if err := walk(b.Children, depth+1, b.Type); err != nil {
				return err
			}
			b.Hash = Hash(*b)
		}
		return nil
	}
	return walk(blocks, 0, "")
}
