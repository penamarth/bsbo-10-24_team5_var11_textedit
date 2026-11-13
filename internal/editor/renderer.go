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
	views     []ErrorView
	errCount  int
	state     string
	path      string
	modified  bool
	updating  bool
	dialog    bool
	commands  Commands
}

func NewRenderer() *Renderer {
	r := &Renderer{
		app:    tview.NewApplication(),
		pages:  tview.NewPages(),
		area:   tview.NewTextArea(),
		status: tview.NewTextView(),
	}
	r.area.SetBorder(true)
	r.area.SetPlaceholder("Введите текст…")
	help := tview.NewTextView().SetText("F1 Справка · F2 Сохранить · F3 Открыть · F4 Новый · F5 Проверка · F6 Сохранить как · F7/F8 Ошибки · F9 Исправить · F10 Выход")
	r.editorCol = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(r.area, 0, 1, true).AddItem(r.status, 1, 0, false).AddItem(help, 2, 0, false)
	r.root = tview.NewFlex().AddItem(r.editorCol, 0, 1, true)
	r.pages.AddPage("editor", r.root, true, true)
	r.app.SetRoot(r.pages, true).EnablePaste(true)
	return r
}

func (r *Renderer) Bind(c Commands) {
	r.commands = c
	r.area.SetChangedFunc(func() {
		if !r.updating {
			c.Change(r.area.GetText())
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
		if event.Key() == tcell.KeyTab && r.errCount > 0 {
			r.showErrorList()
			return nil
		}
		if event.Key() >= tcell.KeyF1 && event.Key() <= tcell.KeyF10 {
			// Let TextArea observe the command boundary (these keys do not edit
			// text), so subsequent typing cannot join an undo group across Save.
			r.area.InputHandler()(event, func(p tview.Primitive) { r.app.SetFocus(p) })
		}
		switch event.Key() {
		case tcell.KeyF1:
			r.Message("F2 — сохранить; F3 — открыть; F4 — новый\nF5 — проверить правописание; F7/F8 — ошибки; F9 — исправить\nF6 — сохранить как; F10 — выйти\nCtrl-Z — отменить; Ctrl-Y — повторить\nShift + стрелки — выделение\nCtrl-C / Ctrl-X / Ctrl-V — внутренний буфер\nTab — список ошибок; Escape — отменить диалог")
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
		line += fmt.Sprintf("  ·  Ошибок: %d (Tab — список)", r.errCount)
	}
	r.status.SetText(line)
}

// ShowDiagnostics stores the latest check results for the status line and the
// on-demand error list. Called after every check and after every text edit
// that invalidates earlier results.
func (r *Renderer) ShowDiagnostics(views []ErrorView) {
	r.views = views
	r.errCount = len(views)
	r.refreshStatus()
}

// HighlightError selects the error's word in the text — the selection is the
// visual indication of the current error (TextArea cannot style ranges).
func (r *Renderer) HighlightError(index, start, end int) {
	r.area.Select(start, end)
}

// CursorPos returns the byte offset of the cursor in the document text.
func (r *Renderer) CursorPos() int {
	_, start, _ := r.area.GetSelection()
	return start
}

// fixMenuAnchored selects the fix-menu style: false shows a centered modal
// (always fully visible), true shows the experimental anchored dropdown
// (chooseFixAnchored — kept for a possible return, see its comment).
const fixMenuAnchored = false

// ChooseFix shows the corrections for one error. Items are the suggested
// corrections followed by "Пропустить" and "Отмена"; done receives the item
// index, Escape closes the menu without calling done.
func (r *Renderer) ChooseFix(word string, suggestions []string, done func(int)) {
	if fixMenuAnchored {
		r.chooseFixAnchored(word, suggestions, done)
		return
	}
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

// chooseFixAnchored shows the corrections as a dropdown list anchored under
// the selected word, like the quick-fix menu in VS Code. EXPERIMENTAL, kept
// for a possible return: near the screen edges the anchored position still
// has quirks (items can be clipped or hard to reach). Enable with
// fixMenuAnchored.
func (r *Renderer) chooseFixAnchored(word string, suggestions []string, done func(int)) {
	list := tview.NewList()
	items := append(append([]string{}, suggestions...), "Пропустить", "Отмена")
	for i, item := range items {
		j := i
		list.AddItem(item, "", 0, func() { r.dismiss(); done(j) })
	}
	list.SetBorder(true).SetTitle(" " + tview.Escape(word) + " ")
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			r.dismiss()
			return nil
		}
		return event
	})

	// Anchor the dropdown under the cursor inside the text area. When the
	// line is too close to the bottom edge, the menu flips above the word.
	_, _, toRow, toCol := r.area.GetCursor()
	rowOff, colOff := r.area.GetOffset()
	ax, ay, aw, ah := r.area.GetInnerRect()
	w := len([]rune(word)) + 8
	for _, s := range items {
		if n := len([]rune(s)) + 6; n > w {
			w = n
		}
	}
	if w > 40 {
		w = 40
	}
	h := len(items) + 2
	if h > 10 {
		h = 10
	}
	x := ax + toCol - colOff
	if x+w > ax+aw {
		x = ax + aw - w
	}
	if x < ax {
		x = ax
	}
	y := ay + toRow - rowOff + 1
	if y+h > ay+ah {
		y = ay + toRow - rowOff - h // flip above the word
	}
	if y < ay {
		y = ay
	}
	grid := tview.NewGrid().SetColumns(x, w, 0).SetRows(y, h, 0).
		AddItem(list, 0, 1, 1, 1, 0, 0, true)
	r.dialog = true
	r.pages.AddPage("dialog", grid, true, true)
	r.app.SetFocus(list)
}

// showErrorList opens a centered modal listing every diagnostic. Enter jumps
// to the chosen error and opens the fix dropdown; Escape closes the list.
func (r *Renderer) showErrorList() {
	list := tview.NewList()
	for i, v := range r.views {
		j, v := i, v
		label := fmt.Sprintf("%d. %s — %s", i+1, v.Word, v.Message)
		if v.Skipped {
			label += " (пропущена)"
		}
		list.AddItem(label, "", 0, func() {
			r.dismiss()
			if r.commands.SelectError != nil {
				r.commands.SelectError(j)
			}
		})
	}
	list.SetBorder(true).SetTitle(" Ошибки ")
	list.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		if index >= 0 && index < len(r.views) {
			v := r.views[index]
			r.area.Select(v.Start, v.End)
		}
	})
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			r.dismiss()
			return nil
		}
		return event
	})
	h := len(r.views) + 2
	if h > 15 {
		h = 15
	}
	r.show(list, 50, h)
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
