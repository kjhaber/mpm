package vault

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kjhaber/mpm/internal/crypt"
)

// syncFile flushes a file or directory to stable storage (swapped out in tests).
var syncFile = func(f *os.File) error { return f.Sync() }

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]*$`)

// ValidateName reports whether name is usable as a repository name.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid name %q (letters, digits, space, . _ - ; must start with a letter or digit)", name)
	}
	return nil
}

// Store manages encrypted repository files under Root.
type Store struct {
	Root    string
	Crypter crypt.Crypter
}

// Handle is an opened repository; Save writes it back if the file is unchanged on disk.
type Handle struct {
	Repo *Repo

	store    *Store
	name     string
	password string
	digest   [sha256.Size]byte // of the file content as last read or written
}

func (s *Store) path(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", fmt.Errorf("repository name: %w", err)
	}
	return filepath.Join(s.Root, name+".enc"), nil
}

// List returns repository names, sorted.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.Root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".enc"); ok && !e.IsDir() {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Create makes a new empty repository.
func (s *Store) Create(name, password string) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("repository %q: %w", name, ErrExists)
	}
	return s.write(p, newRepo(), password)
}

func newRepo() *Repo {
	return &Repo{Fields: []Field{}, Secrets: map[string]map[string]string{}}
}

// Open decrypts a repository.
func (s *Store) Open(name, password string) (*Handle, error) {
	p, err := s.path(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("repository %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	plain, err := s.Crypter.Decrypt(password, data)
	if err != nil {
		return nil, err
	}
	r := newRepo()
	if err := json.Unmarshal(plain, r); err != nil {
		// A wrong password passes openssl's padding check about 1 time in 256 and
		// yields garbage, so undecodable plaintext is reported like any bad password.
		return nil, fmt.Errorf("repository %q: %w", name, crypt.ErrBadPassword)
	}
	if r.Secrets == nil {
		r.Secrets = map[string]map[string]string{}
	}
	return &Handle{Repo: r, store: s, name: name, password: password, digest: sha256.Sum256(data)}, nil
}

// Save encrypts and atomically replaces the file, unless it changed since it was read.
func (h *Handle) Save() error {
	p, err := h.store.path(h.name)
	if err != nil {
		return err
	}
	cur, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if sha256.Sum256(cur) != h.digest {
		return ErrChangedOnDisk
	}
	if err := h.store.write(p, h.Repo, h.password); err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	h.digest = sha256.Sum256(data)
	return nil
}

// Delete removes an empty repository after verifying its password.
func (s *Store) Delete(name, password string) error {
	h, err := s.Open(name, password)
	if err != nil {
		return err
	}
	if len(h.Repo.Secrets) > 0 {
		return ErrRepoNotEmpty
	}
	p, _ := s.path(name)
	return os.Remove(p)
}

// Copy creates repository to as a copy of from (fields, and optionally secrets) under a new password.
func (s *Store) Copy(from, fromPassword, to, toPassword string, includeSecrets bool) error {
	src, err := s.Open(from, fromPassword)
	if err != nil {
		return err
	}
	dst, err := s.path(to)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("repository %q: %w", to, ErrExists)
	}
	r := newRepo()
	r.Fields = append(r.Fields, src.Repo.Fields...)
	if includeSecrets {
		for n, vals := range src.Repo.Secrets {
			cp := make(map[string]string, len(vals))
			for k, v := range vals {
				cp[k] = v
			}
			r.Secrets[n] = cp
		}
	}
	return s.write(dst, r, toPassword)
}

// write encrypts r and atomically places it at path (0600).
func (s *Store) write(path string, r *Repo, password string) error {
	plain, err := json.Marshal(r)
	if err != nil {
		return err
	}
	enc, err := s.Crypter.Encrypt(password, plain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Root, ".mpm-tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(bytes.TrimSpace(enc)); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString("\n"); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := syncFile(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return syncDir(s.Root)
}

// syncDir fsyncs a directory so a rename into it survives a crash.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return syncFile(d)
}
