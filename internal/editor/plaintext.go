package editor

import (
	"unicode"
	"unicode/utf8"
)

type plainText struct {
	text    string
	offsets []int
}

func extractPlainText(src string) plainText {
	var (
		buf     []byte
		offsets []int
	)
	keep := func(b byte, at int) {
		buf = append(buf, b)
		offsets = append(offsets, at)
	}
	isIdent := func(r rune) bool {
		return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	i := 0
	for i < len(src) {
		c := src[i]

		if c == '`' && i+2 < len(src) && src[i+1] == '`' && src[i+2] == '`' {
			end := indexFrom(src, "```", i+3)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end + 3
			continue
		}

		if c == '`' {
			end := indexFrom(src, "`", i+1)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end + 1
			continue
		}

		if c == '/' && i+1 < len(src) && src[i+1] == '/' {
			end := indexFrom(src, "\n", i+2)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end
			continue
		}
		if c == '/' && i+1 < len(src) && src[i+1] == '*' {
			end := indexFrom(src, "*/", i+2)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end + 2
			continue
		}

		if c == '$' {
			end := indexFrom(src, "$", i+1)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end + 1
			continue
		}

		if c == '#' && i+1 < len(src) {
			r, _ := utf8.DecodeRuneInString(src[i+1:])
			if isIdent(r) {
				lineStart := i == 0 || src[i-1] == '\n'
				i++
				for i < len(src) {
					r, size := utf8.DecodeRuneInString(src[i:])
					if !isIdent(r) {
						break
					}
					i += size
				}
				if !lineStart {
					i = skipBalanced(src, i, '(', ')')
					continue
				}
				for i < len(src) {
					if src[i] == '(' {
						i = skipBalanced(src, i, '(', ')')
						continue
					}
					if src[i] == '[' || src[i] == ';' || src[i] == '\n' {
						break
					}
					i++
				}
				continue
			}
		}

		if (c == '<' || c == '@') && i+1 < len(src) {
			r, _ := utf8.DecodeRuneInString(src[i+1:])
			if isIdent(r) {
				j := i + 1
				for j < len(src) {
					r, size := utf8.DecodeRuneInString(src[j:])
					if !isIdent(r) {
						break
					}
					j += size
				}
				if c == '@' || j < len(src) && src[j] == '>' {
					i = j
					if c == '<' {
						i++
					}
					continue
				}
			}
		}

		if c == '*' || c == '_' || c == '[' || c == ']' || c == '{' || c == '}' {
			i++
			continue
		}
		keep(c, i)
		i++
	}
	return plainText{text: string(buf), offsets: offsets}
}

func skipBalanced(src string, i int, open, close byte) int {
	if i >= len(src) || src[i] != open {
		return i
	}
	depth := 0
	for ; i < len(src); i++ {
		if src[i] == open {
			depth++
		} else if src[i] == close {
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return i
}

func indexFrom(s, sub string, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
