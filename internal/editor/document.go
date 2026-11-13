package editor

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Document owns the editable buffer and its last successfully saved state.
// Neither the terminal nor AppContext may modify its fields directly.
type Document struct {
	text, path, savedText string
	original              []byte
	bom                   bool
	newline               string
	files                 *FileSystemAPI
}

func NewDocument(files *FileSystemAPI) *Document {
	return &Document{files: files, newline: "\n"}
}

func (d *Document) Text() string        { return d.text }
func (d *Document) Path() string        { return d.path }
func (d *Document) Modified() bool      { return d.text != d.savedText }
func (d *Document) SetText(text string) { d.text = text }

// Open commits new state only when reading and validation both succeed.
func (d *Document) Open(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	data, err := d.files.Read(abs)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("файл %q не содержит корректный UTF-8", path)
	}
	bom := bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf})
	content := data
	if bom {
		content = content[3:]
	}
	newline := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		newline = "\r\n"
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	d.text, d.savedText, d.path = text, text, abs
	d.bom, d.newline, d.original = bom, newline, bytes.Clone(data)
	return nil
}

func (d *Document) Save(path string) error {
	if path == "" {
		return fmt.Errorf("не указан путь сохранения")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	data := []byte(strings.ReplaceAll(d.text, "\n", d.newline))
	if d.bom {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	if !d.Modified() && d.original != nil {
		data = bytes.Clone(d.original)
	}
	if err := d.files.Write(abs, data); err != nil {
		return err
	}
	d.path, d.savedText, d.original = abs, d.text, data
	return nil
}
