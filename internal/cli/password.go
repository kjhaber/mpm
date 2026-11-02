package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

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

// TTYPrompt reads a hidden line from the controlling terminal. term.ReadPassword
// leaves ISIG on, so Ctrl-C kills the process before its deferred restore runs;
// guardTerminal restores the terminal first so echo is never left off.
func TTYPrompt(label string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", errors.New("no terminal available to prompt for a password (set MPM_PASSWORD)")
	}
	defer func() { _ = tty.Close() }()
	if _, err := fmt.Fprint(tty, label); err != nil {
		return "", err
	}
	fd := int(tty.Fd())
	if saved, err := term.GetState(fd); err == nil {
		stop := guardTerminal(func() {
			_ = term.Restore(fd, saved)
			_, _ = fmt.Fprintln(tty)
		}, reraise)
		defer stop()
	}
	b, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(tty)
	return string(b), err
}

// guardTerminal runs restore and then reraise if SIGINT or SIGTERM arrives
// before the returned stop function is called.
func guardTerminal(restore func(), reraise func(os.Signal)) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	var once sync.Once
	go func() {
		select {
		case s := <-ch:
			restore()
			signal.Stop(ch)
			reraise(s)
		case <-done:
		}
	}()
	return func() {
		once.Do(func() {
			signal.Stop(ch)
			close(done)
		})
	}
}

// reraise restores the default action for s and delivers it to this process.
func reraise(s os.Signal) {
	signal.Reset(s)
	if sig, ok := s.(syscall.Signal); ok {
		_ = syscall.Kill(os.Getpid(), sig)
	}
}
