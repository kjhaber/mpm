package main

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/kjhaber/mpm/internal/clip"
	"github.com/kjhaber/mpm/internal/crypt"
)

type fakeClip struct{ val string }

func (f *fakeClip) Write(s string) error { f.val = s; return nil }

type testEnv struct {
	t       *testing.T
	a       *app
	clip    *fakeClip
	prompts []string // answers consumed by hidden prompts
	vars    map[string]string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	e := &testEnv{t: t, clip: &fakeClip{}, vars: map[string]string{
		"HOME": dir, "XDG_CONFIG_HOME": dir + "/cfg", "MPM_ROOT": dir + "/repos", "MPM_PASSWORD": "pw",
	}}
	e.a = &app{
		env:     func(k string) string { return e.vars[k] },
		crypter: crypt.NewOpenSSL(),
		clip:    e.clip,
		prompt: func(string) (string, error) {
			if len(e.prompts) == 0 {
				return "", errors.New("unexpected prompt")
			}
			a := e.prompts[0]
			e.prompts = e.prompts[1:]
			return a, nil
		},
	}
	return e
}

func (e *testEnv) run(args ...string) (string, error) {
	e.t.Helper()
	var out bytes.Buffer
	cmd := newRootCmd(e.a)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func (e *testEnv) must(args ...string) string {
	e.t.Helper()
	out, err := e.run(args...)
	if err != nil {
		e.t.Fatalf("mpm %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func (e *testEnv) mustFail(args ...string) error {
	e.t.Helper()
	_, err := e.run(args...)
	if err == nil {
		e.t.Fatalf("mpm %s: expected error", strings.Join(args, " "))
	}
	return err
}

func TestRepoLifecycle(t *testing.T) {
	e := newTestEnv(t)
	e.must("repo", "create", "--repo", "web")
	e.must("repo", "create", "--repo", "cards")
	if got := e.must("repo", "ls"); got != "cards\nweb\n" {
		t.Fatalf("list: %q", got)
	}
	e.must("field", "create", "--repo", "web", "--field", "user")
	if got := e.must("repo", "show", "--repo", "web"); !strings.Contains(got, "user") {
		t.Fatalf("show: %q", got)
	}
	e.must("repo", "copy", "--repo", "web2", "--from", "web")
	if got := e.must("field", "list", "--repo", "web2"); got != "user\n" {
		t.Fatalf("copied fields: %q", got)
	}
	e.must("secret", "create", "--repo", "web", "--secret", "gh", "--value", "user:me")
	if err := e.mustFail("repo", "delete", "--repo", "web"); !strings.Contains(err.Error(), "contains secrets") {
		t.Fatalf("got %v", err)
	}
	e.must("secret", "rm", "--repo", "web", "--secret", "gh")
	e.must("repo", "rm", "--repo", "web")
	if got := e.must("repo", "list"); got != "cards\nweb2\n" {
		t.Fatalf("list: %q", got)
	}
	e.must("repo", "copy", "--repo", "web3", "--from", "web2", "--include-secrets")
}

func TestFieldLifecycle(t *testing.T) {
	e := newTestEnv(t)
	e.must("repo", "create", "--repo", "r")
	e.must("field", "create", "--repo", "r", "--field", "pin")
	if got := e.must("field", "list-attribs", "--repo", "r", "--field", "pin"); got != "required\npattern\n" {
		t.Fatalf("list-attribs: %q", got)
	}
	if got := e.must("field", "get-attrib", "--repo", "r", "--field", "pin", "--attrib", "required"); got != "false\n" {
		t.Fatalf("get-attrib: %q", got)
	}
	e.must("field", "set-attrib", "--repo", "r", "--field", "pin", "--attrib", "pattern", "--value", "[0-9]{4}")
	e.must("field", "set-attrib", "--repo", "r", "--field", "pin", "--attrib", "required", "--value", "true")
	if got := e.must("field", "show", "--repo", "r", "--field", "pin"); !strings.Contains(got, "[0-9]{4}") || !strings.Contains(got, "required: true") {
		t.Fatalf("show: %q", got)
	}
	_ = e.mustFail("field", "set-attrib", "--repo", "r", "--field", "pin", "--attrib", "required", "--value", "yes")
	e.must("field", "delete-attrib", "--repo", "r", "--field", "pin", "--attrib", "pattern")
	if got := e.must("field", "get-attrib", "--repo", "r", "--field", "pin", "--attrib", "pattern"); got != ".+\n" {
		t.Fatalf("after delete: %q", got)
	}
	e.must("field", "rm", "--repo", "r", "--field", "pin")
	if got := e.must("field", "list", "--repo", "r"); got != "" {
		t.Fatalf("list: %q", got)
	}
}

func TestSecretLifecycle(t *testing.T) {
	e := newTestEnv(t)
	e.must("repo", "create", "--repo", "r")
	e.must("field", "create", "--repo", "r", "--field", "user")
	e.must("field", "create", "--repo", "r", "--field", "pass")

	e.must("secret", "create", "--repo", "r", "--secret", "gh", "--value", `user:me,pass:a\,b`)
	if got := e.must("secret", "list", "--repo", "r"); got != "gh\n" {
		t.Fatalf("list: %q", got)
	}
	if got := e.must("secret", "show", "--repo", "r", "--secret", "gh"); got != "user: me\npass: a,b\n" {
		t.Fatalf("show: %q", got)
	}
	if got := e.must("secret", "show", "--repo", "r", "--secret", "gh", "--field", "pass"); got != "a,b\n" {
		t.Fatalf("show field: %q", got)
	}
	if got := e.must("secret", "show", "--repo", "r", "--secret", "gh", "--format", "json"); got != `{"pass":"a,b","user":"me"}`+"\n" {
		t.Fatalf("json: %q", got)
	}
	_ = e.mustFail("secret", "create", "--repo", "r", "--secret", "gh", "--value", "user:x")

	e.must("secret", "update", "--repo", "r", "--secret", "gh", "--value", "pass:new")
	if got := e.must("secret", "show", "--repo", "r", "--secret", "gh"); got != "user: me\npass: new\n" {
		t.Fatalf("update should merge: %q", got)
	}
	_ = e.mustFail("secret", "update", "--repo", "r", "--secret", "missing", "--value", "user:x")

	if got := e.must("secret", "copy", "--repo", "r", "--secret", "gh", "--field", "pass"); got != "Copied to clipboard.\n" || e.clip.val != "new" {
		t.Fatalf("out=%q clip=%q", got, e.clip.val)
	}
	_ = e.mustFail("secret", "copy", "--repo", "r", "--secret", "gh", "--no-clear")
	e.must("secret", "delete", "--repo", "r", "--secret", "gh")
	_ = e.mustFail("secret", "show", "--repo", "r", "--secret", "gh")
}

func TestSecretPromptsWhenValueOmitted(t *testing.T) {
	e := newTestEnv(t)
	e.must("repo", "create", "--repo", "r")
	e.must("field", "create", "--repo", "r", "--field", "user")
	e.must("field", "create", "--repo", "r", "--field", "pass")
	e.prompts = []string{"me", "s3cret"}
	e.must("secret", "create", "--repo", "r", "--secret", "gh")
	if got := e.must("secret", "show", "--repo", "r", "--secret", "gh"); got != "user: me\npass: s3cret\n" {
		t.Fatalf("got %q", got)
	}
	e.prompts = []string{"", "changed"} // empty keeps existing value
	e.must("secret", "update", "--repo", "r", "--secret", "gh")
	if got := e.must("secret", "show", "--repo", "r", "--secret", "gh"); got != "user: me\npass: changed\n" {
		t.Fatalf("got %q", got)
	}
}

func TestValidationErrors(t *testing.T) {
	e := newTestEnv(t)
	e.must("repo", "create", "--repo", "r")
	e.must("field", "create", "--repo", "r", "--field", "pin")
	e.must("field", "set-attrib", "--repo", "r", "--field", "pin", "--attrib", "pattern", "--value", "[0-9]+")
	_ = e.mustFail("secret", "create", "--repo", "r", "--secret", "s", "--value", "pin:abc")
	_ = e.mustFail("secret", "create", "--repo", "r", "--secret", "s", "--value", "nofield:abc")
	_ = e.mustFail("secret", "create", "--repo", "r", "--secret", "s", "--value", "garbage")
}

func TestUnknownRepoAndWrongPassword(t *testing.T) {
	e := newTestEnv(t)
	if err := e.mustFail("secret", "list", "--repo", "nope"); !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
	e.must("repo", "create", "--repo", "r")
	e.vars["MPM_PASSWORD"] = "wrong"
	if err := e.mustFail("secret", "list", "--repo", "r"); !strings.Contains(err.Error(), "wrong password") {
		t.Fatalf("got %v", err)
	}
}

func TestRepoDefaultsFromSettingsEnv(t *testing.T) {
	e := newTestEnv(t)
	e.must("repo", "create", "--repo", "r")
	e.vars["MPM_REPO"] = "r"
	e.must("field", "create", "--field", "user")
	if got := e.must("field", "list"); got != "user\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMissingRepoFlagErrors(t *testing.T) {
	e := newTestEnv(t)
	if err := e.mustFail("secret", "list"); !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("got %v", err)
	}
}

func TestClipclearHelperRemoved(t *testing.T) {
	e := newTestEnv(t)
	_ = e.mustFail("_clipclear", "--after", "0s")
}

func TestTUIFlagValidation(t *testing.T) {
	e := newTestEnv(t)
	if err := e.mustFail("tui", "--secret", "x"); !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("got %v", err)
	}
	if err := e.mustFail("tui", "--resume", "--repo", "r"); !strings.Contains(err.Error(), "--resume") {
		t.Fatalf("got %v", err)
	}
}

func TestChildCommandsDoNotInheritPassword(t *testing.T) {
	t.Setenv("MPM_PASSWORD", "hunter2")
	editor, err := editorCommand(func(string) string { return "" }, "/tmp/x")
	if err != nil {
		t.Fatal(err)
	}
	for name, cmd := range map[string]*exec.Cmd{
		"editor": editor,
	} {
		for _, kv := range cmd.Env {
			if strings.HasPrefix(kv, "MPM_PASSWORD=") {
				t.Fatalf("%s command inherits MPM_PASSWORD", name)
			}
		}
		if cmd.Env == nil {
			t.Fatalf("%s command must set Env explicitly", name)
		}
	}
	if got, err := editorCommand(func(string) string { return "  " }, "/tmp/x"); err != nil || got.Args[0] != "vi" {
		t.Fatalf("blank $VISUAL should fall back to vi: %v", got.Args)
	}
}

func TestCopyWithOSC52Clipboard(t *testing.T) {
	e := newTestEnv(t)
	var buf bytes.Buffer
	e.a.clip = clip.Detect(func(string) (string, error) { return "", errors.New("none") }, "linux", &buf)
	e.must("repo", "create", "--repo", "r")
	e.must("field", "create", "--repo", "r", "--field", "f")
	e.must("secret", "create", "--repo", "r", "--secret", "s", "--value", "f:v")
	out := e.must("secret", "copy", "--repo", "r", "--secret", "s")
	if out != "Copied to clipboard.\n" {
		t.Fatalf("out=%q", out)
	}
	if !strings.Contains(buf.String(), "\x1b]52;c;") {
		t.Fatalf("OSC52 not written: %q", buf.String())
	}
}

func TestInvalidRepoNameFailsBeforePrompting(t *testing.T) {
	e := newTestEnv(t)
	delete(e.vars, "MPM_PASSWORD") // no env password and no queued prompts: any prompt fails with "unexpected prompt"
	for _, args := range [][]string{
		{"repo", "create", "--repo", "../evil"},
		{"repo", "copy", "--repo", "../evil", "--from", "web"},
		{"repo", "copy", "--repo", "ok", "--from", "../evil"},
		{"repo", "delete", "--repo", "../evil"},
		{"repo", "show", "--repo", "../evil"},
		{"secret", "list", "--repo", "../evil"},
	} {
		err := e.mustFail(args...)
		if !strings.Contains(err.Error(), "invalid name") {
			t.Errorf("mpm %s: got %v", strings.Join(args, " "), err)
		}
	}
}

func TestEditorCommandHonoursQuotedArguments(t *testing.T) {
	env := func(k string) string {
		if k == "VISUAL" {
			return `"/opt/my ed/ed" --flag 'a b'`
		}
		return ""
	}
	cmd, err := editorCommand(env, "/tmp/x")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/opt/my ed/ed", "--flag", "a b", "/tmp/x"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("got %q want %q", cmd.Args, want)
	}
	if _, err := editorCommand(func(string) string { return `ed "oops` }, "/tmp/x"); err == nil {
		t.Fatal("unterminated quote should error")
	}
}

func TestSecretListNeutralisesControlCharactersInLegacyNames(t *testing.T) {
	e := newTestEnv(t)
	e.must("repo", "create", "--repo", "web")
	// A name from before names were restricted: plant it directly in the repo file.
	st, err := e.a.store()
	if err != nil {
		t.Fatal(err)
	}
	h, err := st.Open("web", "pw")
	if err != nil {
		t.Fatal(err)
	}
	h.Repo.Secrets = map[string]map[string]string{"evil\x1b[2J\nname": {}}
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	got := e.must("secret", "list", "--repo", "web")
	if strings.ContainsAny(got, "\x1b") || got != "evil?[2J?name\n" {
		t.Fatalf("got %q", got)
	}
}

func TestEditorCommandDisablesVimHistoryFiles(t *testing.T) {
	for _, ed := range []string{"vim", "/usr/local/bin/nvim", "vim -u NONE"} {
		cmd, err := editorCommand(func(k string) string {
			if k == "EDITOR" {
				return ed
			}
			return ""
		}, "/tmp/x")
		if err != nil {
			t.Fatal(err)
		}
		joined := " " + strings.Join(cmd.Args, " ") + " "
		for _, want := range []string{" -n ", " -i NONE ", "noundofile", "nobackup", "nowritebackup"} {
			if !strings.Contains(joined, want) {
				t.Errorf("%s: missing %q in %q", ed, want, joined)
			}
		}
		if cmd.Args[len(cmd.Args)-1] != "/tmp/x" {
			t.Errorf("%s: path must be last: %q", ed, cmd.Args)
		}
	}
	cmd, _ := editorCommand(func(k string) string {
		if k == "EDITOR" {
			return "code --wait"
		}
		return ""
	}, "/tmp/x")
	if want := []string{"code", "--wait", "/tmp/x"}; !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("other editors are left alone: %q", cmd.Args)
	}
}
