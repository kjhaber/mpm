package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kjhaber/mpm/internal/clip"
	"github.com/kjhaber/mpm/internal/config"
	"github.com/kjhaber/mpm/internal/crypt"
	"github.com/kjhaber/mpm/internal/vault"
)

type fakeCrypter struct{}

func (fakeCrypter) Encrypt(pw string, in []byte) ([]byte, error) {
	return []byte("enc:" + pw + ":" + string(in)), nil
}

func (fakeCrypter) Decrypt(pw string, in []byte) ([]byte, error) {
	p := "enc:" + pw + ":"
	if !strings.HasPrefix(string(in), p) {
		return nil, crypt.ErrBadPassword
	}
	return in[len(p):], nil
}

type fakeClip struct{ val string }

func (f *fakeClip) Write(s string) error  { f.val = s; return nil }
func (f *fakeClip) Read() (string, error) { return f.val, nil }

type env struct {
	t         *testing.T
	store     *vault.Store
	clip      *fakeClip
	spawned   []string
	statePath string
	editor    func(path string) error // simulates $EDITOR
	deps      Deps
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	e := &env{t: t, clip: &fakeClip{}, statePath: filepath.Join(dir, "state.json"),
		store: &vault.Store{Root: filepath.Join(dir, "repos"), Crypter: fakeCrypter{}}}
	e.deps = Deps{
		Store: e.store, Clip: e.clip, StatePath: e.statePath, ClearAfter: 30 * time.Second,
		SpawnClear: func(v string, _ time.Duration) error { e.spawned = append(e.spawned, v); return nil },
		RunEditor: func(path string, done func(error) tea.Msg) tea.Cmd {
			return func() tea.Msg {
				if e.editor == nil {
					return done(nil)
				}
				return done(e.editor(path))
			}
		},
	}
	return e
}

// seed creates repo "web" (password pw) with fields user/pass and secret gh.
func (e *env) seed() {
	e.t.Helper()
	if err := e.store.Create("web", "pw"); err != nil {
		e.t.Fatal(err)
	}
	h, _ := e.store.Open("web", "pw")
	_ = h.Repo.AddField("user")
	_ = h.Repo.AddField("pass")
	_ = h.Repo.SetSecret("gh", map[string]string{"user": "me", "pass": "s3cret"})
	if err := h.Save(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) repoOf(name string) *vault.Repo {
	e.t.Helper()
	h, err := e.store.Open(name, "pw")
	if err != nil {
		e.t.Fatal(err)
	}
	return h.Repo
}

var (
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	esc   = tea.KeyMsg{Type: tea.KeyEsc}
)

func k(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func press(m *Model, msgs ...tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	for _, x := range msgs {
		_, cmd = m.Update(x)
	}
	return cmd
}

func typ(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func mustContain(t *testing.T, m *Model, want string) {
	t.Helper()
	if v := m.View(); !strings.Contains(v, want) {
		t.Fatalf("view missing %q:\n%s", want, v)
	}
}

func mustNotContain(t *testing.T, m *Model, bad string) {
	t.Helper()
	if v := m.View(); strings.Contains(v, bad) {
		t.Fatalf("view should not contain %q:\n%s", bad, v)
	}
}

func TestDrillDownAndBackWithOnePasswordPromptPerRepo(t *testing.T) {
	e := newEnv(t)
	e.seed()
	m := New(e.deps, Options{})
	mustContain(t, m, "web")
	press(m, enter)
	mustContain(t, m, "Password for web")
	typ(m, "pw")
	mustNotContain(t, m, "pw") // masked
	press(m, enter)
	mustContain(t, m, "gh")
	press(m, enter)
	mustContain(t, m, "user")
	mustContain(t, m, "pass")
	mustNotContain(t, m, "s3cret")
	press(m, esc)
	mustContain(t, m, "gh")
	press(m, esc)
	mustContain(t, m, "web")
	press(m, enter) // cached: no second prompt
	mustNotContain(t, m, "Password for")
	mustContain(t, m, "gh")
	press(m, esc)
	if !isQuit(press(m, esc)) {
		t.Fatal("Esc at repositories should quit")
	}
}

func TestQuitKeys(t *testing.T) {
	e := newEnv(t)
	if !isQuit(press(New(e.deps, Options{}), k("q"))) {
		t.Fatal("q should quit")
	}
	if !isQuit(press(New(e.deps, Options{}), tea.KeyMsg{Type: tea.KeyCtrlC})) {
		t.Fatal("ctrl+c should quit")
	}
}

func TestWrongPasswordReprompts(t *testing.T) {
	e := newEnv(t)
	e.seed()
	m := New(e.deps, Options{})
	press(m, enter)
	typ(m, "nope")
	press(m, enter)
	mustContain(t, m, "wrong password")
	mustContain(t, m, "Password for web")
	typ(m, "pw")
	press(m, enter)
	mustContain(t, m, "gh")
}

func TestEscCancelsPasswordPrompt(t *testing.T) {
	e := newEnv(t)
	e.seed()
	m := New(e.deps, Options{})
	press(m, enter, esc)
	mustNotContain(t, m, "Password for")
	mustContain(t, m, "web")
}

func TestFilterLiveClearAndAccept(t *testing.T) {
	e := newEnv(t)
	for _, n := range []string{"alpha", "beta", "gamma"} {
		_ = e.store.Create(n, "pw")
	}
	m := New(e.deps, Options{})
	press(m, k("/"))
	typ(m, "ga")
	mustContain(t, m, "gamma")
	mustNotContain(t, m, "alpha")
	mustNotContain(t, m, "beta")
	press(m, esc)
	mustContain(t, m, "alpha")
	mustContain(t, m, "beta")

	press(m, k("/"))
	typ(m, "be")
	press(m, enter)
	mustContain(t, m, "beta")
	mustNotContain(t, m, "alpha")
	if m.mode != modeList {
		t.Fatal("Enter should return to list mode with filter applied")
	}
}

func openFields(t *testing.T, e *env) *Model {
	t.Helper()
	e.seed()
	e.deps.EnvPassword = "pw"
	return New(e.deps, Options{Repo: "web", Secret: "gh"})
}

func TestCopyFieldValue(t *testing.T) {
	e := newEnv(t)
	m := openFields(t, e)
	press(m, k("c")) // first field is user
	if e.clip.val != "me" || len(e.spawned) != 1 || e.spawned[0] != "me" {
		t.Fatalf("clip=%q spawned=%v", e.clip.val, e.spawned)
	}
	mustContain(t, m, "Copied")
}

func TestShowRevealsThenHides(t *testing.T) {
	e := newEnv(t)
	m := openFields(t, e)
	press(m, k("s"))
	mustContain(t, m, "me")
	press(m, revealExpiredMsg{seq: m.revealSeq})
	mustNotContain(t, m, "me\n")
	if m.revealed {
		t.Fatal("should be hidden after expiry")
	}
	// A stale timer must not hide a newer reveal.
	press(m, k("s"))
	stale := m.revealSeq - 1
	press(m, revealExpiredMsg{seq: stale})
	if !m.revealed {
		t.Fatal("stale expiry hid a newer reveal")
	}
}

func TestNewRepo(t *testing.T) {
	e := newEnv(t)
	m := New(e.deps, Options{})
	press(m, k("n"))
	typ(m, "cards")
	press(m, enter)
	typ(m, "pw")
	press(m, enter)
	typ(m, "other") // mismatch
	press(m, enter)
	mustContain(t, m, "do not match")
	if names, _ := e.store.List(); len(names) != 0 {
		t.Fatalf("mismatch must not create: %v", names)
	}
	press(m, k("n"))
	typ(m, "cards")
	press(m, enter)
	typ(m, "pw")
	press(m, enter)
	typ(m, "pw")
	press(m, enter)
	if names, _ := e.store.List(); len(names) != 1 || names[0] != "cards" {
		t.Fatalf("got %v", names)
	}
	mustContain(t, m, "cards")
}

func TestDeleteRepo(t *testing.T) {
	e := newEnv(t)
	e.seed()
	_ = e.store.Create("empty", "pw") // sorts before "web"
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{})
	press(m, k("x"), k("y"))
	if names, _ := e.store.List(); len(names) != 1 || names[0] != "web" {
		t.Fatalf("empty repo should be deleted, got %v", names)
	}
	press(m, k("x"), k("y"))
	mustContain(t, m, "contains secrets")
	if names, _ := e.store.List(); len(names) != 1 {
		t.Fatalf("non-empty repo must survive: %v", names)
	}
}

func TestDeleteConfirmCanBeCancelled(t *testing.T) {
	e := newEnv(t)
	_ = e.store.Create("empty", "pw")
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{})
	press(m, k("x"), k("n"))
	if names, _ := e.store.List(); len(names) != 1 {
		t.Fatalf("got %v", names)
	}
}

func TestNewAndDeleteSecret(t *testing.T) {
	e := newEnv(t)
	e.seed()
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{Repo: "web"})
	press(m, k("n"))
	typ(m, "gl")
	press(m, enter)
	typ(m, "glu") // user
	press(m, enter)
	typ(m, "glp") // pass
	press(m, enter)
	r := e.repoOf("web")
	if r.Secrets["gl"]["user"] != "glu" || r.Secrets["gl"]["pass"] != "glp" {
		t.Fatalf("got %v", r.Secrets)
	}
	mustContain(t, m, "gl")
	press(m, k("x"), k("y")) // cursor is on "gh", the first row
	r = e.repoOf("web")
	if _, ok := r.Secrets["gh"]; ok || len(r.Secrets) != 1 {
		t.Fatalf("got %v", r.Secrets)
	}
}

func TestNewSecretValidationError(t *testing.T) {
	e := newEnv(t)
	e.seed()
	h, _ := e.store.Open("web", "pw")
	_ = h.Repo.SetAttrib("user", "pattern", "[a-z]+")
	_ = h.Save()
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{Repo: "web"})
	press(m, k("n"))
	typ(m, "bad")
	press(m, enter)
	typ(m, "UPPER")
	press(m, enter)
	typ(m, "x")
	press(m, enter)
	mustContain(t, m, "does not match")
	if _, ok := e.repoOf("web").Secrets["bad"]; ok {
		t.Fatal("invalid secret must not be saved")
	}
}

func TestRepoFieldsScreen(t *testing.T) {
	e := newEnv(t)
	e.seed()
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{})
	press(m, k("f"))
	mustContain(t, m, "user")
	mustContain(t, m, "pass")
	press(m, k("n"))
	typ(m, "pin")
	press(m, enter)
	mustContain(t, m, "pin")
	if len(e.repoOf("web").Fields) != 3 {
		t.Fatalf("got %v", e.repoOf("web").Fields)
	}
	// make "pin" (last row) pattern-restricted
	press(m, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown}, k("p"))
	typ(m, "[0-9]+")
	press(m, enter)
	if v, _ := e.repoOf("web").GetAttrib("pin", "pattern"); v != "[0-9]+" {
		t.Fatalf("pattern=%q", v)
	}
	press(m, k("r")) // toggling required with secrets present must be refused
	mustContain(t, m, "contains secrets")
	press(m, k("x"), k("y"))
	if len(e.repoOf("web").Fields) != 2 {
		t.Fatalf("got %v", e.repoOf("web").Fields)
	}
	press(m, esc)
	mustContain(t, m, "web")
}

func TestEditorSavesChanges(t *testing.T) {
	e := newEnv(t)
	m := openFields(t, e)
	var dirMode, fileMode os.FileMode
	var seenPath string
	e.editor = func(path string) error {
		seenPath = path
		di, _ := os.Stat(filepath.Dir(path))
		fi, _ := os.Stat(path)
		dirMode, fileMode = di.Mode().Perm(), fi.Mode().Perm()
		b, _ := os.ReadFile(path)
		if !strings.Contains(string(b), "user: me") || !strings.Contains(string(b), "pass: s3cret") {
			t.Errorf("editor content: %q", b)
		}
		return os.WriteFile(path, []byte("user: you\npass: line1\\nline2\n"), 0o600)
	}
	finishEdit(t, m, press(m, k("e")))
	r := e.repoOf("web")
	if r.Secrets["gh"]["user"] != "you" || r.Secrets["gh"]["pass"] != "line1\nline2" {
		t.Fatalf("got %v", r.Secrets["gh"])
	}
	if dirMode != 0o700 || fileMode != 0o600 {
		t.Fatalf("modes dir=%v file=%v", dirMode, fileMode)
	}
	if _, err := os.Stat(filepath.Dir(seenPath)); !os.IsNotExist(err) {
		t.Fatal("temp dir should be removed")
	}
}

// finishEdit runs the editor command returned by Update and delivers its message.
func finishEdit(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("no editor command returned")
	}
	_, _ = m.Update(cmd())
}

func TestEditorErrorLeavesSecretAndCleansUp(t *testing.T) {
	e := newEnv(t)
	m := openFields(t, e)
	var seenPath string
	e.editor = func(path string) error { seenPath = path; return errors.New("editor crashed") }
	finishEdit(t, m, press(m, k("e")))
	if e.repoOf("web").Secrets["gh"]["user"] != "me" {
		t.Fatal("secret must be unchanged")
	}
	if _, err := os.Stat(filepath.Dir(seenPath)); !os.IsNotExist(err) {
		t.Fatal("temp dir should be removed")
	}
	mustContain(t, m, "editor crashed")
}

func TestEditorInvalidValueLeavesSecret(t *testing.T) {
	e := newEnv(t)
	e.seed()
	h, _ := e.store.Open("web", "pw")
	_ = h.Repo.SetAttrib("user", "pattern", "[a-z]+")
	_ = h.Save()
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{Repo: "web", Secret: "gh"})
	var seenPath string
	e.editor = func(path string) error {
		seenPath = path
		return os.WriteFile(path, []byte("user: BAD\npass: x\n"), 0o600)
	}
	finishEdit(t, m, press(m, k("e")))
	mustContain(t, m, "does not match")
	if e.repoOf("web").Secrets["gh"]["user"] != "me" {
		t.Fatal("secret must be unchanged")
	}
	if _, err := os.Stat(filepath.Dir(seenPath)); !os.IsNotExist(err) {
		t.Fatal("temp dir should be removed")
	}
}

func TestEditorDeletedTempFileLeavesSecret(t *testing.T) {
	e := newEnv(t)
	m := openFields(t, e)
	e.editor = func(path string) error { return os.Remove(path) }
	finishEdit(t, m, press(m, k("e")))
	if e.repoOf("web").Secrets["gh"]["user"] != "me" {
		t.Fatal("secret must be unchanged")
	}
}

func TestEditFormatRoundTrip(t *testing.T) {
	in := map[string]string{"a": "x: y", "b": "back\\slash", "c": "l1\nl2"}
	out, err := parseEdit(formatEdit([]string{"a", "b", "c"}, in), []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range in {
		if out[k] != v {
			t.Fatalf("%s: got %q want %q", k, out[k], v)
		}
	}
	if _, err := parseEdit("bogus: 1\n", []string{"a"}); err == nil {
		t.Fatal("unknown field should error")
	}
	if _, err := parseEdit("no colon here\n", []string{"a"}); err == nil {
		t.Fatal("malformed line should error")
	}
}

func TestStartOptions(t *testing.T) {
	e := newEnv(t)
	e.seed()
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{Repo: "web"})
	mustContain(t, m, "gh")
	m = New(e.deps, Options{Repo: "web", Secret: "gh"})
	mustContain(t, m, "user")
	m = New(e.deps, Options{Repo: "web", Secret: "nope"})
	mustContain(t, m, "not found")
	mustContain(t, m, "gh") // falls back to the secrets list
	m = New(e.deps, Options{Repo: "ghost"})
	mustContain(t, m, "not found")
	mustContain(t, m, "web") // falls back to the repositories list
}

func TestStartWithRepoPromptsWhenNoEnvPassword(t *testing.T) {
	e := newEnv(t)
	e.seed()
	m := New(e.deps, Options{Repo: "web"})
	mustContain(t, m, "Password for web")
	typ(m, "pw")
	press(m, enter)
	mustContain(t, m, "gh")
}

func TestResumeAndStateSavedOnQuit(t *testing.T) {
	e := newEnv(t)
	e.seed()
	e.deps.EnvPassword = "pw"
	m := New(e.deps, Options{Repo: "web", Secret: "gh"})
	press(m, k("q"))
	st, _ := config.LoadState(e.statePath)
	if st.Repo != "web" || st.Secret != "gh" {
		t.Fatalf("state=%+v", st)
	}
	m = New(e.deps, Options{Resume: true})
	mustContain(t, m, "user")

	// quitting from the repositories screen clears the saved position
	m = New(e.deps, Options{})
	press(m, k("q"))
	st, _ = config.LoadState(e.statePath)
	if st != (config.State{}) {
		t.Fatalf("state=%+v", st)
	}
}

func TestCleanupIsSafeAgainstConcurrentEdit(t *testing.T) {
	e := newEnv(t)
	m := openFields(t, e)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			m.Cleanup() // as the signal goroutine does
		}
	}()
	for i := 0; i < 50; i++ {
		press(m, k("e"))
	}
	<-done
	m.Cleanup()
}

func TestCopyWithUnclearableClipboardWarns(t *testing.T) {
	e := newEnv(t)
	var buf bytes.Buffer
	e.deps.Clip = clip.Detect(func(string) (string, error) { return "", errors.New("none") }, "linux", &buf)
	m := openFields(t, e)
	press(m, k("c"))
	mustContain(t, m, "cannot be cleared automatically")
	if len(e.spawned) != 0 {
		t.Fatalf("no clear helper should be spawned: %v", e.spawned)
	}
}

func TestParseEditRejectsDuplicateFields(t *testing.T) {
	_, err := parseEdit("a: 1\nb: 2\na: 3\n", []string{"a", "b"})
	if err == nil || !strings.Contains(err.Error(), "line 3") || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("got %v", err)
	}
}
