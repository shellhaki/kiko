package document

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Label struct {
	Text string `json:"text"`
}
type Ref struct {
	ID string `json:"id"`
}
type Element struct {
	Type            string   `json:"type"`
	ID              string   `json:"id,omitempty"`
	X               float64  `json:"x"`
	Y               float64  `json:"y"`
	Width           *float64 `json:"width,omitempty"`
	Height          *float64 `json:"height,omitempty"`
	Text            string   `json:"text,omitempty"`
	FontSize        float64  `json:"fontSize,omitempty"`
	StrokeColor     string   `json:"strokeColor,omitempty"`
	BackgroundColor string   `json:"backgroundColor,omitempty"`
	StrokeStyle     string   `json:"strokeStyle,omitempty"`
	Label           *Label   `json:"label,omitempty"`
	Start           *Ref     `json:"start,omitempty"`
	End             *Ref     `json:"end,omitempty"`
}

var colorPattern = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|transparent)$`)
var escapeText = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func (e Element) shape() bool {
	return e.Type == "rectangle" || e.Type == "ellipse" || e.Type == "diamond"
}
func (e Element) link() bool { return e.Type == "arrow" || e.Type == "line" }
func (e Element) size() (float64, float64) {
	if e.Width == nil || e.Height == nil {
		return 0, 0
	}
	return *e.Width, *e.Height
}
func (e Element) center() (float64, float64) {
	width, height := e.size()
	return e.X + width/2, e.Y + height/2
}
func (e Element) edge(toward [2]float64) [2]float64 {
	cx, cy := e.center()
	width, height := e.size()
	dx, dy := toward[0]-cx, toward[1]-cy
	if dx == 0 && dy == 0 {
		return [2]float64{cx, cy}
	}
	a, b := math.Abs(dx)/(width/2), math.Abs(dy)/(height/2)
	scale := 1 / math.Max(a, b)
	switch e.Type {
	case "ellipse":
		scale = 1 / math.Hypot(a, b)
	case "diamond":
		scale = 1 / (a + b)
	}
	return [2]float64{cx + dx*scale, cy + dy*scale}
}
func (e *Element) check() error {
	if math.Abs(e.X) > 10000 || math.Abs(e.Y) > 10000 {
		return errors.New("coordinates must stay within ±10000")
	}
	if e.FontSize == 0 {
		e.FontSize = 20
	}
	if e.FontSize < 8 || e.FontSize > 72 {
		return errors.New("fontSize must be 8–72")
	}
	for _, color := range []string{e.StrokeColor, e.BackgroundColor} {
		if color != "" && !colorPattern.MatchString(color) {
			return errors.New("colors must be #hex or transparent")
		}
	}
	if e.StrokeStyle != "" && e.StrokeStyle != "solid" && e.StrokeStyle != "dashed" && e.StrokeStyle != "dotted" {
		return errors.New("strokeStyle must be solid, dashed or dotted")
	}
	if e.Label != nil && (!(e.shape() || e.link()) || utf8.RuneCountInString(e.Label.Text) > 200) {
		return errors.New("label is only for shapes and arrows, up to 200 characters")
	}
	return nil
}

func DrawDiagram(elements []Element) (scene, svg string, err error) {
	if len(elements) == 0 || len(elements) > 200 {
		return "", "", errors.New("a diagram needs 1–200 elements")
	}
	shapes := map[string]Element{}
	for i := range elements {
		e := &elements[i]
		if err = e.check(); err == nil {
			err = e.validate(shapes)
		}
		if err != nil {
			return "", "", fmt.Errorf("element %d (%s): %w", i+1, e.Type, err)
		}
	}
	for i := range elements {
		e := &elements[i]
		if !e.link() {
			continue
		}
		width, height := e.size()
		start, end := [2]float64{e.X, e.Y}, [2]float64{e.X + width, e.Y + height}
		from, to := start, end
		for _, ref := range []*Ref{e.Start, e.End} {
			if _, found := shapes[refID(ref)]; ref != nil && !found {
				return "", "", fmt.Errorf("element %d (%s): no shape has id %q", i+1, e.Type, ref.ID)
			}
		}
		if e.Start != nil {
			from = centerOf(shapes, e.Start.ID)
		}
		if e.End != nil {
			to = centerOf(shapes, e.End.ID)
		}
		if e.Start != nil {
			start = shapes[e.Start.ID].edge(to)
		}
		if e.End != nil {
			end = shapes[e.End.ID].edge(from)
		}
		if math.Hypot(end[0]-start[0], end[1]-start[1]) < 1 {
			return "", "", fmt.Errorf("element %d (%s): %s has no length", i+1, e.Type, e.Type)
		}
		e.X, e.Y = round(start[0]), round(start[1])
		w, h := round(end[0]-start[0]), round(end[1]-start[1])
		e.Width, e.Height = &w, &h
	}
	data, err := json.Marshal(map[string]any{"type": "excalidraw", "version": 2, "skeleton": true, "elements": elements})
	if err != nil {
		return "", "", err
	}
	svg, err = SanitizeSVG(render(elements))
	return string(data), svg, err
}
func (e *Element) validate(shapes map[string]Element) error {
	width, height := e.size()
	switch {
	case e.shape():
		if width <= 0 || height <= 0 || width > 2000 || height > 2000 {
			return errors.New("width and height must be 1–2000")
		}
		if e.ID != "" {
			if _, taken := shapes[e.ID]; taken {
				return errors.New("duplicate id " + e.ID)
			}
			shapes[e.ID] = *e
		}
		if e.Start != nil || e.End != nil || e.Text != "" {
			return errors.New("shapes take a label, not text, start or end")
		}
	case e.Type == "text":
		if n := utf8.RuneCountInString(e.Text); n == 0 || n > 500 || e.Width != nil || e.Height != nil || e.Label != nil || e.Start != nil || e.End != nil {
			return errors.New("text needs 1–500 characters and only x, y, text, fontSize and strokeColor")
		}
	case e.link():
		if e.Text != "" || (e.Start == nil && e.End == nil && (e.Width == nil || e.Height == nil)) {
			return errors.New("arrows need start/end ids, or x, y, width and height as the end offset")
		}
		if e.Width == nil || e.Height == nil {
			zero := 0.0
			e.Width, e.Height = &zero, &zero
		}
		if math.Abs(width) > 10000 || math.Abs(height) > 10000 {
			return errors.New("arrow is too long")
		}
	default:
		return errors.New("type must be rectangle, ellipse, diamond, text, arrow or line")
	}
	return nil
}
func refID(ref *Ref) string {
	if ref == nil {
		return ""
	}
	return ref.ID
}
func centerOf(shapes map[string]Element, id string) [2]float64 {
	cx, cy := shapes[id].center()
	return [2]float64{cx, cy}
}
func round(value float64) float64 { return math.Round(value*100) / 100 }
func num(value float64) string    { return strconv.FormatFloat(round(value), 'f', -1, 64) }

func render(elements []Element) string {
	var body strings.Builder
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	grow := func(x, y float64) {
		minX, minY, maxX, maxY = math.Min(minX, x), math.Min(minY, y), math.Max(maxX, x), math.Max(maxY, y)
	}
	measure := func(text string, size float64) (width float64) {
		for _, line := range strings.Split(text, "\n") {
			width = math.Max(width, float64(utf8.RuneCountInString(line))*size*0.55)
		}
		return width
	}
	lines := func(text string, x, top, size float64, anchor, color string) float64 {
		for i, line := range strings.Split(text, "\n") {
			fmt.Fprintf(&body, `<text x="%s" y="%s" font-size="%s" font-family="Helvetica, Arial, sans-serif" text-anchor="%s" fill="%s">%s</text>`, num(x), num(top+(float64(i)+0.5)*size*1.25+size*0.35), num(size), anchor, color, escapeText.Replace(line))
		}
		return measure(text, size)
	}
	paint := func(e Element, fill string) string {
		stroke := "#1e1e1e"
		if e.StrokeColor != "" {
			stroke = e.StrokeColor
		}
		dash := map[string]string{"dashed": ` stroke-dasharray="8 8"`, "dotted": ` stroke-dasharray="2 6"`}[e.StrokeStyle]
		return fmt.Sprintf(`fill="%s" stroke="%s" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"%s`, fill, stroke, dash)
	}
	height := func(text string, size float64) float64 { return float64(strings.Count(text, "\n")+1) * size * 1.25 }
	for _, e := range elements {
		width, tall := e.size()
		switch {
		case e.shape():
			grow(e.X, e.Y)
			grow(e.X+width, e.Y+tall)
			fill := "none"
			if e.BackgroundColor != "" && e.BackgroundColor != "transparent" {
				fill = e.BackgroundColor
			}
			cx, cy := e.center()
			switch e.Type {
			case "rectangle":
				fmt.Fprintf(&body, `<rect x="%s" y="%s" width="%s" height="%s" %s/>`, num(e.X), num(e.Y), num(width), num(tall), paint(e, fill))
			case "ellipse":
				fmt.Fprintf(&body, `<ellipse cx="%s" cy="%s" rx="%s" ry="%s" %s/>`, num(cx), num(cy), num(width/2), num(tall/2), paint(e, fill))
			default:
				fmt.Fprintf(&body, `<polygon points="%s,%s %s,%s %s,%s %s,%s" %s/>`, num(cx), num(e.Y), num(e.X+width), num(cy), num(cx), num(e.Y+tall), num(e.X), num(cy), paint(e, fill))
			}
			if e.Label != nil {
				lines(e.Label.Text, cx, cy-height(e.Label.Text, e.FontSize)/2, e.FontSize, "middle", "#1e1e1e")
			}
		case e.Type == "text":
			color := "#1e1e1e"
			if e.StrokeColor != "" {
				color = e.StrokeColor
			}
			grow(e.X, e.Y)
			grow(e.X+lines(e.Text, e.X, e.Y, e.FontSize, "start", color), e.Y+height(e.Text, e.FontSize))
		}
	}
	for _, e := range elements {
		if !e.link() {
			continue
		}
		width, tall := e.size()
		x2, y2 := e.X+width, e.Y+tall
		grow(e.X, e.Y)
		grow(x2, y2)
		fmt.Fprintf(&body, `<line x1="%s" y1="%s" x2="%s" y2="%s" %s/>`, num(e.X), num(e.Y), num(x2), num(y2), paint(e, "none"))
		if e.Type == "arrow" {
			angle := math.Atan2(tall, width)
			fmt.Fprintf(&body, `<polyline points="%s,%s %s,%s %s,%s" %s/>`, num(x2-16*math.Cos(angle-0.44)), num(y2-16*math.Sin(angle-0.44)), num(x2), num(y2), num(x2-16*math.Cos(angle+0.44)), num(y2-16*math.Sin(angle+0.44)), paint(e, "none"))
		}
		if e.Label != nil && e.Label.Text != "" {
			mx, my := e.X+width/2, e.Y+tall/2
			top, half := my-height(e.Label.Text, e.FontSize)/2, measure(e.Label.Text, e.FontSize)/2
			fmt.Fprintf(&body, `<rect x="%s" y="%s" width="%s" height="%s" fill="#ffffff"/>`, num(mx-half-4), num(top-2), num(half*2+8), num(height(e.Label.Text, e.FontSize)+4))
			lines(e.Label.Text, mx, top, e.FontSize, "middle", "#1e1e1e")
		}
	}
	const pad = 16
	minX, minY, maxX, maxY = minX-pad, minY-pad, maxX+pad, maxY+pad
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s %s %s %s" width="%s" height="%s"><rect x="%s" y="%s" width="%s" height="%s" fill="#ffffff"/>%s</svg>`, num(minX), num(minY), num(maxX-minX), num(maxY-minY), num(maxX-minX), num(maxY-minY), num(minX), num(minY), num(maxX-minX), num(maxY-minY), body.String())
}
