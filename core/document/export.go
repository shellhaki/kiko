package document

import (
	"os"
	"path/filepath"
)

func WriteMarkdown(path string, blocks []Block) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".kiko-export-*.md")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.WriteString(Markdown(blocks) + "\n"); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temporary, path)
}
