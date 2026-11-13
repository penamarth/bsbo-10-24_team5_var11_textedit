package editor

import (
	"testing"

	"github.com/penamarth/bsbo-10-24_team5_var11_textedit/internal/lint"
)

// fakeLinter is the legitimate use of the Linter interface: tests substitute
// it for TextLinter without changing Document.
type fakeLinter struct {
	result lint.Result
	seen   string
}

func (f *fakeLinter) Check(plainText string, _ lint.Options) lint.Result {
	f.seen = plainText
	return f.result
}

func TestRunLintMapsPositionsToDocument(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), lint.NewTextLinter(nil))
	d.SetText("#let x = 1\nЯ проверка програму.")
	res := d.RunLint(lint.Options{})
	if !res.Success {
		t.Fatalf("lint failed: %+v", res)
	}
	diags := d.Diagnostics()
	if len(diags) != 1 {
		t.Fatalf("diagnostics: %+v", diags)
	}
	e := diags[0]
	if e.Word != "програму" {
		t.Fatalf("word: %+v", e)
	}
	if d.Text()[e.Start:e.End] != "програму" {
		t.Fatalf("positions not mapped: %q", d.Text()[e.Start:e.End])
	}
}

func TestRunLintPassesPlainTextWithoutMarkup(t *testing.T) {
	fake := &fakeLinter{result: lint.Result{Success: true}}
	d := NewDocument(NewFileSystemAPI(), fake)
	d.SetText("#set text(lang: \"ru\")\nСлово $x^2$ ещё.")
	d.RunLint(lint.Options{})
	for _, drop := range []string{"set", "lang", "x^2"} {
		if contains(fake.seen, drop) {
			t.Fatalf("markup leaked to linter: %q", fake.seen)
		}
	}
}

func TestRunLintFailureKeepsDiagnostics(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), lint.NewTextLinter(nil))
	d.SetText("програму")
	d.RunLint(lint.Options{})
	if len(d.Diagnostics()) == 0 {
		t.Fatal("no diagnostics after successful run")
	}
	fake := &fakeLinter{result: lint.Result{Success: false, ErrorMessage: "сбой словаря"}}
	d.linter = fake
	res := d.RunLint(lint.Options{})
	if res.Success || res.ErrorMessage == "" {
		t.Fatal("failure not reported")
	}
	if len(d.Diagnostics()) == 0 {
		t.Fatal("failed run wiped diagnostics")
	}
}

func TestRunLintWithoutLinter(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), nil)
	res := d.RunLint(lint.Options{})
	if res.Success || res.ErrorMessage == "" {
		t.Fatal("missing linter not reported")
	}
}

func TestApplyFixReplacesAndRechecks(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), lint.NewTextLinter([]string{"программа"}))
	d.SetText("програму текст")
	res := d.RunLint(lint.Options{})
	if len(res.Errors) != 1 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	res = d.ApplyFix(0, "программа")
	if !res.Success {
		t.Fatal(res.ErrorMessage)
	}
	if d.Text() != "программа текст" {
		t.Fatalf("fix not applied: %q", d.Text())
	}
	if len(d.Diagnostics()) != 0 {
		t.Fatalf("stale diagnostics: %+v", d.Diagnostics())
	}
}

func TestApplyFixInvalidIndex(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), lint.NewTextLinter(nil))
	res := d.ApplyFix(0, "x")
	if res.Success {
		t.Fatal("invalid index accepted")
	}
}

func TestSkipErrorMarksSkipped(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), lint.NewTextLinter(nil))
	d.SetText("програму")
	d.RunLint(lint.Options{})
	d.SkipError(0)
	if !d.Diagnostics()[0].Skipped {
		t.Fatal("skip not marked")
	}
	if d.Text() != "програму" {
		t.Fatal("skip changed text")
	}
}

func TestSuggestionsComputedLazily(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), lint.NewTextLinter([]string{"программа"}))
	d.SetText("програму")
	d.RunLint(lint.Options{})
	if d.Diagnostics()[0].Suggestions != nil {
		t.Fatal("suggestions computed eagerly")
	}
	sugs := d.Suggestions(0)
	if len(sugs) == 0 || sugs[0] != "программа" {
		t.Fatalf("suggestions: %v", sugs)
	}
	if d.Suggestions(0) == nil {
		t.Fatal("cached suggestions lost")
	}
}

func TestSetTextClearsDiagnostics(t *testing.T) {
	d := NewDocument(NewFileSystemAPI(), lint.NewTextLinter(nil))
	d.SetText("програму")
	d.RunLint(lint.Options{})
	d.SetText("новый текст")
	if len(d.Diagnostics()) != 0 {
		t.Fatal("edit kept stale diagnostics")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
