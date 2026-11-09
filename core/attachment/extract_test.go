package attachment_test

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"kiko/core/attachment"
	"kiko/core/document"
	exports "kiko/core/export"
)

func TestImagesTextAndOfficeExtraction(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 8, 6))
	picture.Set(1, 1, color.RGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	png.Encode(&encoded, picture)
	parsed, err := attachment.Extract("image.png", encoded.Bytes())
	if err != nil || len(parsed.Thumbnail) == 0 || parsed.MIME != "image/png" {
		t.Fatal(err, parsed)
	}
	blocks, _ := document.Parse("# Heading\n\nA **bold** paragraph.\n\n| Name | Value |\n| --- | --- |\n| One | Two |", nil)
	data, err := exports.Docx(document.Document{Title: "Test", Blocks: blocks})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err = attachment.Extract("test.docx", data)
	if err != nil || len(parsed.Pages) != 1 || !bytes.Contains([]byte(parsed.Pages[0].Text), []byte("Heading")) {
		t.Fatal(err, parsed)
	}
	if _, err = attachment.Extract("bad.txt", []byte{0xff, 0}); err == nil {
		t.Fatal("invalid text accepted")
	}
	var bomb bytes.Buffer
	writer := zip.NewWriter(&bomb)
	part, _ := writer.Create("word/document.xml")
	part.Write([]byte(`<!DOCTYPE document [<!ENTITY x "value">]><document><t>&x;</t></document>`))
	writer.Close()
	if _, err = attachment.Extract("unsafe.docx", bomb.Bytes()); err == nil {
		t.Fatal("XML directive accepted")
	}
}
