package cli

import (
	"errors"
	"strings"
)

// SplitWords splits s into words the way a POSIX shell would for quoting:
// whitespace separates words, single quotes are literal, double quotes allow
// backslash-escaped quotes and backslashes, and a bare backslash escapes the
// next character. Nothing is expanded.
func SplitWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; r {
		case ' ', '\t', '\n':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case '\\':
			if i+1 >= len(rs) {
				return nil, errors.New("trailing backslash")
			}
			i++
			cur.WriteRune(rs[i])
			inWord = true
		case '\'':
			end := indexRune(rs, i+1, '\'')
			if end < 0 {
				return nil, errors.New("unterminated single quote")
			}
			cur.WriteString(string(rs[i+1 : end]))
			i = end
			inWord = true
		case '"':
			i++
			for ; ; i++ {
				if i >= len(rs) {
					return nil, errors.New("unterminated double quote")
				}
				if rs[i] == '"' {
					break
				}
				if rs[i] == '\\' && i+1 < len(rs) && (rs[i+1] == '"' || rs[i+1] == '\\') {
					i++
				}
				cur.WriteRune(rs[i])
			}
			inWord = true
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

func indexRune(rs []rune, from int, r rune) int {
	for i := from; i < len(rs); i++ {
		if rs[i] == r {
			return i
		}
	}
	return -1
}
