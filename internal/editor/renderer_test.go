package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Exercise actual widget input handlers and drawing on a terminal simulation.
// All callbacks run synchronously on the same thread, just as in tview's loop.
func terminal(t *testing.T) (*AppContext, *Renderer, func(tcell.Key, rune)) {
	t.Helper()
	r := NewRenderer()
	screen := tcell.NewSimulationScreen("UTF-8")
	r.app.SetScreen(screen)
	screen.SetSize(100, 30)
	t.Cleanup(screen.Fini)
	a := NewAppContext(NewFileSystemAPI(), r)
	r.Bind(a.SetText, a.New, a.Open, a.Save, a.SaveAs, a.Quit)
	r.app.SetFocus(r.area)
	draw := func() { r.pages.SetRect(0, 0, 100, 30); r.pages.Draw(screen); screen.Show() }
	draw()
	key := func(key tcell.Key, ch rune) {
		event := r.app.GetInputCapture()(tcell.NewEventKey(key, ch, tcell.ModNone))
		if event != nil {
			focus := r.app.GetFocus()
			if handler := focus.InputHandler(); handler != nil {
				handler(event, func(p tview.Primitive) { r.app.SetFocus(p) })
			}
		}
		draw()
	}
	return a, r, key
}

func typeText(key func(tcell.Key, rune), text string) {
	for _, ch := range text {
		key(tcell.KeyRune, ch)
	}
}

func TestTerminalSaveUndoAndOpen(t *testing.T) {
	a, r, key := terminal(t)
	typeText(key, "Привет")
	if a.document.Text() != "Привет" || !a.document.Modified() {
		t.Fatal("input not propagated")
	}
	path := filepath.Join(t.TempDir(), "текст.txt")
	key(tcell.KeyF2, 0)
	typeText(key, path)
	key(tcell.KeyEnter, 0)
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "Привет" {
		t.Fatalf("save: %q %v", data, err)
	}
	if a.document.Modified() || r.dialog {
		t.Fatal("save did not finish")
	}
	key(tcell.KeyRune, '!')
	key(tcell.KeyCtrlZ, 0)
	if a.document.Modified() {
		t.Fatal("undo did not restore saved state")
	}
	key(tcell.KeyCtrlY, 0)
	if a.document.Text() != "Привет!" {
		t.Fatal("redo failed")
	}
	key(tcell.KeyCtrlZ, 0)
	key(tcell.KeyF4, 0)
	if a.document.Text() != "" {
		t.Fatal("new failed")
	}
	key(tcell.KeyF3, 0)
	typeText(key, path)
	key(tcell.KeyEnter, 0)
	if a.document.Text() != "Привет" || r.app.GetFocus() != r.area {
		t.Fatal("open/focus failed")
	}
}

func TestTerminalGuardsAndFailedSave(t *testing.T) {
	a, r, key := terminal(t)
	typeText(key, "важно")
	key(tcell.KeyF4, 0)
	if !r.dialog {
		t.Fatal("missing unsaved warning")
	}
	key(tcell.KeyEscape, 0)
	if a.document.Text() != "важно" || r.dialog {
		t.Fatal("cancel lost text")
	}
	key(tcell.KeyF4, 0)
	key(tcell.KeyEnter, 0)  // Save before New.
	key(tcell.KeyEscape, 0) // Cancel path entry.
	if a.document.Text() != "важно" {
		t.Fatal("cancelled save continued New")
	}
	key(tcell.KeyF4, 0)
	key(tcell.KeyEnter, 0)
	typeText(key, filepath.Join(t.TempDir(), "missing", "file"))
	key(tcell.KeyEnter, 0)
	if !r.dialog || a.document.Text() != "важно" || !a.document.Modified() {
		t.Fatal("failed save lost text")
	}
	key(tcell.KeyEscape, 0)
	key(tcell.KeyF4, 0)
	key(tcell.KeyRight, 0) // Do not save.
	key(tcell.KeyEnter, 0)
	if a.document.Text() != "" || r.dialog {
		t.Fatal("discard did not create new buffer")
	}
}

func TestTerminalOverwriteAndReadFailure(t *testing.T) {
	a, r, key := terminal(t)
	path := filepath.Join(t.TempDir(), "existing")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	typeText(key, "new")
	key(tcell.KeyF6, 0)
	typeText(key, path)
	key(tcell.KeyEnter, 0)
	key(tcell.KeyEscape, 0)
	data, _ := os.ReadFile(path)
	if string(data) != "old" || a.document.Path() != "" {
		t.Fatal("overwrite cancelled incorrectly")
	}
	key(tcell.KeyF6, 0)
	typeText(key, path)
	key(tcell.KeyEnter, 0)
	key(tcell.KeyEnter, 0)
	data, _ = os.ReadFile(path)
	if string(data) != "new" {
		t.Fatal("overwrite not applied")
	}
	key(tcell.KeyF3, 0)
	typeText(key, path+".missing")
	key(tcell.KeyEnter, 0)
	if !r.dialog || a.document.Text() != "new" {
		t.Fatal("failed open replaced buffer")
	}
	key(tcell.KeyEscape, 0)
	key(tcell.KeyRune, '!')
	if !strings.Contains(r.status.GetText(false), "несохранённые") {
		t.Fatal("status not updated")
	}
	key(tcell.KeyF10, 0)
	if !r.dialog {
		t.Fatal("quit bypassed guard")
	}
	key(tcell.KeyEscape, 0)
	key(tcell.KeyCtrlC, 0)
	if a.document.Text() != "new!" {
		t.Fatal("copy changed document")
	}
}
