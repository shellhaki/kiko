package storage

import (
	"context"
	"kiko/core/document"
	"strings"
	"testing"
)

func TestDiagramOwnershipAndAtomicConflict(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	original, err := s.CreateDocument(ctx, chat, "Original", "Text", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := s.CreateDiagram(ctx, original.ID, sketch(), "Original drawing", "")
	if err != nil {
		t.Fatal(err)
	}
	embed := drawn.Blocks[len(drawn.Blocks)-1]
	reference := document.String(embed.Attrs, "ref")
	if _, err = s.Diagram(ctx, "another-chat", reference); err == nil {
		t.Fatal("cross-chat diagram exposed")
	}
	second, err := s.CreateDocument(ctx, chat, "Second", "Keep me", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveDocument(ctx, second.ID, 1, []document.Block{embed}); err == nil {
		t.Fatal("another document claimed the diagram")
	}
	read, err := s.ReadDocument(ctx, second.ID, ReadOptions{})
	if err != nil || read.Document.Rev != 1 || read.Markdown != "Keep me" {
		t.Fatal("failed save changed document", read, err)
	}
	if document.String(drawn.Blocks[len(drawn.Blocks)-1].Attrs, "svg") == "" {
		t.Fatal("diagram omitted")
	}
}

func TestDiagramSceneSurvivesSaveAndReload(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	original, err := s.CreateDocument(ctx, chat, "Original", "Text", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := s.CreateDiagram(ctx, original.ID, sketch(), "Sketch", "")
	if err != nil {
		t.Fatal(err)
	}
	scene := `{"type":"excalidraw","version":2,"elements":[{"id":"a","type":"rectangle"}]}`
	blocks := drawn.Blocks
	blocks[len(blocks)-1].Attrs["scene"] = scene
	if _, err = s.SaveDocument(ctx, original.ID, drawn.Rev, blocks); err != nil {
		t.Fatal(err)
	}
	read, err := s.ReadDocument(ctx, original.ID, ReadOptions{})
	if err != nil || document.String(read.Document.Blocks[len(read.Document.Blocks)-1].Attrs, "scene") != scene {
		t.Fatal("scene lost after reload", err)
	}
	blocks = read.Document.Blocks
	blocks[len(blocks)-1].Attrs["scene"] = "not json"
	if _, err = s.SaveDocument(ctx, original.ID, read.Document.Rev, blocks); err == nil {
		t.Fatal("invalid scene saved")
	}
}

func sketch() []document.Element {
	width, height := 160.0, 60.0
	return []document.Element{{Type: "rectangle", ID: "a", X: 0, Y: 0, Width: &width, Height: &height, Label: &document.Label{Text: "Start"}}}
}

func TestCreateDiagramStoresEditableSceneAndRenderedSVG(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	original, err := s.CreateDocument(ctx, chat, "Original", "Text", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := s.CreateDiagram(ctx, original.ID, sketch(), "Start here", "")
	if err != nil {
		t.Fatal(err)
	}
	attrs := drawn.Blocks[len(drawn.Blocks)-1].Attrs
	if document.ValidateScene(document.String(attrs, "scene")) != nil || !strings.Contains(document.String(attrs, "svg"), "Start") {
		t.Fatal("diagram lacks its scene or rendered SVG", attrs)
	}
	svg, err := s.Diagram(ctx, chat, document.String(attrs, "ref"))
	if err != nil || svg != document.String(attrs, "svg") {
		t.Fatal("screenshot source differs from the stored SVG", err)
	}
	if _, err = s.CreateDiagram(ctx, original.ID, nil, "Empty", ""); err == nil {
		t.Fatal("empty diagram accepted")
	}
}
