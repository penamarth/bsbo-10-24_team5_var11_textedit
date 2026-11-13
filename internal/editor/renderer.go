package editor

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ErrorView is a diagnostics entry for display: the word, its description and
// its byte offsets in the document text.
type ErrorView struct {
	Word    string
	Message string
	Start   int
	End     int
	Skipped bool
}

// Commands connects user actions to AppContext methods. Grouped into a struct
// so the binding stays readable as commands accumulate.
type Commands struct {
	Change      func(string)
	New         func()
	Open        func()
	Save        func()
	SaveAs      func()
	Lint        func()
	NextError   func()
	PrevError   func()
	FixError    func()
	SelectError func(int)
	Quit        func()
}

// Renderer is the concrete terminal view. It receives values, never a Document.
// Command callbacks are connected at the composition root, avoiding ownership
// of AppContext by the view and keeping domain state encapsulated.
type Renderer struct {
	app       *tview.Application
	pages     *tview.Pages
	area      *tview.TextArea
	status    *tview.TextView
	editorCol *tview.Flex
	root      *tview.Flex
	errors    *tview.List
	views     []ErrorView
	errCount  int
	state     string
	path      string
	modified  bool
	updating  bool
	dialog    bool
}

func NewRenderer() *Renderer {
	r := &Renderer{
		app:    tview.NewApplication(),
		pages:  tview.NewPages(),
		area:   tview.NewTextArea(),
		status: tview.NewTextView(),
		errors: tview.NewList(),
	}
	r.area.SetBorder(true)
	r.area.SetPlaceholder("Введите текст…")
	r.errors.SetBorder(true).SetTitle(" Ошибки ")
	r.errors.ShowSecondaryText(false)
	help := tview.NewTextView().SetText("F1 Справка · F2 Сохранить · F3 Открыть · F4 Новый · F5 Проверка · F6 Сохранить как · F7/F8 Ошибки · F9 Исправить · F10 Выход")
	r.editorCol = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(r.area, 0, 1, true).AddItem(r.status, 1, 0, false).AddItem(help, 2, 0, false)
	// The diagnostics panel stays in the layout permanently: adding or
	// removing it would resize the text area, and tview resets the text
	// selection whenever the width changes.
	r.root = tview.NewFlex().AddItem(r.editorCol, 0, 1, true).AddItem(r.errors, 34, 0, false)
	r.pages.AddPage("editor", r.root, true, true)
	r.app.SetRoot(r.pages, true).EnablePaste(true)
	return r
}

func (r *Renderer) Bind(c Commands) {
	r.area.SetChangedFunc(func() {
		if !r.updating {
			c.Change(r.area.GetText())
		}
	})
	// Navigating the list moves the highlight in the text; Enter opens the
	// suggestion dialog for the chosen error; Escape returns to the text.
	r.errors.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		if index >= 0 && index < len(r.views) {
			v := r.views[index]
			r.area.Select(v.Start, v.End)
		}
	})
	r.errors.SetSelectedFunc(func(index int, _ string, _ string, _ rune) {
		if c.SelectError != nil {
			c.SelectError(index)
		}
	})
	r.errors.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			r.app.SetFocus(r.area)
			return nil
		}
		return event
	})
	r.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// tview otherwise treats Ctrl-C as an unconditional application exit.
		// In the editor it means Copy; in a dialog it means Cancel.
		if event.Key() == tcell.KeyCtrlC {
			if r.dialog {
				r.dismiss()
				return nil
			}
			return tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)
		}
		if r.dialog {
			return event
		}
		if event.Key() == tcell.KeyTab && r.errCount > 0 {
			if r.app.GetFocus() == r.area {
				r.app.SetFocus(r.errors)
			} else {
				r.app.SetFocus(r.area)
			}
			return nil
		}
		if event.Key() >= tcell.KeyF1 && event.Key() <= tcell.KeyF10 {
			// Let TextArea observe the command boundary (these keys do not edit
			// text), so subsequent typing cannot join an undo group across Save.
			r.area.InputHandler()(event, func(p tview.Primitive) { r.app.SetFocus(p) })
		}
		switch event.Key() {
		case tcell.KeyF1:
			r.Message("F2 — сохранить; F3 — открыть; F4 — новый\nF5 — проверить правописание; F7/F8 — ошибки; F9 — исправить\nF6 — сохранить как; F10 — выйти\nCtrl-Z — отменить; Ctrl-Y — повторить\nShift + стрелки — выделение\nCtrl-C / Ctrl-X / Ctrl-V — внутренний буфер\nTab — панель ошибок; Escape — отменить диалог")
		case tcell.KeyF2:
			c.Save()
		case tcell.KeyF3:
			c.Open()
		case tcell.KeyF4:
			c.New()
		case tcell.KeyF5:
			c.Lint()
		case tcell.KeyF6:
			c.SaveAs()
		case tcell.KeyF7:
			c.NextError()
		case tcell.KeyF8:
			c.PrevError()
		case tcell.KeyF9:
			c.FixError()
		case tcell.KeyF10:
			c.Quit()
		default:
			return event
		}
		return nil
	})
}

func (r *Renderer) Render(text, path string, modified, replace bool) {
	if replace {
		r.updating = true
		r.area.SetText(text, false)
		r.updating = false
	}
	name := "Без имени"
	if path != "" {
		name = filepath.Base(path)
	}
	if modified {
		name += " *"
	}
	r.area.SetTitle(" " + tview.Escape(name) + " ")
	r.state = "Сохранён"
	if path == "" {
		r.state = "Новый документ"
	}
	if modified {
		r.state = "Есть несохранённые изменения"
	}
	r.path = path
	r.modified = modified
	r.refreshStatus()
}

// refreshStatus composes the status line: document state, path and the number
// of diagnostics from the last check.
func (r *Renderer) refreshStatus() {
	line := r.state + "  " + r.path
	if r.errCount > 0 {
		line += fmt.Sprintf("  ·  Ошибок: %d", r.errCount)
	}
	r.status.SetText(line)
}

// ShowDiagnostics fills the diagnostics panel with the latest results.
// Called after every check and after every text edit that invalidates
// earlier results.
func (r *Renderer) ShowDiagnostics(views []ErrorView) {
	r.views = views
	r.errors.Clear()
	for i, v := range views {
		label := fmt.Sprintf("%d. %s — %s", i+1, v.Word, v.Message)
		if v.Skipped {
			label += " (пропущена)"
		}
		r.errors.AddItem(label, "", 0, nil)
	}
	r.errCount = len(views)
	if r.errCount == 0 && r.app.GetFocus() == r.errors {
		r.app.SetFocus(r.area)
	}
	r.refreshStatus()
}

// HighlightError selects the error's word in the text and marks the
// corresponding entry in the diagnostics panel.
func (r *Renderer) HighlightError(index, start, end int) {
	if index >= 0 && index < r.errors.GetItemCount() {
		r.errors.SetCurrentItem(index)
	}
	r.area.Select(start, end)
}

// ChooseFix opens the suggestion dialog for one error. Buttons are the
// suggested corrections followed by "Пропустить" and "Отмена". done receives
// the button index; Escape closes the dialog without calling done.
func (r *Renderer) ChooseFix(word string, suggestions []string, done func(int)) {
	buttons := append(append([]string{}, suggestions...), "Пропустить", "Отмена")
	modal := tview.NewModal().SetText(tview.Escape("Ошибка: " + word)).AddButtons(buttons)
	modal.SetDoneFunc(func(index int, _ string) { r.dismiss(); done(index) })
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			r.dismiss()
			return nil
		}
		return event
	})
	r.dialog = true
	r.pages.AddPage("dialog", modal, true, true)
	r.app.SetFocus(modal)
}

func (r *Renderer) show(primitive tview.Primitive, width, height int) {
	r.dialog = true
	grid := tview.NewGrid().SetColumns(0, width, 0).SetRows(0, height, 0).
		AddItem(primitive, 1, 1, 1, 1, 0, 0, true)
	r.pages.AddPage("dialog", grid, true, true)
	r.app.SetFocus(primitive)
}

func (r *Renderer) dismiss() {
	r.pages.RemovePage("dialog")
	r.dialog = false
	r.app.SetFocus(r.area)
}

func (r *Renderer) Choose(message string, buttons []string, done func(int)) {
	modal := tview.NewModal().SetText(tview.Escape(message)).AddButtons(buttons)
	modal.SetDoneFunc(func(index int, _ string) { r.dismiss(); done(index) })
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			r.dismiss()
			return nil
		}
		return event
	})
	// Modal computes its own centred dimensions.
	r.dialog = true
	r.pages.AddPage("dialog", modal, true, true)
	r.app.SetFocus(modal)
}

func (r *Renderer) Message(message string) { r.Choose(message, []string{"ОК"}, func(int) {}) }

func (r *Renderer) AskPath(title, initial string, done func(string)) {
	form := tview.NewForm()
	form.AddInputField("Путь", initial, 0, nil, nil)
	accept := func() {
		path := form.GetFormItem(0).(*tview.InputField).GetText()
		if path == "" {
			return
		}
		r.dismiss()
		done(path)
	}
	form.AddButton("ОК", accept).AddButton("Отмена", r.dismiss)
	form.SetBorder(true).SetTitle(title)
	form.SetCancelFunc(r.dismiss)
	form.GetFormItem(0).(*tview.InputField).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			r.dismiss()
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			accept()
			return nil
		}
		return event
	})
	r.show(form, 64, 7)
}

func (r *Renderer) Run() error { return r.app.Run() }
func (r *Renderer) Stop()      { r.app.Stop() }
