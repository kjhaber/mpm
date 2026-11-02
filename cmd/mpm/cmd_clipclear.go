package main

import (
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/kjhaber/mpm/internal/clip"
)

// clipclearCmd is a hidden helper: it waits, then clears the clipboard if it
// still holds the value read from stdin.
func (a *app) clipclearCmd() *cobra.Command {
	var after time.Duration
	c := &cobra.Command{
		Use: "_clipclear", Hidden: true, Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			want, err := io.ReadAll(a.in)
			if err != nil {
				return err
			}
			time.Sleep(after)
			return clip.ClearIfUnchanged(a.clip, string(want))
		},
	}
	c.Flags().DurationVar(&after, "after", clipboardClearAfter, "delay before clearing")
	return c
}
