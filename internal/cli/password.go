package cli

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"
)

// PasswordSource supplies repository passwords from MPM_PASSWORD or a hidden
// prompt, caching prompted passwords in memory per repository.
type PasswordSource struct {
	Env    func(string) string
	Prompt func(label string) (string, error)

	cache map[string]string
}

// Get returns the password for repo.
func (p *PasswordSource) Get(repo string) (string, error) {
	if v := p.Env("MPM_PASSWORD"); v != "" {
		return v, nil
	}
	if pw, ok := p.cache[repo]; ok {
		return pw, nil
	}
	if p.Prompt == nil {
		return "", errors.New("no password available: set MPM_PASSWORD or run on a terminal")
	}
	pw, err := p.Prompt(fmt.Sprintf("Password for %s: ", repo))
	if err != nil {
		return "", err
	}
	if p.cache == nil {
		p.cache = map[string]string{}
	}
	p.cache[repo] = pw
	return pw, nil
}

// Forget drops a cached password (e.g. after it proved wrong).
func (p *PasswordSource) Forget(repo string) { delete(p.cache, repo) }

// GetNew returns a password for a new repository, prompting twice for confirmation.
func (p *PasswordSource) GetNew(repo string) (string, error) {
	if v := p.Env("MPM_PASSWORD"); v != "" {
		return v, nil
	}
	if p.Prompt == nil {
		return "", errors.New("no password available: set MPM_PASSWORD or run on a terminal")
	}
	a, err := p.Prompt(fmt.Sprintf("New password for %s: ", repo))
	if err != nil {
		return "", err
	}
	b, err := p.Prompt("Confirm password: ")
	if err != nil {
		return "", err
	}
	if a != b {
		return "", errors.New("passwords do not match")
	}
	if a == "" {
		return "", errors.New("password must not be empty")
	}
	return a, nil
}

// TTYPrompt reads a hidden line from the controlling terminal.
func TTYPrompt(label string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", errors.New("no terminal available to prompt for a password (set MPM_PASSWORD)")
	}
	defer func() { _ = tty.Close() }()
	if _, err := fmt.Fprint(tty, label); err != nil {
		return "", err
	}
	b, err := term.ReadPassword(int(tty.Fd()))
	_, _ = fmt.Fprintln(tty)
	return string(b), err
}
