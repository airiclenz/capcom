package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
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

func TestNodes_KeepsOrderAndFields(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{ID: "token-store", Category: config.CategoryTodo, Deps: []string{"login-flow"}},
		{
			ID: "session-refresh", ParentID: "login-flow", Category: config.CategoryWaiting,
			Labels: []string{"auth"}, Aliases: []string{"apogee-kk0.2"},
		},
		{ID: "login-flow", Category: config.CategoryDone},
	}

	got := Nodes(entries)

	want := []issue.Node{
		{ID: "token-store", Category: config.CategoryTodo, Deps: []string{"login-flow"}},
		{
			ID: "session-refresh", ParentID: "login-flow", Category: config.CategoryWaiting,
			Labels: []string{"auth"}, Aliases: []string{"apogee-kk0.2"},
		},
		{ID: "login-flow", Category: config.CategoryDone},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Nodes = %+v; want %+v", got, want)
	}
	if empty := Nodes(nil); empty == nil || len(empty) != 0 {
		t.Errorf("Nodes(nil) = %#v; want an empty, non-nil slice", empty)
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

func TestIssueNames_EveryIDAndAliasOnceInOrder(t *testing.T) {
	t.Parallel()
	x := listFixture(t)

	got, err := x.IssueNames()

	want := []string{"apogee-kk0.4", "login-flow", "old-idea", "release-checklist", "session-refresh", "token-store"}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("IssueNames() = %v, %v; want %v", got, err, want)
	}
}

func TestNoteNames_OnlyTheNamesOfNotes(t *testing.T) {
	t.Parallel()
	_, x := noteFixture(t)

	notes, noteErr := x.NoteNames()
	issues, issueErr := x.IssueNames()

	wantNotes := []string{"no-parallel-cli", "old-cli-rule", "release-flow", "testing-rules"}
	if noteErr != nil || !slices.Equal(notes, wantNotes) {
		t.Errorf("NoteNames() = %v, %v; want %v", notes, noteErr, wantNotes)
	}
	if issueErr != nil || !slices.Equal(issues, []string{"release-flow"}) {
		t.Errorf("IssueNames() = %v, %v; want [release-flow]", issues, issueErr)
	}
}

func TestIssueNames_ExcludedNode_IsLeftOut(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "kept.md", issueText("Kept", "open", ""))
	writeIssue(t, s, "twin.md", issueText("A", "open", ""))
	writeIssue(t, s, "kept/twin.md", issueText("B", "open", ""))
	x := openIndex(t, s)
	refresh(t, x)

	got, err := x.IssueNames()

	if err != nil || !slices.Equal(got, []string{"kept"}) {
		t.Errorf("IssueNames() = %v, %v; want [kept]", got, err)
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
		// prefixed is the answer to fo, as agreementAnswer renders it: the id of every file the
		// tree holds counts as a candidate, one the walk leaves out unread or under an orphan
		// directory none.
		prefixed string
	}{
		{
			name:     "duplicated id",
			files:    map[string]string{"foo.md": issueText("A", "open", ""), "foo-bar/foo.md": issueText("B", "open", "")},
			paths:    []string{".capcom/issues/foo.md", ".capcom/issues/foo-bar/foo.md"},
			prefixed: "ambiguous foo,foo-bar",
		},
		{
			name:      "invalid file",
			files:     map[string]string{"foo.md": "---\nstatus: open\n---\n"},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo.md"},
			prefixed:  "ambiguous foo,foo-bar",
		},
		{
			name:      "alias of an invalid file",
			files:     map[string]string{"foo-rework.md": issueWith("Rework", "bogus", "aliases: [foo]\n")},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo-rework.md"},
			prefixed:  "ambiguous foo-bar,foo-rework",
		},
		{
			name:      "alias of an unparseable file",
			files:     map[string]string{"foo-rework.md": "---\naliases: [foo]\ntitle: [unclosed\n---\n"},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo-rework.md"},
			prefixed:  "ambiguous foo-bar,foo-rework",
		},
		{
			name:      "file over 1 MiB",
			files:     map[string]string{"foo.md": oversizeIssueText("Huge")},
			isInvalid: true,
			paths:     []string{".capcom/issues/foo.md"},
			prefixed:  "id foo-bar",
		},
		{
			name:      "symlinked file",
			symlink:   "foo.md",
			isInvalid: true,
			paths:     []string{".capcom/issues/foo.md"},
			prefixed:  "id foo-bar",
		},
		{
			name:      "file inside an orphan directory",
			files:     map[string]string{"orphan-dir/foo.md": issueText("A", "open", "")},
			isInvalid: true,
			paths:     []string{".capcom/issues/orphan-dir/foo.md"},
			prefixed:  "id foo-bar",
		},
		{
			name:      "alias of a file inside an orphan directory",
			files:     map[string]string{"orphan-dir/deeper/foo-rework.md": aliasing("Rework", "foo")},
			isInvalid: true,
			paths:     []string{".capcom/issues/orphan-dir/deeper/foo-rework.md"},
			prefixed:  "id foo-bar",
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
			if got := agreementAnswer(prefixed.ID, prefixErr); got != tc.prefixed {
				t.Errorf("Resolve(fo) = %q, want %q", got, tc.prefixed)
			}
		})
	}
}

func TestResolve_AliasClash_RefusesOnlyTheSharedName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		files   map[string]string
		answers map[string]string
		// refused is the name refused as a duplicate, which names every file of paths.
		refused string
		paths   []string
	}{
		{
			name: "alias two issues list",
			files: map[string]string{
				"one-issue.md": issueWith("One", "open", "aliases: [shared]\n"),
				"two-issue.md": issueWith("Two", "open", "aliases: [shared]\n"),
			},
			answers: map[string]string{"one-issue": "id one-issue", "two-issue": "id two-issue", "sha": "not found"},
			refused: "Shared",
			paths:   []string{".capcom/issues/one-issue.md", ".capcom/issues/two-issue.md"},
		},
		{
			name: "id another issue aliases",
			files: map[string]string{
				"foo.md":     issueText("Foo", "open", ""),
				"foo-bar.md": issueText("Foo bar", "open", ""),
				"baz.md":     issueWith("Baz", "open", "aliases: [foo]\n"),
			},
			answers: map[string]string{"baz": "id baz", "fo": "ambiguous foo,foo-bar"},
			refused: "Foo",
			paths:   []string{".capcom/issues/baz.md", ".capcom/issues/foo.md"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			for name, content := range tc.files {
				writeIssue(t, s, name, content)
			}
			x := openIndex(t, s)
			refresh(t, x)

			_, refusedErr := x.Resolve(tc.refused)

			for arg, want := range tc.answers {
				entry, err := x.Resolve(arg)
				if got := agreementAnswer(entry.ID, err); got != want {
					t.Errorf("Resolve(%s) = %q, want %q", arg, got, want)
				}
			}
			if !errors.Is(refusedErr, tree.ErrDuplicate) {
				t.Errorf("Resolve(%s) err = %v, want a duplicate", tc.refused, refusedErr)
			}
			for _, path := range tc.paths {
				if refusedErr == nil || !strings.Contains(refusedErr.Error(), path) {
					t.Errorf("Resolve(%s) err = %v, want it to name %s", tc.refused, refusedErr, path)
				}
			}
		})
	}
}

func TestResolve_PrefixOfUnusableID_CountsAsACandidate(t *testing.T) {
	t.Parallel()
	repeated := map[string]string{
		"a.md":     issueText("A", "open", ""),
		"b.md":     issueText("B", "open", ""),
		"a/dup.md": issueText("Dup", "open", ""),
		"b/dup.md": issueText("Dup too", "open", ""),
	}
	withUsable := maps.Clone(repeated)
	withUsable["dup-y.md"] = issueText("Dup y", "open", "")
	cases := []struct {
		name  string
		files map[string]string
		arg   string
		// want is the answer, as agreementAnswer renders it; a refusal names every file of paths.
		want  string
		paths []string
	}{
		{
			name:  "repeated id alone",
			files: repeated,
			arg:   "du",
			want:  "duplicate",
			paths: []string{".capcom/issues/a/dup.md", ".capcom/issues/b/dup.md"},
		},
		{
			name:  "repeated id and a usable id",
			files: withUsable,
			arg:   "du",
			want:  "ambiguous dup,dup-y",
		},
		{
			name:  "invalid file alone",
			files: map[string]string{"bad.md": issueText("Bad", "bogus", "")},
			arg:   "ba",
			want:  "not found",
			paths: []string{".capcom/issues/bad.md"},
		},
		{
			name: "invalid file and a usable id",
			files: map[string]string{
				"bad.md":      issueText("Bad", "bogus", ""),
				"bad-luck.md": issueText("Bad luck", "open", ""),
			},
			arg:  "ba",
			want: "ambiguous bad,bad-luck",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			for name, content := range tc.files {
				writeIssue(t, s, name, content)
			}
			x := openIndex(t, s)
			refresh(t, x)

			entry, err := x.Resolve(tc.arg)

			if got := agreementAnswer(entry.ID, err); got != tc.want {
				t.Errorf("Resolve(%s) = %q, want %q", tc.arg, got, tc.want)
			}
			for _, path := range tc.paths {
				if err == nil || !strings.Contains(err.Error(), path) {
					t.Errorf("Resolve(%s) err = %v, want it to name %s", tc.arg, err, path)
				}
			}
		})
	}
}

func TestResolve_AliasOfARepeatedIDFile_RefusedByWhichFileListsIt(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "a.md", issueText("A", "open", ""))
	writeIssue(t, s, "b.md", issueText("B", "open", ""))
	writeIssue(t, s, "a/dup.md", issueWith("Dup", "open", "aliases: [earlier]\n"))
	writeIssue(t, s, "b/dup.md", issueWith("Dup too", "open", "aliases: [later]\n"))
	x := openIndex(t, s)
	refresh(t, x)

	_, laterErr := x.Resolve("later")
	_, earlierErr := x.Resolve("earlier")

	refusal := `id "later" not found: .capcom/issues/b/dup.md is invalid`
	if !errors.Is(laterErr, tree.ErrNotFound) || laterErr.Error() != refusal {
		t.Errorf("Resolve(later) err = %v, want %q", laterErr, refusal)
	}
	if !errors.Is(earlierErr, tree.ErrDuplicate) || !strings.Contains(earlierErr.Error(), ".capcom/issues/b/dup.md") {
		t.Errorf("Resolve(earlier) err = %v, want the duplicate id naming both files", earlierErr)
	}
}

func TestResolve_NameAnUnusableFileAlsoClaims_RefusesNamingBothFiles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		files map[string]string
		// paths are the files the refusal of foo names.
		paths []string
	}{
		{
			name:  "alias of an invalid file",
			files: map[string]string{"bad.md": issueWith("Bad", "bogus", "aliases: [foo]\n")},
			paths: []string{".capcom/issues/bad.md", ".capcom/issues/foo.md"},
		},
		{
			name:  "alias of an unparseable file",
			files: map[string]string{"zzz.md": "---\naliases: [foo]\ntitle: [unclosed\n---\n"},
			paths: []string{".capcom/issues/zzz.md", ".capcom/issues/foo.md"},
		},
		{
			name:  "id of an invalid file",
			files: map[string]string{"sub.md": issueText("Sub", "open", ""), "sub/foo.md": issueText("Foo", "bogus", "")},
			paths: []string{".capcom/issues/sub/foo.md", ".capcom/issues/foo.md"},
		},
		{
			name:  "id of a file inside an orphan directory",
			files: map[string]string{"ghost/foo.md": issueText("Foo", "open", "")},
			paths: []string{".capcom/issues/ghost/foo.md", ".capcom/issues/foo.md"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			writeIssue(t, s, "foo.md", issueText("Foo", "open", ""))
			for name, content := range tc.files {
				writeIssue(t, s, name, content)
			}
			x := openIndex(t, s)
			refresh(t, x)

			_, err := x.Resolve("foo")

			if !errors.Is(err, tree.ErrDuplicate) {
				t.Errorf("Resolve(foo) err = %v, want a duplicate", err)
			}
			for _, path := range tc.paths {
				if err == nil || !strings.Contains(err.Error(), path) {
					t.Errorf("Resolve(foo) err = %v, want it to name %s", err, path)
				}
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

func TestSearchIssuesAnyWord_TwoWords_FindsEitherWordBothFirst(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "both-words.md", issueText("Both", "open", "The parser drops the cache.\n"))
	writeIssue(t, s, "parser-only.md", issueText("Parser", "open", "The parser is slow.\n"))
	writeIssue(t, s, "cache-only.md", issueText("Cache", "open", "The cache is stale.\n"))
	writeIssue(t, s, "neither-word.md", issueText("Neither", "open", "Nothing to see.\n"))
	x := openIndex(t, s)
	refresh(t, x)

	got, err := x.SearchIssuesAnyWord("parser cache")

	if err != nil || len(got) != 3 || got[0].ID != "both-words" ||
		!slices.Equal(slices.Sorted(slices.Values(ids(got))), []string{"both-words", "cache-only", "parser-only"}) {
		t.Errorf("SearchIssuesAnyWord(parser cache) = %v, %v; want both-words first, then the one-word issues", ids(got), err)
	}
}

func TestSearchIssuesAnyWord_WordInNoIssue_StillFindsTheOthers(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "login-race.md", issueText("Login race", "open", "The login times out.\n"))
	x := openIndex(t, s)
	refresh(t, x)

	anyWord, anyErr := x.SearchIssuesAnyWord("login zeppelin")
	everyWord, everyErr := x.SearchIssues("login zeppelin")

	if anyErr != nil || !slices.Equal(ids(anyWord), []string{"login-race"}) {
		t.Errorf("SearchIssuesAnyWord(login zeppelin) = %v, %v; want [login-race]", ids(anyWord), anyErr)
	}
	if everyErr != nil || len(everyWord) != 0 {
		t.Errorf("SearchIssues(login zeppelin) = %v, %v; want none", ids(everyWord), everyErr)
	}
}

func TestSearchIssuesAnyWord_OperatorWords_MatchLiterally(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "or-word.md", issueText("Or", "open", "Pick this OR that.\n"))
	writeIssue(t, s, "not-word.md", issueText("Not", "open", "Do NOT ship.\n"))
	writeIssue(t, s, "near-word.md", issueText("Near", "open", "Stay NEAR here.\n"))
	writeIssue(t, s, "plain.md", issueText("Plain", "open", "Ordinary words only.\n"))
	x := openIndex(t, s)
	refresh(t, x)
	cases := map[string][]string{
		"OR":         {"or-word"},
		"NOT":        {"not-word"},
		"NEAR":       {"near-word"},
		`"`:          nil,
		`" NEAR`:     {"near-word"},
		"NOT absent": {"not-word"},
	}

	for text, want := range cases {
		got, err := x.SearchIssuesAnyWord(text)

		if err != nil || !slices.Equal(ids(got), want) {
			t.Errorf("SearchIssuesAnyWord(%q) = %v, %v; want %v", text, ids(got), err, want)
		}
	}
}

func TestSearchIssuesAnyWord_BlankText_FindsNothing(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "any.md", issueText("Any", "open", "Some body.\n"))
	x := openIndex(t, s)
	refresh(t, x)

	got, err := x.SearchIssuesAnyWord("   ")

	if err != nil || len(got) != 0 {
		t.Errorf("SearchIssuesAnyWord(blank) = %v, %v; want none", ids(got), err)
	}
}

// countingQuerier passes every statement to q and counts them.
type countingQuerier struct {
	q          querier
	statements int
}

func (c *countingQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	c.statements++
	return c.q.QueryContext(ctx, query, args...)
}

func (c *countingQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	c.statements++
	return c.q.QueryRowContext(ctx, query, args...)
}

// batchFixture indexes issues whose list fields are all set, some in a different order than
// their ids sort in.
func batchFixture(t *testing.T) *Index {
	t.Helper()
	s := newStore(t)
	writeIssue(t, s, "login-flow.md", issueWith("Login flow", "open",
		"labels: [epic, auth]\ndeps: [token-store, db-schema]\nrelated: [release-notes]\naliases: [apogee-kk0.4, old-login]\n"))
	writeIssue(t, s, "login-flow/token-store.md", issueWith("Token store", "open", "labels: [auth]\ndeps: [db-schema]\n"))
	writeIssue(t, s, "db-schema.md", issueWith("Schema", "open", "related: [login-flow, token-store]\n"))
	writeIssue(t, s, "release-notes.md", issueWith("Release notes", "open", "aliases: [notes]\n"))
	return openIndex(t, s)
}

func TestIssues_ManyIDs_SameStatementCountAsOne(t *testing.T) {
	t.Parallel()
	x := batchFixture(t)
	one := &countingQuerier{q: x.db}
	many := &countingQuerier{q: x.db}

	if _, err := x.issues(one, []string{"login-flow"}); err != nil {
		t.Fatalf("issues of one id: %v", err)
	}
	if _, err := x.issues(many, []string{"login-flow", "token-store", "db-schema", "release-notes"}); err != nil {
		t.Fatalf("issues of many ids: %v", err)
	}

	// One row query plus one per list field: labels, deps, related and aliases.
	const want = 5
	if one.statements != want || many.statements != want {
		t.Errorf("statements = %d for one id and %d for many, want %d for both", one.statements, many.statements, want)
	}
}

func TestIssues_BatchRead_MatchesSingleReadsInTheGivenOrder(t *testing.T) {
	t.Parallel()
	x := batchFixture(t)
	order := []string{"release-notes", "login-flow", "db-schema", "token-store"}

	entries, err := x.issues(x.db, order)

	if err != nil {
		t.Fatalf("issues: %v", err)
	}
	if got := ids(entries); !slices.Equal(got, order) {
		t.Fatalf("ids = %v, want %v", got, order)
	}
	for _, entry := range entries {
		if single := mustIssue(t, x, entry.ID); !reflect.DeepEqual(entry, single) {
			t.Errorf("batch read of %q = %+v, single read = %+v", entry.ID, entry, single)
		}
	}
	login := entries[1]
	lists := map[string][2][]string{
		"Labels":  {login.Labels, {"epic", "auth"}},
		"Deps":    {login.Deps, {"token-store", "db-schema"}},
		"Related": {login.Related, {"release-notes"}},
		"Aliases": {login.Aliases, {"apogee-kk0.4", "old-login"}},
	}
	for name, list := range lists {
		if !slices.Equal(list[0], list[1]) {
			t.Errorf("login-flow %s = %v, want %v", name, list[0], list[1])
		}
	}
}

func TestIssues_MissingID_FailsNotFound(t *testing.T) {
	t.Parallel()
	x := batchFixture(t)

	_, err := x.issues(x.db, []string{"login-flow", "gone-issue"})

	if !errors.Is(err, tree.ErrNotFound) || !strings.Contains(err.Error(), `"gone-issue"`) {
		t.Errorf("issues = %v, want a tree.ErrNotFound naming gone-issue", err)
	}
}
