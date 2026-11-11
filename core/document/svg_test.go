package document

import (
	"os"
	"strings"
	"testing"
)

func TestStaticSVGRejectsActiveAndExternalContent(t *testing.T) {
	good := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 500 200"><rect x="10" y="10" width="200" height="100" fill="#30243f"/><text x="20" y="50">Plan &amp; build</text></svg>`
	if _, err := SanitizeSVG(good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`<svg><script>alert(1)</script></svg>`, `<svg onload="alert(1)"/>`, `<svg><image href="https://tracker.test/pixel"/></svg>`, `<svg><foreignObject/></svg>`, `<svg><rect fill="url(https://tracker.test)"/></svg>`, `<svg><rect fill="url(#a) url(https://tracker.test)"/></svg>`, `<!DOCTYPE svg><svg/>`} {
		if _, err := SanitizeSVG(bad); err == nil {
			t.Fatal("unsafe SVG accepted", bad)
		}
	}
}

func TestSanitizerAcceptsCleanedExcalidrawExport(t *testing.T) {
	source, err := os.ReadFile("testdata/excalidraw-export.svg")
	if err != nil {
		t.Fatal(err)
	}
	clean, err := SanitizeSVG(string(source))
	if err != nil || !strings.Contains(clean, "Client") || !strings.Contains(clean, "Server") {
		t.Fatal("Excalidraw export rejected or lost its text", err)
	}
}

func TestSceneValidation(t *testing.T) {
	for _, good := range []string{"", `{"type":"excalidraw","version":2,"elements":[{"id":"a","type":"rectangle"}]}`} {
		if err := ValidateScene(good); err != nil {
			t.Fatal("valid scene rejected", good, err)
		}
	}
	tooLarge := `{"type":"excalidraw","elements":[{"id":"` + strings.Repeat("a", 512*1024) + `"}]}`
	for _, bad := range []string{`not json`, `{"type":"other","elements":[{}]}`, `{"type":"excalidraw","elements":[]}`, tooLarge} {
		if err := ValidateScene(bad); err == nil {
			t.Fatal("invalid scene accepted", len(bad))
		}
	}
}
