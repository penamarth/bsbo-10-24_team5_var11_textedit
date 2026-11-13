package lint

import "testing"

func TestCheckReportsMisspellingsWithOffsets(t *testing.T) {
	l := NewTextLinter([]string{"программа"})
	res := l.Check("прога програму", Options{})
	if !res.Success || len(res.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %+v", res)
	}
	first, second := res.Errors[0], res.Errors[1]
	if first.Word != "прога" || first.Start != 0 || first.End != len("прога") {
		t.Fatalf("first error: %+v", first)
	}
	wantStart := len("прога ")
	if second.Word != "програму" || second.Start != wantStart || second.End != wantStart+len("програму") {
		t.Fatalf("second error: %+v", second)
	}
}

func TestCheckIgnoresKnownWords(t *testing.T) {
	l := NewTextLinter([]string{"колбаса"})
	for _, word := range []string{"колбаса", "Колбаса", "КОЛБАСА"} {
		res := l.Check(word, Options{})
		if len(res.Errors) != 0 {
			t.Fatalf("%q flagged: %+v", word, res.Errors)
		}
	}
}

func TestCheckSkipsNonLetters(t *testing.T) {
	l := NewTextLinter(nil)
	res := l.Check("123 ... --- !!", Options{})
	if len(res.Errors) != 0 {
		t.Fatalf("numbers and punctuation flagged: %+v", res.Errors)
	}
}

func TestSuggestionsRankedByDistance(t *testing.T) {
	l := NewTextLinter([]string{"котлета", "кот", "колдун"})
	res := l.Check("котт", Options{})
	if len(res.Errors) != 1 {
		t.Fatal(res)
	}
	sugg := res.Errors[0].Suggestions
	if len(sugg) == 0 || sugg[0] != "кот" {
		t.Fatalf("suggestions: %v", sugg)
	}
}

func TestSuggestionsLimitedByOptions(t *testing.T) {
	l := NewTextLinter([]string{"кат", "кот", "кит", "котт", "кадр"})
	res := l.Check("котр", Options{MaxSuggestions: 2})
	if len(res.Errors) != 1 || len(res.Errors[0].Suggestions) > 2 {
		t.Fatalf("limit ignored: %+v", res.Errors[0])
	}
}
