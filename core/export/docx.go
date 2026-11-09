package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"image/png"
	"io"
	"strconv"
	"strings"

	"kiko/core/document"
)

func escape(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
func Docx(doc document.Document) ([]byte, error) {
	if err := document.Prepare(doc.Blocks); err != nil {
		return nil, err
	}
	links := []string{}
	media := map[string][]byte{}
	diagrams := map[string]string{}
	dimensions := map[string][2]int{}
	for _, block := range document.Flatten(doc.Blocks) {
		if block.Type == "embed" {
			image, err := SVGPNG(context.Background(), document.String(block.Attrs, "svg"))
			if err != nil {
				return nil, err
			}
			name := fmt.Sprintf("diagram%d.png", len(media)+1)
			media[name] = image
			diagrams[block.ID] = name
			config, err := png.DecodeConfig(bytes.NewReader(image))
			if err != nil {
				return nil, err
			}
			width := 5486400
			height := width * config.Height / config.Width
			if height > 7500000 {
				width = width * 7500000 / height
				height = 7500000
			}
			dimensions[block.ID] = [2]int{width, height}
		}
	}
	runs := func(content []document.Inline) string {
		var out strings.Builder
		for _, inline := range content {
			properties := ""
			link := ""
			for _, mark := range inline.Marks {
				switch mark.Type {
				case "bold":
					properties += "<w:b/>"
				case "italic":
					properties += "<w:i/>"
				case "strike":
					properties += "<w:strike/>"
				case "code":
					properties += `<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/><w:shd w:fill="F0F0F0"/>`
				case "link":
					if document.SafeLink(mark.Href) {
						links = append(links, mark.Href)
						link = "link" + strconv.Itoa(len(links))
						properties += `<w:rStyle w:val="Hyperlink"/>`
					}
				}
			}
			value := "<w:r>"
			if properties != "" {
				value += "<w:rPr>" + properties + "</w:rPr>"
			}
			for i, line := range strings.Split(inline.Text, "\n") {
				if i > 0 {
					value += "<w:br/>"
				}
				value += `<w:t xml:space="preserve">` + escape(line) + "</w:t>"
			}
			value += "</w:r>"
			if link != "" {
				value = `<w:hyperlink r:id="` + link + `">` + value + `</w:hyperlink>`
			}
			out.WriteString(value)
		}
		return out.String()
	}
	var blocks func([]document.Block, int, string) string
	blocks = func(list []document.Block, depth int, prefix string) string {
		var out strings.Builder
		for _, b := range list {
			properties := ""
			content := b.Content
			switch b.Type {
			case "embed":
				name := diagrams[b.ID]
				size := dimensions[b.ID]
				width, height := strconv.Itoa(size[0]), strconv.Itoa(size[1])
				out.WriteString(`<w:p><w:r><w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:extent cx="` + width + `" cy="` + height + `"/><wp:docPr id="` + strconv.Itoa(len(out.String())+1) + `" name="Diagram"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="0" name="Diagram"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="` + name + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + width + `" cy="` + height + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p><w:p>` + runs([]document.Inline{{Text: document.String(b.Attrs, "caption")}}) + `</w:p>`)
				continue
			case "heading":
				properties = `<w:pStyle w:val="Heading` + strconv.Itoa(document.Number(b.Attrs, "level", 1)) + `"/>`
			case "code":
				properties = `<w:pStyle w:val="Code"/>`
			case "pending":
				content = []document.Inline{{Text: "Pending: " + document.String(b.Attrs, "intent"), Marks: []document.Mark{{Type: "italic"}}}}
			case "list":
				ordered, _ := b.Attrs["ordered"].(bool)
				start := document.Number(b.Attrs, "start", 1)
				for i, item := range b.Children {
					marker := "• "
					if ordered {
						marker = strconv.Itoa(start+i) + ". "
					}
					if checked, ok := item.Attrs["checked"].(bool); ok {
						marker = "☐ "
						if checked {
							marker = "☑ "
						}
					}
					out.WriteString(blocks(item.Children, depth+1, marker))
				}
				continue
			case "table":
				out.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4" w:color="DDDDDD"/><w:left w:val="single" w:sz="4" w:color="DDDDDD"/><w:bottom w:val="single" w:sz="4" w:color="DDDDDD"/><w:right w:val="single" w:sz="4" w:color="DDDDDD"/><w:insideH w:val="single" w:sz="4" w:color="DDDDDD"/><w:insideV w:val="single" w:sz="4" w:color="DDDDDD"/></w:tblBorders></w:tblPr>`)
				for _, row := range b.Children {
					out.WriteString("<w:tr>")
					header, _ := row.Attrs["header"].(bool)
					if header {
						out.WriteString("<w:trPr><w:tblHeader/></w:trPr>")
					}
					for _, cell := range row.Children {
						out.WriteString("<w:tc><w:tcPr>")
						if header {
							out.WriteString(`<w:shd w:fill="F3F3F3"/>`)
						}
						out.WriteString("</w:tcPr><w:p>" + runs(cell.Content) + "</w:p></w:tc>")
					}
					out.WriteString("</w:tr>")
				}
				out.WriteString("</w:tbl>")
				continue
			}
			if depth > 0 {
				properties += fmt.Sprintf(`<w:ind w:left="%d"/>`, depth*360)
			}
			if prefix != "" {
				content = append([]document.Inline{{Text: prefix}}, content...)
				prefix = ""
			}
			if b.Type == "code" {
				content = []document.Inline{{Text: document.Plain(b), Marks: []document.Mark{{Type: "code"}}}}
			}
			out.WriteString("<w:p><w:pPr>" + properties + "</w:pPr>" + runs(content) + "</w:p>")
		}
		return out.String()
	}
	body := blocks(doc.Blocks, 0, "")
	rels := `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="styles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`
	for i, link := range links {
		rels += fmt.Sprintf(`<Relationship Id="link%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`, i+1, escape(link))
	}
	for name := range media {
		rels += `<Relationship Id="` + name + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/` + name + `"/>`
	}
	rels += "</Relationships>"
	styles := `<?xml version="1.0" encoding="UTF-8"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style><w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:rPr><w:color w:val="7042BF"/><w:u w:val="single"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:pPr><w:shd w:fill="F5F5F5"/></w:pPr></w:style>`
	for level := 1; level <= 6; level++ {
		styles += fmt.Sprintf(`<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="280" w:after="140"/><w:outlineLvl w:val="%d"/></w:pPr><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr></w:style>`, level, level, level-1, 38-level*2)
	}
	styles += "</w:styles>"
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="document" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` + body + `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1134"/><w:pgNumType w:start="1"/></w:sectPr></w:body></w:document>`,
		"word/styles.xml":     styles, "word/_rels/document.xml.rels": rels,
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, content := range parts {
		file, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = io.WriteString(file, content); err != nil {
			return nil, err
		}
	}
	for name, data := range media {
		file, err := writer.Create("word/media/" + name)
		if err != nil {
			return nil, err
		}
		if _, err = file.Write(data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
