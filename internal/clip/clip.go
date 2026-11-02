// Package clip writes to the system clipboard, autodetecting the platform tool.
package clip

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Clipboard reads and writes the system clipboard.
type Clipboard interface {
	Write(text string) error
	Read() (string, error)
}

type cmdClipboard struct {
	writeCmd []string
	readCmd  []string
}

func (c *cmdClipboard) Write(text string) error {
	cmd := exec.Command(c.writeCmd[0], c.writeCmd[1:]...)
	cmd.Stdin = bytes.NewBufferString(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", c.writeCmd[0], err)
	}
	return nil
}

func (c *cmdClipboard) Read() (string, error) {
	out, err := exec.Command(c.readCmd[0], c.readCmd[1:]...).Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", c.readCmd[0], err)
	}
	return string(out), nil
}

type osc52 struct{ out io.Writer }

func (o osc52) Write(text string) error {
	_, err := fmt.Fprintf(o.out, "\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(text)))
	return err
}

func (osc52) Read() (string, error) { return "", errors.New("clipboard cannot be read via OSC52") }

// Detect picks a clipboard for the platform: pbcopy on darwin; wl-copy, xclip
// or xsel on linux; otherwise an OSC52 terminal escape written to osc52Out
// (os.Stdout when nil).
func Detect(lookPath func(string) (string, error), goos string, osc52Out io.Writer) Clipboard {
	has := func(n string) bool { _, err := lookPath(n); return err == nil }
	switch goos {
	case "darwin":
		if has("pbcopy") && has("pbpaste") {
			return &cmdClipboard{[]string{"pbcopy"}, []string{"pbpaste"}}
		}
	case "linux":
		switch {
		case has("wl-copy") && has("wl-paste"):
			return &cmdClipboard{[]string{"wl-copy"}, []string{"wl-paste", "--no-newline"}}
		case has("xclip"):
			return &cmdClipboard{[]string{"xclip", "-selection", "clipboard"}, []string{"xclip", "-selection", "clipboard", "-o"}}
		case has("xsel"):
			return &cmdClipboard{[]string{"xsel", "--clipboard", "--input"}, []string{"xsel", "--clipboard", "--output"}}
		}
	}
	if osc52Out == nil {
		osc52Out = os.Stdout
	}
	return osc52{osc52Out}
}

// ClearIfUnchanged empties the clipboard only if it still holds want.
func ClearIfUnchanged(c Clipboard, want string) error {
	cur, err := c.Read()
	if err != nil {
		return err
	}
	if cur != want {
		return nil
	}
	return c.Write("")
}
