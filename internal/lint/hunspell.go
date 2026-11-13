package lint

import (
	"embed"
	"unicode"
	"unicode/utf8"

	"github.com/client9/gospell"
)

// Russian Hunspell dictionary by Alexander I. Lebedev, taken from the
// LibreOffice dictionaries repository. License: BSD, see
// hunspell/README_ru_RU.txt.
//
//go:embed hunspell/ru_RU.aff hunspell/ru_RU.dic
var hunspellFS embed.FS

// HunspellLinter is a Linter implementation backed by a real Hunspell
// dictionary with full Russian morphology (~150k base entries, all inflected
// forms are recognized). It is the primary checker; TextLinter remains as the
// lightweight fallback — the two are interchangeable through the Linter
// interface.
type HunspellLinter struct {
	spell *gospell.GoSpell
}

// NewHunspellLinter loads the embedded ru_RU dictionary. Loading takes a
// few hundred milliseconds, so it should happen once at startup.
func NewHunspellLinter() (*HunspellLinter, error) {
	aff, err := hunspellFS.Open("hunspell/ru_RU.aff")
	if err != nil {
		return nil, err
	}
	defer aff.Close()
	dic, err := hunspellFS.Open("hunspell/ru_RU.dic")
	if err != nil {
		return nil, err
	}
	defer dic.Close()
	gs, err := gospell.NewGoSpellReader(aff, dic)
	if err != nil {
		return nil, err
	}
	return &HunspellLinter{spell: gs}, nil
}

// Check tokenizes plainText and reports every Russian word the dictionary
// does not know. Tokens without Cyrillic letters (Latin, digits) are skipped:
// the bundle ships only a Russian dictionary.
func (l *HunspellLinter) Check(plainText string, opts Options) Result {
	maxSugg := opts.MaxSuggestions
	if maxSugg <= 0 {
		maxSugg = defaultMaxSuggestions
	}
	var errs []Error
	eachWord(plainText, func(start, end int) {
		word := plainText[start:end]
		if !hasCyrillic(word) || l.spell.Spell(word) {
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

// suggest asks Hunspell for corrections and normalizes their case to match
// the misspelled word (the engine returns base forms).
func (l *HunspellLinter) suggest(word string, max int) []string {
	sugs, err := l.spell.Suggest(word, max)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(sugs))
	for _, s := range sugs {
		out = append(out, matchCase(word, s.Word))
	}
	return out
}

// matchCase applies the misspelled word's capitalization to a suggestion:
// "Програму" gets "Программу", not "программу".
func matchCase(word, sugg string) string {
	first, _ := utf8.DecodeRuneInString(word)
	if !unicode.IsUpper(first) {
		return sugg
	}
	sFirst, size := utf8.DecodeRuneInString(sugg)
	return string(unicode.ToUpper(sFirst)) + sugg[size:]
}
