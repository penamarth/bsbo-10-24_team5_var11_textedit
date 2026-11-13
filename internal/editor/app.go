package editor

import (
	"os"
	"path/filepath"
)

// AppContext owns the document and coordinates commands. Its explicit dependency
// on Renderer follows AppContext -> Renderer in the class diagram.
type AppContext struct {
	document *Document
	renderer *Renderer
	files    *FileSystemAPI
}

func NewAppContext(files *FileSystemAPI, renderer *Renderer) *AppContext {
	a := &AppContext{files: files, renderer: renderer, document: NewDocument(files)}
	a.refresh(true)
	return a
}

func (a *AppContext) refresh(replace bool) {
	a.renderer.Render(a.document.Text(), a.document.Path(), a.document.Modified(), replace)
}

func (a *AppContext) SetText(text string) {
	a.document.SetText(text)
	a.refresh(false)
}

func (a *AppContext) OpenInitial(path string) { a.open(path) }

func (a *AppContext) open(path string) {
	d := NewDocument(a.files)
	if err := d.Open(path); err != nil {
		a.renderer.Message(err.Error())
		return
	}
	a.document = d
	a.refresh(true)
}

func (a *AppContext) New() {
	a.guard(func() {
		a.document = NewDocument(a.files)
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
