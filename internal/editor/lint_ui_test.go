package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/lint"
)

// terminalWith mirrors terminal() but wires a real linter and lint commands.
func terminalWithLinter(t *testing.T, linter lint.Linter) (*AppContext, *Renderer, func(tcell.Key, rune)) {
	t.Helper()
	r := NewRenderer()
	screen := tcell.NewSimulationScreen("UTF-8")
	r.app.SetScreen(screen)
	screen.SetSize(100, 30)
	t.Cleanup(screen.Fini)
	a := NewAppContext(NewFileSystemAPI(), r, linter)
	r.Bind(Commands{
		Change:      a.SetText,
		New:         a.New,
		Open:        a.Open,
		Save:        a.Save,
		SaveAs:      a.SaveAs,
		Lint:        a.Lint,
		NextError:   a.NextError,
		PrevError:   a.PrevError,
		FixError:    a.FixError,
		SelectError: a.SelectError,
		Quit:        a.Quit,
	})
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

func TestTerminalLintHighlightsAndFixes(t *testing.T) {
	a, r, key := terminalWithLinter(t, lint.NewTextLinter(nil))
	typeText(key, "програму")
	key(tcell.KeyF5, 0)
	if r.errCount != 1 || r.errors.GetItemCount() != 1 {
		t.Fatalf("diagnostics not shown: %d", r.errCount)
	}
	if sel, _, _ := r.area.GetSelection(); sel != "програму" {
		t.Fatalf("error not highlighted: %q", sel)
	}
	// Open the fix dialog and apply the first suggestion.
	key(tcell.KeyF9, 0)
	if !r.dialog {
		t.Fatal("fix dialog not opened")
	}
	key(tcell.KeyEnter, 0)
	// Zero remaining errors show the «Ошибок не найдено» message.
	if !r.dialog {
		t.Fatal("missing 'no errors' message")
	}
	key(tcell.KeyEscape, 0)
	if a.document.Text() == "програму" {
		t.Fatal("fix not applied")
	}
	if r.errCount != 0 {
		t.Fatal("recheck did not run")
	}
}

func TestTerminalLintNoErrorsAndSkip(t *testing.T) {
	a, r, key := terminalWithLinter(t, lint.NewTextLinter(nil))
	typeText(key, "программа")
	key(tcell.KeyF5, 0)
	if !r.dialog || r.errCount != 0 {
		t.Fatal("missing 'no errors' message")
	}
	key(tcell.KeyEscape, 0)

	// New text with an error: lint, then skip the error via the fix dialog.
	typeText(key, " програму")
	key(tcell.KeyF5, 0)
	if r.errCount != 1 {
		t.Fatalf("diagnostics not shown: %d", r.errCount)
	}
	key(tcell.KeyF9, 0)
	// Move to "Пропустить": it is the button right after the last suggestion.
	sugg := len(a.document.Diagnostics()[0].Suggestions)
	for i := 0; i < sugg; i++ {
		key(tcell.KeyRight, 0)
	}
	key(tcell.KeyEnter, 0)
	diags := a.document.Diagnostics()
	if len(diags) != 1 || !diags[0].Skipped {
		t.Fatal("error not skipped")
	}
	if a.document.Text() != "программа програму" {
		t.Fatalf("skip changed text: %q", a.document.Text())
	}
}

func TestTerminalLintNavigation(t *testing.T) {
	_, r, key := terminalWithLinter(t, lint.NewTextLinter(nil))
	typeText(key, "абвгд ежзик")
	key(tcell.KeyF5, 0)
	if r.errCount != 2 {
		t.Fatalf("expected 2 errors, got %d", r.errCount)
	}
	if sel, _, _ := r.area.GetSelection(); sel != "абвгд" {
		t.Fatalf("first error selected: %q", sel)
	}
	key(tcell.KeyF7, 0)
	if sel, _, _ := r.area.GetSelection(); sel != "ежзик" {
		t.Fatalf("next error: %q", sel)
	}
	key(tcell.KeyF7, 0)
	if sel, _, _ := r.area.GetSelection(); sel != "абвгд" {
		t.Fatalf("wrap to first: %q", sel)
	}
	key(tcell.KeyF8, 0)
	if sel, _, _ := r.area.GetSelection(); sel != "ежзик" {
		t.Fatalf("prev error: %q", sel)
	}
}

func TestTerminalLintFailureDoesNotBlock(t *testing.T) {
	a, r, key := terminalWithLinter(t, &fakeLinter{result: lint.Result{Success: false, ErrorMessage: "сбой"}})
	typeText(key, "текст")
	key(tcell.KeyF5, 0)
	if !r.dialog {
		t.Fatal("failure not notified")
	}
	key(tcell.KeyEscape, 0)
	key(tcell.KeyRune, '!')
	if a.document.Text() != "текст!" {
		t.Fatal("editing blocked after linter failure")
	}
}

func TestTerminalLintListEnterOpensDialog(t *testing.T) {
	_, r, key := terminalWithLinter(t, lint.NewTextLinter(nil))
	typeText(key, "програму")
	key(tcell.KeyF5, 0)
	key(tcell.KeyTab, 0)
	if r.app.GetFocus() != r.errors {
		t.Fatal("tab did not focus error list")
	}
	key(tcell.KeyEnter, 0)
	if !r.dialog {
		t.Fatal("list enter did not open fix dialog")
	}
}
