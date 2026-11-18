// Package pwgen generates random passwords.
package pwgen

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
)

const (
	DefaultLength  = 20
	MaxLength      = 1024
	DefaultSymbols = "!@#$%^&*-_+=?"

	lower  = "abcdefghijklmnopqrstuvwxyz"
	upper  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits = "0123456789"

	maxAttempts = 10000
)

// reader is the randomness source; replaced in tests.
var reader io.Reader = rand.Reader

// Options control the generated password. The zero value gives a
// DefaultLength password with lowercase, uppercase, digits and DefaultSymbols,
// at least one of each, and no character repeated back to back.
type Options struct {
	Length                                int    // 0 means DefaultLength
	NoLower, NoUpper, NoDigits, NoSymbols bool   // leave a class out
	Symbols                               string // symbol set; "" means DefaultSymbols
	Exclude                               string // characters never to use (e.g. look-alikes)
	AllowRepeats                          bool   // permit the same character twice in a row
}

// Generate returns a new random password.
func Generate(o Options) (string, error) {
	length := o.Length
	if length == 0 {
		length = DefaultLength
	}
	if length < 0 || length > MaxLength {
		return "", fmt.Errorf("length must be between 1 and %d", MaxLength)
	}
	symbols := o.Symbols
	if symbols == "" {
		symbols = DefaultSymbols
	}
	var classes [][]rune
	add := func(skip bool, set string) {
		if skip {
			return
		}
		var rs []rune
		for _, r := range set {
			if !strings.ContainsRune(o.Exclude, r) {
				rs = append(rs, r)
			}
		}
		if len(rs) > 0 {
			classes = append(classes, rs)
		}
	}
	add(o.NoLower, lower)
	add(o.NoUpper, upper)
	add(o.NoDigits, digits)
	add(o.NoSymbols, symbols)
	if len(classes) == 0 {
		return "", errors.New("no characters left to choose from")
	}
	if length < len(classes) {
		return "", fmt.Errorf("length %d is too short to include every character class (need at least %d)", length, len(classes))
	}
	var pool []rune
	for _, c := range classes {
		pool = append(pool, c...)
	}
	if !o.AllowRepeats && len(pool) < 2 && length > 1 {
		return "", errors.New("only one character available; cannot avoid repeats")
	}

	for range maxAttempts {
		pw, err := draw(pool, length, o.AllowRepeats)
		if err != nil {
			return "", err
		}
		if coversAll(pw, classes) {
			return string(pw), nil
		}
	}
	return "", errors.New("could not generate a password satisfying the options")
}

// draw picks length characters from pool, never the same one twice in a row
// unless repeats are allowed.
func draw(pool []rune, length int, repeats bool) ([]rune, error) {
	out := make([]rune, 0, length)
	for len(out) < length {
		i, err := rand.Int(reader, big.NewInt(int64(len(pool))))
		if err != nil {
			return nil, fmt.Errorf("random source: %w", err)
		}
		r := pool[i.Int64()]
		if !repeats && len(out) > 0 && out[len(out)-1] == r {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func coversAll(pw []rune, classes [][]rune) bool {
	for _, c := range classes {
		found := false
		for _, r := range pw {
			if strings.ContainsRune(string(c), r) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
