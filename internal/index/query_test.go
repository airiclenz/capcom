package index

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// issueWith is the content of a valid issue file with the given extra frontmatter lines.
func issueWith(title, status, extra string) string {
	closing := ""
	if status == "done" || status == "dropped" {
		closing = "closed: 2026-10-12T09:00:00Z\nreason: finished\n"
	}
	return fmt.Sprintf(
		"---\ntitle: %s\nstatus: %s\ntype: task\npriority: 2\n%screated: 2026-10-11T09:00:00Z\n%s---\nBody of %s.\n",
		title, status, extra, closing, title,
	)
}

// listFixture indexes a small tree exercising every list filter.
func listFixture(t *testing.T) *Index {
	t.Helper()
	s := newStore(t)
	writeIssue(t, s, "login-flow.md", issueWith("Login flow", "open", "labels: [auth, epic]\nplan: 2026-10-11 - 00 - login-plan\n"))
	writeIssue(t, s, "login-flow/session-refresh.md", issueWith("Session refresh", "parked", "labels: [auth]\n"))
	writeIssue(t, s, "login-flow/token-store.md", issueWith("Token store", "done", ""))
	writeIssue(t, s, "release-checklist.md",
		"---\ntitle: Release checklist\nstatus: open\ntype: chore\npriority: 0\ncreated: 2026-10-11T09:00:00Z\n---\n")
	writeIssue(t, s, "old-idea.md", issueWith("Old idea", "dropped", "aliases: [apogee-kk0.4]\n"))
	x := openIndex(t, s)
	refresh(t, x)
	return x
}

func ids(entries []Entry) []string {
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.ID)
	}
	return values
}

// deleteOnFirstEntryRead swaps the package's beforeEntryRead seam so that, on its first call, a
// second connection deletes the row of id from table, the way a refresh another command commits
// between the read of the ids and the read of an entry would. Its callers never run in parallel.
func deleteOnFirstEntryRead(t *testing.T, s *store.Store, table, id string) {
	t.Helper()
	original := beforeEntryRead
	t.Cleanup(func() { beforeEntryRead = original })
	isDeleted := false
	beforeEntryRead = func(string) {
		if isDeleted {
			return
		}
		isDeleted = true
		other, err := sql.Open(driverName, Path(s)+dsnOptions)
		if err != nil {
			t.Fatalf("sql.Open: %v", err)
		}
		defer func() {
			if err := other.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}()
		if _, err := other.Exec(`DELETE FROM `+table+` WHERE id = ?`, id); err != nil {
			t.Fatalf("DELETE: %v", err)
		}
	}
}

// TestList_RowsDeletedMidRead_ReturnsOneSnapshot swaps the package's beforeEntryRead seam, so it
// never runs in parallel.
func TestList_RowsDeletedMidRead_ReturnsOneSnapshot(t *testing.T) {
	s := newStore(t)
	writeIssue(t, s, "login-flow.md", issueWith("Login flow", "open", ""))
	writeIssue(t, s, "login-flow/session-refresh.md", issueWith("Session refresh", "open", "labels: [auth]\n"))
	writeIssue(t, s, "release-checklist.md", issueWith("Release checklist", "open", ""))
	x := openIndex(t, s)
	want, err := x.ListIDs(Filter{})
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	deleteOnFirstEntryRead(t, s, "issues", want[len(want)-1])

	entries, err := x.List(Filter{})

	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := ids(entries); !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestList_Filters_SelectMatchingIssuesInTreeOrder(t *testing.T) {
	t.Parallel()
	x := listFixture(t)
	cases := []struct {
		name   string
		filter Filter
		want   []string
	}{
		{"default leaves closed out", Filter{}, []string{"login-flow", "session-refresh", "release-checklist"}},
		{"closed included", Filter{IsClosedIncluded: true},
			[]string{"login-flow", "session-refresh", "token-store", "old-idea", "release-checklist"}},
		{"status", Filter{Statuses: []string{"parked", "done"}, IsClosedIncluded: true}, []string{"session-refresh", "token-store"}},
		{"category", Filter{Categories: []config.Category{config.CategoryDropped}, IsClosedIncluded: true}, []string{"old-idea"}},
		{"every label", Filter{Labels: []string{"auth", "epic"}}, []string{"login-flow"}},
		{"no label", Filter{NoLabels: []string{"epic"}}, []string{"session-refresh", "release-checklist"}},
		{"type", Filter{Types: []string{"chore"}}, []string{"release-checklist"}},
		{"priority", Filter{Priorities: []int{0, 1}}, []string{"release-checklist"}},
		{"parent", Filter{ParentID: "login-flow", IsClosedIncluded: true}, []string{"session-refresh", "token-store"}},
		{"plan substring", Filter{PlanContains: "login-plan"}, []string{"login-flow"}},
		{"plan substring is literal", Filter{PlanContains: "%"}, []string{}},
		{"planned", Filter{Plan: PlanSet}, []string{"login-flow"}},
		{"unplanned", Filter{Plan: PlanUnset}, []string{"session-refresh", "release-checklist"}},
		{"limit", Filter{Limit: 2}, []string{"login-flow", "session-refresh"}},
	}

	for _, current := range cases {
		got, err := x.List(current.filter)

		if err != nil || !slices.Equal(ids(got), current.want) {
			t.Errorf("%s: List = %v, %v; want %v", current.name, ids(got), err, current.want)
		}
	}
}

func TestList_Entry_CarriesListsAndParent(t *testing.T) {
	t.Parallel()
	x := listFixture(t)

	got, err := x.List(Filter{ParentID: "login-flow"})

	if err != nil || len(got) != 1 || got[0].ParentID != "login-flow" || !slices.Equal(got[0].Labels, []string{"auth"}) {
		t.Errorf("List = %+v, %v; want session-refresh under login-flow labelled auth", got, err)
	}
}

func TestEntryNode_IndexedIssue_CarriesWhatTheReadyRulesRead(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "login-flow.md", issueWith("Login flow", "open", ""))
	writeIssue(t, s, "login-flow/session-refresh.md", issueWith("Session refresh", "parked",
		"labels: [auth, ui]\ndeps: [token-store]\naliases: [apogee-kk0.2]\n"))
	x := openIndex(t, s)
	refresh(t, x)
	entries, err := x.List(Filter{ParentID: "login-flow"})
	if err != nil || len(entries) != 1 {
		t.Fatalf("List = %+v, %v; want session-refresh alone", entries, err)
	}

	got := entries[0].Node()

	want := issue.Node{
		ID: "session-refresh", ParentID: "login-flow", Category: config.CategoryWaiting,
		Labels: []string{"auth", "ui"}, Deps: []string{"token-store"}, Aliases: []string{"apogee-kk0.2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Node = %+v; want %+v", got, want)
	}
}

func TestResolve_IDAliasAndPrefix_FindTheIssue(t *testing.T) {
	t.Parallel()
	x := listFixture(t)
	cases := map[string]string{
		"token-store":   "token-store",
		"apogee-kk0.4":  "old-idea",
		"rel":           "release-checklist",
		"session-r":     "session-refresh",
		"login-flow":    "login-flow",
		"old-idea":      "old-idea",
		"release-check": "release-checklist",
	}

	for arg, want := range cases {
		got, err := x.Resolve(arg)

		if err != nil || got.ID != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", arg, got.ID, err, want)
		}
	}
}

func TestResolve_NoMatch_IsNotFound(t *testing.T) {
	t.Parallel()
	x := listFixture(t)

	for _, arg := range []string{"absent", ""} {
		_, err := x.Resolve(arg)

		if !errors.Is(err, tree.ErrNotFound) {
			t.Errorf("Resolve(%q) err = %v, want ErrNotFound", arg, err)
		}
	}
}

func TestResolve_AmbiguousPrefix_ListsCandidates(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "login-bug.md", issueText("A", "open", ""))
	writeIssue(t, s, "login-flow.md", issueText("B", "open", ""))
	x := openIndex(t, s)
	refresh(t, x)

	_, err := x.Resolve("login")

	var ambiguous *tree.AmbiguousError
	if !errors.As(err, &ambiguous) || !slices.Equal(ambiguous.Candidates, []string{"login-bug", "login-flow"}) {
		t.Errorf("Resolve err = %v, want ambiguous between login-bug and login-flow", err)
	}
}

func TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching(t *testing.T) {
	t.Parallel()
	aliasing := func(title, alias string) string {
		return issueWith(title, "open", "aliases: ["+alias+"]\n")
	}
	cases := []struct {
		name  string
		files map[string]string
		// symlink, when set, is the name of a symlink to foo-bar.md written beside the files.
		symlink   string
		isInvalid bool
		paths     []string
	}{
		{
			name:  "duplicated id",
			files: map[string]string{"foo.md": issueText("A", "open", ""), "foo-bar/foo.md": issueText("B", "open", "")},
			paths: []string{".capcom/issues/foo.md", ".capcom/issues/foo-bar/foo.md"},
		},
		{
			name:  "id another issue aliases",
			files: map[string]string{"foo.md": issueText("A", "open", ""), "baz.md": aliasing("Baz", "foo")},
			paths: []string{".capcom/issues/baz.md", ".capcom/issues/foo.md"},
		},
		{
			name:  "id of an issue left out for its alias",
			files: map[string]string{"foo.md": aliasing("A", "qux"), "qux.md": issueText("Qux", "open", "")},
			paths: []string{".capcom/issues/foo.md", ".capcom/issues/qux.md"},
		},
		{
			name:      "invalid file",
			files:     map[string]string{"foo.md": "---\nstatus: open\n---\n"},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo.md"},
		},
		{
			name:      "alias of an invalid file",
			files:     map[string]string{"foo-rework.md": issueWith("Rework", "bogus", "aliases: [foo]\n")},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo-rework.md"},
		},
		{
			name:      "alias of an unparseable file",
			files:     map[string]string{"foo-rework.md": "---\naliases: [foo]\ntitle: [unclosed\n---\n"},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo-rework.md"},
		},
		{
			name:      "file over 1 MiB",
			files:     map[string]string{"foo.md": oversizeIssueText("Huge")},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo.md"},
		},
		{
			name:      "symlinked file",
			symlink:   "foo.md",
			isInvalid: true,
			paths:     []string{".capcom/issues/foo.md"},
		},
		{
			name:      "file inside an orphan directory",
			files:     map[string]string{"orphan-dir/foo.md": issueText("A", "open", "")},
			isInvalid: true,
			paths:     []string{".capcom/issues/orphan-dir/foo.md"},
		},
		{
			name:      "alias of a file inside an orphan directory",
			files:     map[string]string{"orphan-dir/deeper/foo-rework.md": aliasing("Rework", "foo")},
			isInvalid: true,
			paths:     []string{".capcom/issues/orphan-dir/deeper/foo-rework.md"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			writeIssue(t, s, "foo-bar.md", issueText("Foo bar", "open", ""))
			for name, content := range tc.files {
				writeIssue(t, s, name, content)
			}
			if tc.symlink != "" {
				makeSymlink(t, "foo-bar.md", filepath.Join(s.IssuesDir(), tc.symlink))
			}
			x := openIndex(t, s)
			refresh(t, x)

			_, err := x.Resolve("Foo")
			prefixed, prefixErr := x.Resolve("fo")

			if errors.Is(err, tree.ErrNotFound) != tc.isInvalid || errors.Is(err, tree.ErrDuplicate) == tc.isInvalid {
				t.Errorf("Resolve(Foo) err = %v, want invalid %v", err, tc.isInvalid)
			}
			for _, path := range tc.paths {
				if err == nil || !strings.Contains(err.Error(), path) {
					t.Errorf("Resolve(Foo) err = %v, want it to name %s", err, path)
				}
			}
			if prefixErr != nil || prefixed.ID != "foo-bar" {
				t.Errorf("Resolve(fo) = %q, %v; want foo-bar", prefixed.ID, prefixErr)
			}
		})
	}
}

func TestResolveNote_DuplicatedID_RefusesInsteadOfPrefixMatching(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNote(t, s, "foo-bar.md", "---\ntitle: Foo bar\n---\n")
	writeNote(t, s, "foo.md", "---\ntitle: A\n---\n")
	writeNote(t, s, "foo-bar/foo.md", "---\ntitle: B\n---\n")
	x := openIndex(t, s)
	refresh(t, x)

	_, err := x.ResolveNote("foo")

	if !errors.Is(err, tree.ErrDuplicate) || errors.Is(err, tree.ErrNotFound) {
		t.Errorf("ResolveNote(foo) err = %v, want ErrDuplicate", err)
	}
}

func TestSearchIssues_Punctuation_MatchesWordsLiterally(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "slug-mention.md", issueText("Slug", "open", "Blocked by login-timeout-race.\n"))
	writeIssue(t, s, "apostrophe-note.md", issueText("Apostrophe", "open", "The refresh doesn't wait.\n"))
	writeIssue(t, s, "dotted-file.md", issueText("Dotted", "open", "Edit config.yaml first.\n"))
	writeIssue(t, s, "ranked-other.md", issueText("Other", "open", "timeout mentioned once\n"))
	x := openIndex(t, s)
	refresh(t, x)
	cases := map[string][]string{
		"login-timeout-race": {"slug-mention"},
		"doesn't":            {"apostrophe-note"},
		"config.yaml":        {"dotted-file"},
		`"`:                  nil,
	}

	for text, want := range cases {
		got, err := x.SearchIssues(text)

		if err != nil || !slices.Equal(ids(got), want) {
			t.Errorf("SearchIssues(%q) = %v, %v; want %v", text, ids(got), err, want)
		}
	}
}
