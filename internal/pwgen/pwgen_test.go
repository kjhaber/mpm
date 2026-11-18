package pwgen

import (
	"errors"
	"strings"
	"testing"
	"unicode"
)

func has(s string, f func(rune) bool) bool { return strings.ContainsFunc(s, f) }

func isSymbol(r rune) bool { return strings.ContainsRune(DefaultSymbols, r) }

func TestDefaultsAreLengthTwentyWithEveryClass(t *testing.T) {
	for range 200 {
		pw, err := Generate(Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len([]rune(pw)) != DefaultLength || DefaultLength != 20 {
			t.Fatalf("len=%d %q", len(pw), pw)
		}
		if !has(pw, unicode.IsLower) || !has(pw, unicode.IsUpper) || !has(pw, unicode.IsDigit) || !has(pw, isSymbol) {
			t.Fatalf("missing a class: %q", pw)
		}
		for _, r := range pw {
			if !unicode.IsLower(r) && !unicode.IsUpper(r) && !unicode.IsDigit(r) && !isSymbol(r) {
				t.Fatalf("unexpected %q in %q", r, pw)
			}
		}
	}
}

func TestNoConsecutiveRepeatsByDefault(t *testing.T) {
	// A tiny pool makes accidental repeats near-certain if they are not prevented.
	for range 200 {
		pw, err := Generate(Options{Length: 30, NoUpper: true, NoDigits: true, NoSymbols: true, Exclude: "cdefghijklmnopqrstuvwxyz"})
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(pw); i++ {
			if pw[i] == pw[i-1] {
				t.Fatalf("repeat in %q", pw)
			}
		}
	}
}

func TestAllowRepeats(t *testing.T) {
	repeated := false
	for range 200 {
		pw, err := Generate(Options{Length: 30, AllowRepeats: true, NoUpper: true, NoDigits: true, NoSymbols: true, Exclude: "cdefghijklmnopqrstuvwxyz"})
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(pw); i++ {
			if pw[i] == pw[i-1] {
				repeated = true
			}
		}
	}
	if !repeated {
		t.Fatal("expected repeats when allowed")
	}
}

func TestClassToggles(t *testing.T) {
	cases := []struct {
		name string
		o    Options
		bad  func(rune) bool
	}{
		{"no lower", Options{NoLower: true}, unicode.IsLower},
		{"no upper", Options{NoUpper: true}, unicode.IsUpper},
		{"no digits", Options{NoDigits: true}, unicode.IsDigit},
		{"no symbols", Options{NoSymbols: true}, isSymbol},
	}
	for _, c := range cases {
		for range 50 {
			pw, err := Generate(c.o)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if has(pw, c.bad) {
				t.Fatalf("%s: %q", c.name, pw)
			}
		}
	}
}

func TestCustomSymbols(t *testing.T) {
	for range 50 {
		pw, err := Generate(Options{Symbols: "~", Length: 12})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(pw, "~") || strings.ContainsAny(pw, "!@#$%^&*-_+=?") {
			t.Fatalf("%q", pw)
		}
	}
}

func TestExclude(t *testing.T) {
	for range 100 {
		pw, err := Generate(Options{Exclude: "0Ol1I!"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(pw, "0Ol1I!") {
			t.Fatalf("%q", pw)
		}
	}
}

func TestLengthEqualToClassCountStillHasEveryClass(t *testing.T) {
	for range 50 {
		pw, err := Generate(Options{Length: 4})
		if err != nil {
			t.Fatal(err)
		}
		if !has(pw, unicode.IsLower) || !has(pw, unicode.IsUpper) || !has(pw, unicode.IsDigit) || !has(pw, isSymbol) {
			t.Fatalf("missing a class: %q", pw)
		}
	}
}

func TestErrors(t *testing.T) {
	cases := map[string]Options{
		"negative length":      {Length: -1},
		"too long":             {Length: MaxLength + 1},
		"shorter than classes": {Length: 3},
		"no classes":           {NoLower: true, NoUpper: true, NoDigits: true, NoSymbols: true},
		"all excluded":         {NoUpper: true, NoDigits: true, NoSymbols: true, Exclude: "abcdefghijklmnopqrstuvwxyz"},
		"single char no repeat": {Length: 5, NoUpper: true, NoDigits: true, NoSymbols: true,
			Exclude: "bcdefghijklmnopqrstuvwxyz"},
	}
	for name, o := range cases {
		if pw, err := Generate(o); err == nil {
			t.Errorf("%s: expected error, got %q", name, pw)
		}
	}
}

func TestSingleCharPoolWorksWhenRepeatsAllowed(t *testing.T) {
	pw, err := Generate(Options{Length: 5, AllowRepeats: true, NoUpper: true, NoDigits: true, NoSymbols: true,
		Exclude: "bcdefghijklmnopqrstuvwxyz"})
	if err != nil || pw != "aaaaa" {
		t.Fatalf("%q %v", pw, err)
	}
}

func TestRandomnessSourceFailureIsReported(t *testing.T) {
	old := reader
	defer func() { reader = old }()
	reader = failingReader{}
	if _, err := Generate(Options{}); err == nil {
		t.Fatal("expected error")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestPasswordsDiffer(t *testing.T) {
	a, _ := Generate(Options{})
	b, _ := Generate(Options{})
	if a == b {
		t.Fatalf("identical: %q", a)
	}
}
