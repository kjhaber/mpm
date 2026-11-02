package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kjhaber/mpm/internal/crypt"
)

// fakeCrypter is a reversible, password-checking stand-in for openssl.
type fakeCrypter struct{}

func (fakeCrypter) Encrypt(pw string, in []byte) ([]byte, error) {
	return []byte("enc:" + pw + ":" + string(in)), nil
}

func (fakeCrypter) Decrypt(pw string, in []byte) ([]byte, error) {
	prefix := "enc:" + pw + ":"
	if !strings.HasPrefix(string(in), prefix) {
		return nil, crypt.ErrBadPassword
	}
	return in[len(prefix):], nil
}

func newStore(t *testing.T) *Store {
	t.Helper()
	return &Store{Root: filepath.Join(t.TempDir(), "root"), Crypter: fakeCrypter{}}
}

func mustCreateOpen(t *testing.T, s *Store, name string) *Handle {
	t.Helper()
	if err := s.Create(name, "pw"); err != nil {
		t.Fatal(err)
	}
	h, err := s.Open(name, "pw")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestFieldDefaults(t *testing.T) {
	r := &Repo{}
	if err := r.AddField("user"); err != nil {
		t.Fatal(err)
	}
	if v, _ := r.GetAttrib("user", "required"); v != "false" {
		t.Fatalf("required=%q", v)
	}
	if v, _ := r.GetAttrib("user", "pattern"); v != ".+" {
		t.Fatalf("pattern=%q", v)
	}
	if err := r.AddField("user"); err == nil {
		t.Fatal("duplicate field should error")
	}
}

func TestSetAttribValidation(t *testing.T) {
	r := &Repo{}
	_ = r.AddField("f")
	if err := r.SetAttrib("f", "required", "True"); err == nil {
		t.Fatal("required is case-sensitive")
	}
	if err := r.SetAttrib("f", "pattern", "(unclosed"); err == nil {
		t.Fatal("invalid regex should error")
	}
	if err := r.SetAttrib("f", "bogus", "x"); err == nil {
		t.Fatal("unknown attrib should error")
	}
	if err := r.SetAttrib("nope", "required", "true"); err == nil {
		t.Fatal("unknown field should error")
	}
	if err := r.SetAttrib("f", "required", "true"); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteAttrib("f", "required"); err != nil {
		t.Fatal(err)
	}
	if v, _ := r.GetAttrib("f", "required"); v != "false" {
		t.Fatalf("after delete required=%q", v)
	}
}

func TestRequiredRefusedWithSecrets(t *testing.T) {
	r := &Repo{}
	_ = r.AddField("a")
	_ = r.AddField("b")
	if err := r.SetSecret("s", map[string]string{"a": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetAttrib("b", "required", "true"); !errors.Is(err, ErrRequiredWithSecrets) {
		t.Fatalf("got %v", err)
	}
}

func TestSetSecretValidation(t *testing.T) {
	r := &Repo{}
	_ = r.AddField("user")
	_ = r.AddField("pin")
	_ = r.SetAttrib("user", "required", "true")
	_ = r.SetAttrib("pin", "pattern", `[0-9]{4}`)

	if err := r.SetSecret("s", map[string]string{"pin": "1234"}); err == nil {
		t.Fatal("missing required field should error")
	}
	if err := r.SetSecret("s", map[string]string{"user": "u", "pin": "12a4"}); err == nil {
		t.Fatal("pattern mismatch should error")
	}
	if err := r.SetSecret("s", map[string]string{"user": "u", "pin": "12345"}); err == nil {
		t.Fatal("pattern must match the whole value")
	}
	if err := r.SetSecret("s", map[string]string{"user": "u", "zzz": "x"}); err == nil {
		t.Fatal("unknown field should error")
	}
	if err := r.SetSecret("s", map[string]string{"user": "u", "pin": "1234"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetSecret("m", map[string]string{"user": "line1\nline2"}); err != nil {
		t.Fatalf("multi-line value should match default pattern: %v", err)
	}
	if err := r.DeleteSecret("s"); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteSecret("s"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteFieldRemovesValues(t *testing.T) {
	r := &Repo{}
	_ = r.AddField("a")
	_ = r.AddField("b")
	_ = r.SetSecret("s", map[string]string{"a": "1", "b": "2"})
	if err := r.DeleteField("b"); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Secrets["s"]["b"]; ok {
		t.Fatal("value for deleted field should be removed")
	}
}

func TestCreateOpenDelete(t *testing.T) {
	s := newStore(t)
	if err := s.Create("web", "pw"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create("web", "pw"); !errors.Is(err, ErrExists) {
		t.Fatalf("got %v", err)
	}
	h, err := s.Open("web", "pw")
	if err != nil {
		t.Fatal(err)
	}
	_ = h.Repo.AddField("user")
	_ = h.Repo.SetSecret("gh", map[string]string{"user": "me"})
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("web", "pw"); !errors.Is(err, ErrRepoNotEmpty) {
		t.Fatalf("got %v", err)
	}
	h2, _ := s.Open("web", "pw")
	_ = h2.Repo.DeleteSecret("gh")
	if err := h2.Save(); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("web", "pw"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open("web", "pw"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestWrongPasswordLeavesFileUntouched(t *testing.T) {
	s := newStore(t)
	_ = s.Create("web", "pw")
	path := filepath.Join(s.Root, "web.enc")
	before, _ := os.ReadFile(path)
	if _, err := s.Open("web", "bad"); !errors.Is(err, crypt.ErrBadPassword) {
		t.Fatalf("got %v", err)
	}
	if err := s.Delete("web", "bad"); !errors.Is(err, crypt.ErrBadPassword) {
		t.Fatalf("got %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("file changed")
	}
}

func TestSaveRefusesWhenChangedOnDisk(t *testing.T) {
	s := newStore(t)
	h := mustCreateOpen(t, s, "web")
	other, _ := s.Open("web", "pw")
	_ = other.Repo.AddField("x")
	if err := other.Save(); err != nil {
		t.Fatal(err)
	}
	_ = h.Repo.AddField("y")
	if err := h.Save(); !errors.Is(err, ErrChangedOnDisk) {
		t.Fatalf("got %v", err)
	}
	// A second save from the same handle after a successful one works.
	_ = other.Repo.AddField("z")
	if err := other.Save(); err != nil {
		t.Fatalf("consecutive saves: %v", err)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	s := newStore(t)
	h := mustCreateOpen(t, s, "web")
	_ = h.Save()
	entries, _ := os.ReadDir(s.Root)
	if len(entries) != 1 || entries[0].Name() != "web.enc" {
		t.Fatalf("entries: %v", entries)
	}
	fi, _ := os.Stat(filepath.Join(s.Root, "web.enc"))
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}

func TestCopy(t *testing.T) {
	s := newStore(t)
	h := mustCreateOpen(t, s, "a")
	_ = h.Repo.AddField("user")
	_ = h.Repo.SetSecret("gh", map[string]string{"user": "me"})
	_ = h.Save()

	if err := s.Copy("a", "pw", "b", "pw2", false); err != nil {
		t.Fatal(err)
	}
	b, err := s.Open("b", "pw2")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Repo.Fields) != 1 || len(b.Repo.Secrets) != 0 {
		t.Fatalf("copy w/o secrets: %+v", b.Repo)
	}
	if err := s.Copy("a", "pw", "c", "pw3", true); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Open("c", "pw3")
	if c.Repo.Secrets["gh"]["user"] != "me" {
		t.Fatalf("copy with secrets: %+v", c.Repo)
	}
	if err := s.Copy("a", "pw", "b", "pw", false); !errors.Is(err, ErrExists) {
		t.Fatalf("got %v", err)
	}
	if err := s.Copy("a", "bad", "d", "pw", false); !errors.Is(err, crypt.ErrBadPassword) {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidNamesRejected(t *testing.T) {
	s := newStore(t)
	for _, n := range []string{"", "../x", "a/b", ".hidden", `a\b`, ".."} {
		if err := s.Create(n, "pw"); err == nil {
			t.Fatalf("name %q should be rejected", n)
		}
		if _, err := s.Open(n, "pw"); err == nil {
			t.Fatalf("open %q should be rejected", n)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.Root), "x.enc")); err == nil {
		t.Fatal("path traversal wrote a file")
	}
}

func TestListSortedIgnoresOthers(t *testing.T) {
	s := newStore(t)
	names, err := s.List()
	if err != nil || len(names) != 0 {
		t.Fatalf("missing root: %v %v", names, err)
	}
	_ = s.Create("zeta", "pw")
	_ = s.Create("alpha", "pw")
	_ = os.WriteFile(filepath.Join(s.Root, "notes.txt"), []byte("x"), 0o600)
	names, _ = s.List()
	if strings.Join(names, ",") != "alpha,zeta" {
		t.Fatalf("got %v", names)
	}
}

func TestRealOpenSSLRoundTrip(t *testing.T) {
	s := &Store{Root: t.TempDir(), Crypter: crypt.NewOpenSSL()}
	h := mustCreateOpen(t, s, "real")
	_ = h.Repo.AddField("user")
	_ = h.Repo.SetSecret("x", map[string]string{"user": "u"})
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	h2, err := s.Open("real", "pw")
	if err != nil || h2.Repo.Secrets["x"]["user"] != "u" {
		t.Fatalf("%+v %v", h2, err)
	}
}
