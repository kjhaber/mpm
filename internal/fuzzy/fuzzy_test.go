package fuzzy

import (
	"reflect"
	"testing"
)

func indexes(ms []Match) []int {
	out := []int{}
	for _, m := range ms {
		out = append(out, m.Index)
	}
	return out
}

func TestEmptyQueryReturnsAllInOrder(t *testing.T) {
	got := indexes(Filter("", []string{"b", "a", "c"}))
	if !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("got %v", got)
	}
}

func TestSubsequenceMatching(t *testing.T) {
	got := indexes(Filter("gh", []string{"github", "gitlab", "other"}))
	if !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("got %v", got)
	}
}

func TestCaseInsensitive(t *testing.T) {
	if len(Filter("GH", []string{"github"})) != 1 {
		t.Fatal("expected match")
	}
}

func TestConsecutiveOutranksScattered(t *testing.T) {
	got := indexes(Filter("abc", []string{"axbxc", "abc", "xabcx"}))
	if got[0] != 1 {
		t.Fatalf("exact/prefix should rank first, got %v", got)
	}
	if got[len(got)-1] != 0 {
		t.Fatalf("scattered should rank last, got %v", got)
	}
}

func TestPositions(t *testing.T) {
	ms := Filter("gh", []string{"github"})
	if !reflect.DeepEqual(ms[0].Positions, []int{0, 3}) {
		t.Fatalf("got %v", ms[0].Positions)
	}
}

func TestNoMatch(t *testing.T) {
	if got := Filter("zzz", []string{"github"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestUnicodeDoesNotPanic(t *testing.T) {
	ms := Filter("é", []string{"café", "日本語"})
	if len(ms) != 1 || ms[0].Positions[0] != 3 {
		t.Fatalf("got %+v", ms)
	}
}

func TestStableForEqualScores(t *testing.T) {
	got := indexes(Filter("a", []string{"a1", "a2", "a3"}))
	if !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("got %v", got)
	}
}
