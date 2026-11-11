package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGuardedFilesAndRootPaths(t *testing.T) {
	path := t.TempDir()
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	created, err := Write(root, "/docs/plan.md", "First", "")
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(path, "docs", "plan.md")); err != nil || string(data) != "First" {
		t.Fatal(string(data), err)
	}
	if _, err = Write(root, "docs/plan.md", "Clobbered", ""); err == nil {
		t.Fatal("unguarded overwrite")
	}
	if _, err = Write(root, "docs/plan.md", "Clobbered", "old-hash"); err == nil {
		t.Fatal("stale overwrite")
	}
	updated, err := Write(root, "docs/plan.md", "Second", created.Hash)
	if err != nil || updated.Hash == created.Hash {
		t.Fatal(updated, err)
	}
	canonical, err := Path("./docs//plan.md")
	if err != nil || canonical != filepath.Join("docs", "plan.md") {
		t.Fatal(canonical, err)
	}
	file, err := Read(root, "docs/plan.md")
	if err != nil || file.Content != "Second" {
		t.Fatal(file, err)
	}
	entries, err := List(root, "docs")
	if err != nil || len(entries) != 1 || entries[0].Name != "plan.md" {
		t.Fatal(entries, err)
	}
	for _, path := range []string{"../escape", "docs/../../escape", ".git/config", ".env", ".ssh/id_rsa", "C:\\secret", "docs\\..\\secret"} {
		if _, err := Write(root, path, "bad", ""); err == nil {
			t.Errorf("accepted %s", path)
		}
	}
}
func TestFilesRefuseSymlinkEscapeBinaryAndOversize(t *testing.T) {
	path := t.TempDir()
	outside := t.TempDir()
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = os.Symlink(outside, filepath.Join(path, "escape")); err != nil {
		t.Skip(err)
	}
	if _, err = Write(root, "escape/outside.txt", "bad", ""); err == nil {
		t.Fatal("symlink escape")
	}
	if err = os.Mkdir(filepath.Join(path, "inside"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink("inside", filepath.Join(path, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err = Write(root, "alias/plan.md", "bad", ""); err == nil {
		t.Fatal("symlink write alias accepted")
	}
	if _, err = Read(root, "escape/outside.txt"); err == nil {
		t.Fatal("symlink read escape")
	}
	if err = os.WriteFile(filepath.Join(path, "binary"), []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Read(root, "binary"); err == nil {
		t.Fatal("binary accepted")
	}
	if _, err = Write(root, "large", string(make([]byte, MaxFileSize+1)), ""); err == nil {
		t.Fatal("oversized write")
	}
	if _, err = Read(root, "."); err == nil {
		t.Fatal("directory read as text")
	}
}
