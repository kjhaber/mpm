package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/kjhaber/mpm/internal/childenv"
	"github.com/kjhaber/mpm/internal/cli"
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

			// We own signal handling (bubbletea's handler is disabled) so that on
			// SIGTERM/SIGHUP we remove decrypted temp files, let bubbletea restore
			// the terminal via Kill, and then exit with the conventional status.
			p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithoutSignalHandler())
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
			defer signal.Stop(sigs)
			var got atomic.Int32
			go func() {
				if s, ok := <-sigs; ok {
					got.Store(int32(s.(syscall.Signal)))
					m.Cleanup()
					p.Kill()
				}
			}()

			_, err = p.Run()
			m.Cleanup()
			if s := got.Load(); s != 0 {
				os.Exit(128 + int(s))
			}
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
		c, err := editorCommand(env, path)
		if err != nil {
			return func() tea.Msg { return done(err) }
		}
		return tea.ExecProcess(c, func(err error) tea.Msg { return done(err) })
	}
}

// editorCommand builds the $VISUAL/$EDITOR command (default vi) for path,
// without MPM_PASSWORD in its environment.
func editorCommand(env func(string) string, path string) (*exec.Cmd, error) {
	editor := strings.TrimSpace(env("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(env("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}
	parts, err := cli.SplitWords(editor)
	if err != nil || len(parts) == 0 {
		return nil, fmt.Errorf("cannot parse editor %q (check $VISUAL / $EDITOR)", editor)
	}
	args := append(privateEditorArgs(parts[0]), parts[1:]...)
	cmd := exec.Command(parts[0], append(args, path)...)
	cmd.Env = childenv.Env()
	return cmd, nil
}

// privateEditorArgs returns flags that stop vim and nvim writing the decrypted
// secret to swap, backup, undo or viminfo files. Other editors get none.
func privateEditorArgs(editor string) []string {
	switch filepath.Base(editor) {
	case "vim", "nvim":
		return []string{"-n", "-i", "NONE", "-c", "set noswapfile noundofile nobackup nowritebackup"}
	}
	return nil
}
