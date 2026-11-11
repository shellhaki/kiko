package workspace

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxFileSize = 1024 * 1024

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash"`
}
type Entry struct {
	Name      string `json:"name"`
	Directory bool   `json:"directory"`
}

func Hash(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }
func Path(value string) (string, error) {
	value = strings.TrimPrefix(filepath.ToSlash(value), "/")
	if value == "" {
		value = "."
	}
	if strings.ContainsAny(value, "\x00\\:") {
		return "", errors.New("Use a path relative to the selected folder.")
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." || part == ".git" || part == ".ssh" || part == ".codex" || part == ".claude" || part == ".env" || strings.HasPrefix(part, ".env.") {
			return "", errors.New("This path is outside the permitted workspace files.")
		}
	}
	value = filepath.Clean(filepath.FromSlash(value))
	if value != "." && !filepath.IsLocal(value) {
		return "", errors.New("Use a path inside the selected folder.")
	}
	return value, nil
}
func Open(path string) (*os.Root, error) { return os.OpenRoot(path) }
func Read(root *os.Root, path string) (File, error) {
	path, err := Path(path)
	if err != nil {
		return File{}, err
	}
	info, err := root.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxFileSize {
		return File{}, errors.New("Choose a text file up to 1 MB.")
	}
	file, err := root.Open(path)
	if err != nil {
		return File{}, errors.New("Couldn’t read this file in the selected folder.")
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxFileSize {
		return File{}, errors.New("Choose a text file up to 1 MB.")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil || len(data) > MaxFileSize || !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		return File{}, errors.New("Choose a text file up to 1 MB.")
	}
	return File{filepath.ToSlash(path), string(data), Hash(data)}, nil
}
func List(root *os.Root, path string) ([]Entry, error) {
	path, err := Path(path)
	if err != nil {
		return nil, err
	}
	info, err := root.Stat(path)
	if err != nil || !info.IsDir() {
		return nil, errors.New("Choose a directory inside the workspace.")
	}
	directory, err := root.Open(path)
	if err != nil {
		return nil, errors.New("Couldn’t open this folder.")
	}
	defer directory.Close()
	entries, err := directory.ReadDir(501)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > 500 {
		return nil, errors.New("Folder has more than 500 entries; choose a subfolder.")
	}
	result := []Entry{}
	for _, entry := range entries {
		if _, err := Path(filepath.Join(path, entry.Name())); err == nil && (entry.IsDir() || entry.Type().IsRegular()) && !strings.HasPrefix(entry.Name(), ".kiko-") {
			result = append(result, Entry{entry.Name(), entry.IsDir()})
		}
	}
	return result, nil
}
func Write(root *os.Root, path, content, expected string) (File, error) {
	path, err := Path(path)
	if err != nil || path == "." {
		return File{}, errors.New("Choose a file path inside the selected folder.")
	}
	if len(content) > MaxFileSize || !utf8.ValidString(content) || strings.ContainsRune(content, '\x00') {
		return File{}, errors.New("File content must be text up to 1 MB.")
	}
	current := ""
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		current = filepath.Join(current, part)
		info, statErr := root.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return File{}, errors.New("Write to the actual workspace path instead of a symbolic link.")
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return File{}, errors.New("Couldn’t inspect the destination path.")
		}
	}

	if err = root.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return File{}, errors.New("Couldn’t create the destination folder.")
	}
	previous, readErr := Read(root, path)
	if readErr != nil {
		if _, statErr := root.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			return File{}, errors.New("Destination is not a writable text file.")
		}
		if expected != "" {
			return File{}, errors.New("File was removed. Read it again before writing.")
		}
	} else if expected == "" || expected != previous.Hash {
		return File{}, errors.New("File changed or already exists. Read it and supply its current hash before writing.")
	}
	temporary := filepath.Join(filepath.Dir(path), ".kiko-"+rand.Text())
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return File{}, err
	}
	defer root.Remove(temporary)
	if _, err = file.WriteString(content); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return File{}, err
	}
	if readErr == nil {
		current, e := Read(root, path)
		if e != nil || current.Hash != expected {
			return File{}, errors.New("File changed while saving. Read it again.")
		}
	}
	if readErr != nil {
		err = root.Link(temporary, path)
	} else {
		err = root.Rename(temporary, path)
	}
	if err != nil {
		return File{}, errors.New("Couldn’t save the file.")
	}
	return File{filepath.ToSlash(path), content, Hash([]byte(content))}, nil
}
func Remove(root *os.Root, path, expected string) error {
	path, err := Path(path)
	if err != nil || path == "." {
		return errors.New("Choose a file path inside the selected folder.")
	}
	current := ""
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		current = filepath.Join(current, part)
		info, statErr := root.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Delete the actual workspace path instead of a symbolic link.")
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return errors.New("Couldn’t inspect this path.")
		}
	}
	previous, readErr := Read(root, path)
	if readErr != nil {
		if _, statErr := root.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			return errors.New("This is not a deletable text file.")
		}
		if expected != "" {
			return errors.New("File was removed. Read it again before deleting.")
		}
		return errors.New("File is already gone. Nothing to delete.")
	}
	if expected == "" || expected != previous.Hash {
		return errors.New("File changed since it was read. Read it again before deleting.")
	}
	if err = root.Remove(path); err != nil {
		return errors.New("Couldn’t delete this file.")
	}
	return nil
}
