package main

import (
	"errors"
	"fmt"
	"maps"

	"github.com/spf13/cobra"

	"github.com/kjhaber/mpm/internal/pwgen"
	"github.com/kjhaber/mpm/internal/vault"
)

func (a *app) pwgenCmd() *cobra.Command {
	var repo, secret, field, symbols, exclude string
	var length int
	var noLower, noUpper, noDigits, noSymbols, allowRepeats, toClip bool

	c := &cobra.Command{
		Use:   "pwgen",
		Short: "Generate a random password (print it, copy it, or store it in a secret's field)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (secret == "") != (field == "") {
				return errors.New("--secret and --field must be given together")
			}
			if secret == "" && repo != "" {
				return errors.New("--repo only applies with --secret and --field")
			}
			if length < 1 {
				return errors.New("--length must be at least 1")
			}
			pw, err := pwgen.Generate(pwgen.Options{
				Length: length, NoLower: noLower, NoUpper: noUpper, NoDigits: noDigits, NoSymbols: noSymbols,
				Symbols: symbols, Exclude: exclude, AllowRepeats: allowRepeats,
			})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if secret != "" {
				err := a.modify(repo, func(r *vault.Repo) error {
					if err := r.ValidateField(field, pw); err != nil {
						return fmt.Errorf("generated password rejected: %w (adjust the options)", err)
					}
					merged := maps.Clone(r.Secrets[secret])
					if merged == nil {
						merged = map[string]string{}
					}
					merged[field] = pw
					return r.SetSecret(secret, merged)
				})
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "Stored generated password in field %q of secret %q.\n", field, secret)
			}
			if toClip {
				if err := a.clip.Write(pw); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(out, "Copied to clipboard.")
			}
			if secret == "" && !toClip {
				_, _ = fmt.Fprintln(out, pw)
			}
			return nil
		},
	}
	f := c.Flags()
	f.IntVar(&length, "length", pwgen.DefaultLength, "password length")
	f.BoolVar(&noLower, "no-lower", false, "leave out lowercase letters")
	f.BoolVar(&noUpper, "no-upper", false, "leave out uppercase letters")
	f.BoolVar(&noDigits, "no-digits", false, "leave out digits")
	f.BoolVar(&noSymbols, "no-symbols", false, "leave out symbols")
	f.StringVar(&symbols, "symbols", "", "symbol characters to use (default "+pwgen.DefaultSymbols+")")
	f.StringVar(&exclude, "exclude", "", "characters never to use, e.g. look-alikes like 0Ol1I")
	f.BoolVar(&allowRepeats, "allow-repeats", false, "allow the same character twice in a row")
	f.BoolVar(&toClip, "copy", false, "copy to the clipboard instead of printing")
	f.StringVar(&repo, "repo", "", "repository name (with --secret and --field)")
	f.StringVar(&secret, "secret", "", "store the password in this secret (created if missing)")
	f.StringVar(&field, "field", "", "field to store the password in")
	return c
}
