package deck

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/ledongthuc/pdf"
)

func example() Deck {
	return Deck{Slides: []Slide{{Layout: "title", Title: "Project plan", Body: []string{"A clear path forward"}, Notes: "Introduce the project."}, {Layout: "bullets", Title: "Goals", Body: []string{"Make the editor reliable", "Keep every change recoverable"}}, {Layout: "columns", Title: "Tradeoffs", Left: []string{"Local ownership"}, Right: []string{"Explicit cloud services"}}, {Layout: "chart", Title: "Budget", Chart: &Chart{Labels: []string{"Build", "Test", "Release"}, Values: []float64{30, 20, 10}}}}}
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
