package lint

import (
	"embed"
	"unicode"
	"unicode/utf8"

	"github.com/client9/gospell"
)

//go:embed hunspell/ru_RU.aff hunspell/ru_RU.dic
var hunspellFS embed.FS

type HunspellLinter struct {
	spell *gospell.GoSpell
}

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

func (l *HunspellLinter) Check(plainText string, opts Options) Result {
	var errs []Error
	eachWord(plainText, func(start, end int) {
		word := plainText[start:end]
		if !hasCyrillic(word) || l.spell.Spell(word) {
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

func (l *HunspellLinter) Suggest(word string, max int) []string {
	if max <= 0 {
		max = defaultMaxSuggestions
	}
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

func matchCase(word, sugg string) string {
	first, _ := utf8.DecodeRuneInString(word)
	if !unicode.IsUpper(first) {
		return sugg
	}
	sFirst, size := utf8.DecodeRuneInString(sugg)
	return string(unicode.ToUpper(sFirst)) + sugg[size:]
}
