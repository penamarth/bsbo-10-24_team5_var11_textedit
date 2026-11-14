package lint

import (
	"unicode"
	"unicode/utf8"
)

func eachWord(plainText string, fn func(start, end int)) {
	pos := 0
	for pos < len(plainText) {
		r, size := utf8.DecodeRuneInString(plainText[pos:])
		if !unicode.IsLetter(r) {
			pos += size
			continue
		}
		start := pos
		for pos < len(plainText) {
			r, size = utf8.DecodeRuneInString(plainText[pos:])
			if !unicode.IsLetter(r) {
				break
			}
			pos += size
		}
		fn(start, pos)
	}
}

func hasCyrillic(word string) bool {
	for _, r := range word {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}
