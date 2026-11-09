package document

import (
	"encoding/json"
	"strings"
	"testing"
)

func withoutIDs(blocks []Block) string {
	var clear func([]Block)
	clear = func(list []Block) {
		for i := range list {
			list[i].ID = ""
			list[i].Hash = ""
			if len(list[i].Attrs) == 0 {
				list[i].Attrs = nil
			}
			if len(list[i].Content) == 0 {
				list[i].Content = nil
			}
			if len(list[i].Children) == 0 {
				list[i].Children = nil
			}
			clear(list[i].Children)
		}
	}
	copied := Clone(Document{Blocks: blocks}).Blocks
	clear(copied)
	data, _ := json.Marshal(copied)
	return string(data)
}
func TestMarkdownRoundTrips(t *testing.T) {
	samples := []string{
		"# Plan\n\nA **bold** and _italic_ [link](https://example.com \"Title\") with ~~strike~~ and `code`.\n\nSecond  \nline.",
		"## Nested\n\n- First\n  - Nested **bold**\n- Second\n\n3. Third\n4. Fourth",
		"- [ ] Pending\n- [x] Done",
		"| Name | Value |\n| :--- | ---: |\n| A \\| B | **Two** |",
		"```go\nfmt.Println(\"你好\")\n```\n\n    indented\n",
		"Escaped \\*text\\* &amp; \\[brackets\\] and 😀.",
		"A **bold _nested_ text** and _before **bold** after_.",
	}
	for _, sample := range samples {
		t.Run(sample[:min(20, len(sample))], func(t *testing.T) {
			first, err := Parse(sample, nil)
			if err != nil {
				t.Fatal(err)
			}
			markdown := Markdown(first)
			second, err := Parse(markdown, nil)
			if err != nil {
				t.Fatal(markdown, err)
			}
			if withoutIDs(first) != withoutIDs(second) {
				t.Fatalf("round trip changed:\n%s\n%s\nMarkdown:\n%s", withoutIDs(first), withoutIDs(second), markdown)
			}
		})
	}
}
func TestPendingAndSafeContent(t *testing.T) {
	blocks, err := Parse("# Plan\n\n<?kiko block s1?>", map[string]Placeholder{"s1": {Type: "pending", Intent: "Risks and mitigation"}})
	if err != nil || len(blocks) != 2 || blocks[1].Type != "pending" {
		t.Fatal(err, blocks)
	}
	for _, input := range []string{"<script>alert(1)</script>", "Text <img src=x>", "![remote](https://example.com/pixel)", "[unsafe](javascript:alert)", "<?kiko block unknown?>"} {
		if _, err := Parse(input, nil); err == nil {
			t.Fatal("unsafe or unsupported input accepted", input)
		}
	}
}
func TestHashesAndValidation(t *testing.T) {
	a, _ := Parse("Hello **world**", nil)
	b, _ := Parse("Hello **world**", nil)
	if a[0].Hash != b[0].Hash || a[0].ID == b[0].ID {
		t.Fatal("hashes must ignore fresh IDs")
	}
	b[0].Content[0].Text = "Changed"
	Prepare(b)
	if a[0].Hash == b[0].Hash {
		t.Fatal("hash did not change")
	}
	a = append(a, a[0])
	if Prepare(a) == nil {
		t.Fatal("duplicate IDs accepted")
	}
	if _, err := Parse(strings.Repeat("x", 4*1024*1024+1), nil); err == nil {
		t.Fatal("oversized document accepted")
	}
}
