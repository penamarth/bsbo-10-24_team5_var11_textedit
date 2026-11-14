package lint

import (
	_ "embed"
	"sort"
	"strings"
	"unicode/utf8"
)

//go:embed dict_ru.txt
var dictRU string

const (
	defaultMaxSuggestions = 5
	maxEditDistance       = 2
)

type TextLinter struct {
	words map[string]struct{}
	list  []string
}

func NewTextLinter(extra []string) *TextLinter {
	l := &TextLinter{words: make(map[string]struct{})}
	for _, line := range strings.Split(dictRU, "\n") {
		l.add(line)
	}
	for _, w := range extra {
		l.add(w)
	}
	return l
}

func (l *TextLinter) add(word string) {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return
	}
	if _, ok := l.words[w]; !ok {
		l.words[w] = struct{}{}
		l.list = append(l.list, w)
	}
}

func (l *TextLinter) Check(plainText string, opts Options) Result {
	var errs []Error
	eachWord(plainText, func(start, end int) {
		word := plainText[start:end]
		if !utf8.ValidString(word) || l.known(word) {
			return
		}
		errs = append(errs, Error{
			Start:   start,
			End:     end,
			Word:    word,
			Message: "Возможная опечатка",
		})
	})
	return Result{Errors: errs, Success: true}
}

func (l *TextLinter) known(word string) bool {
	_, ok := l.words[strings.ToLower(word)]
	return ok
}

func (l *TextLinter) Suggest(word string, max int) []string {
	if max <= 0 {
		max = defaultMaxSuggestions
	}
	type cand struct {
		w string
		d int
	}
	lower := strings.ToLower(word)
	var cands []cand
	for _, w := range l.list {
		d := distance(lower, w, maxEditDistance)
		if d <= maxEditDistance {
			cands = append(cands, cand{w, d})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].d != cands[j].d {
			return cands[i].d < cands[j].d
		}
		return cands[i].w < cands[j].w
	})
	var out []string
	for i, c := range cands {
		if i >= max {
			break
		}
		out = append(out, c.w)
	}
	return out
}
