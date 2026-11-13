package editor

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileSystemAPI adapts operating-system file operations to the document's needs.
// It is concrete: the application does not need interchangeable implementations.
type FileSystemAPI struct{}

func NewFileSystemAPI() *FileSystemAPI { return &FileSystemAPI{} }

func (f *FileSystemAPI) Read(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("открытие %q: %w", path, err)
	}
	return data, nil
}

// Write replaces a file only after its complete contents have been written.
// Existing symbolic links retain their identity; their target is replaced.
func (f *FileSystemAPI) Write(path string, data []byte) error {
	target := path
	mode := os.FileMode(0600)
	info, err := os.Stat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%q не является обычным файлом", path)
		}
		mode = info.Mode().Perm()
		target, err = filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".textedit-*")
	if err != nil {
		return fmt.Errorf("сохранение %q: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("сохранение %q: %w", path, err)
	}
	return nil
}
