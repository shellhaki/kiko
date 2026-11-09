package deck

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ledongthuc/pdf"
)

func example() Deck {
	return Deck{Slides: []Slide{{Layout: "title", Title: "Project plan", Body: []string{"A clear path forward"}, Notes: "Introduce the project."}, {Layout: "bullets", Title: "Goals", Body: []string{"Make the editor reliable", "Keep every change recoverable"}}, {Layout: "columns", Title: "Tradeoffs", Left: []string{"Local ownership"}, Right: []string{"Explicit cloud services"}}, {Layout: "chart", Title: "Budget", Chart: &Chart{Labels: []string{"Build", "Test", "Release"}, Values: []float64{30, 20, 10}}}}}
}
func TestPPTXContainsEditableSlidesNotesAndValidRelationships(t *testing.T) {
	data, err := PPTX(example(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, file := range reader.File {
		stream, _ := file.Open()
		content, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[file.Name] = string(content)
		if !bytes.HasSuffix([]byte(file.Name), []byte(".xml")) && !bytes.HasSuffix([]byte(file.Name), []byte(".rels")) {
			continue
		}
		decoder := xml.NewDecoder(bytes.NewReader(content))
		for {
			_, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(file.Name, err)
			}
		}
	}
	for _, name := range []string{"ppt/presentation.xml", "ppt/theme/theme1.xml", "ppt/slides/slide4.xml", "ppt/notesSlides/notesSlide1.xml", "ppt/slideMasters/slideMaster1.xml"} {
		if parts[name] == "" {
			t.Fatal("missing part", name)
		}
	}
	if !bytes.Contains([]byte(parts["ppt/slides/slide2.xml"]), []byte("Keep every change recoverable")) {
		t.Fatal("editable text missing")
	}
	if !bytes.Contains([]byte(parts["ppt/notesSlides/notesSlide1.xml"]), []byte("Introduce the project")) {
		t.Fatal("notes missing")
	}
}
func TestDeckPDFHasOnePagePerSlide(t *testing.T) {
	if os.Getenv("KIKO_TYPST_BINARY") == "" {
		t.Skip("set bundled compiler for integration")
	}
	data, err := PDF(context.Background(), example(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || reader.NumPage() != 4 {
		t.Fatal(err, reader.NumPage())
	}
}

func TestImageSlideHasMediaAndScopedResolver(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 320, 160))); err != nil {
		t.Fatal(err)
	}
	slides := Deck{Slides: []Slide{{Layout: "image", Title: "Image", ImageID: "local-image"}}}
	resolver := func(id string) ([]byte, error) {
		if id != "local-image" {
			t.Fatal("wrong image reference")
		}
		return encoded.Bytes(), nil
	}
	data, err := PPTX(slides, resolver)
	if err != nil {
		t.Fatal(err)
	}
	reader, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	found := false
	for _, part := range reader.File {
		if strings.HasPrefix(part.Name, "ppt/media/") {
			found = true
		}
	}
	if !found {
		t.Fatal("slide image omitted")
	}
	if os.Getenv("KIKO_TYPST_BINARY") != "" {
		data, err = PDF(context.Background(), slides, resolver)
		if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
			t.Fatal("image PDF failed", err)
		}
	}
}
