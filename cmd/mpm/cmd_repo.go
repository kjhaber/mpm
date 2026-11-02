package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *app) repoCmd() *cobra.Command {
	repo := &cobra.Command{Use: "repo", Short: "Manage secret repositories"}

	var format string
	list := &cobra.Command{
		Use: "list", Aliases: []string{"ls"}, Short: "List repositories", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := a.format(format)
			if err != nil {
				return err
			}
			st, err := a.store()
			if err != nil {
				return err
			}
			names, err := st.List()
			if err != nil {
				return err
			}
			if f == "json" {
				if names == nil {
					names = []string{}
				}
				return printJSON(cmd.OutOrStdout(), names)
			}
			printLines(cmd.OutOrStdout(), names)
			return nil
		},
	}
	list.Flags().StringVar(&format, "format", "", "output format: text or json")

	var createName string
	create := &cobra.Command{
		Use: "create", Short: "Create a repository", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			st, err := a.store()
			if err != nil {
				return err
			}
			pw, err := a.passwords().GetNew(createName)
			if err != nil {
				return err
			}
			return st.Create(createName, pw)
		},
	}
	create.Flags().StringVar(&createName, "repo", "", "repository name")
	_ = create.MarkFlagRequired("repo")

	var showName, showFormat string
	show := &cobra.Command{
		Use: "show", Short: "Show a repository's fields and secret count", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := a.format(showFormat)
			if err != nil {
				return err
			}
			name, h, err := a.open(showName)
			if err != nil {
				return err
			}
			if f == "json" {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"name": name, "fields": h.Repo.Fields, "secrets": len(h.Repo.Secrets),
				})
			}
			w := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(w, "repo: %s\nsecrets: %d\nfields:\n", name, len(h.Repo.Secrets))
			for _, fld := range h.Repo.Fields {
				req, _ := h.Repo.GetAttrib(fld.Name, "required")
				pat, _ := h.Repo.GetAttrib(fld.Name, "pattern")
				_, _ = fmt.Fprintf(w, "  %s (required: %s, pattern: %s)\n", fld.Name, req, pat)
			}
			return nil
		},
	}
	show.Flags().StringVar(&showName, "repo", "", "repository name")
	show.Flags().StringVar(&showFormat, "format", "", "output format: text or json")

	var delName string
	del := &cobra.Command{
		Use: "delete", Aliases: []string{"rm"}, Short: "Delete an empty repository", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			name, err := a.repoName(delName)
			if err != nil {
				return err
			}
			st, err := a.store()
			if err != nil {
				return err
			}
			pw, err := a.passwords().Get(name)
			if err != nil {
				return err
			}
			return st.Delete(name, pw)
		},
	}
	del.Flags().StringVar(&delName, "repo", "", "repository name")
	_ = del.MarkFlagRequired("repo")

	var copyName, copyFrom string
	var include bool
	cp := &cobra.Command{
		Use: "copy", Short: "Copy a repository (fields, optionally secrets) under a new password", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			st, err := a.store()
			if err != nil {
				return err
			}
			fromPw, err := a.passwords().Get(copyFrom)
			if err != nil {
				return err
			}
			toPw, err := a.passwords().GetNew(copyName)
			if err != nil {
				return err
			}
			return st.Copy(copyFrom, fromPw, copyName, toPw, include)
		},
	}
	cp.Flags().StringVar(&copyName, "repo", "", "name of the new repository")
	cp.Flags().StringVar(&copyFrom, "from", "", "name of the repository to copy")
	cp.Flags().BoolVar(&include, "include-secrets", false, "also copy secrets")
	_ = cp.MarkFlagRequired("repo")
	_ = cp.MarkFlagRequired("from")

	repo.AddCommand(list, create, show, del, cp)
	return repo
}
