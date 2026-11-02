package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

func printJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

func printLines(w io.Writer, lines []string) {
	for _, l := range lines {
		_, _ = fmt.Fprintln(w, l)
	}
}

func jsonString(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func sortStrings(s []string) { sort.Strings(s) }
