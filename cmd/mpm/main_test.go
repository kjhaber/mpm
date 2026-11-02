package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCmd(&app{})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "mpm version dev" {
		t.Fatalf("got %q", got)
	}
}
