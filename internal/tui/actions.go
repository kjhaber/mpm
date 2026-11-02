package tui

import (
	"errors"
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kjhaber/mpm/internal/clip"
	"github.com/kjhaber/mpm/internal/vault"
)

func sortStrings(s []string) { sort.Strings(s) }

// screenKey handles keys specific to the current screen.
func (m *Model) screenKey(key string) tea.Cmd {
	switch m.screen {
	case screenRepos:
		m.reposKey(key)
	case screenSecrets:
		m.secretsKey(key)
	case screenFields:
		return m.fieldsKey(key)
	case screenRepoFields:
		m.repoFieldsKey(key)
	}
	return nil
}

// --- passwords and opening repositories ---

func (m *Model) knownPassword(repo string) (string, bool) {
	if pw, ok := m.passwords[repo]; ok {
		return pw, true
	}
	if m.deps.EnvPassword != "" {
		return m.deps.EnvPassword, true
	}
	return "", false
}

// withPassword calls fn with the repo's password, prompting if it isn't known yet.
func (m *Model) withPassword(repo string, fn func(pw string)) {
	if pw, ok := m.knownPassword(repo); ok {
		fn(pw)
		return
	}
	m.ask("Password for "+repo+": ", true, fn)
}

func (m *Model) enterRepo(repo string) {
	m.withPassword(repo, func(pw string) { m.openRepo(repo, pw) })
}

func (m *Model) openRepo(repo, pw string) {
	h, err := m.deps.Store.Open(repo, pw)
	if err != nil {
		delete(m.passwords, repo)
		m.msg = errText(err)
		m.nextScreen, m.startSecret = screenSecrets, ""
		if errors.Is(err, errBadPassword) {
			m.ask("Password for "+repo+": ", true, func(pw string) { m.openRepo(repo, pw) })
		}
		return
	}
	m.passwords[repo] = pw
	m.handle, m.repo, m.secret = h, repo, ""
	next, start := m.nextScreen, m.startSecret
	m.nextScreen, m.startSecret = screenSecrets, ""
	m.goTo(next)
	if next == screenSecrets && start != "" {
		if _, ok := h.Repo.Secrets[start]; ok {
			m.selectSecret(start)
		} else {
			m.msg = fmt.Sprintf("secret %q not found", start)
		}
	}
}

// mutate applies fn to the open repo and saves; on failure it reloads from disk.
func (m *Model) mutate(fn mutateFunc) bool {
	err := fn(m.handle.Repo)
	if err == nil {
		err = m.handle.Save()
	}
	if err != nil {
		m.msg = errText(err)
		if h, rerr := m.deps.Store.Open(m.repo, m.passwords[m.repo]); rerr == nil {
			m.handle = h
		}
		m.reload()
		return false
	}
	m.reload()
	return true
}

// --- repositories screen ---

func (m *Model) reposKey(key string) {
	sel := m.selected()
	switch key {
	case "enter":
		if sel != "" {
			m.enterRepo(sel)
		}
	case "f":
		if sel != "" {
			m.nextScreen = screenRepoFields
			m.enterRepo(sel)
		}
	case "n":
		m.newRepo()
	case "x":
		if sel != "" {
			m.confirm(fmt.Sprintf("Delete repository %s?", sel), func() { m.deleteRepo(sel) })
		}
	}
}

func (m *Model) newRepo() {
	m.ask("New repository name: ", false, func(name string) {
		if name == "" {
			return
		}
		if pw := m.deps.EnvPassword; pw != "" {
			m.createRepo(name, pw)
			return
		}
		m.ask("New password for "+name+": ", true, func(a string) {
			m.ask("Confirm password: ", true, func(b string) {
				switch {
				case a != b:
					m.msg = "passwords do not match"
				case a == "":
					m.msg = "password must not be empty"
				default:
					m.createRepo(name, a)
				}
			})
		})
	})
}

func (m *Model) createRepo(name, pw string) {
	if err := m.deps.Store.Create(name, pw); err != nil {
		m.msg = errText(err)
		return
	}
	m.passwords[name] = pw
	m.reload()
	m.msg = "created " + name
}

func (m *Model) deleteRepo(name string) {
	m.withPassword(name, func(pw string) {
		if err := m.deps.Store.Delete(name, pw); err != nil {
			m.msg = errText(err)
			return
		}
		delete(m.passwords, name)
		m.reload()
		m.msg = "deleted " + name
	})
}

// --- secrets screen ---

func (m *Model) secretsKey(key string) {
	sel := m.selected()
	switch key {
	case "enter":
		if sel != "" {
			m.selectSecret(sel)
		}
	case "n":
		m.newSecret()
	case "x":
		if sel != "" {
			m.confirm(fmt.Sprintf("Delete secret %s?", sel), func() {
				if m.mutate(func(r *vault.Repo) error { return r.DeleteSecret(sel) }) {
					m.msg = "deleted " + sel
				}
			})
		}
	}
}

func (m *Model) selectSecret(name string) {
	m.secret = name
	m.goTo(screenFields)
}

func (m *Model) newSecret() {
	r := m.handle.Repo
	if len(r.Fields) == 0 {
		m.msg = "repository has no fields; press Esc, then F to add fields"
		return
	}
	m.ask("New secret name: ", false, func(name string) {
		if name == "" {
			return
		}
		if _, exists := r.Secrets[name]; exists {
			m.msg = fmt.Sprintf("secret %q already exists", name)
			return
		}
		vals := map[string]string{}
		var next func(i int)
		next = func(i int) {
			if i == len(r.Fields) {
				if m.mutate(func(r *vault.Repo) error { return r.SetSecret(name, vals) }) {
					m.msg = "created " + name
				}
				return
			}
			f := r.Fields[i].Name
			m.ask(f+": ", true, func(v string) {
				if v != "" {
					vals[f] = v
				}
				next(i + 1)
			})
		}
		next(0)
	})
}

// --- fields screen (one secret) ---

func (m *Model) fieldsKey(key string) tea.Cmd {
	sel := m.selected()
	if sel == "" {
		return nil
	}
	switch key {
	case "enter", "s":
		if m.revealed && m.revealName == sel {
			m.revealed = false
			return nil
		}
		m.revealed, m.revealName = true, sel
		m.revealSeq++
		seq := m.revealSeq
		return tea.Tick(revealFor, func(time.Time) tea.Msg { return revealExpiredMsg{seq} })
	case "c":
		v, ok := m.handle.Repo.Secrets[m.secret][sel]
		if !ok {
			m.msg = fmt.Sprintf("no value for %s", sel)
			return nil
		}
		if err := m.deps.Clip.Write(v); err != nil {
			m.msg = err.Error()
			return nil
		}
		m.msg = fmt.Sprintf("Copied %s to clipboard", sel)
		if !clip.SupportsClear(m.deps.Clip) {
			m.msg += " (this clipboard type cannot be cleared automatically)"
		} else if m.deps.SpawnClear != nil && m.deps.ClearAfter > 0 {
			if err := m.deps.SpawnClear(v, m.deps.ClearAfter); err != nil {
				m.msg = "Copied, but could not schedule clipboard clear: " + err.Error()
			} else {
				m.msg += fmt.Sprintf(" (clears in %s)", m.deps.ClearAfter)
			}
		}
	case "e":
		return m.startEdit()
	}
	return nil
}

func (m *Model) fieldNames() []string {
	names := make([]string, len(m.handle.Repo.Fields))
	for i, f := range m.handle.Repo.Fields {
		names[i] = f.Name
	}
	return names
}

func (m *Model) startEdit() tea.Cmd {
	m.Cleanup()
	sess, err := newEditSession(m.secret, formatEdit(m.fieldNames(), m.handle.Repo.Secrets[m.secret]))
	if err != nil {
		m.msg = err.Error()
		return nil
	}
	m.setEdit(sess)
	return m.deps.RunEditor(sess.path, func(err error) tea.Msg { return editDoneMsg{err} })
}

func (m *Model) finishEdit(editErr error) {
	sess := m.currentEdit()
	if sess == nil {
		return
	}
	defer m.Cleanup()
	if editErr != nil {
		m.msg = "editor failed: " + editErr.Error() + " (secret unchanged)"
		return
	}
	data, err := readFile(sess.path)
	if err != nil {
		m.msg = "could not read edited file: " + err.Error() + " (secret unchanged)"
		return
	}
	vals, err := parseEdit(string(data), m.fieldNames())
	if err != nil {
		m.msg = err.Error() + " (secret unchanged)"
		return
	}
	if m.mutate(func(r *vault.Repo) error { return r.SetSecret(sess.secret, vals) }) {
		m.msg = "saved " + sess.secret
	} else {
		m.msg += " (secret unchanged)"
	}
}

// --- repository fields screen ---

func (m *Model) repoFieldsKey(key string) {
	sel := m.selected()
	switch key {
	case "n":
		m.ask("New field name: ", false, func(name string) {
			if name != "" {
				m.mutate(func(r *vault.Repo) error { return r.AddField(name) })
			}
		})
	case "x":
		if sel != "" {
			m.confirm(fmt.Sprintf("Delete field %s and its values?", sel), func() {
				m.mutate(func(r *vault.Repo) error { return r.DeleteField(sel) })
			})
		}
	case "r":
		if sel != "" {
			m.mutate(func(r *vault.Repo) error {
				cur, err := r.GetAttrib(sel, "required")
				if err != nil {
					return err
				}
				return r.SetAttrib(sel, "required", fmt.Sprint(cur != "true"))
			})
		}
	case "p":
		if sel != "" {
			m.ask(fmt.Sprintf("Pattern for %s (regex): ", sel), false, func(p string) {
				if p == "" {
					m.mutate(func(r *vault.Repo) error { return r.DeleteAttrib(sel, "pattern") })
				} else {
					m.mutate(func(r *vault.Repo) error { return r.SetAttrib(sel, "pattern", p) })
				}
			})
		}
	}
}
