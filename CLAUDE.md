# Project overview

`mpm` ("My Password Manager") is a Go CLI/TUI (cobra, bubbletea) that stores secrets in openssl-encrypted repository files. See README.md for the user contract and `docs/superpowers/specs/2026-11-01-mpm-design.md` for the design.

# Build and verify

Always use `make build` (outputs `./build/mpm`); never output binaries elsewhere. Before reporting a code change complete you **must** run `make all` (fmt, lint, coverage, build) and it must succeed.

# Conventions

- Logic lives in `internal/<pkg>`; `cmd/mpm/cmd_<group>.go` holds only cobra wiring.
- Flag names are consistent across subcommands: `--repo --secret --field --attrib --value --format`.
- Never pass passwords on argv or env to child processes; openssl gets the password on fd 3.
- Update README.md whenever commands or flags change.
- Red/green TDD: write the failing test first.
