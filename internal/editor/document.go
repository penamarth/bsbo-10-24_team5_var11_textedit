package editor

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/lint"
)

// Diagnostic is a lint error anchored to the document text. Start and End are
// byte offsets into Document.Text(), half-open. Skipped marks errors the user
// chose to ignore in this session.
type Diagnostic struct {
	lint.Error
	Skipped bool
}

// Document owns the editable buffer, its last successfully saved state and
// lint diagnostics. Neither the terminal nor AppContext may modify its fields
// directly.
type Document struct {
	text, path, savedText string
	original              []byte
	bom                   bool
	newline               string
	files                 *FileSystemAPI
	linter                lint.Linter
	lintOpts              lint.Options
	diagnostics           []Diagnostic
}

func NewDocument(files *FileSystemAPI, linter lint.Linter) *Document {
	return &Document{files: files, newline: "\n", linter: linter}
}

func (d *Document) Text() string   { return d.text }
func (d *Document) Path() string   { return d.path }
func (d *Document) Modified() bool { return d.text != d.savedText }

// SetText replaces the buffer. Diagnostics from an earlier check are stale
// afterwards and therefore dropped; the user may re-run the check.
func (d *Document) SetText(text string) {
	d.text = text
	d.diagnostics = nil
}

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
	d.diagnostics = nil
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

// Diagnostics returns a snapshot of the last check results.
func (d *Document) Diagnostics() []Diagnostic {
	out := make([]Diagnostic, len(d.diagnostics))
	copy(out, d.diagnostics)
	return out
}

// RunLint extracts plain text without Typst markup, asks the linter to check
// it and maps reported positions back into document coordinates. The text
// itself is never modified by checking. A failed check reports through
// Result.Success and leaves existing diagnostics untouched.
func (d *Document) RunLint(opts lint.Options) lint.Result {
	if d.linter == nil {
		return lint.Result{ErrorMessage: "средство проверки правописания недоступно"}
	}
	plain := extractPlainText(d.text)
	res := d.linter.Check(plain.text, opts)
	if !res.Success {
		return res
	}
	d.lintOpts = opts
	d.diagnostics = make([]Diagnostic, len(res.Errors))
	for i, e := range res.Errors {
		e.Start, e.End = plain.mapRange(e.Start, e.End)
		d.diagnostics[i] = Diagnostic{Error: e}
	}
	res.Errors = nil
	for _, diag := range d.diagnostics {
		res.Errors = append(res.Errors, diag.Error)
	}
	return res
}

// mapRange converts half-open byte offsets in the plain text into half-open
// byte offsets in the source document.
func (p plainText) mapRange(start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(p.offsets) {
		end = len(p.offsets)
	}
	if start >= end {
		if start < len(p.offsets) {
			return p.offsets[start], p.offsets[start]
		}
		if len(p.offsets) > 0 {
			last := p.offsets[len(p.offsets)-1] + 1
			return last, last
		}
		return 0, 0
	}
	return p.offsets[start], p.offsets[end-1] + 1
}

// ApplyFix replaces the word of diagnostic index in the in-memory buffer and
// re-runs the check so diagnostics reflect the new text. The file on disk is
// not touched.
func (d *Document) ApplyFix(index int, replacement string) lint.Result {
	if index < 0 || index >= len(d.diagnostics) {
		return lint.Result{ErrorMessage: "ошибка устарела, повторите проверку"}
	}
	e := d.diagnostics[index]
	d.text = d.text[:e.Start] + replacement + d.text[e.End:]
	return d.RunLint(d.lintOpts)
}

// SkipError marks diagnostic index as ignored for this session.
// The document text is not modified.
func (d *Document) SkipError(index int) {
	if index >= 0 && index < len(d.diagnostics) {
		d.diagnostics[index].Skipped = true
	}
}
