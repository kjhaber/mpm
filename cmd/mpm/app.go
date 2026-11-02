package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/kjhaber/mpm/internal/cli"
	"github.com/kjhaber/mpm/internal/clip"
	"github.com/kjhaber/mpm/internal/config"
	"github.com/kjhaber/mpm/internal/crypt"
	"github.com/kjhaber/mpm/internal/vault"
)

// app holds the process-level dependencies shared by all commands.
type app struct {
	env     func(string) string
	in      io.Reader
	crypter crypt.Crypter
	clip    clip.Clipboard
	prompt  func(label string) (string, error)

	pw *cli.PasswordSource
}

func newApp() *app {
	return &app{
		env:     os.Getenv,
		in:      os.Stdin,
		crypter: crypt.NewOpenSSL(),
		clip:    clip.Detect(exec.LookPath, runtime.GOOS, nil),
		prompt:  cli.TTYPrompt,
	}
}

func (a *app) settings() (config.Settings, error) { return config.Load(a.env, config.Settings{}) }

func (a *app) store() (*vault.Store, error) {
	s, err := a.settings()
	if err != nil {
		return nil, err
	}
	return &vault.Store{Root: s.Root, Crypter: a.crypter}, nil
}

func (a *app) passwords() *cli.PasswordSource {
	if a.pw == nil {
		a.pw = &cli.PasswordSource{Env: a.env, Prompt: a.prompt}
	}
	return a.pw
}

// repoName resolves --repo, falling back to the default repository setting.
func (a *app) repoName(flag string) (string, error) {
	if flag != "" {
		return flag, vault.ValidateName(flag)
	}
	s, err := a.settings()
	if err != nil {
		return "", err
	}
	if s.Repo == "" {
		return "", errors.New("no repository given: pass --repo or set MPM_REPO / the default repository setting")
	}
	return s.Repo, vault.ValidateName(s.Repo)
}

// format resolves --format, falling back to the default format setting.
func (a *app) format(flag string) (string, error) {
	s, err := a.settings()
	if err != nil {
		return "", err
	}
	f := flag
	if f == "" {
		f = s.Format
	}
	if f != "text" && f != "json" {
		return "", fmt.Errorf("invalid format %q (want text or json)", f)
	}
	return f, nil
}

// open opens the named repository with its password.
func (a *app) open(repoFlag string) (string, *vault.Handle, error) {
	name, err := a.repoName(repoFlag)
	if err != nil {
		return "", nil, err
	}
	st, err := a.store()
	if err != nil {
		return "", nil, err
	}
	pw, err := a.passwords().Get(name)
	if err != nil {
		return "", nil, err
	}
	h, err := st.Open(name, pw)
	if err != nil {
		return "", nil, fmt.Errorf("repository %q: %w", name, err)
	}
	return name, h, nil
}

// modify opens a repository, applies fn, and saves it.
func (a *app) modify(repoFlag string, fn func(r *vault.Repo) error) error {
	_, h, err := a.open(repoFlag)
	if err != nil {
		return err
	}
	if err := fn(h.Repo); err != nil {
		return err
	}
	return h.Save()
}

func newRootCmd(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "mpm",
		Short:         "My Password Manager",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("mpm version {{.Version}}\n")
	root.AddCommand(a.repoCmd(), a.fieldCmd(), a.secretCmd(), a.tuiCmd())
	return root
}
