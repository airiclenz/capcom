package index

import (
	"fmt"
	"testing"
)

// benchIssueCount is the number of issues BenchmarkList generates: about the size of a large
// store.
const benchIssueCount = 3000

// BenchmarkList lists every issue of a generated store of benchIssueCount issues, each a Parent of
// up to nine children and each with labels, deps, related links and an alias.
func BenchmarkList(b *testing.B) {
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
	x := openIndex(b, s)
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
