package document

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Refusal struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Rev     int    `json:"rev"`
	Current *Block `json:"current,omitempty"`
}

func (r *Refusal) Error() string { return r.Message }
func Refuse(code, message string, doc Document, block *Block) error {
	return &Refusal{code, message, doc.Rev, block}
}

type Target struct {
	IDs  []string `json:"ids,omitempty"`
	Find string   `json:"find,omitempty"`
	Nth  int      `json:"nth,omitempty"`
}
type Operation struct {
	Type              string                 `json:"type"`
	Target            Target                 `json:"target"`
	Markdown          *string                `json:"markdown,omitempty"`
	Text              *string                `json:"text,omitempty"`
	Blocks            map[string]Placeholder `json:"blocks,omitempty"`
	IfRev             int                    `json:"ifRev,omitempty"`
	IfHash            map[string]string      `json:"ifHash,omitempty"`
	ParentID          string                 `json:"parentId,omitempty"`
	After             string                 `json:"after,omitempty"`
	AllowAnchorChange bool                   `json:"allowAnchorChange,omitempty"`
}

func Clone(doc Document) Document {
	data, _ := json.Marshal(doc)
	var copy Document
	_ = json.Unmarshal(data, &copy)
	return copy
}
func Flatten(blocks []Block) []Block {
	result := []Block{}
	for _, b := range blocks {
		result = append(result, b)
		result = append(result, Flatten(b.Children)...)
	}
	return result
}
func Find(blocks []Block, id string) *Block {
	for i := range blocks {
		if blocks[i].ID == id {
			return &blocks[i]
		}
		if b := Find(blocks[i].Children, id); b != nil {
			return b
		}
	}
	return nil
}
func siblings(blocks *[]Block, id string) (*[]Block, int) {
	for i := range *blocks {
		if (*blocks)[i].ID == id {
			return blocks, i
		}
		if list, index := siblings(&(*blocks)[i].Children, id); list != nil {
			return list, index
		}
	}
	return nil, -1
}
func guard(doc Document, b Block, op Operation) error {
	if b.Type == "pending" && op.Type == "replace" {
		return nil
	}
	if op.IfRev == 0 || op.IfHash[b.ID] == "" {
		return Refuse("missing_guard", "Read the document and supply ifRev and ifHash for each whole-block change.", doc, &b)
	}
	if op.IfRev != doc.Rev || op.IfHash[b.ID] != b.Hash {
		return Refuse("guard_mismatch", "The document changed. Read the current blocks and retry without overwriting user edits.", doc, &b)
	}
	return nil
}
func Apply(original Document, ops []Operation) (Document, error) {
	if len(ops) == 0 || len(ops) > 100 {
		return original, Refuse("invalid_ops", "Provide 1–100 operations.", original, nil)
	}
	doc := Clone(original)
	for _, op := range ops {
		if op.Type != "replace" && op.Type != "insert" && op.Type != "delete" && op.Type != "move" {
			return original, Refuse("invalid_operation", "Unknown document operation.", doc, nil)
		}
		if op.Target.Find != "" {
			if len(op.Target.IDs) > 0 || op.Type != "replace" || op.Text == nil || op.Markdown != nil {
				return original, Refuse("invalid_target", "Use find with a text replacement only.", doc, nil)
			}
			type match struct {
				id    string
				start int
			}
			matches := []match{}
			for _, b := range Flatten(doc.Blocks) {
				if len(b.Content) == 0 {
					continue
				}
				value := Plain(b)
				offset := 0
				for {
					index := strings.Index(value[offset:], op.Target.Find)
					if index < 0 {
						break
					}
					matches = append(matches, match{b.ID, offset + index})
					offset += index + len(op.Target.Find)
				}
			}
			if len(matches) == 0 {
				return original, Refuse("find_none", "The exact quoted text was not found. Read the current document.", doc, nil)
			}
			nth := op.Target.Nth
			if nth == 0 && len(matches) > 1 {
				return original, Refuse("find_ambiguous", "The text occurs more than once. Supply a 1-based nth match.", doc, nil)
			}
			if nth == 0 {
				nth = 1
			}
			if nth < 1 || nth > len(matches) {
				return original, Refuse("find_none", "That match does not exist.", doc, nil)
			}
			hit := matches[nth-1]
			b := Find(doc.Blocks, hit.id)
			b.Content = replaceText(b.Content, hit.start, len(op.Target.Find), *op.Text)
			if err := Prepare(doc.Blocks); err != nil {
				return original, err
			}
			continue
		}
		if op.Text != nil {
			return original, Refuse("invalid_content", "Text edits require an exact find target.", doc, nil)
		}
		if op.Type == "insert" {
			if op.Markdown == nil || len(op.Target.IDs) > 0 {
				return original, Refuse("invalid_content", "Insert requires Markdown and an optional after/parentId.", doc, nil)
			}
			parsed, err := Parse(*op.Markdown, op.Blocks)
			if err != nil {
				return original, err
			}
			if err = insert(&doc, parsed, op.ParentID, op.After); err != nil {
				return original, err
			}
		} else {
			if len(op.Target.IDs) == 0 {
				return original, Refuse("invalid_target", "Select block IDs.", doc, nil)
			}
			list, index := siblings(&doc.Blocks, op.Target.IDs[0])
			if list == nil {
				return original, Refuse("block_gone", "The selected block was deleted. Do not recreate it automatically.", doc, nil)
			}
			if index+len(op.Target.IDs) > len(*list) {
				return original, Refuse("invalid_target", "Select contiguous sibling blocks.", doc, nil)
			}
			selected := append([]Block{}, (*list)[index:index+len(op.Target.IDs)]...)
			for i, b := range selected {
				if b.ID != op.Target.IDs[i] {
					return original, Refuse("invalid_target", "Select contiguous sibling blocks.", doc, nil)
				}
				if err := guard(doc, b, op); err != nil {
					return original, err
				}
			}
			switch op.Type {
			case "replace":
				if op.Markdown == nil {
					return original, Refuse("invalid_content", "Replacement requires Markdown.", doc, nil)
				}
				parsed, err := Parse(*op.Markdown, op.Blocks)
				if err != nil {
					return original, err
				}
				*list = append(append(append([]Block{}, (*list)[:index]...), parsed...), (*list)[index+len(selected):]...)
			case "delete", "move":
				if op.Markdown != nil {
					return original, Refuse("invalid_content", "Delete/move cannot contain Markdown.", doc, nil)
				}
				*list = append((*list)[:index], (*list)[index+len(selected):]...)
				if op.Type == "move" {
					if err := insert(&doc, selected, op.ParentID, op.After); err != nil {
						return original, err
					}
				}
			}
		}
		if err := Prepare(doc.Blocks); err != nil {
			return original, Refuse("invalid_content", err.Error(), doc, nil)
		}
	}
	return doc, nil
}
func insert(doc *Document, blocks []Block, parent, after string) error {
	list := &doc.Blocks
	if parent != "" {
		b := Find(doc.Blocks, parent)
		if b == nil {
			return Refuse("block_gone", "The destination parent no longer exists.", *doc, nil)
		}
		list = &b.Children
	}
	index := len(*list)
	if after != "" {
		index = -1
		for i, b := range *list {
			if b.ID == after {
				index = i + 1
				break
			}
		}
		if index < 0 {
			return Refuse("block_gone", "The destination block is not in that parent.", *doc, nil)
		}
	}
	*list = append(append(append([]Block{}, (*list)[:index]...), blocks...), (*list)[index:]...)
	return nil
}
func replaceText(content []Inline, start, length int, replacement string) []Inline {
	result := []Inline{}
	offset := 0
	inserted := false
	for _, inline := range content {
		end := offset + len(inline.Text)
		if end <= start || offset >= start+length {
			result = append(result, inline)
		} else {
			if offset < start {
				result = append(result, Inline{Text: inline.Text[:start-offset], Marks: inline.Marks})
			}
			if !inserted {
				result = append(result, Inline{Text: replacement, Marks: inline.Marks})
				inserted = true
			}
			if end > start+length {
				result = append(result, Inline{Text: inline.Text[start+length-offset:], Marks: inline.Marks})
			}
		}
		offset = end
	}
	return compact(result)
}

type Outline struct {
	ID       string `json:"id"`
	Hash     string `json:"h"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	ParentID string `json:"parentId,omitempty"`
}

func Outlines(blocks []Block, parent string) []Outline {
	result := []Outline{}
	for _, b := range blocks {
		value := []rune(Plain(b))
		if len(value) > 90 {
			value = append(value[:90], '…')
		}
		result = append(result, Outline{b.ID, b.Hash, b.Type, string(value), parent})
		result = append(result, Outlines(b.Children, b.ID)...)
	}
	return result
}
func Ack(doc Document) string {
	var out strings.Builder
	fmt.Fprintf(&out, "Document %s · %s · rev %d\n", doc.ID, doc.Title, doc.Rev)
	for _, item := range Outlines(doc.Blocks, "") {
		fmt.Fprintf(&out, "%s %s h=%s %s\n", item.Type, item.ID, item.Hash, item.Text)
	}
	return out.String()
}
