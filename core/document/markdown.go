package document

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	ext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type Placeholder struct {
	Type   string `json:"type"`
	Intent string `json:"intent"`
}

var pendingToken = regexp.MustCompile(`<\?kiko block ([a-zA-Z0-9_.-]+)\?>`)

func Parse(markdown string, placeholders map[string]Placeholder) ([]Block, error) {
	if len(markdown) > 4*1024*1024 || !utf8.ValidString(markdown) {
		return nil, errors.New("document must be valid UTF-8 and smaller than 4 MB")
	}
	sentinels := map[string]Placeholder{}
	var tokenError error
	markdown = pendingToken.ReplaceAllStringFunc(markdown, func(token string) string {
		key := pendingToken.FindStringSubmatch(token)[1]
		p, ok := placeholders[key]
		if !ok || p.Type != "pending" || strings.TrimSpace(p.Intent) == "" {
			tokenError = errors.New("invalid pending block token")
		}
		marker := "KIKOPENDING" + key + "TOKEN"
		sentinels[marker] = p
		return "\n\n" + marker + "\n\n"
	})
	if tokenError != nil {
		return nil, tokenError
	}
	source := []byte(markdown)
	parser := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList))
	root := parser.Parser().Parse(text.NewReader(source))
	var convert func(ast.Node) (Block, error)
	convert = func(n ast.Node) (Block, error) {
		b := Block{Attrs: map[string]any{}}
		switch node := n.(type) {
		case *ast.Heading:
			b.Type = "heading"
			b.Attrs["level"] = node.Level
		case *ast.Paragraph, *ast.TextBlock:
			b.Type = "paragraph"
		case *ast.List:
			b.Type = "list"
			b.Attrs["ordered"] = node.IsOrdered()
			if node.IsOrdered() {
				b.Attrs["start"] = node.Start
			}
		case *ast.ListItem:
			b.Type = "listItem"
		case *ast.FencedCodeBlock:
			b.Type = "code"
			if language := string(node.Language(source)); language != "" {
				b.Attrs["language"] = language
			}
			var value strings.Builder
			for i := 0; i < node.Lines().Len(); i++ {
				segment := node.Lines().At(i)
				value.Write(segment.Value(source))
			}
			b.Content = []Inline{{Text: strings.TrimSuffix(value.String(), "\n")}}
			return b, nil
		case *ast.CodeBlock:
			b.Type = "code"
			var value strings.Builder
			for i := 0; i < node.Lines().Len(); i++ {
				segment := node.Lines().At(i)
				value.Write(segment.Value(source))
			}
			b.Content = []Inline{{Text: strings.TrimSuffix(value.String(), "\n")}}
			return b, nil
		case *ext.Table:
			b.Type = "table"
		case *ext.TableHeader:
			b.Type = "row"
			b.Attrs["header"] = true
		case *ext.TableRow:
			b.Type = "row"
		case *ext.TableCell:
			b.Type = "cell"
			b.Attrs["align"] = node.Alignment.String()
		default:
			return b, fmt.Errorf("unsupported Markdown block: %s", n.Kind())
		}
		if b.Type == "paragraph" || b.Type == "heading" || b.Type == "cell" {
			content, err := parseInline(n, source, nil)
			if err != nil {
				return b, err
			}
			b.Content = content
			if p, ok := sentinels[Plain(b)]; ok {
				b.Type = "pending"
				b.Content = nil
				b.Attrs = map[string]any{"intent": p.Intent}
			}
		} else {
			for child := n.FirstChild(); child != nil; child = child.NextSibling() {
				c, err := convert(child)
				if err != nil {
					return b, err
				}
				b.Children = append(b.Children, c)
			}
		}
		if b.Type == "listItem" {
			if first := n.FirstChild(); first != nil {
				if box, ok := first.FirstChild().(*ext.TaskCheckBox); ok {
					b.Attrs["checked"] = box.IsChecked
				}
			}
		}
		if len(b.Attrs) == 0 {
			b.Attrs = nil
		}
		return b, nil
	}
	blocks := []Block{}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		b, err := convert(n)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, Prepare(blocks)
}
func parseInline(parent ast.Node, source []byte, marks []Mark) ([]Inline, error) {
	result := []Inline{}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		next := append([]Mark{}, marks...)
		switch node := n.(type) {
		case *ast.Text:
			value := string(node.Segment.Value(source))
			value = html.UnescapeString(string(util.UnescapePunctuations([]byte(value))))
			result = append(result, Inline{Text: value, Marks: marks})
			if node.HardLineBreak() {
				result = append(result, Inline{Text: "\n"})
			} else if node.SoftLineBreak() {
				result = append(result, Inline{Text: " ", Marks: marks})
			}
			continue
		case *ast.String:
			result = append(result, Inline{Text: string(node.Value), Marks: marks})
			continue
		case *ast.CodeSpan:
			var value strings.Builder
			for c := node.FirstChild(); c != nil; c = c.NextSibling() {
				value.Write(c.Text(source))
			}
			next = append(next, Mark{Type: "code"})
			result = append(result, Inline{Text: strings.ReplaceAll(value.String(), "\n", " "), Marks: next})
			continue
		case *ast.Emphasis:
			kind := "italic"
			if node.Level == 2 {
				kind = "bold"
			}
			next = append(next, Mark{Type: kind})
		case *ext.Strikethrough:
			next = append(next, Mark{Type: "strike"})
		case *ast.Link:
			href := html.UnescapeString(string(node.Destination))
			if !SafeLink(href) {
				return nil, errors.New("unsafe Markdown link")
			}
			next = append(next, Mark{Type: "link", Href: href, Title: string(node.Title)})
		case *ast.AutoLink:
			href := string(node.URL(source))
			if node.AutoLinkType == ast.AutoLinkEmail {
				href = "mailto:" + href
			}
			if !SafeLink(href) {
				return nil, errors.New("unsafe link")
			}
			next = append(next, Mark{Type: "link", Href: href})
			result = append(result, Inline{Text: string(node.Label(source)), Marks: next})
			continue
		case *ext.TaskCheckBox:
			continue
		default:
			return nil, fmt.Errorf("unsupported Markdown inline: %s", n.Kind())
		}
		children, err := parseInline(n, source, next)
		if err != nil {
			return nil, err
		}
		result = append(result, children...)
	}
	return compact(result), nil
}
func compact(inlines []Inline) []Inline {
	result := []Inline{}
	for _, inline := range inlines {
		inline.Marks = append([]Mark{}, inline.Marks...)
		unique := []Mark{}
		for _, mark := range inline.Marks {
			if len(unique) == 0 || unique[len(unique)-1] != mark {
				unique = append(unique, mark)
			}
		}
		inline.Marks = unique
		if inline.Text == "" {
			continue
		}
		if len(result) > 0 && fmt.Sprint(result[len(result)-1].Marks) == fmt.Sprint(inline.Marks) {
			result[len(result)-1].Text += inline.Text
		} else {
			result = append(result, inline)
		}
	}
	return result
}
func escape(value string) string {
	var out strings.Builder
	for _, r := range value {
		if strings.ContainsRune("\\`*_{}[]<>|#!+-.~", r) {
			out.WriteByte('\\')
		}
		out.WriteRune(r)
	}
	return strings.ReplaceAll(out.String(), "\n", "  \n")
}
func inlineMarkdown(content []Inline) string {
	var out strings.Builder
	for i := 0; i < len(content); {
		current := content[i]
		if len(current.Marks) == 0 {
			out.WriteString(escape(current.Text))
			i++
			continue
		}
		mark := current.Marks[0]
		group := []Inline{}
		j := i
		for j < len(content) && len(content[j].Marks) > 0 && content[j].Marks[0] == mark {
			part := content[j]
			part.Marks = part.Marks[1:]
			group = append(group, part)
			j++
		}
		value := inlineMarkdown(group)
		leading := value[:len(value)-len(strings.TrimLeftFunc(value, unicode.IsSpace))]
		trailing := value[len(strings.TrimRightFunc(value, unicode.IsSpace)):]
		value = strings.TrimSpace(value)
		switch mark.Type {
		case "bold":
			value = "**" + value + "**"
		case "italic":
			value = "_" + value + "_"
		case "strike":
			value = "~~" + value + "~~"
		case "code":
			raw := ""
			for _, part := range group {
				raw += part.Text
			}
			fence := "`"
			for strings.Contains(raw, fence) {
				fence += "`"
			}
			value = fence + " " + raw + " " + fence
			if strings.TrimSpace(raw) == "" {
				value = fence + raw + fence
			}
			leading = ""
			trailing = ""
		case "link":
			title := ""
			if mark.Title != "" {
				title = " " + strconv.Quote(mark.Title)
			}
			value = "[" + value + "](<" + strings.ReplaceAll(mark.Href, ">", "%3E") + ">" + title + ")"
		}
		out.WriteString(leading + value + trailing)
		i = j
	}
	return out.String()
}
func Markdown(blocks []Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		value := inlineMarkdown(b.Content)
		switch b.Type {
		case "heading":
			value = strings.Repeat("#", Number(b.Attrs, "level", 1)) + " " + value
		case "embed":
			value = "[Diagram: " + escape(String(b.Attrs, "caption")) + "]\n\n" + String(b.Attrs, "svg")
		case "pending":
			value = "<?kiko block " + b.ID + "?>"
		case "code":
			raw := Plain(b)
			fence := "```"
			for strings.Contains(raw, fence) {
				fence += "`"
			}
			value = fence + String(b.Attrs, "language") + "\n" + raw + "\n" + fence
		case "list":
			items := []string{}
			ordered, _ := b.Attrs["ordered"].(bool)
			start := Number(b.Attrs, "start", 1)
			for i, item := range b.Children {
				prefix := "- "
				if ordered {
					prefix = strconv.Itoa(start+i) + ". "
				}
				if checked, ok := item.Attrs["checked"].(bool); ok {
					if checked {
						prefix += "[x] "
					} else {
						prefix += "[ ] "
					}
				}
				body := Markdown(item.Children)
				items = append(items, prefix+strings.ReplaceAll(body, "\n", "\n"+strings.Repeat(" ", len(prefix))))
			}
			value = strings.Join(items, "\n")
		case "table":
			rows := []string{}
			for i, row := range b.Children {
				cells := []string{}
				for _, cell := range row.Children {
					cells = append(cells, inlineMarkdown(cell.Content))
				}
				rows = append(rows, "| "+strings.Join(cells, " | ")+" |")
				if i == 0 {
					rules := []string{}
					for _, cell := range row.Children {
						rule := "---"
						switch String(cell.Attrs, "align") {
						case "left":
							rule = ":---"
						case "right":
							rule = "---:"
						case "center":
							rule = ":---:"
						}
						rules = append(rules, rule)
					}
					rows = append(rows, "| "+strings.Join(rules, " | ")+" |")
				}
			}
			value = strings.Join(rows, "\n")
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n\n")
}
