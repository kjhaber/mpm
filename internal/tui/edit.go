package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// editSession tracks the private temp dir holding a decrypted secret during an editor run.
type editSession struct {
	dir, path, secret string
	isNew             bool // the secret does not exist yet
}

// formatEdit renders values as "field: value" lines. Backslashes and newlines in
// values are escaped as \\ and \n so every value stays on one line.
func formatEdit(fields []string, vals map[string]string) string {
	var b strings.Builder
	for _, f := range fields {
		v := strings.ReplaceAll(vals[f], `\`, `\\`)
		v = strings.ReplaceAll(v, "\n", `\n`)
		b.WriteString(f + ": " + v + "\n")
	}
	return b.String()
}

// parseEdit is the inverse of formatEdit. Fields missing from the text are treated as empty.
func parseEdit(text string, fields []string) (map[string]string, error) {
	known := map[string]bool{}
	for _, f := range fields {
		known[f] = true
	}
	out := map[string]string{}
	for i, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, rest, ok := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("line %d: expected \"field: value\"", i+1)
		}
		if !known[name] {
			return nil, fmt.Errorf("line %d: unknown field %q", i+1, name)
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("line %d: field %q given more than once", i+1, name)
		}
		out[name] = unescapeEdit(strings.TrimPrefix(rest, " "))
	}
	return out, nil
}

func unescapeEdit(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// newEditSession writes content to a 0600 file inside a fresh 0700 temp dir.
func newEditSession(secret, content string) (*editSession, error) {
	dir, err := os.MkdirTemp("", "mpm-edit-") // created 0700
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return &editSession{dir: dir, path: path, secret: secret}, nil
}

func (s *editSession) cleanup() { _ = os.RemoveAll(s.dir) }
