package clip

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
}

func TestClipboardChildDoesNotInheritPassword(t *testing.T) {
	dump := filepath.Join(t.TempDir(), "env.txt")
	t.Setenv("MPM_PASSWORD", "hunter2")
	c := &cmdClipboard{writeCmd: []string{"sh", "-c", "env > " + dump}}
	if err := c.Write("x"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dump)
	if strings.Contains(string(b), "MPM_PASSWORD") {
		t.Fatal("clipboard child inherited MPM_PASSWORD")
	}
}

func TestDetectNeedsOnlyTheCopyTool(t *testing.T) {
	if got := toolOf(Detect(lookPathOf("pbcopy"), "darwin", nil)); got != "pbcopy" {
		t.Fatalf("darwin: got %s", got)
	}
	if got := toolOf(Detect(lookPathOf("wl-copy"), "linux", nil)); got != "wl-copy" {
		t.Fatalf("linux: got %s", got)
	}
}
