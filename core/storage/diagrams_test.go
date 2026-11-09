package storage

import (
	"context"
	"kiko/core/document"
	"testing"
)

func TestDiagramOwnershipAndAtomicConflict(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	original, err := s.CreateDocument(ctx, chat, "Original", "Text", nil, "user")
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := s.CreateDiagram(ctx, original.ID, `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50"><rect width="100" height="50"/></svg>`, "Original drawing", "")
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
