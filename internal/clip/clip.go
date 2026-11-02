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

	"github.com/kjhaber/mpm/internal/childenv"
)

// Clipboard writes to the system clipboard. mpm never reads or clears it: that
// risks wiping something else the user copied in the meantime.
type Clipboard interface {
	Write(text string) error
}

type cmdClipboard struct {
	writeCmd []string
}

func (c *cmdClipboard) Write(text string) error {
	cmd := exec.Command(c.writeCmd[0], c.writeCmd[1:]...)
	cmd.Env = childenv.Env()
	cmd.Stdin = bytes.NewBufferString(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", c.writeCmd[0], err)
	}
	return nil
}

type osc52 struct{ out io.Writer }

func (o osc52) Write(text string) error {
	_, err := fmt.Fprintf(o.out, "\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(text)))
	return err
}

// Detect picks a clipboard for the platform: pbcopy on darwin; wl-copy, xclip
// or xsel on linux; otherwise an OSC52 terminal escape written to osc52Out
// (the controlling terminal when nil).
func Detect(lookPath func(string) (string, error), goos string, osc52Out io.Writer) Clipboard {
	has := func(n string) bool { _, err := lookPath(n); return err == nil }
	switch goos {
	case "darwin":
		if has("pbcopy") {
			return &cmdClipboard{[]string{"pbcopy"}}
		}
	case "linux":
		switch {
		case has("wl-copy"):
			return &cmdClipboard{[]string{"wl-copy"}}
		case has("xclip"):
			return &cmdClipboard{[]string{"xclip", "-selection", "clipboard"}}
		case has("xsel"):
			return &cmdClipboard{[]string{"xsel", "--clipboard", "--input"}}
		}
	}
	if osc52Out == nil {
		osc52Out = ttyWriter{}
	}
	return osc52{osc52Out}
}

// ttyWriter writes to the controlling terminal so OSC52 never lands in a
// redirected stdout.
type ttyWriter struct{}

func (ttyWriter) Write(p []byte) (int, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return 0, errors.New("no terminal available for OSC52 clipboard")
	}
	defer func() { _ = tty.Close() }()
	return tty.Write(p)
}
