package lint

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// shared instance: loading the embedded dictionary takes ~400ms.
var sharedHunspell *HunspellLinter

func testHunspell(t *testing.T) *HunspellLinter {
	t.Helper()
	if sharedHunspell == nil {
		l, err := NewHunspellLinter()
		if err != nil {
			t.Fatalf("load embedded dictionary: %v", err)
		}
		sharedHunspell = l
	}
	return sharedHunspell
}

func TestHunspellRecognizesInflectedForms(t *testing.T) {
	l := testHunspell(t)
	// Inflected forms are the whole point of the morphology dictionary:
	// TextLinter's word list would flag every one of these.
	res := l.Check("Документа работает текста проверяет", Options{})
	for _, e := range res.Errors {
		if e.Word == "работает" || e.Word == "текста" || e.Word == "проверяет" || e.Word == "Документа" {
			t.Fatalf("inflected form flagged: %s", e.Word)
		}
	}
}

func TestHunspellReportsMisspellingsWithOffsets(t *testing.T) {
	l := testHunspell(t)
	text := "Это програму работает"
	res := l.Check(text, Options{})
	if !res.Success || len(res.Errors) != 1 {
		t.Fatalf("expected 1 error, got %+v", res)
	}
	e := res.Errors[0]
	if text[e.Start:e.End] != "програму" || e.Word != "програму" {
		t.Fatalf("error: %+v", e)
	}
	sugs := l.Suggest(e.Word, 0)
	found := false
	for _, s := range sugs {
		if s == "программу" {
			found = true
		}
	}
	if !found {
		t.Fatalf("suggestions: %v", sugs)
	}
}

func TestHunspellSkipsNonCyrillic(t *testing.T) {
	l := testHunspell(t)
	res := l.Check("typst http qwerty 12345", Options{})
	if len(res.Errors) != 0 {
		t.Fatalf("latin tokens flagged: %+v", res.Errors)
	}
}

func TestHunspellSuggestionsMatchCase(t *testing.T) {
	l := testHunspell(t)
	sugs := l.Suggest("Програму", 0)
	if len(sugs) == 0 {
		t.Fatal("no suggestions")
	}
	for _, s := range sugs {
		first, _ := utf8.DecodeRuneInString(s)
		if !unicode.IsUpper(first) {
			t.Fatalf("suggestion not capitalized: %q", s)
		}
	}
	if !strings.HasPrefix(sugs[0], "П") {
		t.Fatalf("suggestions: %v", sugs)
	}
}
