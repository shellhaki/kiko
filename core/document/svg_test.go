package document

import "testing"

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
