package childenv

import (
	"strings"
	"testing"
)

func TestEnvStripsPassword(t *testing.T) {
	t.Setenv("MPM_PASSWORD", "hunter2")
	t.Setenv("MPM_ROOT", "/keep")
	var sawRoot bool
	for _, kv := range Env() {
		if strings.HasPrefix(kv, "MPM_PASSWORD=") {
			t.Fatal("MPM_PASSWORD leaked into child environment")
		}
		if kv == "MPM_ROOT=/keep" {
			sawRoot = true
		}
	}
	if !sawRoot {
		t.Fatal("other variables must be kept")
	}
}
