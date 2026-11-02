package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseValue(t *testing.T) {
	got, err := ParseValue(`user:me,pass:a\,b\:c\\d`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"user": "me", "pass": `a,b:c\d`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestParseValueAllowsNewlineAndBareColon(t *testing.T) {
	got, err := ParseValue("note:line1\nline2,url:http://x")
	if err != nil {
		t.Fatal(err)
	}
	if got["note"] != "line1\nline2" || got["url"] != "http://x" {
		t.Fatalf("got %q", got)
	}
}

func TestParseValueErrors(t *testing.T) {
	for _, in := range []string{"nocolon", ":v", "a:1,a:2", "a:1,,b:2", `a:trailing\`} {
		if _, err := ParseValue(in); err == nil {
			t.Fatalf("%q should error", in)
		}
	}
}

func TestFormatValueRoundTrip(t *testing.T) {
	in := map[string]string{"b": "x,y", "a": `p:q\r`, "c": "l1\nl2"}
	s := FormatValue(in)
	if s[:2] != "a:" {
		t.Fatalf("keys should be sorted: %q", s)
	}
	out, err := ParseValue(s)
	if err != nil || !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip: %v %v", out, err)
	}
}

func TestParseValueKeepsBackslashBeforeOtherCharacters(t *testing.T) {
	got, err := ParseValue(`pw:a\b\nc\\d,path:C:\temp`)
	if err != nil {
		t.Fatal(err)
	}
	if got["pw"] != `a\b\nc\d` || got["path"] != `C:\temp` {
		t.Fatalf("got %q", got)
	}
}

func TestParseValueErrorsDoNotEchoValues(t *testing.T) {
	for _, in := range []string{"hunter2", "user:me,hunter2", ":hunter2", "user:me,:hunter2,x:y", `user:hunter2\`} {
		_, err := ParseValue(in)
		if err == nil {
			t.Fatalf("%q should error", in)
		}
		if strings.Contains(err.Error(), "hunter2") {
			t.Errorf("%q: error echoes the value: %v", in, err)
		}
	}
	if _, err := ParseValue("user:me,hunter2"); err == nil || !strings.Contains(err.Error(), "entry 2") {
		t.Errorf("error should name the entry position, got %v", err)
	}
}
