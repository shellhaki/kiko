package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"kiko/core/document"
)

func typstString(value string) string {
	return strconv.Quote(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return '�'
		}
		return r
	}, value))
}
func Typst(doc document.Document) string {
	inline := func(content []document.Inline) string {
		var out strings.Builder
		for _, part := range content {
			value := "#text(" + typstString(part.Text) + ")"
			for i := len(part.Marks) - 1; i >= 0; i-- {
				mark := part.Marks[i]
				switch mark.Type {
				case "bold":
					value = "#strong[" + value + "]"
				case "italic":
					value = "#emph[" + value + "]"
				case "strike":
					value = "#strike[" + value + "]"
				case "code":
					value = "#raw(" + typstString(part.Text) + ")"
				case "link":
					value = "#link(" + typstString(mark.Href) + ")[" + value + "]"
				}
			}
			out.WriteString(value)
		}
		return out.String()
	}
	var blocks func([]document.Block) string
	blocks = func(list []document.Block) string {
		var out strings.Builder
		for _, b := range list {
			value := inline(b.Content)
			switch b.Type {
			case "embed":
				value = "#figure(image(bytes(" + typstString(document.String(b.Attrs, "svg")) + "), format: \"svg\", width: 100%), caption: [#text(" + typstString(document.String(b.Attrs, "caption")) + ")])"
			case "heading":
				value = fmt.Sprintf("#heading(level: %d)[%s]", document.Number(b.Attrs, "level", 1), value)
			case "code":
				value = "#raw(" + typstString(document.Plain(b)) + ", block: true, lang: " + typstString(document.String(b.Attrs, "language")) + ")"
			case "pending":
				value = "#block(fill: luma(95%), inset: 10pt, width: 100%)[#emph[#text(" + typstString("Pending: "+document.String(b.Attrs, "intent")) + ")]]"
			case "list":
				ordered, _ := b.Attrs["ordered"].(bool)
				kind := "#list("
				if ordered {
					kind = fmt.Sprintf("#enum(start: %d,", document.Number(b.Attrs, "start", 1))
				}
				items := []string{}
				for _, item := range b.Children {
					prefix := ""
					if checked, ok := item.Attrs["checked"].(bool); ok {
						prefix = "#text(\"☐ \" )"
						if checked {
							prefix = "#text(\"☑ \" )"
						}
					}
					items = append(items, "["+prefix+blocks(item.Children)+"]")
				}
				value = kind + strings.Join(items, ",") + ")"
			case "table":
				if len(b.Children) == 0 {
					continue
				}
				columns := len(b.Children[0].Children)
				if columns == 0 {
					continue
				}
				cells := []string{}
				for _, row := range b.Children {
					for _, cell := range row.Children {
						cells = append(cells, "["+inline(cell.Content)+"]")
					}
				}
				value = fmt.Sprintf("#table(columns: %d, stroke: 0.5pt + luma(80%%), inset: 7pt, %s)", columns, strings.Join(cells, ","))
			}
			out.WriteString(value + "\n\n")
		}
		return out.String()
	}
	return "#set document(title: " + typstString(doc.Title) + ")\n#set page(paper: \"a4\", margin: 20mm, numbering: \"1\")\n#set text(size: 11pt)\n#set par(leading: 0.65em)\n#set heading(numbering: none)\n" + blocks(doc.Blocks)
}
func TypstBinary() string {
	if configured := os.Getenv("KIKO_TYPST_BINARY"); configured != "" {
		return configured
	}
	if executable, err := os.Executable(); err == nil {
		for _, candidate := range []string{filepath.Join(filepath.Dir(executable), "typst"), filepath.Join(filepath.Dir(executable), "..", "Resources", "typst"), filepath.Join(filepath.Dir(executable), "typst.exe")} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	path, _ := exec.LookPath("typst")
	return path
}
func PDF(ctx context.Context, doc document.Document) ([]byte, error) {
	if err := document.Prepare(doc.Blocks); err != nil {
		return nil, err
	}
	binary := TypstBinary()
	if binary == "" {
		return nil, errors.New("PDF export needs the bundled Typst compiler. Run the documented runtime setup before packaging.")
	}
	dir, err := os.MkdirTemp("", "kiko-pdf-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	source := filepath.Join(dir, "document.typ")
	target := filepath.Join(dir, "document.pdf")
	if err = os.WriteFile(source, []byte(Typst(doc)), 0600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "compile", "--root", dir, source, target)
	command.Dir = dir
	if err = command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("Couldn’t typeset this PDF. The source document is unchanged.")
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if info.Size() > 64*1024*1024 {
		return nil, errors.New("PDF output exceeds 64 MB")
	}
	return os.ReadFile(target)
}
func Write(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".kiko-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closed := file.Close()
	if err != nil {
		return err
	}
	if closed != nil {
		return closed
	}
	return os.Rename(file.Name(), path)
}

func SVGPNG(ctx context.Context, source string) ([]byte, error) {
	sanitized, err := document.SanitizeSVG(source)
	if err != nil {
		return nil, err
	}
	binary := TypstBinary()
	if binary == "" {
		return nil, errors.New("Diagram rendering needs the bundled Typst compiler.")
	}
	dir, err := os.MkdirTemp("", "kiko-diagram-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	markup := "#set page(width: auto, height: auto, margin: 0pt)\n#image(bytes(" + typstString(sanitized) + "), format: \"svg\", width: 600pt)"
	input := filepath.Join(dir, "diagram.typ")
	output := filepath.Join(dir, "diagram.png")
	if err = os.WriteFile(input, []byte(markup), 0600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "compile", "--root", dir, "--ppi", "96", input, output)
	command.Dir = dir
	if err = command.Run(); err != nil {
		return nil, errors.New("Couldn’t render the diagram.")
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	if info.Size() > 8*1024*1024 {
		return nil, errors.New("Rendered diagram exceeds 8 MB.")
	}
	return os.ReadFile(output)
}

func Quote(value string) string { return typstString(value) }
func CompilePDF(ctx context.Context, source string, assets map[string][]byte) ([]byte, error) {
	binary := TypstBinary()
	if binary == "" {
		return nil, errors.New("PDF export requires the bundled Typst compiler.")
	}
	dir, err := os.MkdirTemp("", "kiko-export-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	total := 0
	for name, data := range assets {
		if name == "" || strings.ContainsAny(name, `/\\`) || name == "." || name == ".." {
			return nil, errors.New("invalid export asset name")
		}
		total += len(data)
		if total > 128*1024*1024 {
			return nil, errors.New("export assets exceed 128 MB")
		}
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return nil, err
		}
	}
	input, output := filepath.Join(dir, "presentation.typ"), filepath.Join(dir, "presentation.pdf")
	if err = os.WriteFile(input, []byte(source), 0600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "compile", "--root", dir, input, output)
	command.Dir = dir
	if err = command.Run(); err != nil {
		return nil, errors.New("Couldn’t typeset this export. The saved artifact is unchanged.")
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	if info.Size() > 128*1024*1024 {
		return nil, errors.New("export exceeds 128 MB")
	}
	return os.ReadFile(output)
}
