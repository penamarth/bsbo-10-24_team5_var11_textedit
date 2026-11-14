package editor

import (
	"strings"
	"testing"
)

func TestExtractKeepsPlainText(t *testing.T) {
	src := "Привет, мир!\nВторая строка."
	p := extractPlainText(src)
	if p.text != src {
		t.Fatalf("text changed: %q", p.text)
	}
	for i, off := range p.offsets {
		if off != i {
			t.Fatalf("offset %d maps to %d", i, off)
		}
	}
}

func TestExtractStripsTypstMarkup(t *testing.T) {
	src := "#set text(lang: \"ru\")\n#let x = 1\nЗаголовок\n```\nкод ошибки\n```\nФормула $x + y$ готова.\n// комментарий\n/* блок */\nТекст *важный* и _курсив_."
	p := extractPlainText(src)
	for _, want := range []string{"Заголовок", "Формула", "готова", "Текст", "важный", "курсив"} {
		if !strings.Contains(p.text, want) {
			t.Fatalf("%q missing in %q", want, p.text)
		}
	}
	for _, drop := range []string{"set", "let", "lang", "код", "ошибки", "комментарий", "блок"} {
		if strings.Contains(p.text, drop) {
			t.Fatalf("%q leaked into %q", drop, p.text)
		}
	}
}

func TestExtractKeepsContentBlockText(t *testing.T) {
	p := extractPlainText("#title[Название документа]")
	if !strings.Contains(p.text, "Название документа") {
		t.Fatalf("content block dropped: %q", p.text)
	}
	if strings.Contains(p.text, "title") {
		t.Fatalf("command leaked: %q", p.text)
	}
}

func TestExtractStripsInlineCommand(t *testing.T) {
	p := extractPlainText("Сумма #total рублей.")
	if !strings.Contains(p.text, "Сумма") || !strings.Contains(p.text, "рублей") {
		t.Fatalf("text lost: %q", p.text)
	}
	if strings.Contains(p.text, "total") {
		t.Fatalf("command leaked: %q", p.text)
	}
}

func TestMapRangeTranslatesPositions(t *testing.T) {
	src := "Привет #cmd мир"
	p := extractPlainText(src)
	want := "Привет  мир"
	if p.text != want {
		t.Fatalf("plain text: %q != %q", p.text, want)
	}

	start, end := p.mapRange(len("Привет  "), len(p.text))
	if src[start:end] != "мир" {
		t.Fatalf("mapped range %d..%d = %q", start, end, src[start:end])
	}
}
