package clip

import (
	"bytes"
	"errors"
	"testing"
)

func lookPathOf(found ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, f := range found {
			if f == name {
				return "/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func toolOf(c Clipboard) string {
	if cmd, ok := c.(*cmdClipboard); ok {
		return cmd.writeCmd[0]
	}
	return "osc52"
}

func TestDetectDarwinPicksPbcopy(t *testing.T) {
	c := Detect(lookPathOf("pbcopy", "pbpaste", "xclip"), "darwin", nil)
	if toolOf(c) != "pbcopy" {
		t.Fatalf("got %s", toolOf(c))
	}
}

func TestDetectLinuxPreference(t *testing.T) {
	if got := toolOf(Detect(lookPathOf("xsel", "xclip", "wl-copy", "wl-paste"), "linux", nil)); got != "wl-copy" {
		t.Fatalf("got %s", got)
	}
	if got := toolOf(Detect(lookPathOf("xsel", "xclip"), "linux", nil)); got != "xclip" {
		t.Fatalf("got %s", got)
	}
	if got := toolOf(Detect(lookPathOf("xsel"), "linux", nil)); got != "xsel" {
		t.Fatalf("got %s", got)
	}
}

func TestDetectFallsBackToOSC52(t *testing.T) {
	var buf bytes.Buffer
	c := Detect(lookPathOf(), "linux", &buf)
	if toolOf(c) != "osc52" {
		t.Fatalf("got %s", toolOf(c))
	}
	if err := c.Write("hi"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "\x1b]52;c;aGk=\x07" {
		t.Fatalf("got %q", buf.String())
	}
	if _, err := c.Read(); err == nil {
		t.Fatal("OSC52 cannot read; expected error")
	}
}

type fake struct{ val string }

func (f *fake) Write(s string) error  { f.val = s; return nil }
func (f *fake) Read() (string, error) { return f.val, nil }

func TestClearIfUnchanged(t *testing.T) {
	f := &fake{val: "secret"}
	if err := ClearIfUnchanged(f, "secret"); err != nil || f.val != "" {
		t.Fatalf("should clear: %q %v", f.val, err)
	}
	f.val = "other"
	if err := ClearIfUnchanged(f, "secret"); err != nil || f.val != "other" {
		t.Fatalf("should leave alone: %q %v", f.val, err)
	}
}
