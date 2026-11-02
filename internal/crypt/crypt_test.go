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

func TestCorruptCiphertextReturnsErrBadPassword(t *testing.T) {
	c := NewOpenSSL()
	if _, err := c.Decrypt("pw", []byte("not ciphertext at all")); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("got %v", err)
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
