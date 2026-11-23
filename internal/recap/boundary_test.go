package recap

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/store"
)

// commitAt commits files as commit does, with author and committer time at.
func commitAt(t *testing.T, s *store.Store, at time.Time, files map[string]string) string {
	t.Helper()
	date := fmt.Sprintf("%d +0000", at.Unix())
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
	return commit(t, s, files)
}

func TestBoundary_EachSinceForm_ResolvesToTheCommitBefore(t *testing.T) {
	s := newStore(t)
	now := time.Now()
	base := commitAt(t, s, now.AddDate(0, 0, -30), baseline)
	commitAt(t, s, now.AddDate(0, 0, -1), map[string]string{"issues/new.md": issueFile("New", "open", "")})
	tests := []struct {
		name  string
		since string
		want  string
	}{
		{name: "days", since: "7d", want: base},
		{name: "date", since: now.AddDate(0, 0, -10).Format(time.DateOnly), want: base},
		{name: "ref", since: "HEAD~1", want: base},
		{name: "before the first commit", since: "40d", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Boundary(s.WorktreeRoot, test.since, now)

			if err != nil || got != test.want {
				t.Errorf("Boundary(%q) = %q, %v; want %q, nil", test.since, got, err, test.want)
			}
		})
	}
}

func TestBoundary_UnknownSince_FailsWithErrUnknownBoundary(t *testing.T) {
	s := newStore(t)
	commit(t, s, baseline)
	for _, since := range []string{"no-such-ref", "-x"} {
		t.Run(since, func(t *testing.T) {
			_, err := Boundary(s.WorktreeRoot, since, time.Now())

			if !errors.Is(err, ErrUnknownBoundary) {
				t.Errorf("Boundary(%q) error = %v; want ErrUnknownBoundary", since, err)
			}
		})
	}
}
