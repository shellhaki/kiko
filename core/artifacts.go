package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/xuri/excelize/v2"
	"strings"
	"time"

	"kiko/core/deck"
	"kiko/core/provider"
	"kiko/core/sheet"
	"kiko/core/storage"
)

func (s *Service) deleteArtifactTool(ctx context.Context, run storage.Run, call provider.ToolCall) (string, error) {
	var args struct {
		DocID      string `json:"doc_id"`
		ArtifactID string `json:"artifact_id"`
	}
	if err := decodeArguments(call.Function.Arguments, &args); err != nil {
		return "", err
	}
	var value any
	var err error
	if call.Function.Name == "delete_document" {
		value, err = s.deleteDocument(ctx, run, args.DocID)
	} else {
		value, err = s.deleteArtifact(ctx, run, args.ArtifactID)
	}
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(value)
	return string(raw), err
}
func (s *Service) deleteDocument(ctx context.Context, run storage.Run, id string) (map[string]string, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("Choose a document to delete.")
	}
	var title, chat string
	err := s.Store.DB().QueryRowContext(ctx, "SELECT title,chat_id FROM documents WHERE id=?", id).Scan(&title, &chat)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("That document does not exist.")
	}
	if err != nil {
		return nil, err
	}
	if chat != run.ChatID {
		return nil, errors.New("That document belongs to another chat.")
	}
	if err = s.approveDestructive(ctx, run, title, id, "document"); err != nil {
		return nil, err
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = s.Store.Transaction(saveCtx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(saveCtx, "DELETE FROM workspace_files WHERE source_id=?", id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(saveCtx, "DELETE FROM workspace_sync_errors WHERE source_id=?", id); err != nil {
			return err
		}
		result, err := tx.ExecContext(saveCtx, "DELETE FROM documents WHERE id=?", id)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return errors.New("That document is already gone.")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"deleted": title}, nil
}
func (s *Service) deleteArtifact(ctx context.Context, run storage.Run, id string) (map[string]string, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("Choose a spreadsheet or deck to delete.")
	}
	var kind, title, chat string
	err := s.Store.DB().QueryRowContext(ctx, "SELECT kind,title,chat_id FROM artifacts WHERE id=?", id).Scan(&kind, &title, &chat)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("That spreadsheet or deck does not exist.")
	}
	if err != nil {
		return nil, err
	}
	if chat != run.ChatID {
		return nil, errors.New("That spreadsheet or deck belongs to another chat.")
	}
	if err = s.approveDestructive(ctx, run, title, id, kind); err != nil {
		return nil, err
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = s.Store.Transaction(saveCtx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(saveCtx, "DELETE FROM workspace_files WHERE source_id=?", id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(saveCtx, "DELETE FROM workspace_sync_errors WHERE source_id=?", id); err != nil {
			return err
		}
		result, err := tx.ExecContext(saveCtx, "DELETE FROM artifacts WHERE id=?", id)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return errors.New("That spreadsheet or deck is already gone.")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"deleted": title}, nil
}
func artifactTools() []provider.Tool {
	str := map[string]any{"type": "string"}
	integer := map[string]any{"type": "integer"}
	object := map[string]any{"type": "object"}
	definitions := []provider.ToolDefinition{
		{Name: "create_sheet", Description: "Create a spreadsheet artifact. Optional path sets its root-relative .sheet.json source file. tabs is an array of {name,cells:{A1:{value,formula,bold,fill,format}}}. Formulas use Excel syntax; no external/volatile functions.", Parameters: toolSchema(map[string]any{"path": str, "name": str, "tabs": map[string]any{"type": "array", "items": object}}, "name", "tabs")},
		{Name: "set_cells", Description: "Update named cells atomically in a tab. Requires the current artifact revision; read_range before editing after user changes. Empty cell objects clear values.", Parameters: toolSchema(map[string]any{"artifact_id": str, "tab": str, "cells": object, "ifRev": integer}, "artifact_id", "tab", "cells", "ifRev")},
		{Name: "read_range", Description: "Read spreadsheet tabs/cells with calculated values. Optional tab filters one tab; ranges use cell addresses.", Parameters: toolSchema(map[string]any{"artifact_id": str, "tab": str, "range": str}, "artifact_id")},
		{Name: "delete_artifact", Description: "Delete a spreadsheet or slide deck by its artifact_id. Only delete what the user asked to delete. Pauses for user approval; do not claim completion until successful.", Parameters: toolSchema(map[string]any{"artifact_id": str}, "artifact_id")},
	}
	result := []provider.Tool{}
	for _, d := range definitions {
		result = append(result, provider.Tool{Type: "function", Function: d})
	}
	return result
}
func (s *Service) artifactTool(ctx context.Context, run storage.Run, call provider.ToolCall) (string, error) {
	if call.Function.Name == "create_sheet" {
		var args struct {
			Path string      `json:"path"`
			Name string      `json:"name"`
			Tabs []sheet.Tab `json:"tabs"`
		}
		if err := decodeArguments(call.Function.Arguments, &args); err != nil {
			return "", err
		}
		raw, _ := json.Marshal(sheet.Book{Tabs: args.Tabs})
		record, err := s.Store.CreateArtifact(ctx, run.ChatID, "sheet", args.Name, raw, "agent")
		if err != nil {
			return "", err
		}
		result, _ := json.Marshal(map[string]any{"id": record.ID, "rev": record.Rev, "message": "Use read_range to inspect cells and set_cells with ifRev to update."})
		return string(result), nil
	}
	var args struct {
		ID    string                `json:"artifact_id"`
		Tab   string                `json:"tab"`
		Range string                `json:"range"`
		Cells map[string]sheet.Cell `json:"cells"`
		Rev   int                   `json:"ifRev"`
	}
	if err := decodeArguments(call.Function.Arguments, &args); err != nil {
		return "", err
	}
	record, err := s.Store.ReadArtifact(ctx, args.ID)
	if err != nil || record.ChatID != run.ChatID {
		return "", errors.New("artifact is unavailable in this chat")
	}
	if record.Kind != "sheet" {
		return "", errors.New("choose a spreadsheet artifact")
	}
	var book sheet.Book
	if err = json.Unmarshal(record.Payload, &book); err != nil {
		return "", err
	}
	if call.Function.Name == "set_cells" {
		found := false
		for i := range book.Tabs {
			if book.Tabs[i].Name == args.Tab {
				found = true
				if book.Tabs[i].Cells == nil {
					book.Tabs[i].Cells = map[string]sheet.Cell{}
				}
				for address, cell := range args.Cells {
					book.Tabs[i].Cells[address] = cell
				}
			}
		}
		if !found {
			return "", errors.New("tab does not exist")
		}
		raw, _ := json.Marshal(book)
		record, err = s.Store.SaveArtifact(ctx, record.ID, args.Rev, raw, "agent")
		if err != nil {
			return "", err
		}
	}
	values, err := sheet.Calculate(book)
	if err != nil {
		return "", err
	}
	if args.Tab != "" {
		selected := []sheet.Tab{}
		for _, tab := range book.Tabs {
			if tab.Name == args.Tab {
				selected = append(selected, tab)
			}
		}
		book.Tabs = selected
	}
	if args.Range != "" {
		bounds := strings.Split(args.Range, ":")
		if len(bounds) == 1 {
			bounds = append(bounds, bounds[0])
		}
		if len(bounds) != 2 {
			return "", errors.New("Use a range such as A1:C10")
		}
		left, top, err := excelize.CellNameToCoordinates(bounds[0])
		if err != nil {
			return "", err
		}
		right, bottom, err := excelize.CellNameToCoordinates(bounds[1])
		if err != nil || right < left || bottom < top || (right-left+1)*(bottom-top+1) > 1000 {
			return "", errors.New("Read at most 1000 cells in an ordered range")
		}
		for i := range book.Tabs {
			selected := map[string]sheet.Cell{}
			for address, cell := range book.Tabs[i].Cells {
				col, row, _ := excelize.CellNameToCoordinates(address)
				if col >= left && col <= right && row >= top && row <= bottom {
					selected[address] = cell
				}
			}
			book.Tabs[i].Cells = selected
		}
	}
	filteredValues := map[string]map[string]sheet.Display{}
	for _, tab := range book.Tabs {
		filteredValues[tab.Name] = map[string]sheet.Display{}
		for address := range tab.Cells {
			filteredValues[tab.Name][address] = values[tab.Name][address]
		}
	}
	data, _ := json.Marshal(map[string]any{"id": record.ID, "rev": record.Rev, "workbook": book, "values": filteredValues})
	if len(data) > 256*1024 {
		return "", errors.New("spreadsheet read is too large; choose one tab")
	}
	return string(data), nil
}

func deckTools() []provider.Tool {
	str := map[string]any{"type": "string"}
	definitions := []provider.ToolDefinition{
		{Name: "create_deck", Description: "Create an editable deck. Optional path sets its root-relative .deck.json source file. slides: [{layout:title|bullets|columns|image|chart,title,body:[],left:[],right:[],notes,imageId,chart:{labels:[],values:[]}}]. Keep titles under 100 characters and at most six short points per column.", Parameters: toolSchema(map[string]any{"path": str, "name": str, "slides": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}, "name", "slides")},
		{Name: "read_deck", Description: "Read a deck’s current slides, IDs and revision before updating it.", Parameters: toolSchema(map[string]any{"artifact_id": str}, "artifact_id")},
		{Name: "update_slide", Description: "Replace one slide by stable slide_id with ifRev. Preserve unrelated slides and user edits.", Parameters: toolSchema(map[string]any{"artifact_id": str, "slide_id": str, "ifRev": map[string]any{"type": "integer"}, "slide": map[string]any{"type": "object"}}, "artifact_id", "slide_id", "ifRev", "slide")},
	}
	result := []provider.Tool{}
	for _, definition := range definitions {
		result = append(result, provider.Tool{Type: "function", Function: definition})
	}
	return result
}
func (s *Service) deckTool(ctx context.Context, run storage.Run, call provider.ToolCall) (string, error) {
	var args struct {
		Path    string       `json:"path"`
		Name    string       `json:"name"`
		ID      string       `json:"artifact_id"`
		Slides  []deck.Slide `json:"slides"`
		SlideID string       `json:"slide_id"`
		Slide   deck.Slide   `json:"slide"`
		Rev     int          `json:"ifRev"`
	}
	if err := decodeArguments(call.Function.Arguments, &args); err != nil {
		return "", err
	}
	var record storage.Artifact
	var slides deck.Deck
	var err error
	if call.Function.Name == "create_deck" {
		slides = deck.Deck{Slides: args.Slides}
		if err = deck.Prepare(&slides); err != nil {
			return "", err
		}
		if err = s.validateSlideImages(ctx, run.ChatID, slides); err != nil {
			return "", err
		}
		data, _ := json.Marshal(slides)
		record, err = s.Store.CreateArtifact(ctx, run.ChatID, "deck", args.Name, data, "agent")
	} else {
		record, err = s.Store.ReadArtifact(ctx, args.ID)
		if err != nil || record.ChatID != run.ChatID || record.Kind != "deck" {
			return "", errors.New("deck is unavailable in this chat")
		}
		if err = json.Unmarshal(record.Payload, &slides); err != nil {
			return "", err
		}
		if call.Function.Name == "update_slide" {
			found := false
			for i, slide := range slides.Slides {
				if slide.ID == args.SlideID {
					args.Slide.ID = slide.ID
					slides.Slides[i] = args.Slide
					found = true
				}
			}
			if !found {
				return "", errors.New("slide was deleted; read the current deck")
			}
			if err = deck.Prepare(&slides); err != nil {
				return "", err
			}
			if err = s.validateSlideImages(ctx, run.ChatID, slides); err != nil {
				return "", err
			}
			data, _ := json.Marshal(slides)
			record, err = s.Store.SaveArtifact(ctx, record.ID, args.Rev, data, "agent")
		}
	}
	if err != nil {
		return "", err
	}
	result, _ := json.Marshal(record)
	return string(result), nil
}
func (s *Service) validateSlideImages(ctx context.Context, chat string, slides deck.Deck) error {
	for _, slide := range slides.Slides {
		if slide.Layout == "image" {
			_, mime, err := s.Store.AttachmentData(ctx, chat, slide.ImageID, true)
			if err != nil || !strings.HasPrefix(mime, "image/") {
				return errors.New("image slides must use an image attachment from this chat")
			}
		}
	}
	return nil
}
