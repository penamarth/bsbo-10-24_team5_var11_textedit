package main

import (
	"fmt"
	"os"

	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/editor"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "Использование: textedit [путь]")
		os.Exit(2)
	}
	renderer := editor.NewRenderer()
	app := editor.NewAppContext(editor.NewFileSystemAPI(), renderer)
	renderer.Bind(app.SetText, app.New, app.Open, app.Save, app.SaveAs, app.Quit)
	if len(os.Args) == 2 {
		app.OpenInitial(os.Args[1])
	}
	if err := renderer.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
