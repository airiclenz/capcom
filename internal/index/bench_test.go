package index

import (
	"fmt"
	"testing"
)

// benchIssueCount is the number of issues the benchmarks generate: about the size of a large
// store.
const benchIssueCount = 3000

// benchExcerptText is the search text BenchmarkSearchExcerpt builds excerpts for: a word every
// generated body holds.
const benchExcerptText = "bench"

// benchIndex indexes a generated store of benchIssueCount issues, each a Parent of up to nine
// children and each with labels, deps, related links, an alias and a body.
func benchIndex(b *testing.B) *Index {
	b.Helper()
	s := newStore(b)
	for number := range benchIssueCount {
		id := fmt.Sprintf("bench-%04d", number)
		name := id + ".md"
		if parent := number / 10 * 10; parent != number {
			name = fmt.Sprintf("bench-%04d/%s", parent, name)
		}
		extra := fmt.Sprintf("labels: [area-%d, bench]\ndeps: [bench-%04d]\nrelated: [bench-%04d]\naliases: [old-%04d]\n",
			number%7, (number+1)%benchIssueCount, (number+2)%benchIssueCount, number)
		writeIssue(b, s, name, issueWith(fmt.Sprintf("Bench issue %d", number), "open", extra))
	}
	return openIndex(b, s)
}

// BenchmarkList lists every issue of the generated store.
func BenchmarkList(b *testing.B) {
	x := benchIndex(b)
	b.ResetTimer()

	for range b.N {
		entries, err := x.List(Filter{})
		if err != nil {
			b.Fatalf("List: %v", err)
		}
		if len(entries) != benchIssueCount {
			b.Fatalf("List returned %d issues, want %d", len(entries), benchIssueCount)
		}
	}
}

// BenchmarkSearchExcerpt reads the body of every issue of the generated store and builds its
// excerpt, as search --excerpt does for its hits.
func BenchmarkSearchExcerpt(b *testing.B) {
	x := benchIndex(b)
	entries, err := x.List(Filter{})
	if err != nil {
		b.Fatalf("List: %v", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	b.ResetTimer()

	for range b.N {
		bodies, err := x.Bodies(paths)
		if err != nil {
			b.Fatalf("Bodies: %v", err)
		}
		if len(bodies) != benchIssueCount {
			b.Fatalf("Bodies returned %d bodies, want %d", len(bodies), benchIssueCount)
		}
		for _, path := range paths {
			if excerpt := Excerpt(bodies[path], benchExcerptText); len(excerpt) == 0 {
				b.Fatalf("Excerpt of %s is empty, want the line holding %q", path, benchExcerptText)
			}
		}
	}
}
