package cli

import (
	"reflect"
	"testing"
)

func TestSplitWords(t *testing.T) {
	for in, want := range map[string][]string{
		`vim`:                         {"vim"},
		`  code   --wait  `:           {"code", "--wait"},
		`emacsclient -a "" -c`:        {"emacsclient", "-a", "", "-c"},
		`"/Applications/My Ed/ed" -w`: {"/Applications/My Ed/ed", "-w"},
		`ed --cmd 'set x=1 y=2'`:      {"ed", "--cmd", "set x=1 y=2"},
		`ed My\ File`:                 {"ed", "My File"},
		`ed "say \"hi\""`:             {"ed", `say "hi"`},
		`ed 'it'\''s'`:                {"ed", "it's"},
		`ed a"b c"d`:                  {"ed", "ab cd"},
		``:                            nil,
	} {
		got, err := SplitWords(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %q, %v want %q", in, got, err, want)
		}
	}
}

func TestSplitWordsErrors(t *testing.T) {
	for _, in := range []string{`ed "open`, `ed 'open`, `ed trailing\`} {
		if _, err := SplitWords(in); err == nil {
			t.Errorf("%q should error", in)
		}
	}
}
