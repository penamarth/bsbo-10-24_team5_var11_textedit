package editor

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Renderer is the concrete terminal view. It receives values, never a Document.
// Command callbacks are connected at the composition root, avoiding ownership
// of AppContext by the view and keeping domain state encapsulated.
type Renderer struct {
	app              *tview.Application
	pages            *tview.Pages
	area             *tview.TextArea
	status           *tview.TextView
	updating, dialog bool
}

func NewRenderer() *Renderer {
	r := &Renderer{app: tview.NewApplication(), pages: tview.NewPages(), area: tview.NewTextArea(), status: tview.NewTextView()}
	r.area.SetBorder(true)
	r.area.SetPlaceholder("Введите текст…")
	help := tview.NewTextView().SetText("F1 Справка · F2 Сохранить · F3 Открыть · F4 Новый · F6 Сохранить как · F10 Выход")
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(r.area, 0, 1, true).AddItem(r.status, 1, 0, false).AddItem(help, 2, 0, false)
	r.pages.AddPage("editor", layout, true, true)
	r.app.SetRoot(r.pages, true).EnablePaste(true)
	return r
}

func (r *Renderer) Bind(change func(string), newDocument, open, save, saveAs, quit func()) {
	r.area.SetChangedFunc(func() {
		if !r.updating {
			change(r.area.GetText())
		}
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
		if event.Key() >= tcell.KeyF1 && event.Key() <= tcell.KeyF10 {
			// Let TextArea observe the command boundary (these keys do not edit
			// text), so subsequent typing cannot join an undo group across Save.
			r.area.InputHandler()(event, func(p tview.Primitive) { r.app.SetFocus(p) })
		}
		switch event.Key() {
		case tcell.KeyF1:
			r.Message("F2 — сохранить; F3 — открыть; F4 — новый\nF6 — сохранить как; F10 — выйти\nCtrl-Z — отменить; Ctrl-Y — повторить\nShift + стрелки — выделение\nCtrl-C / Ctrl-X / Ctrl-V — внутренний буфер\nEscape — отменить диалог")
		case tcell.KeyF2:
			save()
		case tcell.KeyF3:
			open()
		case tcell.KeyF4:
			newDocument()
		case tcell.KeyF6:
			saveAs()
		case tcell.KeyF10:
			quit()
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
	state := "Сохранён"
	if path == "" {
		state = "Новый документ"
	}
	if modified {
		state = "Есть несохранённые изменения"
	}
	r.status.SetText(state + "  " + path)
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
