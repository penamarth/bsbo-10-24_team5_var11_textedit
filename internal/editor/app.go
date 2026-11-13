package editor

import (
	"os"
	"path/filepath"

	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/lint"
)

// AppContext owns the document and coordinates commands. Its explicit dependency
// on Renderer follows AppContext -> Renderer in the class diagram.
type AppContext struct {
	document     *Document
	renderer     *Renderer
	files        *FileSystemAPI
	linter       lint.Linter
	lintOpts     lint.Options
	currentError int
}

func NewAppContext(files *FileSystemAPI, renderer *Renderer, linter lint.Linter) *AppContext {
	a := &AppContext{
		files:        files,
		renderer:     renderer,
		linter:       linter,
		lintOpts:     lint.Options{Language: "ru"},
		currentError: -1,
	}
	a.document = NewDocument(files, linter)
	a.refresh(true)
	return a
}

func (a *AppContext) refresh(replace bool) {
	a.renderer.Render(a.document.Text(), a.document.Path(), a.document.Modified(), replace)
}

// syncDiagnostics pushes the document's current diagnostics to the view.
func (a *AppContext) syncDiagnostics() {
	diagnostics := a.document.Diagnostics()
	views := make([]ErrorView, len(diagnostics))
	for i, d := range diagnostics {
		views[i] = ErrorView{
			Word:    d.Word,
			Message: d.Message,
			Start:   d.Start,
			End:     d.End,
			Skipped: d.Skipped,
		}
	}
	a.renderer.ShowDiagnostics(views)
}

func (a *AppContext) SetText(text string) {
	a.document.SetText(text)
	a.currentError = -1
	a.syncDiagnostics()
	a.refresh(false)
}

func (a *AppContext) OpenInitial(path string) { a.open(path) }

func (a *AppContext) open(path string) {
	d := NewDocument(a.files, a.linter)
	if err := d.Open(path); err != nil {
		a.renderer.Message(err.Error())
		return
	}
	a.document = d
	a.currentError = -1
	a.syncDiagnostics()
	a.refresh(true)
}

func (a *AppContext) New() {
	a.guard(func() {
		a.document = NewDocument(a.files, a.linter)
		a.currentError = -1
		a.syncDiagnostics()
		a.refresh(true)
	})
}

func (a *AppContext) Open() {
	a.renderer.AskPath("Открыть файл", "", func(path string) {
		a.guard(func() { a.open(path) })
	})
}

func (a *AppContext) Save()   { a.save(false, func() {}) }
func (a *AppContext) SaveAs() { a.save(true, func() {}) }
func (a *AppContext) Quit()   { a.guard(a.renderer.Stop) }

// guard continues a destructive command only after an explicit decision and,
// when requested, a successful save. Cancelled dialogs never call continuation.
func (a *AppContext) guard(continuation func()) {
	if !a.document.Modified() {
		continuation()
		return
	}
	a.renderer.Choose("Есть несохранённые изменения.", []string{"Сохранить", "Не сохранять", "Отмена"}, func(index int) {
		switch index {
		case 0:
			a.save(false, continuation)
		case 1:
			continuation()
		}
	})
}

func (a *AppContext) save(as bool, continuation func()) {
	write := func(path string) {
		if err := a.document.Save(path); err != nil {
			a.renderer.Message(err.Error())
			return
		}
		a.refresh(false)
		continuation()
	}
	if !as && a.document.Path() != "" {
		write(a.document.Path())
		return
	}
	a.renderer.AskPath("Сохранить как", a.document.Path(), func(path string) {
		abs, err := filepath.Abs(path)
		if err != nil {
			a.renderer.Message(err.Error())
			return
		}
		_, err = os.Lstat(abs)
		if err == nil && abs != a.document.Path() {
			a.renderer.Choose("Заменить существующий файл?", []string{"Заменить", "Отмена"}, func(index int) {
				if index == 0 {
					write(abs)
				}
			})
			return
		}
		if err != nil && !os.IsNotExist(err) {
			a.renderer.Message(err.Error())
			return
		}
		write(abs)
	})
}

// Lint runs the spelling check. Results are shown in the diagnostics panel;
// a failed check only notifies the user and never blocks editing.
func (a *AppContext) Lint() {
	res := a.document.RunLint(a.lintOpts)
	if !res.Success {
		a.renderer.Message(res.ErrorMessage)
		return
	}
	diagnostics := a.document.Diagnostics()
	if len(diagnostics) == 0 {
		a.currentError = -1
		a.syncDiagnostics()
		a.renderer.Message("Ошибок не найдено")
		return
	}
	a.currentError = 0
	a.syncDiagnostics()
	a.focusError(0)
}

// NextError and PrevError cycle the highlight over unskipped diagnostics.
func (a *AppContext) NextError() { a.moveError(1) }
func (a *AppContext) PrevError() { a.moveError(-1) }

func (a *AppContext) moveError(dir int) {
	diagnostics := a.document.Diagnostics()
	if len(diagnostics) == 0 {
		return
	}
	i := a.currentError
	if i < 0 {
		i = 0
	}
	for range diagnostics {
		i = (i + dir + len(diagnostics)) % len(diagnostics)
		if !diagnostics[i].Skipped {
			a.currentError = i
			a.focusError(i)
			return
		}
	}
}

func (a *AppContext) focusError(i int) {
	diagnostics := a.document.Diagnostics()
	if i < 0 || i >= len(diagnostics) {
		return
	}
	a.renderer.HighlightError(i, diagnostics[i].Start, diagnostics[i].End)
}

// FixError opens the suggestion dialog for the currently highlighted error.
func (a *AppContext) FixError() {
	if a.currentError >= 0 {
		a.SelectError(a.currentError)
	}
}

// SelectError opens the suggestion dialog for diagnostic index.
// Choice indexes: 0..len(suggestions)-1 apply that fix,
// len(suggestions) skips the error, anything else cancels.
func (a *AppContext) SelectError(index int) {
	diagnostics := a.document.Diagnostics()
	if index < 0 || index >= len(diagnostics) {
		return
	}
	d := diagnostics[index]
	a.renderer.ChooseFix(d.Word, d.Suggestions, func(choice int) {
		switch {
		case choice >= 0 && choice < len(d.Suggestions):
			a.applyFix(index, d.Suggestions[choice])
		case choice == len(d.Suggestions):
			a.skipError(index)
		}
	})
}

func (a *AppContext) applyFix(index int, replacement string) {
	res := a.document.ApplyFix(index, replacement)
	a.refresh(true)
	if !res.Success {
		a.renderer.Message(res.ErrorMessage)
		a.syncDiagnostics()
		return
	}
	diagnostics := a.document.Diagnostics()
	a.syncDiagnostics()
	if len(diagnostics) == 0 {
		a.currentError = -1
		a.renderer.Message("Ошибок не найдено")
		return
	}
	a.currentError = 0
	a.focusError(0)
}

func (a *AppContext) skipError(index int) {
	a.document.SkipError(index)
	a.syncDiagnostics()
	a.moveError(1)
}
