package document

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownExportPreservesContentAndExistingFileOnFailure(t *testing.T) {
	blocks, err := Parse("# Plan\n\n- **Bold** item\n\n| Name | Value |\n| --- | --- |\n| One | Two |\n\n```go\nhello()\n```", nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Plan.md")
	if err = WriteMarkdown(path, blocks); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != Markdown(blocks)+"\n" {
		t.Fatal(err, string(contents))
	}
	if err = WriteMarkdown(filepath.Join(path, "invalid.md"), blocks); err == nil {
		t.Fatal("invalid destination accepted")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(contents) {
		t.Fatal("previous output was damaged")
	}
}
