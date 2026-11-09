package document

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
)

func SanitizeSVG(source string) (string, error) {
	if len(source) == 0 || len(source) > 256*1024 {
		return "", errors.New("SVG must contain 1–256 KB of static markup")
	}
	elements := map[string]bool{"svg": true, "g": true, "rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true, "polygon": true, "path": true, "text": true, "tspan": true, "title": true, "desc": true, "defs": true, "marker": true, "linearGradient": true, "radialGradient": true, "stop": true}
	attributes := map[string]bool{"id": true, "width": true, "height": true, "viewBox": true, "x": true, "y": true, "x1": true, "y1": true, "x2": true, "y2": true, "cx": true, "cy": true, "r": true, "rx": true, "ry": true, "d": true, "points": true, "transform": true, "fill": true, "stroke": true, "stroke-width": true, "stroke-dasharray": true, "stroke-linecap": true, "stroke-linejoin": true, "opacity": true, "fill-opacity": true, "stroke-opacity": true, "font-family": true, "font-size": true, "font-weight": true, "text-anchor": true, "dominant-baseline": true, "dx": true, "dy": true, "marker-end": true, "marker-start": true, "markerWidth": true, "markerHeight": true, "refX": true, "refY": true, "orient": true, "markerUnits": true, "offset": true, "stop-color": true, "stop-opacity": true, "gradientUnits": true, "gradientTransform": true, "preserveAspectRatio": true}
	decoder := xml.NewDecoder(strings.NewReader(source))
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	depth := 0
	root := false
	nodes := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.New("SVG is not valid XML")
		}
		switch value := token.(type) {
		case xml.StartElement:
			nodes++
			depth++
			if depth > 30 || nodes > 3000 {
				return "", errors.New("SVG is too complex")
			}
			if !elements[value.Name.Local] || value.Name.Space != "" && value.Name.Space != "http://www.w3.org/2000/svg" {
				return "", errors.New("SVG contains unsupported or active elements")
			}
			if !root {
				if value.Name.Local != "svg" {
					return "", errors.New("SVG root is required")
				}
				root = true
			} else if depth == 1 {
				return "", errors.New("SVG has multiple roots")
			}
			attrs := []xml.Attr{}
			for _, attr := range value.Attr {
				if attr.Name.Local == "xmlns" || attr.Name.Space == "xmlns" {
					continue
				}
				if attr.Name.Space != "" || !attributes[attr.Name.Local] {
					return "", errors.New("SVG contains unsupported attributes")
				}
				lower := strings.ToLower(attr.Value)
				if strings.Contains(lower, "url(") && !regexp.MustCompile(`^url\(#[a-zA-Z0-9_.-]+\)$`).MatchString(lower) {
					return "", errors.New("SVG cannot load external resources")
				}
				if strings.ContainsAny(attr.Value, "\x00\r") {
					return "", errors.New("invalid SVG attribute")
				}
				attrs = append(attrs, xml.Attr{Name: xml.Name{Local: attr.Name.Local}, Value: attr.Value})
			}
			value.Name = xml.Name{Local: value.Name.Local}
			value.Attr = attrs
			if depth == 1 {
				value.Attr = append(value.Attr, xml.Attr{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/svg"})
			}
			if err = encoder.EncodeToken(value); err != nil {
				return "", err
			}
		case xml.EndElement:
			depth--
			value.Name = xml.Name{Local: value.Name.Local}
			if err = encoder.EncodeToken(value); err != nil {
				return "", err
			}
		case xml.CharData:
			if !root && strings.TrimSpace(string(value)) != "" {
				return "", errors.New("text outside SVG")
			}
			if err = encoder.EncodeToken(value); err != nil {
				return "", err
			}
		case xml.Comment:
		default:
			return "", errors.New("SVG processing instructions and declarations are not allowed")
		}
	}
	if !root || depth != 0 {
		return "", errors.New("incomplete SVG")
	}
	if err := encoder.Flush(); err != nil {
		return "", err
	}
	return out.String(), nil
}
