// Package cli holds helpers shared by the mpm CLI and TUI: --value parsing and password input.
package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ParseValue parses "field:value,field:value". A backslash escapes a following
// comma, colon or backslash. Only the first unescaped colon in an entry separates
// field from value.
func ParseValue(s string) (map[string]string, error) {
	out := map[string]string{}
	var entries []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\\':
			if i+1 >= len(rs) {
				return nil, errors.New("trailing backslash in --value")
			}
			i++
			cur.WriteRune('\\')
			cur.WriteRune(rs[i])
		case ',':
			entries = append(entries, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(rs[i])
		}
	}
	entries = append(entries, cur.String())

	for _, e := range entries {
		name, val, ok := splitEntry(e)
		if !ok {
			return nil, fmt.Errorf("invalid --value entry %q (want field:value)", e)
		}
		if name == "" {
			return nil, fmt.Errorf("empty field name in --value entry %q", e)
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("field %q given more than once", name)
		}
		out[name] = val
	}
	return out, nil
}

// splitEntry splits at the first unescaped colon and unescapes both halves.
func splitEntry(e string) (name, val string, ok bool) {
	rs := []rune(e)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' {
			i++
			continue
		}
		if rs[i] == ':' {
			return unescape(string(rs[:i])), unescape(string(rs[i+1:])), true
		}
	}
	return "", "", false
}

func unescape(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) && strings.ContainsRune(`,:\`, rs[i+1]) {
			i++
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

// FormatValue is the inverse of ParseValue, with keys sorted.
func FormatValue(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	esc := strings.NewReplacer(`\`, `\\`, `,`, `\,`, `:`, `\:`)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, esc.Replace(k)+":"+esc.Replace(m[k]))
	}
	return strings.Join(parts, ",")
}
