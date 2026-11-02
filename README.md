# mpm
My Password Manager

A simple CLI/TUI tool for storing and retrieving encrypted secrets.  I don't want to use a centralized cloud-based password manager, and I don't want my passwords/secrets to be vendor locked to my laptop or its OS (Apple macOS in my case).  This tool uses openssl AES-256 encryption with a password on local files, so it's basically good enough to defeat grep, avoid vendor lock-in, and not keep files in a plain text file.  It also makes it slightly easier to copy secrets to clipboard for form inputs without displaying them on screen.  In other words, there are more secure solutions out there, but this meets my personal preferences for security vs vendor lock-in tradeoffs.

## Installation
```
brew install kjhaber/tap/mpm
```

If desired, set environment variables to override default storage locations:


## Usage

(For all commands, we can alias "ls" for "list" and "rm" for "delete").

### Secret repositories
Secret repositories store a set of secrets.  Repositories include a set of fields to be tracked per secret.  Each secret repository has a password that is required to access its secrets.  For safety, secret repositories cannot be deleted if they contain any secrets (user must delete all secrets first).

```
mpm repo list
mpm repo create --name <repo-name>
mpm repo show --name <repo-name>
mpm repo delete --name <repo-name>
mpm repo copy --name <repo-name> --from <original-repo-name> [--include-secrets]
```


### Fields
Secret repositories include a set of fields to be tracked per secret.  For example, a website password repository might have fields for username and password, while a credit card repository might have fields for name-on-card, card-number, expire-month, expire-year, and cvv.  Fields can be set as required or optional, and may have validation rules applied to help ensure values are set correctly.  If a repository contains any secrets, no new required fields can be added to the repository.

```
mpm field list --name <repo-name>
mpm field create --name <repo-name> --name <field-name>
mpm field show --name <repo-name> --name <field-name>
mpm field list-attribs --name <repo-name> --name <field-name>
mpm field get-attrib --name <repo-name> --name <field-name> --attrib <attribute-name>
mpm field set-attrib --name <repo-name> --name <field-name> --attrib <attribute-name> --value <attribute-value>
mpm field delete-attrib --name <repo-name> --name <field-name> --attrib <attribute-name>
mpm field delete --name <repo-name> --name <field-name>
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
mpm secret create --repo <repo-name> --name <secret-name> --value <field:value,field:value,...>
mpm secret update --repo <repo-name> --name <secret-name> --value <field:value,field:value,...>
mpm secret show --repo <repo-name> --name <secret-name> [--field <field-name>] [--format text|json]
mpm secret copy --repo <repo-name> --name <secret-name> [--field <field-name>] [--format text|json]
mpm secret delete --repo <repo-name> --name <secret-name>
```

## TUI mode
Running `mpm tui` runs mpm in a TUI mode.

TUI interface has following "screens":
* Repository selection: lists repositories
  - <Enter> to select repository
  - <ESC> to exit TUI
  - "N" to create new repository
  - "X" to delete repository (if no secrets in repository)
  - "F" to enter repository fields to show/modify repository's fields
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
  - "S" to show selected field value on screen
  - "E" to edit secret in editor
  - "Q" or <Ctrl-C> to exit TUI



In general <Enter> key drills down toward secret value, and <ESC> key steps back out a level.  Arrows move up and down the list.  Pressing <ESC> enough times will exit out of the program.  "Q" and <Ctrl-C> will immediately exit from the program.

"/" key shows a search filter text box (similar to vim search command).  The list of matches should live-update while typing in the filter.  <ESC> from the filter control should clear the filter and exit back to normal view for the screen.  <Enter> from the filter control should accept the filter leaving it applied and exit back to normal view for the screen.  (Maybe this can be a fuzzy search across all repos and secrets?  Or search down from currently selected location: at repository selection, "/" search can help jump straight to a specific secret right away.)

Command line args to `mpm tui` can start the TUI mode with some values pre-selected.
* `mpm tui`: start at repository selection (lists repositories)
* `mpm tui --repo <repo-name>`: start at secret selection (lists secrets in repository)
* `mpm tui --repo <repo-name> --secret <secret-name>`: start at secret field selection (list fields in secret)
* `mpm tui --resume`: start at same screen/secret that was selected when mpm tui was previously exited

## Settings
Written to ~/.config/mpm/settings.json by default.

- Repository root location (~/.local/mpm by default)
- Default format (text by default)
- Default repository name


## License
MIT

