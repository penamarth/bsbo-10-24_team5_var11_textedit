package editor

import (
	"unicode"
	"unicode/utf8"
)

// plainText holds document text without Typst markup plus a map back to the
// source: offsets[i] is the byte offset in the source of the byte at
// position i in text. Lint works on plain text while diagnostics must be
// shown in the original document — the map performs that translation.
type plainText struct {
	text    string
	offsets []int
}

// extractPlainText strips Typst markup from src and keeps only natural
// language text. It is a heuristic scanner, not a full Typst parser:
// commands (#ident plus a parenthesised argument list), fenced code blocks,
// inline math $...$, comments and markup punctuation are dropped;
// the contents of [...] content blocks are kept because they are real text.
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

		// Fenced code block: ``` ... ```
		if c == '`' && i+2 < len(src) && src[i+1] == '`' && src[i+2] == '`' {
			end := indexFrom(src, "```", i+3)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end + 3
			continue
		}
		// Single-backtick inline code.
		if c == '`' {
			end := indexFrom(src, "`", i+1)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end + 1
			continue
		}
		// Comments: // to end of line, /* ... */.
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
		// Inline or block math: $ ... $.
		if c == '$' {
			end := indexFrom(src, "$", i+1)
			if end < 0 {
				i = len(src)
				continue
			}
			i = end + 1
			continue
		}
		// Typst command: #ident. A command at the start of a line is a
		// statement and consumes the rest of the line up to a [...] content
		// block, a semicolon or the newline (e.g. #set text(lang: "ru"),
		// #let x = 1). An inline command consumes only a directly attached
		// (...) argument list, so following real text stays visible.
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
		// Label or reference: <name>, @name.
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
		// Markup punctuation kept verbatim in Typst output is dropped here;
		// the text inside *, _, [ ], { } pairs still flows through.
		if c == '*' || c == '_' || c == '[' || c == ']' || c == '{' || c == '}' {
			i++
			continue
		}
		keep(c, i)
		i++
	}
	return plainText{text: string(buf), offsets: offsets}
}

// skipBalanced returns the offset just past a (...) group that starts at i.
// If src[i] is not the opening character, i is returned unchanged.
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
