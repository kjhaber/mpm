package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kjhaber/mpm/internal/vault"
)

func (a *app) fieldCmd() *cobra.Command {
	field := &cobra.Command{Use: "field", Short: "Manage a repository's fields"}
	var repo, name, attrib, value, format string

	common := func(c *cobra.Command, needField, needAttrib, needValue bool) *cobra.Command {
		c.Args = cobra.NoArgs
		c.Flags().StringVar(&repo, "repo", "", "repository name")
		if needField {
			c.Flags().StringVar(&name, "field", "", "field name")
			_ = c.MarkFlagRequired("field")
		}
		if needAttrib {
			c.Flags().StringVar(&attrib, "attrib", "", "attribute name (required, pattern)")
			_ = c.MarkFlagRequired("attrib")
		}
		if needValue {
			c.Flags().StringVar(&value, "value", "", "attribute value")
			_ = c.MarkFlagRequired("value")
		}
		return c
	}

	list := common(&cobra.Command{
		Use: "list", Aliases: []string{"ls"}, Short: "List fields",
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
			for _, fld := range h.Repo.Fields {
				names = append(names, fld.Name)
			}
			if f == "json" {
				return printJSON(cmd.OutOrStdout(), names)
			}
			printLines(cmd.OutOrStdout(), names)
			return nil
		},
	}, false, false, false)
	list.Flags().StringVar(&format, "format", "", "output format: text or json")

	create := common(&cobra.Command{
		Use: "create", Short: "Add a field",
		RunE: func(*cobra.Command, []string) error {
			return a.modify(repo, func(r *vault.Repo) error { return r.AddField(name) })
		},
	}, true, false, false)

	show := common(&cobra.Command{
		Use: "show", Short: "Show a field's attributes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, h, err := a.open(repo)
			if err != nil {
				return err
			}
			req, err := h.Repo.GetAttrib(name, "required")
			if err != nil {
				return err
			}
			pat, _ := h.Repo.GetAttrib(name, "pattern")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "name: %s\nrequired: %s\npattern: %s\n", name, req, pat)
			return nil
		},
	}, true, false, false)

	listAttribs := common(&cobra.Command{
		Use: "list-attribs", Short: "List a field's attribute names",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, h, err := a.open(repo)
			if err != nil {
				return err
			}
			if _, err := h.Repo.GetAttrib(name, "required"); err != nil {
				return err
			}
			printLines(cmd.OutOrStdout(), vault.AttribNames())
			return nil
		},
	}, true, false, false)

	getAttrib := common(&cobra.Command{
		Use: "get-attrib", Short: "Print a field attribute's value",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, h, err := a.open(repo)
			if err != nil {
				return err
			}
			v, err := h.Repo.GetAttrib(name, attrib)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}, true, true, false)

	setAttrib := common(&cobra.Command{
		Use: "set-attrib", Short: "Set a field attribute",
		RunE: func(*cobra.Command, []string) error {
			return a.modify(repo, func(r *vault.Repo) error { return r.SetAttrib(name, attrib, value) })
		},
	}, true, true, true)

	deleteAttrib := common(&cobra.Command{
		Use: "delete-attrib", Short: "Reset a field attribute to its default",
		RunE: func(*cobra.Command, []string) error {
			return a.modify(repo, func(r *vault.Repo) error { return r.DeleteAttrib(name, attrib) })
		},
	}, true, true, false)

	del := common(&cobra.Command{
		Use: "delete", Aliases: []string{"rm"}, Short: "Delete a field (and its stored values)",
		RunE: func(*cobra.Command, []string) error {
			return a.modify(repo, func(r *vault.Repo) error { return r.DeleteField(name) })
		},
	}, true, false, false)

	var before string
	move := common(&cobra.Command{
		Use: "move", Short: "Move a field before another field (or to the end)",
		RunE: func(*cobra.Command, []string) error {
			return a.modify(repo, func(r *vault.Repo) error { return r.MoveField(name, before) })
		},
	}, true, false, false)
	move.Flags().StringVar(&before, "before", "", "field to move in front of (default: move to the end)")

	field.AddCommand(list, create, show, move, listAttribs, getAttrib, setAttrib, deleteAttrib, del)
	return field
}
