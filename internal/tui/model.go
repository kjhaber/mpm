// Package tui is mpm's bubbletea interface: repositories -> secrets -> fields.
package tui

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kjhaber/mpm/internal/clip"
	"github.com/kjhaber/mpm/internal/config"
	"github.com/kjhaber/mpm/internal/crypt"
	"github.com/kjhaber/mpm/internal/fuzzy"
	"github.com/kjhaber/mpm/internal/vault"
)

const revealFor = 5 * time.Second

// Options choose where the TUI starts.
type Options struct {
	Repo, Secret string
	Resume       bool
}

// Deps are the TUI's collaborators, injectable for tests.
type Deps struct {
	Store       *vault.Store
	Clip        clip.Clipboard
	EnvPassword string // MPM_PASSWORD; when set, no password prompts for existing repos
	StatePath   string
	// RunEditor opens path in $EDITOR and later delivers done(err) as a message.
	RunEditor func(path string, done func(error) tea.Msg) tea.Cmd
}

type screen int

const (
	screenRepos screen = iota
	screenSecrets
	screenFields
	screenRepoFields
)

type mode int

const (
	modeList mode = iota
	modeFilter
	modePrompt
	modeConfirm
)

type (
	editDoneMsg       struct{ err error }
	revealExpiredMsg  struct{ seq int }
	promptFunc        func(string)
	confirmedFunc     func()
	mutateFunc        func(r *vault.Repo) error
	passwordCacheType map[string]string
)

// Model is the bubbletea model for the whole TUI.
type Model struct {
	deps Deps

	screen screen
	mode   mode
	repo   string
	secret string
	handle *vault.Handle

	items  []string
	cursor int
	query  string

	filterInput  textinput.Model
	promptInput  textinput.Model
	promptLabel  string
	promptFn     promptFunc
	pending      tea.Cmd            // command a prompt callback wants run once the prompt closes
	promptCheck  func(string) error // optional: Enter is refused (and the input kept) while it errors
	confirmLabel string
	confirmFn    confirmedFunc

	msg       string
	passwords passwordCacheType

	nextScreen  screen // where a successful repo open lands
	startSecret string

	revealed   bool
	revealName string
	revealSeq  int

	editMu sync.Mutex // guards edit; Cleanup may run on a signal goroutine
	edit   *editSession
}

// New builds the model, navigating to any pre-selected repo/secret.
func New(deps Deps, opts Options) *Model {
	m := &Model{deps: deps, passwords: passwordCacheType{}, nextScreen: screenSecrets}
	m.filterInput = textinput.New()
	m.filterInput.Prompt = "/"
	if opts.Resume && opts.Repo == "" && deps.StatePath != "" {
		if st, err := config.LoadState(deps.StatePath); err == nil {
			opts.Repo, opts.Secret = st.Repo, st.Secret
		}
	}
	m.reload()
	if opts.Repo != "" {
		m.startSecret = opts.Secret
		m.enterRepo(opts.Repo)
	}
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

// Cleanup removes any decrypted temp files; safe to call from a signal handler.
func (m *Model) Cleanup() {
	m.editMu.Lock()
	defer m.editMu.Unlock()
	if m.edit != nil {
		m.edit.cleanup()
		m.edit = nil
	}
}

func (m *Model) setEdit(s *editSession) {
	m.editMu.Lock()
	m.edit = s
	m.editMu.Unlock()
}

func (m *Model) currentEdit() *editSession {
	m.editMu.Lock()
	defer m.editMu.Unlock()
	return m.edit
}

func (m *Model) quit() tea.Cmd {
	m.Cleanup()
	if m.deps.StatePath != "" {
		st := config.State{}
		if m.screen != screenRepos && m.handle != nil {
			st.Repo = m.repo
			if m.screen == screenFields {
				st.Secret = m.secret
			}
		}
		_ = config.SaveState(m.deps.StatePath, st)
	}
	return tea.Quit
}

func errText(err error) string {
	if errors.Is(err, crypt.ErrBadPassword) {
		return "wrong password"
	}
	return err.Error()
}

// reload rebuilds the item list for the current screen and clamps the cursor.
func (m *Model) reload() {
	m.items = nil
	switch m.screen {
	case screenRepos:
		names, err := m.deps.Store.List()
		if err != nil {
			m.msg = err.Error()
		}
		m.items = names
	case screenSecrets:
		for n := range m.handle.Repo.Secrets {
			m.items = append(m.items, n)
		}
		sortStrings(m.items)
	case screenFields, screenRepoFields:
		for _, f := range m.handle.Repo.Fields {
			m.items = append(m.items, f.Name)
		}
	}
	if n := len(m.visible()); m.cursor >= n {
		m.cursor = max(n-1, 0)
	}
}

// visible returns indexes into items after applying the filter query.
func (m *Model) visible() []int {
	ms := fuzzy.Filter(m.query, m.items)
	idx := make([]int, len(ms))
	for i, x := range ms {
		idx[i] = x.Index
	}
	return idx
}

func (m *Model) selected() string {
	v := m.visible()
	if m.cursor < 0 || m.cursor >= len(v) {
		return ""
	}
	return m.items[v[m.cursor]]
}

func (m *Model) goTo(s screen) {
	m.screen = s
	m.query = ""
	m.cursor = 0
	m.revealed = false
	m.reload()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case revealExpiredMsg:
		if msg.seq == m.revealSeq {
			m.revealed = false
		}
	case editDoneMsg:
		m.finishEdit(msg.err)
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	m.msg = ""
	if msg.Type == tea.KeyCtrlC {
		return m.quit()
	}
	switch m.mode {
	case modePrompt:
		return m.promptKey(msg)
	case modeConfirm:
		m.mode = modeList
		if s := msg.String(); (s == "y" || s == "Y") && m.confirmFn != nil {
			fn := m.confirmFn
			m.confirmFn = nil
			fn()
		}
		return nil
	case modeFilter:
		switch msg.Type {
		case tea.KeyEsc:
			m.query = ""
			m.mode = modeList
		case tea.KeyEnter:
			m.mode = modeList
		default:
			m.filterInput, _ = m.filterInput.Update(msg)
			m.query = m.filterInput.Value()
			m.cursor = 0
		}
		return nil
	}
	return m.listKey(msg)
}

func (m *Model) promptKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeList
		m.promptFn = nil
		m.promptCheck = nil
	case tea.KeyEnter:
		if m.promptCheck != nil {
			if err := m.promptCheck(m.promptInput.Value()); err != nil {
				m.msg = errText(err)
				return nil
			}
		}
		fn, v := m.promptFn, m.promptInput.Value()
		m.promptCheck = nil
		m.mode = modeList
		m.promptFn = nil
		if fn != nil {
			fn(v)
		}
		cmd := m.pending
		m.pending = nil
		return cmd
	default:
		m.promptInput, _ = m.promptInput.Update(msg)
	}
	return nil
}

// ask shows a one-line input; fn receives the entered text on Enter.
func (m *Model) ask(label string, hidden bool, fn promptFunc) {
	in := textinput.New()
	in.Prompt = ""
	if hidden {
		in.EchoMode = textinput.EchoPassword
	}
	in.Focus()
	m.promptInput, m.promptLabel, m.promptFn, m.mode = in, label, fn, modePrompt
	m.promptCheck = nil
}

// askChecked is ask with a validator: the view shows a live ✓/✗, and Enter is
// refused with the validator's message until the input passes.
func (m *Model) askChecked(label string, hidden bool, check func(string) error, fn promptFunc) {
	m.ask(label, hidden, fn)
	m.promptCheck = check
}

func (m *Model) confirm(label string, fn confirmedFunc) {
	m.confirmLabel, m.confirmFn, m.mode = label, fn, modeConfirm
}

func (m *Model) listKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if len(msg.Runes) == 1 {
		key = strings.ToLower(key)
	}
	switch key {
	case "ctrl+c", "q":
		return m.quit()
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
		m.revealed = false
	case "down", "j":
		m.cursor = min(m.cursor+1, max(len(m.visible())-1, 0))
		m.revealed = false
	case "/":
		m.filterInput.SetValue(m.query)
		m.filterInput.Focus()
		m.mode = modeFilter
	case "esc":
		if m.query != "" {
			m.query, m.cursor = "", 0
			return nil
		}
		return m.back()
	default:
		return m.screenKey(key)
	}
	return nil
}

func (m *Model) back() tea.Cmd {
	switch m.screen {
	case screenFields:
		m.goTo(screenSecrets)
	case screenSecrets, screenRepoFields:
		m.handle, m.secret = nil, ""
		m.goTo(screenRepos)
	default:
		return m.quit()
	}
	return nil
}
