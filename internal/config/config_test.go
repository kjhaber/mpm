package config

import (
	"os"
	"path/filepath"
	"testing"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func writeSettings(t *testing.T, dir, body string) {
	t.Helper()
	p := filepath.Join(dir, "mpm")
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDefaults(t *testing.T) {
	env := envOf(map[string]string{"HOME": "/home/u"})
	s, err := Load(env, Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Root != "/home/u/.local/share/mpm" || s.Format != "text" || s.Repo != "" {
		t.Fatalf("got %+v", s)
	}
	p := ResolvePaths(env)
	if p.Settings != "/home/u/.config/mpm/settings.json" || p.State != "/home/u/.local/state/mpm/state.json" {
		t.Fatalf("got %+v", p)
	}
}

func TestXDGHonored(t *testing.T) {
	env := envOf(map[string]string{"HOME": "/h", "XDG_DATA_HOME": "/d", "XDG_CONFIG_HOME": "/c", "XDG_STATE_HOME": "/s"})
	s, _ := Load(env, Settings{})
	p := ResolvePaths(env)
	if s.Root != "/d/mpm" || p.Settings != "/c/mpm/settings.json" || p.State != "/s/mpm/state.json" {
		t.Fatalf("got %+v %+v", s, p)
	}
}

func TestPrecedence(t *testing.T) {
	cfg := t.TempDir()
	writeSettings(t, cfg, `{"root":"/file","format":"json","repo":"fromfile"}`)
	base := map[string]string{"HOME": "/h", "XDG_CONFIG_HOME": cfg}

	s, err := Load(envOf(base), Settings{})
	if err != nil || s.Root != "/file" || s.Format != "json" || s.Repo != "fromfile" {
		t.Fatalf("file: %+v %v", s, err)
	}

	base["MPM_ROOT"] = "/env"
	s, _ = Load(envOf(base), Settings{})
	if s.Root != "/env" {
		t.Fatalf("env should beat file: %+v", s)
	}

	s, _ = Load(envOf(base), Settings{Root: "/flag"})
	if s.Root != "/flag" {
		t.Fatalf("flag should beat env: %+v", s)
	}
}

func TestInvalidFormatErrors(t *testing.T) {
	env := envOf(map[string]string{"HOME": "/h", "MPM_FORMAT": "xml"})
	if _, err := Load(env, Settings{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestMalformedSettingsErrors(t *testing.T) {
	cfg := t.TempDir()
	writeSettings(t, cfg, `{nope`)
	if _, err := Load(envOf(map[string]string{"HOME": "/h", "XDG_CONFIG_HOME": cfg}), Settings{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestStateRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "state.json")
	got, err := LoadState(p)
	if err != nil || got != (State{}) {
		t.Fatalf("missing state: %+v %v", got, err)
	}
	if err := SaveState(p, State{Repo: "r", Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	got, err = LoadState(p)
	if err != nil || got != (State{Repo: "r", Secret: "s"}) {
		t.Fatalf("got %+v %v", got, err)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}
