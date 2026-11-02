// Package fuzzy implements simple case-insensitive subsequence matching with scoring.
package fuzzy

import (
	"sort"
	"unicode"
)

// Match is one matching item: its index in the input, a score (higher is
// better) and the rune positions of the matched characters.
type Match struct {
	Index     int
	Score     int
	Positions []int
}

// Filter returns items matching query as a case-insensitive subsequence, best
// score first (ties keep input order). An empty query returns every item in order.
func Filter(query string, items []string) []Match {
	q := []rune(query)
	var out []Match
	for i, item := range items {
		if len(q) == 0 {
			out = append(out, Match{Index: i})
			continue
		}
		if score, pos, ok := match(q, []rune(item)); ok {
			out = append(out, Match{Index: i, Score: score, Positions: pos})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Score > out[b].Score })
	return out
}

func match(q, s []rune) (int, []int, bool) {
	pos := make([]int, 0, len(q))
	score, prev := 0, -2
	qi := 0
	for si, r := range s {
		if qi == len(q) {
			break
		}
		if unicode.ToLower(r) != unicode.ToLower(q[qi]) {
			continue
		}
		score++
		if si == prev+1 {
			score += 5 // consecutive
		}
		if si == 0 {
			score += 8 // prefix
		} else if !unicode.IsLetter(s[si-1]) && !unicode.IsDigit(s[si-1]) {
			score += 3 // word start
		}
		pos = append(pos, si)
		prev = si
		qi++
	}
	if qi < len(q) {
		return 0, nil, false
	}
	return score - (pos[len(pos)-1] - pos[0] - (len(q) - 1)), pos, true
}
