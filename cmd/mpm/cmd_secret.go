package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kjhaber/mpm/internal/cli"
	"github.com/kjhaber/mpm/internal/vault"
)

// renderSecret formats a secret (or one field) without a trailing newline.
func renderSecret(r *vault.Repo, secret, field, format string) (string, error) {
	vals, ok := r.Secrets[secret]
	if !ok {
		return "", fmt.Errorf("secret %q: %w", secret, vault.ErrNotFound)
	}
	if field != "" {
		if _, err := r.GetAttrib(field, "required"); err != nil {
			return "", err
		}
		v, ok := vals[field]
		if !ok {
			return "", fmt.Errorf("secret %q has no value for field %q", secret, field)
		}
		if format == "json" {
			return jsonString(map[string]string{field: v})
		}
		return v, nil
	}
	if format == "json" {
		return jsonString(vals)
	}
	var lines []string
	for _, f := range r.Fields {
		if v, ok := vals[f.Name]; ok {
			lines = append(lines, f.Name+": "+v)
		}
	}
	return strings.Join(lines, "\n"), nil
}

// promptValues asks for each field's value with hidden input; blank answers are skipped.
func (a *app) promptValues(r *vault.Repo) (map[string]string, error) {
	if len(r.Fields) == 0 {
		return nil, errors.New("repository has no fields; create fields first (mpm field create)")
	}
	out := map[string]string{}
	for _, f := range r.Fields {
		v, err := a.prompt(f.Name + ": ")
		if err != nil {
			return nil, err
		}
		if v != "" {
			out[f.Name] = v
		}
	}
	return out, nil
}

func (a *app) secretCmd() *cobra.Command {
	secret := &cobra.Command{Use: "secret", Short: "Manage secrets"}
	var repo, name, field, value, format string
	var noClear bool

	common := func(c *cobra.Command, needName bool) *cobra.Command {
		c.Args = cobra.NoArgs
		c.Flags().StringVar(&repo, "repo", "", "repository name")
		if needName {
			c.Flags().StringVar(&name, "secret", "", "secret name")
			_ = c.MarkFlagRequired("secret")
		}
		return c
	}

	list := common(&cobra.Command{
		Use: "list", Aliases: []string{"ls"}, Short: "List secrets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := a.format(format)
			if err != nil {
				return err
			}
			_, h, err := a.open(repo)
			if err != nil {
				return err
			}
			names := []string{}
			for n := range h.Repo.Secrets {
				names = append(names, n)
			}
			sortStrings(names)
			if f == "json" {
				return printJSON(cmd.OutOrStdout(), names)
			}
			printLines(cmd.OutOrStdout(), names)
			return nil
		},
	}, false)
	list.Flags().StringVar(&format, "format", "", "output format: text or json")

	write := func(mustExist bool) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, _ []string) error {
			return a.modify(repo, func(r *vault.Repo) error {
				existing, exists := r.Secrets[name]
				switch {
				case mustExist && !exists:
					return fmt.Errorf("secret %q: %w", name, vault.ErrNotFound)
				case !mustExist && exists:
					return fmt.Errorf("secret %q: %w (use secret update)", name, vault.ErrExists)
				}
				var vals map[string]string
				var err error
				if cmd.Flags().Changed("value") {
					vals, err = cli.ParseValue(value)
				} else {
					vals, err = a.promptValues(r)
				}
				if err != nil {
					return err
				}
				merged := map[string]string{}
				for k, v := range existing {
					merged[k] = v
				}
				for k, v := range vals {
					merged[k] = v
				}
				return r.SetSecret(name, merged)
			})
		}
	}
	create := common(&cobra.Command{Use: "create", Short: "Create a secret", RunE: write(false)}, true)
	create.Flags().StringVar(&value, "value", "", "field:value,field:value,... (prompts if omitted)")
	update := common(&cobra.Command{Use: "update", Short: "Update a secret's fields", RunE: write(true)}, true)
	update.Flags().StringVar(&value, "value", "", "field:value,field:value,... (prompts if omitted; unlisted fields are kept)")

	show := common(&cobra.Command{
		Use: "show", Short: "Print a secret or one of its fields",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := a.format(format)
			if err != nil {
				return err
			}
			_, h, err := a.open(repo)
			if err != nil {
				return err
			}
			s, err := renderSecret(h.Repo, name, field, f)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}, true)
	show.Flags().StringVar(&field, "field", "", "field name (default: all fields)")
	show.Flags().StringVar(&format, "format", "", "output format: text or json")

	cp := common(&cobra.Command{
		Use: "copy", Short: "Copy a secret or one of its fields to the clipboard",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := a.format(format)
			if err != nil {
				return err
			}
			_, h, err := a.open(repo)
			if err != nil {
				return err
			}
			s, err := renderSecret(h.Repo, name, field, f)
			if err != nil {
				return err
			}
			if err := a.clip.Write(s); err != nil {
				return err
			}
			if !noClear {
				if err := a.spawnClear(s, clipboardClearAfter); err != nil {
					return fmt.Errorf("copied, but could not schedule clipboard clear: %w", err)
				}
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Copied to clipboard.")
			return nil
		},
	}, true)
	cp.Flags().StringVar(&field, "field", "", "field name (default: all fields)")
	cp.Flags().StringVar(&format, "format", "", "output format: text or json")
	cp.Flags().BoolVar(&noClear, "no-clear", false, "do not clear the clipboard after 30 seconds")

	del := common(&cobra.Command{
		Use: "delete", Aliases: []string{"rm"}, Short: "Delete a secret",
		RunE: func(*cobra.Command, []string) error {
			return a.modify(repo, func(r *vault.Repo) error { return r.DeleteSecret(name) })
		},
	}, true)

	secret.AddCommand(list, create, update, show, cp, del)
	return secret
}
