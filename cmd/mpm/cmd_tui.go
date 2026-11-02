package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/kjhaber/mpm/internal/config"
	"github.com/kjhaber/mpm/internal/tui"
)

func (a *app) tuiCmd() *cobra.Command {
	var repo, secret string
	var resume bool
	c := &cobra.Command{
		Use: "tui", Short: "Browse and edit secrets in a terminal UI", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if secret != "" && repo == "" {
				return errors.New("--secret requires --repo")
			}
			if resume && (repo != "" || secret != "") {
				return errors.New("--resume cannot be combined with --repo or --secret")
			}
			st, err := a.store()
			if err != nil {
				return err
			}
			m := tui.New(tui.Deps{
				Store:       st,
				Clip:        a.clip,
				EnvPassword: a.env("MPM_PASSWORD"),
				StatePath:   config.ResolvePaths(a.env).State,
				ClearAfter:  clipboardClearAfter,
				SpawnClear:  a.spawnClear,
				RunEditor:   runEditor(a.env),
			}, tui.Options{Repo: repo, Secret: secret, Resume: resume})

			// Remove decrypted temp files if we are killed while editing.
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGTERM, syscall.SIGHUP)
			defer signal.Stop(sigs)
			go func() {
				if s, ok := <-sigs; ok {
					m.Cleanup()
					signal.Stop(sigs)
					_ = syscall.Kill(os.Getpid(), s.(syscall.Signal))
				}
			}()

			_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
			m.Cleanup()
			return err
		},
	}
	c.Flags().StringVar(&repo, "repo", "", "start in this repository")
	c.Flags().StringVar(&secret, "secret", "", "start at this secret (requires --repo)")
	c.Flags().BoolVar(&resume, "resume", false, "start where the previous session exited")
	return c
}

// runEditor suspends the TUI and runs $VISUAL/$EDITOR (default vi) on path.
func runEditor(env func(string) string) func(string, func(error) tea.Msg) tea.Cmd {
	return func(path string, done func(error) tea.Msg) tea.Cmd {
		editor := env("VISUAL")
		if editor == "" {
			editor = env("EDITOR")
		}
		if editor == "" {
			editor = "vi"
		}
		parts := strings.Fields(editor)
		cmd := exec.Command(parts[0], append(parts[1:], path)...)
		return tea.ExecProcess(cmd, func(err error) tea.Msg { return done(err) })
	}
}
