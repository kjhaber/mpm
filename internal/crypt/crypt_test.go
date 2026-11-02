package crypt

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	c := NewOpenSSL()
	for _, in := range []string{"", "hello", "line1\nline2\n", "ünïcode 🔐"} {
		enc, err := c.Encrypt("pw", []byte(in))
		if err != nil {
			t.Fatal(err)
		}
		dec, err := c.Decrypt("pw", enc)
		if err != nil {
			t.Fatal(err)
		}
		if string(dec) != in {
			t.Fatalf("got %q want %q", dec, in)
		}
	}
}

func TestWrongPasswordReturnsErrBadPassword(t *testing.T) {
	c := NewOpenSSL()
	enc, _ := c.Encrypt("pw", []byte("secret data"))
	if _, err := c.Decrypt("nope", enc); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("got %v", err)
	}
}

func TestCorruptCiphertextIsNotReportedAsBadPassword(t *testing.T) {
	c := NewOpenSSL()
	_, err := c.Decrypt("pw", []byte("not ciphertext at all"))
	if err == nil || errors.Is(err, ErrBadPassword) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "error reading input file") {
		t.Fatalf("error should carry openssl's message, got %v", err)
	}
}

func TestOpenSSLFailureShowsStderr(t *testing.T) {
	dir := t.TempDir()
	shim := "#!/bin/sh\necho 'enc: Unknown option: -pbkdf2' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "openssl"), []byte(shim), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for name, fn := range map[string]func() error{
		"encrypt": func() error { _, err := NewOpenSSL().Encrypt("pw", []byte("x")); return err },
		"decrypt": func() error { _, err := NewOpenSSL().Decrypt("pw", []byte("x")); return err },
	} {
		err := fn()
		if err == nil || errors.Is(err, ErrBadPassword) || !strings.Contains(err.Error(), "Unknown option: -pbkdf2") {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestInvalidPasswordsRejected(t *testing.T) {
	c := NewOpenSSL()
	for name, pw := range map[string]string{
		"newline":  "ab\ncd",
		"too long": strings.Repeat("x", 1024),
		"huge":     strings.Repeat("x", 200000),
	} {
		if _, err := c.Encrypt(pw, []byte("data")); !errors.Is(err, ErrInvalidPassword) {
			t.Errorf("%s: encrypt got %v", name, err)
		}
		if _, err := c.Decrypt(pw, []byte("data")); !errors.Is(err, ErrInvalidPassword) {
			t.Errorf("%s: decrypt got %v", name, err)
		}
	}
}

func TestMaxLengthPasswordRoundTrips(t *testing.T) {
	c := NewOpenSSL()
	pw := strings.Repeat("x", 1023)
	enc, err := c.Encrypt(pw, []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Decrypt(pw[:1022]+"y", enc); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("last byte must matter, got %v", err)
	}
}

var plainArgs = []string{"enc", "-aes-256-cbc", "-a", "-salt", "-pbkdf2", "-iter", "100000", "-pass", "pass:pw"}

func TestVicryptCompatible(t *testing.T) {
	c := NewOpenSSL()
	enc, err := c.Encrypt("pw", []byte("compat"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("openssl", append([]string{"enc", "-d"}, plainArgs[1:]...)...)
	cmd.Stdin = bytes.NewReader(enc)
	out, err := cmd.Output()
	if err != nil || string(out) != "compat" {
		t.Fatalf("plain openssl decrypt: %q %v", out, err)
	}

	cmd = exec.Command("openssl", plainArgs...)
	cmd.Stdin = strings.NewReader("from-openssl")
	blob, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	dec, err := c.Decrypt("pw", blob)
	if err != nil || string(dec) != "from-openssl" {
		t.Fatalf("Decrypt of plain openssl blob: %q %v", dec, err)
	}
}

func TestOpenSSLMissing(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := NewOpenSSL().Encrypt("pw", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "openssl") {
		t.Fatalf("got %v", err)
	}
}

func TestOpenSSLChildDoesNotInheritPassword(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "env.txt")
	shim := "#!/bin/sh\nenv > " + dump + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "openssl"), []byte(shim), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("MPM_PASSWORD", "hunter2")
	_, _ = NewOpenSSL().Encrypt("pw", []byte("x"))
	b, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "MPM_PASSWORD") {
		t.Fatal("openssl child inherited MPM_PASSWORD")
	}
}
