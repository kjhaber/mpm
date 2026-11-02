// Package crypt encrypts and decrypts data by shelling out to openssl using
// the same parameters as the vicrypt script, so files are readable without mpm.
package crypt

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/kjhaber/mpm/internal/childenv"
)

// ErrBadPassword is returned when decryption fails (wrong password or corrupt data).
var ErrBadPassword = errors.New("wrong password or corrupt data")

// Crypter encrypts and decrypts byte slices with a password.
type Crypter interface {
	Encrypt(password string, plaintext []byte) ([]byte, error)
	Decrypt(password string, ciphertext []byte) ([]byte, error)
}

type opensslCrypter struct{}

// NewOpenSSL returns a Crypter backed by the openssl binary on PATH.
func NewOpenSSL() Crypter { return opensslCrypter{} }

func (opensslCrypter) Encrypt(password string, plaintext []byte) ([]byte, error) {
	out, err := run(password, plaintext, false)
	if err != nil {
		return nil, fmt.Errorf("encrypting: %w", err)
	}
	return out, nil
}

func (opensslCrypter) Decrypt(password string, ciphertext []byte) ([]byte, error) {
	out, err := run(password, ciphertext, true)
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, ErrBadPassword
		}
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	return out, nil
}

func run(password string, in []byte, decrypt bool) ([]byte, error) {
	path, err := exec.LookPath("openssl")
	if err != nil {
		return nil, errors.New("openssl not found on PATH (brew install openssl)")
	}
	args := []string{"enc"}
	if decrypt {
		args = append(args, "-d")
	}
	args = append(args, "-aes-256-cbc", "-a", "-salt", "-pbkdf2", "-iter", "100000", "-pass", "fd:3")

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer func() { _ = pr.Close() }()

	cmd := exec.Command(path, args...)
	cmd.Env = childenv.Env()
	cmd.Stdin = bytes.NewReader(in)
	cmd.ExtraFiles = []*os.File{pr}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		return nil, err
	}
	// The password is small enough to fit in the pipe buffer, so this won't block.
	_, werr := pw.WriteString(password + "\n")
	_ = pw.Close()
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	if werr != nil {
		return nil, werr
	}
	return stdout.Bytes(), nil
}
