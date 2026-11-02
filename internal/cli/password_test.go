package cli

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestGetUsesEnvFirst(t *testing.T) {
	p := &PasswordSource{Env: envOf(map[string]string{"MPM_PASSWORD": "e"}),
		Prompt: func(string) (string, error) { t.Fatal("should not prompt"); return "", nil }}
	if got, err := p.Get("r"); err != nil || got != "e" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestGetCachesPerRepo(t *testing.T) {
	calls := 0
	p := &PasswordSource{Env: envOf(nil), Prompt: func(string) (string, error) { calls++; return "pw", nil }}
	_, _ = p.Get("a")
	_, _ = p.Get("a")
	_, _ = p.Get("b")
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestGetWithoutPromptErrors(t *testing.T) {
	p := &PasswordSource{Env: envOf(nil)}
	if _, err := p.Get("r"); err == nil {
		t.Fatal("expected error")
	}
}

func TestInvalidateForcesReprompt(t *testing.T) {
	calls := 0
	p := &PasswordSource{Env: envOf(nil), Prompt: func(string) (string, error) { calls++; return "pw", nil }}
	_, _ = p.Get("a")
	p.Forget("a")
	_, _ = p.Get("a")
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestGetNewMismatch(t *testing.T) {
	answers := []string{"one", "two"}
	p := &PasswordSource{Env: envOf(nil), Prompt: func(string) (string, error) {
		a := answers[0]
		answers = answers[1:]
		return a, nil
	}}
	if _, err := p.GetNew("r"); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestGetNewMatchAndEnv(t *testing.T) {
	p := &PasswordSource{Env: envOf(nil), Prompt: func(string) (string, error) { return "same", nil }}
	if got, err := p.GetNew("r"); err != nil || got != "same" {
		t.Fatalf("%q %v", got, err)
	}
	p = &PasswordSource{Env: envOf(map[string]string{"MPM_PASSWORD": "e"}), Prompt: func(string) (string, error) { return "", errors.New("no") }}
	if got, err := p.GetNew("r"); err != nil || got != "e" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestGuardTerminalRestoresThenReraisesOnInterrupt(t *testing.T) {
	restored := make(chan struct{})
	reraised := make(chan os.Signal, 1)
	stop := guardTerminal(func() { close(restored) }, func(s os.Signal) { reraised <- s })
	defer stop()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-restored:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal was not restored")
	}
	select {
	case s := <-reraised:
		if s != syscall.SIGINT {
			t.Fatalf("reraised %v", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("signal was not re-raised")
	}
}

func TestGuardTerminalStopDisarms(t *testing.T) {
	stop := guardTerminal(func() { t.Error("restore ran after stop") }, func(os.Signal) { t.Error("reraised after stop") })
	stop()
	time.Sleep(50 * time.Millisecond)
}
