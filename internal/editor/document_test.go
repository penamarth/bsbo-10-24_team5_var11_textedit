package editor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentRoundTrip(t *testing.T) {
	for _, original := range []string{"", "Привет", "Привет\n", "Первая\nВторая\n", "Первая\r\nВторая\r\n", "\ufeffТекст\r\n", "a\r\nb\nc"} {
		t.Run(original, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "текст.txt")
			if err := os.WriteFile(path, []byte(original), 0640); err != nil {
				t.Fatal(err)
			}
			d := NewDocument(NewFileSystemAPI(), nil)
			if err := d.Open(path); err != nil {
				t.Fatal(err)
			}
			if d.Modified() {
				t.Fatal("loaded document is dirty")
			}
			loaded := d.Text()
			d.SetText(loaded + "!")
			if !d.Modified() {
				t.Fatal("edit not tracked")
			}
			d.SetText(loaded)
			if d.Modified() {
				t.Fatal("restored document is dirty")
			}
			if err := d.Save(path); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != original {
				t.Fatalf("round trip: %q != %q", data, original)
			}
			info, _ := os.Stat(path)
			if info.Mode().Perm() != 0640 {
				t.Fatal("permissions changed")
			}
		})
	}
}

func TestEditedEncodingAndSaveAs(t *testing.T) {
	dir := t.TempDir()
	path, other := filepath.Join(dir, "source"), filepath.Join(dir, "copy")
	if err := os.WriteFile(path, []byte("\ufeffОдин\r\nДва"), 0600); err != nil {
		t.Fatal(err)
	}
	d := NewDocument(NewFileSystemAPI(), nil)
	if err := d.Open(path); err != nil {
		t.Fatal(err)
	}
	d.SetText(d.Text() + "\nТри")
	if err := d.Save(other); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(other)
	if string(data) != "\ufeffОдин\r\nДва\r\nТри" {
		t.Fatalf("encoding changed: %q", data)
	}
	if d.Modified() || d.Path() != other {
		t.Fatal("save state incorrect")
	}
	data, _ = os.ReadFile(path)
	if string(data) != "\ufeffОдин\r\nДва" {
		t.Fatal("source changed on Save As")
	}
}

func TestFailuresPreserveDocument(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), nil)
	d.SetText("не терять")
	dir := t.TempDir()
	bad := filepath.Join(dir, "invalid")
	if err := os.WriteFile(bad, []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{bad, filepath.Join(dir, "missing"), dir} {
		if err := d.Open(path); err == nil {
			t.Fatalf("expected read error for %s", path)
		}
		if d.Text() != "не терять" || !d.Modified() || d.Path() != "" {
			t.Fatal("failed open changed state")
		}
	}
	for _, path := range []string{"", dir, filepath.Join(dir, "missing", "file")} {
		if err := d.Save(path); err == nil {
			t.Fatalf("expected write error for %s", path)
		}
		if d.Text() != "не терять" || !d.Modified() || d.Path() != "" {
			t.Fatal("failed save changed state")
		}
	}
}

func TestNewDocumentAndSymlinkSave(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), nil)
	if d.Text() != "" || d.Path() != "" || d.Modified() {
		t.Fatal("invalid new document")
	}
	dir := t.TempDir()
	path, link := filepath.Join(dir, "file"), filepath.Join(dir, "link")
	d.SetText("строка\n")
	if err := d.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := d.Open(link); err != nil {
		t.Fatal(err)
	}
	d.SetText("новая\n")
	if err := d.Save(link); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(link)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink replaced")
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, []byte("новая\n")) {
		t.Fatalf("wrong target contents: %q", data)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("temporary file leaked")
	}
}
