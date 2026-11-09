package provider

import "testing"

func TestImageInputBoundsAndProviderEligibility(t *testing.T) {
	for _, image := range []Image{{MIME: "image/svg+xml", Data: []byte("<svg/>")}, {MIME: "image/png"}, {MIME: "image/png", Data: make([]byte, 4*1024*1024+1)}} {
		if image.Validate() == nil {
			t.Fatal("unsupported or unbounded image accepted")
		}
	}
	if (Image{MIME: "image/png", Data: []byte("fixture")}).Validate() != nil {
		t.Fatal("bounded PNG rejected")
	}
	if DirectImages("muse") || DirectImages("ollama") || !DirectImages("openai") || !DirectImages("anthropic") {
		t.Fatal("unexpected direct-image adapter")
	}
}
