package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kiko/core/document"
)

func sample(t *testing.T) document.Document {
	t.Helper()
	blocks, err := document.Parse("# Project plan\n\n**Bold**, _italic_, ~~old~~, [Source](https://example.com?a=1&b=2), and `code`.\n\n- First\n  - Nested\n- Second\n\n| Name | Cost |\n| --- | --- |\n| Alpha | 12 |\n\n```go\nhello(\"world\")\n```", nil)
	if err != nil {
		t.Fatal(err)
	}
	return document.Document{Title: "Project & plan", Blocks: blocks}
}
func TestDocxHasValidPartsAndFormattedContent(t *testing.T) {
	data, err := Docx(sample(t))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[file.Name] = string(content)
		decoder := xml.NewDecoder(bytes.NewReader(content))
		for {
			_, err = decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(file.Name, err)
			}
		}
	}
	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/styles.xml", "word/_rels/document.xml.rels"} {
		if parts[required] == "" {
			t.Fatal("missing part", required)
		}
	}
	for _, text := range []string{"<w:b/>", "<w:i/>", "<w:strike/>", "<w:tbl>", "Nested", "&amp;", "w:hyperlink"} {
		if !strings.Contains(parts["word/document.xml"]+parts["word/_rels/document.xml.rels"], text) {
			t.Fatal("missing content", text)
		}
	}
}
func TestTypstQuotesContentInsteadOfEvaluatingIt(t *testing.T) {
	doc := sample(t)
	doc.Title = `#read("/etc/passwd")`
	doc.Blocks = append(doc.Blocks, document.Block{Type: "paragraph", Content: []document.Inline{{Text: `#read("/etc/passwd")`}}})
	source := Typst(doc)
	if strings.Contains(source, `#read("/etc/passwd")`) {
		t.Fatal("raw executable markup escaped its string")
	}
	for _, value := range []string{"#heading(level: 1)", "#strong[", "#table(columns: 2", "#list(", "#raw("} {
		if !strings.Contains(source, value) {
			t.Fatal("missing structure", value)
		}
	}
}

func TestPDFWithConfiguredCompiler(t *testing.T) {
	if os.Getenv("KIKO_TYPST_BINARY") == "" {
		t.Skip("set KIKO_TYPST_BINARY for the bundled compiler integration check")
	}
	data, err := PDF(context.Background(), sample(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("invalid PDF output")
	}
	if err = os.WriteFile(filepath.Join(t.TempDir(), "sample.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPDFWithStaticDiagram(t *testing.T) {
	if os.Getenv("KIKO_TYPST_BINARY") == "" {
		t.Skip("bundled compiler integration")
	}
	doc := sample(t)
	svg, err := document.SanitizeSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 120"><rect x="10" y="10" width="380" height="100" fill="#eeeeee"/><text x="30" y="60">Simple flow</text></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	doc.Blocks = append(doc.Blocks, document.Block{Type: "embed", Attrs: map[string]any{"ref": "diagram", "svg": svg, "caption": "A safe diagram"}})
	if _, err = PDF(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
}

func TestPDFAndDocxWithExcalidrawDiagram(t *testing.T) {
	if os.Getenv("KIKO_TYPST_BINARY") == "" {
		t.Skip("bundled compiler integration")
	}
	source, err := os.ReadFile("../document/testdata/excalidraw-export.svg")
	if err != nil {
		t.Fatal(err)
	}
	svg, err := document.SanitizeSVG(string(source))
	if err != nil {
		t.Fatal(err)
	}
	doc := document.Document{Title: "Flow", Blocks: []document.Block{{Type: "embed", Attrs: map[string]any{"ref": "drawn", "svg": svg, "caption": "Client to server"}}}}
	pdf, err := PDF(context.Background(), doc)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("PDF rejected the Excalidraw export", err)
	}
	if _, err = Docx(doc); err != nil {
		t.Fatal("DOCX rejected the Excalidraw export", err)
	}
}

func TestDocxDiagramIncludesRenderedMedia(t *testing.T) {
	if os.Getenv("KIKO_TYPST_BINARY") == "" {
		t.Skip("bundled compiler integration")
	}
	doc := document.Document{Title: "Diagram", Blocks: []document.Block{{Type: "embed", Attrs: map[string]any{"ref": "node", "svg": `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100"><rect width="200" height="100" fill="#eeeeee"/></svg>`, "caption": "A rectangle"}}}}
	data, err := Docx(doc)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, part := range reader.File {
		if strings.HasPrefix(part.Name, "word/media/") {
			stream, _ := part.Open()
			media, _ := io.ReadAll(stream)
			stream.Close()
			if !bytes.HasPrefix(media, []byte("\x89PNG")) {
				t.Fatal("diagram image is invalid")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("diagram was silently omitted")
	}
}
