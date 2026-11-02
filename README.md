# mpm
My Password Manager

A simple CLI/TUI tool for storing and retrieving encrypted secrets.  I don't want to use a centralized cloud-based password manager, and I don't want my passwords/secrets to be vendor locked to my laptop or its OS (Apple macOS in my case).  This tool uses openssl AES-256 encryption with a password on local files, so it's basically good enough to defeat grep, avoid vendor lock-in, and not keep files in a plain text file.  It also makes it slightly easier to copy secrets to clipboard for form inputs without displaying them on screen.  In other words, there are more secure solutions out there, but this meets my personal preferences for security vs vendor lock-in tradeoffs.

## Installation
```
brew install kjhaber/tap/mpm
```

`openssl` must be available on the machine.

## Storage and encryption
Each secret repository is one file, `<root>/<repo-name>.enc`.  The file is the base64 output of:

```
openssl enc -aes-256-cbc -a -salt -pbkdf2 -iter 100000
```

over a JSON document holding the repository's fields and secrets.  These are the same parameters used by `vicrypt`, so repository files can be opened without mpm:

```
openssl enc -d -aes-256-cbc -a -salt -pbkdf2 -iter 100000 -in ~/.local/share/mpm/<repo-name>.enc
```

(or with `vicrypt <file>`).  mpm saves by writing a temp file and renaming it over the original (if a repository file is a symlink, the link is kept and its target is updated), and refuses to overwrite a file that changed on disk since it was read.  Saves take a per-repository lock (a hidden `.<repo-name>.lock` file next to it), so two mpm processes saving at once cannot silently overwrite each other: the second one fails with a changed-on-disk error.

### Passwords
mpm reads a repository's password from the `MPM_PASSWORD` environment variable if set (useful for scripts and tests), otherwise from a hidden prompt on the terminal.  It errors if neither is available.  Passwords are never accepted as command line arguments.  `MPM_PASSWORD` is not passed to child processes (openssl gets the password on a private pipe; editors, clipboard tools and helpers never see it).  A password is cached in memory for the life of the process only (the TUI asks once per repository until it exits).

### Locations
Paths follow the XDG base directory spec:

| Purpose | Default | Override |
|---|---|---|
| Settings | `$XDG_CONFIG_HOME/mpm/settings.json` (`~/.config/mpm/settings.json`) | `XDG_CONFIG_HOME` |
| Repositories | `$XDG_DATA_HOME/mpm` (`~/.local/share/mpm`) | `MPM_ROOT` |
| `--resume` state | `$XDG_STATE_HOME/mpm/state.json` (`~/.local/state/mpm/state.json`) | `XDG_STATE_HOME` |

Resume state holds only the repository and secret names last selected, never secret values.

## Usage

For all commands, `ls` is an alias for `list` and `rm` is an alias for `delete`.

Flag names are the same across all subcommands: `--repo`, `--secret`, `--field`, `--attrib`, `--value`, `--format`.

Errors go to stderr with a non-zero exit code.  `mpm --version` prints the version, and `mpm completion zsh|bash|fish` prints a shell completion script.

### Secret repositories
Secret repositories store a set of secrets.  Repositories include a set of fields to be tracked per secret.  Each secret repository has a password that is required to access its secrets.  For safety, secret repositories cannot be deleted if they contain any secrets (user must delete all secrets first).

```
mpm repo list
mpm repo create --repo <repo-name>
mpm repo show --repo <repo-name>
mpm repo delete --repo <repo-name>
mpm repo copy --repo <repo-name> --from <original-repo-name> [--include-secrets]
```

`repo create` prompts for the new password twice.  `repo copy` prompts for the original repository's password and then a new password for the copy.

### Fields
Secret repositories include a set of fields to be tracked per secret.  For example, a website password repository might have fields for username and password, while a credit card repository might have fields for name-on-card, card-number, expire-month, expire-year, and cvv.  Fields can be set as required or optional, and may have validation rules applied to help ensure values are set correctly.  If a repository contains any secrets, no new required fields can be added to the repository.  Likewise a `pattern` change is refused while any existing secret value would not match it; the error names those secrets (never their values) so they can be fixed first.

```
mpm field list --repo <repo-name>
mpm field create --repo <repo-name> --field <field-name>
mpm field show --repo <repo-name> --field <field-name>
mpm field list-attribs --repo <repo-name> --field <field-name>
mpm field get-attrib --repo <repo-name> --field <field-name> --attrib <attribute-name>
mpm field set-attrib --repo <repo-name> --field <field-name> --attrib <attribute-name> --value <attribute-value>
mpm field delete-attrib --repo <repo-name> --field <field-name> --attrib <attribute-name>
mpm field delete --repo <repo-name> --field <field-name>
```

Valid attribute names:
* `required`. Valid values are "true" or "false" (case-sensitive). Default is false.
* `pattern`. Regex pattern (RE2) that secret field value must match.  Default is `.+` (match any non-empty string).

(Later mpm may support a "type" attribute with some pre-defined regex patterns, e.g. "numeric" for `[0-9]+`.)

### Secrets
Once a secret repository is created, secrets can be stored, retrieved, updated and removed.

`mpm secret show` returns the value to stdout, while `mpm secret copy` reads the value to clipboard.  If `--field` is omitted, all fields are returned according to the requested `--format` setting. Default format is 'text' which outputs each value on a separate line with "name: value" - this will work best for secret values that have no newlines.

```
mpm secret list --repo <repo-name>
mpm secret create --repo <repo-name> --secret <secret-name> [--value <field:value,field:value,...>]
mpm secret update --repo <repo-name> --secret <secret-name> [--value <field:value,field:value,...>]
mpm secret show --repo <repo-name> --secret <secret-name> [--field <field-name>] [--format text|json]
mpm secret copy --repo <repo-name> --secret <secret-name> [--field <field-name>] [--format text|json]
mpm secret delete --repo <repo-name> --secret <secret-name>
```

Secret names may contain any characters except control characters (newlines, escapes and the like).  Names created by older versions that contain control characters are shown with `?` in their place.

`secret update` merges: fields not listed in `--value` keep their current values (when prompting, a blank answer keeps the current value).

In `--value`, a literal comma, colon or backslash in a value can be escaped with a backslash (a backslash before any other character is kept as written).  If `--value` is omitted on `create` or `update`, mpm prompts for each field with hidden input, which keeps secrets out of shell history.  Values are validated against each field's `required` and `pattern` attributes.

`secret copy` uses the platform clipboard (`pbcopy` on macOS; `wl-copy`, `xclip` or `xsel` on Linux; OSC52 terminal escape as a last resort).  mpm never clears or reads the clipboard (clearing risks wiping something else you copied meanwhile), so a copied secret stays there until you replace it.  OSC52 is written to the terminal, never to redirected stdout.

## TUI mode
Running `mpm tui` runs mpm in a TUI mode.

TUI interface has following "screens":
* Repository selection: lists repositories
  - <Enter> to select repository
  - <ESC> to exit TUI
  - "N" to create new repository
  - "X" to delete repository (if no secrets in repository)
  - "F" to enter repository fields to show/modify repository's fields (on that screen: "N" add field, "X" delete field, "R" toggle required, "P" set regex pattern)
  - "/" to filter list (fuzzy search)
  - "Q" or <Ctrl-C> to exit TUI

* Secret selection: lists secrets within selected repository
  - <Enter> to select secret
  - <ESC> to return to repository selection
  - "N" to create new secret
  - "X" to delete secret
  - "/" to filter list (fuzzy search)
  - "Q" or <Ctrl-C> to exit TUI

* Secret field selection: lists fields within selected secret
  - <Enter> to select field
  - <ESC> to return to Secret selection
  - "C" to copy selected field value to clipboard
  - <Enter> or "S" to show selected field value on screen (it is hidden again after a few seconds)
  - "E" to edit secret in `$EDITOR` (`$VISUAL` is tried first; vim and nvim are started without swap, backup, undo or viminfo files so the decrypted text is not left behind (other editors are launched as configured, so disable their history files yourself); arguments may be quoted like in a shell, e.g. `EDITOR="code --wait"`; the secret is written as `field: value` lines to a private temp file that is removed afterward; values are validated on save; a backslash or newline inside a value appears as `\\` or `\n`)
  - "Q" or <Ctrl-C> to exit TUI

In general <Enter> key drills down toward secret value, and <ESC> key steps back out a level.  Arrows move up and down the list.  Pressing <ESC> enough times will exit out of the program.  "Q" and <Ctrl-C> will immediately exit from the program.

"/" key shows a search filter text box (similar to vim search command).  The list of matches live-updates while typing in the filter.  <ESC> from the filter control clears the filter and exits back to normal view for the screen.  <Enter> from the filter control accepts the filter, leaving it applied, and exits back to normal view for the screen.  While a filter is applied, <ESC> clears it before stepping back.  Fuzzy matching is built in (no fzf needed).  The filter applies to the list on the current screen only: secret names live inside each encrypted repository, so searching across all repositories would require every repository's password.

Command line args to `mpm tui` can start the TUI mode with some values pre-selected.
* `mpm tui`: start at repository selection (lists repositories)
* `mpm tui --repo <repo-name>`: start at secret selection (lists secrets in repository)
* `mpm tui --repo <repo-name> --secret <secret-name>`: start at secret field selection (list fields in secret)
* `mpm tui --resume`: start at same screen/secret that was selected when mpm tui was previously exited (cannot be combined with `--repo`/`--secret`)

### tmux
To pop the TUI open in a new pane for quick access to secrets, add to `tmux.conf`:

```
bind-key M split-window -h 'mpm tui'
```

## Settings
Written to `$XDG_CONFIG_HOME/mpm/settings.json` (`~/.config/mpm/settings.json`) by default.

| Setting | Default | Env override |
|---|---|---|
| Repository root location | `~/.local/share/mpm` | `MPM_ROOT` |
| Default format | `text` | `MPM_FORMAT` |
| Default repository name | (none) | `MPM_REPO` |

Precedence is command line flag, then environment variable, then settings.json, then default.

## License
MIT
