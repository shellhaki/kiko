package attachment

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
	_ "golang.org/x/image/webp"
)

const MaxFile = 20 * 1024 * 1024
const MaxText = 2 * 1024 * 1024

type Page struct {
	Name string `json:"name"`
	Text string `json:"text"`
}
type Extracted struct {
	MIME      string `json:"mime"`
	Pages     []Page `json:"pages"`
	Thumbnail []byte `json:"-"`
}

func PDF(data []byte) (pages []Page, err error) {
	defer func() {
		if recover() != nil {
			pages = nil
			err = errors.New("This PDF could not be read safely.")
		}
	}()
	if len(data) > MaxFile {
		return nil, errors.New("file exceeds 20 MB")
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("This PDF is invalid, encrypted, or unsupported.")
	}
	count := reader.NumPage()
	if count < 1 || count > 200 {
		return nil, errors.New("PDF must contain 1–200 pages")
	}
	total := 0
	pages = []Page{}
	for i := 1; i <= count; i++ {
		text, err := reader.Page(i).GetPlainText(nil)
		if err != nil {
			return nil, errors.New("Couldn’t extract this PDF’s text.")
		}
		total += len(text)
		if total > MaxText {
			return nil, errors.New("extracted text exceeds 2 MB")
		}
		pages = append(pages, Page{Name: fmt.Sprintf("Page %d", i), Text: text})
	}
	return pages, nil
}
func Extract(name string, data []byte) (result Extracted, err error) {
	defer func() {
		if recover() != nil {
			result = Extracted{}
			err = errors.New("This file could not be parsed safely.")
		}
	}()
	if len(data) == 0 || len(data) > MaxFile {
		return result, errors.New("Choose a nonempty file up to 20 MB.")
	}
	extension := strings.ToLower(filepath.Ext(name))
	result.Pages = []Page{}
	switch extension {
	case ".png", ".jpg", ".jpeg", ".webp":
		result.MIME = http.DetectContentType(data)
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 20000 || config.Height > 20000 || int64(config.Width)*int64(config.Height) > 40000000 {
			return result, errors.New("Image dimensions are invalid or too large.")
		}
		source, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return result, err
		}
		width, height := config.Width, config.Height
		ratio := float64(width) / 1024
		if height > width {
			ratio = float64(height) / 1024
		}
		if ratio > 1 {
			width = int(float64(width) / ratio)
			height = int(float64(height) / ratio)
		}
		thumb := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				thumb.Set(x, y, source.At(x*config.Width/width, y*config.Height/height))
			}
		}
		var encoded bytes.Buffer
		if err = png.Encode(&encoded, thumb); err != nil {
			return result, err
		}
		result.Thumbnail = encoded.Bytes()
		result.Pages = []Page{{Name: "Image", Text: "Image attachment; use describe_image for visual content."}}
	case ".pdf":
		result.MIME = "application/pdf"
		result.Pages, err = PDF(data)
	case ".docx", ".pptx":
		result.MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		if extension == ".pptx" {
			result.MIME = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		}
		result.Pages, err = office(data, extension)
	case ".xlsx":
		result.MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		book, openErr := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 64 * 1024 * 1024, UnzipXMLSizeLimit: 8 * 1024 * 1024})
		if openErr != nil {
			return result, errors.New("Couldn’t read this spreadsheet.")
		}
		defer book.Close()
		if len(book.GetSheetList()) > 50 {
			return result, errors.New("spreadsheet exceeds 50 sheets")
		}
		total := 0
		for _, sheet := range book.GetSheetList() {
			rows, err := book.Rows(sheet)
			if err != nil {
				return result, err
			}
			var out strings.Builder
			row := 0
			for rows.Next() {
				row++
				if row > 10000 {
					rows.Close()
					return result, errors.New("spreadsheet exceeds 10000 rows")
				}
				columns, err := rows.Columns()
				if err != nil {
					rows.Close()
					return result, err
				}
				if len(columns) > 256 {
					rows.Close()
					return result, errors.New("spreadsheet exceeds 256 columns")
				}
				for i, value := range columns {
					cell, _ := excelize.CoordinatesToCellName(i+1, row)
					formula, _ := book.GetCellFormula(sheet, cell)
					if formula != "" {
						value += " (=" + formula + ")"
					}
					out.WriteString(strings.ReplaceAll(value, "|", "\\|") + " | ")
				}
				out.WriteByte('\n')
				if out.Len()+total > MaxText {
					rows.Close()
					return result, errors.New("extracted text exceeds 2 MB")
				}
			}
			rowErr := rows.Error()
			rows.Close()
			if rowErr != nil {
				return result, rowErr
			}
			total += out.Len()
			result.Pages = append(result.Pages, Page{Name: sheet, Text: out.String()})
		}
	case ".csv":
		result.MIME = "text/csv"
		reader := csv.NewReader(bytes.NewReader(data))
		reader.FieldsPerRecord = -1
		var out strings.Builder
		count := 0
		for {
			row, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return result, errors.New("CSV is malformed.")
			}
			count++
			if count > 10000 || len(row) > 256 {
				return result, errors.New("CSV exceeds 10000 rows or 256 columns")
			}
			for _, value := range row {
				out.WriteString(strings.ReplaceAll(value, "|", "\\|") + " | ")
			}
			out.WriteByte('\n')
			if out.Len() > MaxText {
				return result, errors.New("extracted text exceeds 2 MB")
			}
		}
		result.Pages = []Page{{Name: "Rows", Text: out.String()}}
	case ".md", ".txt", ".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".rs", ".json", ".yaml", ".yml", ".html", ".css", ".sql":
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 || len(data) > MaxText {
			return result, errors.New("text must be UTF-8 and up to 2 MB")
		}
		result.MIME = "text/plain"
		result.Pages = []Page{{Name: "Text", Text: string(data)}}
	default:
		return result, errors.New("Supported files: images, PDF, DOCX, XLSX, PPTX, CSV, Markdown, text, and source code.")
	}
	return result, err
}
func office(data []byte, extension string) ([]Page, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("invalid Office archive")
	}
	files := []*zip.File{}
	var total uint64
	for _, file := range archive.File {
		total += file.UncompressedSize64
		if total > 64*1024*1024 || len(archive.File) > 5000 {
			return nil, errors.New("Office archive expands beyond the import limit")
		}
		if extension == ".docx" && file.Name == "word/document.xml" || extension == ".pptx" && strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		var left, right int
		fmt.Sscanf(filepath.Base(files[i].Name), "slide%d.xml", &left)
		fmt.Sscanf(filepath.Base(files[j].Name), "slide%d.xml", &right)
		return left < right
	})
	if len(files) == 0 || len(files) > 200 {
		return nil, errors.New("Office document has no readable content or too many slides")
	}
	pages := []Page{}
	textTotal := 0
	for i, file := range files {
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		decoder := xml.NewDecoder(io.LimitReader(stream, 8*1024*1024+1))
		var out strings.Builder
		inside := false
		depth := 0
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				stream.Close()
				return nil, errors.New("Office XML is invalid or too large")
			}
			switch value := token.(type) {
			case xml.StartElement:
				depth++
				if depth > 100 {
					stream.Close()
					return nil, errors.New("Office XML nesting exceeds limit")
				}
				if value.Name.Local == "t" {
					inside = true
				}
				if value.Name.Local == "tab" {
					out.WriteByte('\t')
				}
			case xml.EndElement:
				depth--
				if value.Name.Local == "t" {
					inside = false
				}
				if value.Name.Local == "p" || value.Name.Local == "tr" {
					out.WriteByte('\n')
				}
				if value.Name.Local == "tc" {
					out.WriteString(" | ")
				}
			case xml.CharData:
				if inside {
					out.Write(value)
				}
			case xml.Directive:
				stream.Close()
				return nil, errors.New("XML directives are not supported")
			}
			if out.Len()+textTotal > MaxText {
				stream.Close()
				return nil, errors.New("extracted text exceeds 2 MB")
			}
		}
		stream.Close()
		name := "Document"
		if extension == ".pptx" {
			name = fmt.Sprintf("Slide %d", i+1)
		}
		textTotal += out.Len()
		pages = append(pages, Page{Name: name, Text: out.String()})
	}
	return pages, nil
}
