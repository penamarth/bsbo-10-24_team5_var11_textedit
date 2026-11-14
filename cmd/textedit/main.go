package main

import (
	"fmt"
	"os"

	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/editor"
	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/lint"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "Использование: textedit [путь]")
		os.Exit(2)
	}
	renderer := editor.NewRenderer()

	linter, err := lint.NewHunspellLinter()
	var l lint.Linter = lint.NewTextLinter(nil)
	if err == nil {
		l = linter
	}
	app := editor.NewAppContext(editor.NewFileSystemAPI(), renderer, l)
	renderer.Bind(editor.Commands{
		Change:      app.SetText,
		New:         app.New,
		Open:        app.Open,
		Save:        app.Save,
		SaveAs:      app.SaveAs,
		Lint:        app.Lint,
		NextError:   app.NextError,
		PrevError:   app.PrevError,
		FixError:    app.FixError,
		SelectError: app.SelectError,
		Quit:        app.Quit,
	})
	if len(os.Args) == 2 {
		app.OpenInitial(os.Args[1])
	}
	if err := renderer.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
