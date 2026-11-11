package document

import (
	"strings"
	"testing"
)

func box(shape, id string, x, y float64, label string) Element {
	width, height := 160.0, 60.0
	return Element{Type: shape, ID: id, X: x, Y: y, Width: &width, Height: &height, Label: &Label{Text: label}}
}

func TestDrawDiagramConnectsBoundShapesEdgeToEdge(t *testing.T) {
	elements := []Element{
		box("rectangle", "a", 0, 0, "Client"),
		box("rectangle", "b", 300, 0, "Server & <DB>"),
		{Type: "arrow", Start: &Ref{ID: "a"}, End: &Ref{ID: "b"}, Label: &Label{Text: "request"}},
	}
	scene, svg, err := DrawDiagram(elements)
	if err != nil {
		t.Fatal(err)
	}
	if ValidateScene(scene) != nil || !strings.Contains(scene, `"skeleton":true`) {
		t.Fatal("scene is not a skeleton Excalidraw scene", scene)
	}
	arrow := elements[2]
	if arrow.X != 160 || arrow.Y != 30 || *arrow.Width != 140 || *arrow.Height != 0 {
		t.Fatal("arrow does not run edge to edge", arrow.X, arrow.Y, *arrow.Width, *arrow.Height)
	}
	for _, want := range []string{"Client", "Server &amp; &lt;DB&gt;", "request", "<line", "<polyline"} {
		if !strings.Contains(svg, want) {
			t.Fatal("SVG is missing", want, svg)
		}
	}
}

func TestDrawDiagramClipsArrowsToEllipseAndDiamond(t *testing.T) {
	elements := []Element{box("ellipse", "e", 0, 0, "E"), box("diamond", "d", 300, 0, "D"), {Type: "arrow", Start: &Ref{ID: "e"}, End: &Ref{ID: "d"}}}
	if _, _, err := DrawDiagram(elements); err != nil {
		t.Fatal(err)
	}
	if arrow := elements[2]; arrow.X != 160 || *arrow.Width != 140 {
		t.Fatal("curved shapes should be hit on their horizontal extremes", arrow.X, *arrow.Width)
	}
}

func TestDrawDiagramSupportsFreeArrowsAndText(t *testing.T) {
	width, height := 100.0, -50.0
	elements := []Element{{Type: "text", X: 10, Y: 10, Text: "Title\nSecond line", StrokeColor: "#7950cc"}, {Type: "line", X: 0, Y: 100, Width: &width, Height: &height, StrokeStyle: "dashed"}}
	_, svg, err := DrawDiagram(elements)
	if err != nil || !strings.Contains(svg, "Second line") || !strings.Contains(svg, "stroke-dasharray") || strings.Contains(svg, "<polyline") {
		t.Fatal("text or free line not drawn", svg, err)
	}
}

func TestDrawDiagramRejectsUnusableElements(t *testing.T) {
	zero := 0.0
	many := make([]Element, 201)
	for i := range many {
		many[i] = box("rectangle", "", 0, 0, "")
	}
	for name, elements := range map[string][]Element{
		"none":             {},
		"too many":         many,
		"unknown type":     {{Type: "image"}},
		"duplicate id":     {box("rectangle", "a", 0, 0, ""), box("ellipse", "a", 200, 0, "")},
		"missing size":     {{Type: "rectangle", X: 0, Y: 0}},
		"unknown binding":  {box("rectangle", "a", 0, 0, ""), {Type: "arrow", Start: &Ref{ID: "a"}, End: &Ref{ID: "ghost"}}},
		"arrow no target":  {{Type: "arrow", X: 5, Y: 5}},
		"zero length":      {{Type: "arrow", X: 5, Y: 5, Width: &zero, Height: &zero}},
		"bad color":        {{Type: "text", Text: "x", StrokeColor: "url(https://tracker.test)"}},
		"bad stroke style": {{Type: "line", X: 0, Y: 0, Width: &zero, Height: &zero, StrokeStyle: "wavy"}},
		"label on text":    {{Type: "text", Text: "x", Label: &Label{Text: "y"}}},
		"far away":         {{Type: "text", Text: "x", X: 50000}},
		"overlapping ends": {box("rectangle", "a", 0, 0, ""), {Type: "arrow", Start: &Ref{ID: "a"}, End: &Ref{ID: "a"}}},
	} {
		if _, _, err := DrawDiagram(elements); err == nil {
			t.Fatal("accepted:", name)
		}
	}
}
