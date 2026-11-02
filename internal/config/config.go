// Package config resolves mpm's XDG paths, settings and resume state.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Settings are mpm's user-tunable settings.
type Settings struct {
	Root   string `json:"root,omitempty"`
	Format string `json:"format,omitempty"`
	Repo   string `json:"repo,omitempty"`
}

// Paths are the resolved locations of mpm's config and state files.
type Paths struct {
	Settings string
	State    string
}

// State is where the TUI was when it last exited (names only, never values).
type State struct {
	Repo   string `json:"repo,omitempty"`
	Secret string `json:"secret,omitempty"`
}

func xdg(env func(string) string, name, fallback string) string {
	if v := env(name); v != "" {
		return v
	}
	return filepath.Join(env("HOME"), fallback)
}

// ResolvePaths returns the settings and state file paths.
func ResolvePaths(env func(string) string) Paths {
	return Paths{
		Settings: filepath.Join(xdg(env, "XDG_CONFIG_HOME", ".config"), "mpm", "settings.json"),
		State:    filepath.Join(xdg(env, "XDG_STATE_HOME", ".local/state"), "mpm", "state.json"),
	}
}

// Load merges settings with precedence flags > MPM_* env > settings.json > defaults.
func Load(env func(string) string, flags Settings) (Settings, error) {
	s := Settings{
		Root:   filepath.Join(xdg(env, "XDG_DATA_HOME", ".local/share"), "mpm"),
		Format: "text",
	}
	path := ResolvePaths(env).Settings
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		var f Settings
		if err := json.Unmarshal(data, &f); err != nil {
			return s, fmt.Errorf("reading %s: %w", path, err)
		}
		s = overlay(s, f)
	case !errors.Is(err, fs.ErrNotExist):
		return s, err
	}
	s = overlay(s, Settings{Root: env("MPM_ROOT"), Format: env("MPM_FORMAT"), Repo: env("MPM_REPO")})
	s = overlay(s, flags)
	if s.Format != "text" && s.Format != "json" {
		return s, fmt.Errorf("invalid format %q (want text or json)", s.Format)
	}
	return s, nil
}

func overlay(base, over Settings) Settings {
	if over.Root != "" {
		base.Root = over.Root
	}
	if over.Format != "" {
		base.Format = over.Format
	}
	if over.Repo != "" {
		base.Repo = over.Repo
	}
	return base
}

// LoadState reads the resume state; a missing file yields the zero State.
func LoadState(path string) (State, error) {
	var st State
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(data, &st)
}

// SaveState writes the resume state with 0600 permissions, creating directories.
func SaveState(path string, st State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
