// Package childenv builds the environment handed to child processes.
package childenv

import (
	"os"
	"strings"
)

// Env returns the current environment without MPM_PASSWORD, so editors,
// clipboard tools and helpers never see the repository password.
func Env() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "MPM_PASSWORD=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
