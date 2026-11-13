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

// TextLinter is the concrete spelling checker: an embedded dictionary plus
// edit-distance suggestions. It has no external dependencies.
type TextLinter struct {
	words map[string]struct{}
	list  []string
}

// NewTextLinter builds a checker over the built-in Russian dictionary.
// extra words are added to the dictionary (project-specific vocabulary,
// tests, user additions).
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

// Check tokenizes plainText into words and reports every word missing from
// the dictionary. It never modifies the input.
func (l *TextLinter) Check(plainText string, opts Options) Result {
	maxSugg := opts.MaxSuggestions
	if maxSugg <= 0 {
		maxSugg = defaultMaxSuggestions
	}
	var errs []Error
	eachWord(plainText, func(start, end int) {
		word := plainText[start:end]
		if !utf8.ValidString(word) || l.known(word) {
			return
		}
		errs = append(errs, Error{
			Start:       start,
			End:         end,
			Word:        word,
			Message:     "Возможная опечатка",
			Suggestions: l.suggest(word, maxSugg),
		})
	})
	return Result{Errors: errs, Success: true}
}

// known reports whether the word occurs in the dictionary, case-insensitively.
func (l *TextLinter) known(word string) bool {
	_, ok := l.words[strings.ToLower(word)]
	return ok
}

// suggest ranks dictionary words by edit distance to the misspelled word and
// returns at most max corrections.
func (l *TextLinter) suggest(word string, max int) []string {
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
