# mpm design spec

Date: 2026-11-01

## Purpose

mpm ("My Password Manager") is a personal, local-only CLI/TUI for storing and retrieving encrypted secrets. It avoids cloud password managers and OS vendor lock-in. It is "good enough to defeat grep", not a high-security vault. Primary platform is macOS; Linux supported.

Success criteria:
- Every command and TUI screen described in README.md works.
- Repository files decrypt with plain `openssl`/`vicrypt` (same parameters).
- `mpm tui` starts fast enough to pop open in a tmux pane (like `tzc`).
- `make all` (fmt, lint, test, build) passes; Homebrew release pipeline mirrors `wn`.

Non-goals (v1): the field "type" attribute, a password-caching daemon, cross-repo secret search (would require every repo password), our own crypto.

## Decisions (agreed with user)

- Language/stack: Go 1.26, cobra (CLI), bubbletea/bubbles/lipgloss (TUI). Module `github.com/kjhaber/mpm`. Layout follows `wn`: `cmd/mpm` + `internal/`.
- Encryption: shell out to `openssl enc -aes-256-cbc -a -salt -pbkdf2 -iter 100000` (identical to `vicrypt`). Password passed via extra pipe (`-pass fd:3`), never argv/env of the child.
- Password: `MPM_PASSWORD` env, else hidden TTY prompt, else error (no TTY). Cached in memory per process (TUI: once per repo until exit). No on-disk/daemon cache.
- Fuzzy search: in-process, no fzf dependency.
- Flags consistent across all subcommands: `--repo`, `--secret`, `--field`, `--attrib`, `--value`, `--format`. README's `--name` forms are accepted as undocumented aliases. `ls`/`rm` alias `list`/`delete`.

## Units

### internal/crypt
`Encrypt(password, plaintext) ([]byte, error)` / `Decrypt(password, ciphertext)`. Interface `Crypter` so other packages can use a fake. Errors: `ErrBadPassword` (wrong password/corrupt), clear message when openssl is missing.

### internal/vault
Model: `Repo{Fields []Field, Secrets map[string]map[string]string}`; `Field{Name, Required, Pattern}` (defaults: required=false, pattern `.+`). Serialized as JSON, encrypted as `<root>/<repo>.enc`.
Rules:
- Repo cannot be deleted while it contains secrets.
- No new required field once the repo has secrets.
- Secret values are validated against field `required`/`pattern` (RE2) on create/update.
- Save is atomic (temp file + rename). Load records mtime; Save refuses to overwrite if the file changed since load.
- `repo copy --from X [--include-secrets]` prompts for source password and a new password.
- `repo create` prompts for the new password twice.

### internal/config
XDG paths: settings `$XDG_CONFIG_HOME/mpm/settings.json` (~/.config/mpm); repos `$XDG_DATA_HOME/mpm` (~/.local/share/mpm; README to document the change from ~/.local/mpm); resume state `$XDG_STATE_HOME/mpm/state.json` (repo/secret names only, never values).
Settings: repository root, default format (`text`), default repository. Every setting overridable by `MPM_*` env (`MPM_ROOT`, `MPM_FORMAT`, `MPM_REPO`, ...); `MPM_PASSWORD` for scripts/tests. Precedence: flag > env > settings.json > default. Env list refined after first cut.

### internal/clip
Interface `Clipboard`. Autodetect: `pbcopy` (macOS), `wl-copy`, `xclip`/`xsel` (Linux), OSC52 fallback. Clears after 30s only if clipboard still holds our value; `--no-clear` disables. CLI `secret copy` spawns a detached hidden `mpm _clipclear` helper to do the delayed compare-and-clear; TUI uses a timer.

### internal/fuzzy
Subsequence match with scoring and match positions; no dependencies.

### internal/tui
Bubbletea with a screen stack: repositories -> secrets -> fields, with the keys in README.md (Enter drill in, Esc back/exit, Q/Ctrl-C quit, N/X/F per screen, `/` filter, C copy, S show, E edit).
- `/` filter: live-updating; Esc clears and exits filter; Enter keeps filter applied. Filters within the current list only (repo screen filters repo names; secret screen filters that repo's secrets).
- `S` reveals the value for a few seconds, then hides it.
- `E` writes `field: value` lines to a 0600 temp file in a 0700 dir, opens `$EDITOR`, validates on exit, saves, removes the temp file (also on signals).
- Password entry via masked textinput.
- `mpm tui [--repo R [--secret S]] [--resume]`.

### cmd/mpm
Thin cobra wiring: `repo`, `field`, `secret`, `tui`, `completion` (zsh/bash/fish), `--version`. Separate `cmd_<feature>.go` per command group. `--value field:value,field:value` with backslash escaping; if omitted on `secret create/update`, prompt per field with hidden input (keeps secrets out of shell history).

## Behavior

- `list`/`show` write to stdout; errors to stderr with non-zero exit.
- `--format text|json`, default `text`. `secret show` without `--field` prints `name: value` lines.
- Decrypted temp files are always removed on exit.

## Testing (red/green TDD, `make all` gate)

- Unit: vault rules, fuzzy, config precedence, `--value` parsing/escaping, field validation.
- `crypt` integration with real openssl, including round trip against a plain `openssl enc -d` invocation (vicrypt compatibility).
- TUI: feed `tea.Msg` into `Update`, assert model state/view.
- CLI end-to-end: run built binary with temp `MPM_ROOT` and `MPM_PASSWORD`.
- Fakes for `Crypter` and `Clipboard` elsewhere.

## Delivery

Makefile (`fmt`, `lint`, `test`, `build`, `all`; outputs in `build/`), `mise.toml`, `.goreleaser.yaml` (darwin/linux, amd64/arm64, completions, brew formula to `kjhaber/homebrew-tap`), GHA `ci.yml` and `release.yml`, `CLAUDE.md` mirroring `wn` conventions, README updated with final flags, XDG paths, env vars, and a tmux binding example (`bind-key M split-window -h 'mpm tui'`).
