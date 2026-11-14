package editor

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/lint"
)

type Diagnostic struct {
	lint.Error
	Skipped bool
}

type Document struct {
	text, path, savedText string
	original              []byte
	bom                   bool
	newline               string
	files                 *FileSystemAPI
	linter                lint.Linter
	lintOpts              lint.Options
	diagnostics           []Diagnostic
	suggCache             map[string][]string
}

func NewDocument(files *FileSystemAPI, linter lint.Linter) *Document {
	return &Document{files: files, newline: "\n", linter: linter}
}

func (d *Document) Text() string   { return d.text }
func (d *Document) Path() string   { return d.path }
func (d *Document) Modified() bool { return d.text != d.savedText }

func (d *Document) SetText(text string) {
	d.text = text
	d.diagnostics = nil
	d.suggCache = nil
}

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

func (d *Document) Diagnostics() []Diagnostic {
	out := make([]Diagnostic, len(d.diagnostics))
	copy(out, d.diagnostics)
	return out
}

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
	d.suggCache = make(map[string][]string)
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

func (d *Document) Suggestions(index int) []string {
	if index < 0 || index >= len(d.diagnostics) {
		return nil
	}
	e := &d.diagnostics[index]
	if e.Suggestions != nil {
		return e.Suggestions
	}
	s, ok := d.linter.(lint.Suggester)
	if !ok {
		return nil
	}
	key := strings.ToLower(e.Word)
	if sugg, ok := d.suggCache[key]; ok {
		e.Suggestions = sugg
		return sugg
	}
	sugg := s.Suggest(e.Word, d.lintOpts.MaxSuggestions)
	d.suggCache[key] = sugg
	e.Suggestions = sugg
	return sugg
}

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

func (d *Document) ApplyFix(index int, replacement string) lint.Result {
	if index < 0 || index >= len(d.diagnostics) {
		return lint.Result{ErrorMessage: "ошибка устарела, повторите проверку"}
	}
	e := d.diagnostics[index]
	d.text = d.text[:e.Start] + replacement + d.text[e.End:]
	return d.RunLint(d.lintOpts)
}

func (d *Document) SkipError(index int) {
	if index >= 0 && index < len(d.diagnostics) {
		d.diagnostics[index].Skipped = true
	}
}
