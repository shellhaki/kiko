package deck

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	exports "kiko/core/export"
)

func PDF(ctx context.Context, deck Deck, resolveImage func(string) ([]byte, error)) ([]byte, error) {
	if err := Prepare(&deck); err != nil {
		return nil, err
	}
	var source strings.Builder
	assets := map[string][]byte{}
	source.WriteString("#set page(width: 1280pt, height: 720pt, margin: 0pt, fill: rgb(\"181818\"))\n#set text(size: 22pt, fill: rgb(\"eeeeee\"))\n")
	place := func(x, y, w, h int, body string) {
		fmt.Fprintf(&source, "#place(top + left, dx: %dpt, dy: %dpt)[#block(width: %dpt, height: %dpt)[%s]]\n", x, y, w, h, body)
	}
	text := func(value string, size int) string {
		return "#text(size: " + strconv.Itoa(size) + "pt," + exports.Quote(value) + ")"
	}
	list := func(values []string) string {
		items := []string{}
		for _, v := range values {
			items = append(items, "["+text(v, 20)+"]")
		}
		if len(items) == 0 {
			return ""
		}
		return "#list(spacing: 18pt," + strings.Join(items, ",") + ")"
	}
	for i, slide := range deck.Slides {
		if i > 0 {
			source.WriteString("#pagebreak()\n")
		}
		if slide.Layout != "title" {
			place(60, 45, 1160, 120, "#strong["+text(slide.Title, 32)+"]")
		}
		switch slide.Layout {
		case "title":
			place(80, 180, 1120, 180, "#strong["+text(slide.Title, 44)+"]")
			for j, line := range slide.Body {
				place(80, 390+j*40, 1120, 45, text(line, 22))
			}
		case "bullets":
			place(80, 210, 1120, 430, list(slide.Body))
		case "columns":
			place(70, 210, 520, 430, list(slide.Left))
			place(660, 210, 540, 430, list(slide.Right))
		case "image":
			data, err := resolveImage(slide.ImageID)
			if err != nil {
				return nil, err
			}
			name := fmt.Sprintf("image%d.png", i)
			assets[name] = data
			place(190, 180, 900, 390, "#image("+exports.Quote(name)+", width: 900pt, height: 390pt, fit: \"contain\")")
			for j, line := range slide.Body {
				place(70, 600+j*30, 1140, 35, text(line, 16))
			}
		case "chart":
			low, high := 0.0, 0.0
			for _, v := range slide.Chart.Values {
				low = math.Min(low, v)
				high = math.Max(high, v)
			}
			if low == high {
				high = low + 1
			}
			step := float64(1080) / float64(len(slide.Chart.Values))
			zero := 210 + int(high/(high-low)*330)
			for index, value := range slide.Chart.Values {
				x := 90 + int(float64(index)*step)
				y := 210 + int((high-value)/(high-low)*330)
				height := max(1, int(math.Abs(float64(y-zero))))
				place(x, min(y, zero), int(step*.67), height, fmt.Sprintf("#rect(width: %dpt, height: %dpt, fill: rgb(\"7950cc\"), stroke: none)", int(step*.67), height))
				place(x, 565, int(step*.9), 100, text(slide.Chart.Labels[index], 10))
				place(x, min(y, zero)-25, int(step*.9), 25, text(strconv.FormatFloat(value, 'f', -1, 64), 12))
			}
		}
		place(1190, 680, 40, 25, text(strconv.Itoa(i+1), 10))
	}
	return exports.CompilePDF(ctx, source.String(), assets)
}
