package attachment

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"testing"
)

func testPDF(transparent ...bool) []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << >> /Contents 4 0 R >>",
		"", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 50] /Resources << >> /Contents 6 0 R >>", "",
	}
	first := "0 0 1 rg 0 0 100 100 re f\n"
	if len(transparent) > 0 && transparent[0] {
		objects[2] = "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /ExtGState << /GS0 << /Type /ExtGState /ca 0.5 /CA 0.5 >> >> >> /Contents 4 0 R >>"
		first = "/GS0 gs " + first
	}
	for index, content := range map[int]string{3: first, 5: "1 0 0 rg 0 0 100 50 re f\n"} {
		objects[index] = fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content)
	}
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	start := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), start)
	return out.Bytes()
}
func TestPDFPagesRenderActualGraphicsWithinBounds(t *testing.T) {
	if rendererRaceTest {
		t.Skip("PDFium JIT integration runs separately without race instrumentation")
	}
	for page := 1; page <= 2; page++ {
		data, err := RenderPDF(context.Background(), testPDF(), page)
		if err != nil {
			t.Fatal(err)
		}
		picture, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		bounds := picture.Bounds()
		if bounds.Dx() > 1600 || bounds.Dy() > 1600 || bounds.Dx() < 1 || bounds.Dy() < 1 {
			t.Fatal(bounds)
		}
		red, _, blue, alpha := picture.At(bounds.Dx()/2, bounds.Dy()/2).RGBA()
		if alpha != 65535 || page == 1 && (blue < 60000 || red > 1000) || page == 2 && (red < 60000 || blue > 1000) {
			t.Fatal("wrong page graphics", page, red, blue, alpha)
		}
		if page == 2 && bounds.Dx() != bounds.Dy()*2 {
			t.Fatal("page aspect ratio changed", bounds)
		}
	}
}
func TestPDFRenderingRejectsInvalidInputAndCancellation(t *testing.T) {
	if _, err := RenderPDF(context.Background(), testPDF(), 0); err == nil {
		t.Fatal("zero-based page accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RenderPDF(ctx, testPDF(), 1); err == nil {
		t.Fatal("canceled render continued")
	}
}
func TestPDFRenderingRejectsMalformedDocument(t *testing.T) {
	if rendererRaceTest {
		t.Skip("PDFium JIT integration runs separately without race instrumentation")
	}
	if _, err := RenderPDF(context.Background(), []byte("%PDF-1.4 invalid PDF"), 1); err == nil {
		t.Fatal("invalid PDF accepted")
	}
}

func TestPDFWorkerCancellationWhileSlotsAreBusy(t *testing.T) {
	renderSlots <- struct{}{}
	renderSlots <- struct{}{}
	defer func() { <-renderSlots; <-renderSlots }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RenderPDFWorker(ctx, testPDF(), 1); err != context.Canceled {
		t.Fatal("queued rendering ignored cancellation", err)
	}
}

func TestPDFTransparencyUsesAnOpaqueWhitePage(t *testing.T) {
	if rendererRaceTest {
		t.Skip("PDFium JIT integration runs separately without race instrumentation")
	}
	data, err := RenderPDF(context.Background(), testPDF(true), 1)
	if err != nil {
		t.Fatal(err)
	}
	picture, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	red, green, blue, alpha := picture.At(picture.Bounds().Dx()/2, picture.Bounds().Dy()/2).RGBA()
	if alpha != 65535 || red < 30000 || red > 35000 || green < 30000 || green > 35000 || blue < 60000 {
		t.Fatal("transparent PDF was not composed onto paper white", red, green, blue, alpha)
	}
}
