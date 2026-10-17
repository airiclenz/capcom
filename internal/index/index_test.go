package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// newStore returns a store in a temp dir with the default config and an empty issue tree. The
// index needs no git, so GitDir is a plain directory.
func newStore(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	opened := &store.Store{
		Root:         filepath.Join(root, ".capcom"),
		WorktreeRoot: root,
		GitDir:       filepath.Join(root, ".git"),
		GitCommonDir: filepath.Join(root, ".git"),
		Config:       config.Default(),
	}
	if err := os.MkdirAll(opened.IssuesDir(), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return opened
}

// issueText is the content of a valid issue file.
func issueText(title, status, body string) string {
	return fmt.Sprintf(
		"---\ntitle: %s\nstatus: %s\ntype: task\npriority: 2\ncreated: 2026-10-11T09:00:00Z\n---\n%s",
		title, status, body,
	)
}

// writeIssue writes content to the issue file at name, a slash path below the issue tree.
func writeIssue(t *testing.T, s *store.Store, name, content string) string {
	t.Helper()
	path := filepath.Join(s.IssuesDir(), filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// laterClock returns a clock an hour ahead, so every file written so far reads as clean.
func laterClock() Option {
	return withClock(func() time.Time { return time.Now().Add(time.Hour) })
}

func openIndex(t *testing.T, s *store.Store, options ...Option) *Index {
	t.Helper()
	opened, err := Open(s, ModeRefresh, options...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := opened.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return opened
}

func refresh(t *testing.T, x *Index) Report {
	t.Helper()
	report, err := x.refresh(false)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return report
}

func mustIssue(t *testing.T, x *Index, id string) Entry {
	t.Helper()
	entry, found, err := x.Issue(id)
	if err != nil || !found {
		t.Fatalf("Issue(%q) = found %v, err %v; want it indexed", id, found, err)
	}
	return entry
}

func isIndexed(t *testing.T, x *Index, id string) bool {
	t.Helper()
	_, found, err := x.Issue(id)
	if err != nil {
		t.Fatalf("Issue(%q): %v", id, err)
	}
	return found
}

func warningStrings(report Report) []string {
	lines := make([]string, 0, len(report.Warnings))
	for _, warning := range report.Warnings {
		lines = append(lines, warning.String())
	}
	return lines
}

// warningKinds returns "path kind" for each warning of report, sorted.
func warningKinds(report Report) []string {
	lines := make([]string, 0, len(report.Warnings))
	for _, warning := range report.Warnings {
		lines = append(lines, warning.Path+" "+string(warning.Code))
	}
	slices.Sort(lines)
	return lines
}

func TestRefresh_NewTree_IndexesEveryIssue(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "parent-issue.md", issueText("Parent", "open", "parent body\n"))
	writeIssue(t, s, "parent-issue/child-issue.md", "---\ntitle: Child\nstatus: done\ntype: bug\n"+
		"priority: 1\nlabels: [ui, auth]\ndeps: [parent-issue]\nplan: some-plan\naliases: [old-child]\n"+
		"created: 2026-10-11T09:00:00Z\nclosed: 2026-10-11T10:00:00Z\nreason: fixed\n---\nbody\n")

	x := openIndex(t, s)
	report := x.Report()

	if !report.rebuilt || report.Indexed != 2 || len(report.parsed) != 2 || len(report.Warnings) != 0 {
		t.Fatalf("report = %+v, want a rebuilt index of 2 issues and no warnings", report)
	}
	child := mustIssue(t, x, "child-issue")
	want := Entry{
		ID: "child-issue", Path: ".capcom/issues/parent-issue/child-issue.md", ParentID: "parent-issue",
		Status: "done", Category: config.CategoryDone, Type: "bug", Priority: 1, Plan: "some-plan",
		Created: time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC), Closed: time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC),
		Reason: "fixed", Title: "Child", Labels: []string{"ui", "auth"}, Deps: []string{"parent-issue"},
		Aliases: []string{"old-child"},
	}
	if fmt.Sprintf("%+v", child) != fmt.Sprintf("%+v", want) {
		t.Errorf("child = %+v\nwant    %+v", child, want)
	}
}

func TestRefresh_OneFileEdited_ReparsesOnlyThatFile(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "first-issue.md", issueText("First", "open", ""))
	writeIssue(t, s, "second-issue.md", issueText("Second", "open", ""))
	x := openIndex(t, s, laterClock())
	refresh(t, x)

	writeIssue(t, s, "second-issue.md", issueText("Second, edited", "open", ""))
	report := refresh(t, x)

	if want := []string{".capcom/issues/second-issue.md"}; !slices.Equal(report.parsed, want) || report.rebuilt {
		t.Fatalf("report = %+v, want only %v parsed", report, want)
	}
	if title := mustIssue(t, x, "second-issue").Title; title != "Second, edited" {
		t.Errorf("title = %q, want the edit", title)
	}
}

func TestRefresh_SameSizeEditInOneMtimeTick_IsSeen(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	tick := time.Date(2026, 10, 11, 12, 0, 0, 500_000_000, time.UTC)
	path := writeIssue(t, s, "racy-issue.md", issueText("Title A", "open", ""))
	if err := os.Chtimes(path, tick, tick); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	x := openIndex(t, s, withClock(func() time.Time { return tick.Add(200 * time.Millisecond) }))
	refresh(t, x)

	writeIssue(t, s, "racy-issue.md", issueText("Title B", "open", ""))
	if err := os.Chtimes(path, tick, tick); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	refresh(t, x)

	if title := mustIssue(t, x, "racy-issue").Title; title != "Title B" {
		t.Errorf("title = %q, want the same-size edit seen", title)
	}
}

// exportedRefresh runs the exported Refresh on x, failing the test on an error.
func exportedRefresh(t *testing.T, x *Index) bool {
	t.Helper()
	isChanged, err := x.Refresh()
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return isChanged
}

// changeFixture is a store holding two issues and a Note, opened with laterClock so every file
// reads as clean.
func changeFixture(t *testing.T) (*store.Store, *Index) {
	t.Helper()
	s := newStore(t)
	writeIssue(t, s, "first-issue.md", issueText("First", "open", ""))
	writeIssue(t, s, "second-issue.md", issueText("Second", "open", ""))
	writeNote(t, s, "some-note.md", "---\ntitle: Some note\n---\nNote body.\n")
	return s, openIndex(t, s, laterClock())
}

func TestRefreshExported_TreeChanged_ReportsChanged(t *testing.T) {
	cases := []struct {
		name   string
		change func(t *testing.T, s *store.Store)
	}{
		{
			name: "issue added",
			change: func(t *testing.T, s *store.Store) {
				writeIssue(t, s, "third-issue.md", issueText("Third", "open", ""))
			},
		},
		{
			name: "issue changed",
			change: func(t *testing.T, s *store.Store) {
				writeIssue(t, s, "second-issue.md", issueText("Second, edited", "open", ""))
			},
		},
		{
			name: "issue removed",
			change: func(t *testing.T, s *store.Store) {
				if err := os.Remove(filepath.Join(s.IssuesDir(), "second-issue.md")); err != nil {
					t.Fatalf("Remove: %v", err)
				}
			},
		},
		{
			name: "note changed",
			change: func(t *testing.T, s *store.Store) {
				writeNote(t, s, "some-note.md", "---\ntitle: Some note, edited\n---\nNote body.\n")
			},
		},
		{
			name: "oversize issue added",
			change: func(t *testing.T, s *store.Store) {
				writeIssue(t, s, "huge-issue.md", oversizeIssueText("Huge"))
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			s, x := changeFixture(t)
			testCase.change(t, s)

			isChanged := exportedRefresh(t, x)

			if !isChanged {
				t.Errorf("Refresh = false, want true; report %+v", x.Report())
			}
		})
	}
}

func TestRefreshExported_NothingChanged_ReportsUnchangedAndParsesNothing(t *testing.T) {
	t.Parallel()
	s, x := changeFixture(t)
	writeIssue(t, s, "huge-issue.md", oversizeIssueText("Huge"))
	exportedRefresh(t, x)

	isChanged := exportedRefresh(t, x)

	if report := x.Report(); isChanged || len(report.parsed) != 0 {
		t.Errorf("Refresh = %v, parsed %v; want false and nothing parsed", isChanged, report.parsed)
	}
}

func TestRefreshExported_NothingChanged_KeepsMembershipWarnings(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "top-issue.md", issueText("Top", "open", ""))
	writeIssue(t, s, "top-issue/dup-issue.md", issueText("Nested dup", "open", ""))
	writeIssue(t, s, "dup-issue.md", issueText("Top dup", "open", ""))
	x := openIndex(t, s, laterClock())

	isChanged := exportedRefresh(t, x)

	if got := warningStrings(x.Report()); isChanged || !slices.Equal(got, repeatedIDWarnings) {
		t.Errorf("Refresh = %v, warnings %q; want false and %q", isChanged, got, repeatedIDWarnings)
	}
	if isIndexed(t, x, "dup-issue") {
		t.Errorf("dup-issue indexed after a no-change Refresh, want it still excluded")
	}
}

func TestRefreshExported_SameSizeEditInOneMtimeTick_ReparsesIt(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	tick := time.Date(2026, 10, 11, 12, 0, 0, 500_000_000, time.UTC)
	path := writeIssue(t, s, "racy-issue.md", issueText("Title A", "open", ""))
	if err := os.Chtimes(path, tick, tick); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	x := openIndex(t, s, withClock(func() time.Time { return tick.Add(200 * time.Millisecond) }))
	writeIssue(t, s, "racy-issue.md", issueText("Title B", "open", ""))
	if err := os.Chtimes(path, tick, tick); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	isChanged := exportedRefresh(t, x)

	if title := mustIssue(t, x, "racy-issue").Title; !isChanged || title != "Title B" {
		t.Errorf("Refresh = %v, title %q; want true and the same-size edit seen", isChanged, title)
	}
}

func TestRefresh_FileDeleted_DropsItsRows(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	path := writeIssue(t, s, "gone-issue.md", issueText("Gone", "open", "unique word\n"))
	writeIssue(t, s, "kept-issue.md", issueText("Kept", "open", ""))
	x := openIndex(t, s, laterClock())
	refresh(t, x)

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	report := refresh(t, x)

	if want := []string{".capcom/issues/gone-issue.md"}; !slices.Equal(report.removed, want) || report.Indexed != 1 {
		t.Fatalf("report = %+v, want %v removed and 1 indexed", report, want)
	}
	found, err := x.Search("unique")
	if isIndexed(t, x, "gone-issue") || err != nil || len(found) != 0 {
		t.Errorf("gone-issue still indexed: search = %v, err %v", found, err)
	}
}

func TestRefresh_ParseFailure_WarnsOnEveryRefreshAndIndexesTheRest(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "broken-issue.md", "---\ntitle: Broken\nstatus: open\nno closing fence\n")
	writeIssue(t, s, "bad-status.md", "---\ntitle: Bad\nstatus: nonsense\ntype: task\ncreated: 2026-10-11T09:00:00Z\n---\n")
	writeIssue(t, s, "good-issue.md", issueText("Good", "open", ""))
	x := openIndex(t, s, laterClock())

	first := refresh(t, x)
	second := refresh(t, x)

	want := []string{
		`.capcom/issues/bad-status.md:3: status "nonsense" is not declared in config; use one of [open active parked done dropped]`,
		".capcom/issues/broken-issue.md:1: the frontmatter opened here is never closed by a --- line",
	}
	for name, report := range map[string]Report{"first": first, "second": second} {
		if got := warningStrings(report); !slices.Equal(got, want) || report.Indexed != 1 {
			t.Errorf("%s refresh: warnings = %q indexed = %d, want %q and 1", name, got, report.Indexed, want)
		}
	}
	if len(second.parsed) != 0 {
		t.Errorf("second refresh parsed %v, want nothing", second.parsed)
	}
}

func TestRefresh_DuplicateName_WarnsAndExcludesBoth(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		excluded []string
		rivals   []string
		remove   string
	}{
		{
			name: "id across subtrees",
			files: map[string]string{
				"top-issue.md":           issueText("Top", "open", ""),
				"top-issue/dup-issue.md": issueText("Nested dup", "open", ""),
				"dup-issue.md":           issueText("Top dup", "open", ""),
			},
			excluded: []string{"dup-issue"},
			rivals:   []string{"dup-issue.md", "top-issue/dup-issue.md"},
			remove:   "top-issue/dup-issue.md",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			for name, content := range testCase.files {
				writeIssue(t, s, name, content)
			}
			x := openIndex(t, s, laterClock())

			report := refresh(t, x)

			for _, id := range testCase.excluded {
				if isIndexed(t, x, id) {
					t.Errorf("%s indexed, want it excluded", id)
				}
			}
			warnings := strings.Join(warningStrings(report), "\n")
			for _, rival := range testCase.rivals {
				// Each rival is warned about and named in the other's warning.
				if path := ".capcom/issues/" + rival; strings.Count(warnings, path) != 2 {
					t.Errorf("warnings do not name %s on both sides:\n%s", path, warnings)
				}
			}
			if err := os.Remove(filepath.Join(s.IssuesDir(), filepath.FromSlash(testCase.remove))); err != nil {
				t.Fatalf("Remove: %v", err)
			}
			if after := refresh(t, x); len(after.Warnings) != 0 || !isIndexed(t, x, testCase.excluded[0]) {
				t.Errorf("after removing the rival: warnings %q, want %s indexed again", warningStrings(after), testCase.excluded[0])
			}
		})
	}
}

// repeatedIDWarnings are the warnings about dup-issue.md and top-issue/dup-issue.md, two files of
// the id dup-issue.
var repeatedIDWarnings = []string{
	`.capcom/issues/dup-issue.md: duplicate id "dup-issue": also the id of .capcom/issues/top-issue/dup-issue.md; both are left out`,
	`.capcom/issues/top-issue/dup-issue.md: duplicate id "dup-issue": also the id of .capcom/issues/dup-issue.md; both are left out`,
}

// unparsedIssueText is the content of an issue file whose frontmatter does not parse.
const unparsedIssueText = "---\ntitle: [unclosed\n---\n"

func TestRefresh_RepeatedIDWithAFileWithProblems_WarnsAndExcludesBoth(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "unparsed file", content: unparsedIssueText},
		{name: "invalid file", content: issueText("Nested dup", "bogus", "")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			writeIssue(t, s, "top-issue.md", issueText("Top", "open", ""))
			writeIssue(t, s, "dup-issue.md", issueText("Top dup", "open", ""))
			writeIssue(t, s, "top-issue/dup-issue.md", testCase.content)
			x := openIndex(t, s)

			report := refresh(t, x)
			_, err := x.Resolve("dup-issue")

			if isIndexed(t, x, "dup-issue") {
				t.Errorf("dup-issue indexed, want it excluded")
			}
			warnings := warningStrings(report)
			for _, want := range repeatedIDWarnings {
				if !slices.Contains(warnings, want) {
					t.Errorf("warnings = %q, want %q among them", warnings, want)
				}
			}
			if len(warnings) != len(repeatedIDWarnings)+1 {
				t.Errorf("warnings = %q, want the repeated id and the file's own problem", warnings)
			}
			if !errors.Is(err, tree.ErrDuplicate) {
				t.Errorf("Resolve(dup-issue) err = %v, want ErrDuplicate", err)
			}
		})
	}
}

func TestRefresh_SiblingAdded_DecidesFromStoredRowsWithoutRereading(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "top-issue.md", issueText("Top", "open", ""))
	writeIssue(t, s, "top-issue/dup-issue.md", unparsedIssueText)
	x := openIndex(t, s, laterClock())
	refresh(t, x)
	_, beforeErr := x.Resolve("dup-issue")

	writeIssue(t, s, "dup-issue.md", issueText("Top dup", "open", ""))
	report := refresh(t, x)
	_, afterErr := x.Resolve("dup-issue")

	if want := []string{".capcom/issues/dup-issue.md"}; !slices.Equal(report.parsed, want) {
		t.Fatalf("parsed = %q, want only %q", report.parsed, want)
	}
	if !errors.Is(beforeErr, tree.ErrNotFound) {
		t.Errorf("Resolve(dup-issue) before the sibling err = %v, want ErrNotFound", beforeErr)
	}
	if isIndexed(t, x, "dup-issue") {
		t.Errorf("dup-issue indexed, want it excluded beside the unparsed file")
	}
	warnings := warningStrings(report)
	for _, want := range repeatedIDWarnings {
		if !slices.Contains(warnings, want) {
			t.Errorf("warnings = %q, want %q among them", warnings, want)
		}
	}
	if !errors.Is(afterErr, tree.ErrDuplicate) {
		t.Errorf("Resolve(dup-issue) after the sibling err = %v, want ErrDuplicate", afterErr)
	}
}

func TestRefresh_ValidFileBesideAnOrphanFileOfItsID_StaysListed(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "login.md", issueText("Login", "open", ""))
	writeIssue(t, s, "old/login.md", issueText("Old login", "open", ""))
	x := openIndex(t, s)

	report := refresh(t, x)
	ids, err := x.ListIDs(Filter{})

	if err != nil || !slices.Equal(ids, []string{"login"}) {
		t.Errorf("ListIDs() = %q, %v; want login listed", ids, err)
	}
	want := []string{".capcom/issues/old: directory without its node file: no old.md beside it; left out"}
	if got := warningStrings(report); !slices.Equal(got, want) {
		t.Errorf("warnings = %q, want only %q", got, want)
	}
}

func TestRefresh_AliasClash_WarnsOnceAndKeepsBothNodes(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		warning string
		remove  string
	}{
		{
			name: "alias two issues list",
			files: map[string]string{
				"one-issue.md": issueWith("One", "open", "aliases: [shared]\n"),
				"two-issue.md": issueWith("Two", "open", "aliases: [shared]\n"),
			},
			warning: `.capcom/issues/two-issue.md: duplicate alias "shared": also an alias of .capcom/issues/one-issue.md`,
			remove:  "one-issue.md",
		},
		{
			name: "alias that is another issue's id",
			files: map[string]string{
				"real-issue.md":  issueText("Real", "open", ""),
				"other-issue.md": issueWith("Other", "open", "aliases: [real-issue]\n"),
			},
			warning: `.capcom/issues/other-issue.md: duplicate alias "real-issue": the id of .capcom/issues/real-issue.md`,
			remove:  "other-issue.md",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			for name, content := range testCase.files {
				writeIssue(t, s, name, content)
			}
			x := openIndex(t, s, laterClock())

			report := refresh(t, x)

			for name := range testCase.files {
				if id := strings.TrimSuffix(name, ".md"); !isIndexed(t, x, id) {
					t.Errorf("%s excluded, want it indexed", id)
				}
			}
			if got := warningStrings(report); !slices.Equal(got, []string{testCase.warning}) {
				t.Errorf("warnings = %q, want only %q", got, testCase.warning)
			}
			if err := os.Remove(filepath.Join(s.IssuesDir(), testCase.remove)); err != nil {
				t.Fatalf("Remove: %v", err)
			}
			if after := refresh(t, x); len(after.Warnings) != 0 {
				t.Errorf("after removing the rival: warnings %q, want none", warningStrings(after))
			}
		})
	}
}

func TestRefresh_StatusRemapped_CategoryFollows(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "parked-issue.md", issueText("Parked", "parked", ""))
	x := openIndex(t, s, laterClock())
	refresh(t, x)
	s.Config.Statuses = slices.Clone(s.Config.Statuses)
	for i := range s.Config.Statuses {
		if s.Config.Statuses[i].Name == "parked" {
			s.Config.Statuses[i].Category = config.CategoryDoing
		}
	}

	report := refresh(t, x)

	if !report.rebuilt {
		t.Errorf("report = %+v, want a rebuild after the status map changed", report)
	}
	if category := mustIssue(t, x, "parked-issue").Category; category != config.CategoryDoing {
		t.Errorf("category = %s, want %s", category, config.CategoryDoing)
	}
}

func TestRefresh_SchemaVersionChanged_Rebuilds(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "some-issue.md", issueText("Some", "open", ""))
	x := openIndex(t, s, laterClock())
	refresh(t, x)
	db, err := sql.Open(driverName, Path(s))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = db.Exec(`UPDATE meta SET value = '0' WHERE key = ?`, metaSchemaVersion)
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("bump schema version: %v, %v", err, closeErr)
	}

	report := refresh(t, x)

	if !report.rebuilt || len(report.parsed) != 1 || report.Indexed != 1 {
		t.Errorf("report = %+v, want a rebuild that parsed and indexed the one issue", report)
	}
}

func TestIsSchemaForeign_CurrentIndex_IsFalse(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	x := openIndex(t, s)

	isForeign, err := x.IsSchemaForeign()

	if err != nil || isForeign {
		t.Errorf("IsSchemaForeign = %v, %v, want false, nil", isForeign, err)
	}
}

func TestIsSchemaForeign_OtherVersionRecorded_IsTrue(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	x := openIndex(t, s)
	db, err := sql.Open(driverName, Path(s))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = db.Exec(`UPDATE meta SET value = ? WHERE key = ?`, schemaVersion+1, metaSchemaVersion)
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("record another schema version: %v, %v", err, closeErr)
	}

	isForeign, err := x.IsSchemaForeign()

	if err != nil || !isForeign {
		t.Errorf("IsSchemaForeign = %v, %v, want true, nil", isForeign, err)
	}
}

func TestSearch_BodyWord_FindsTheIssue(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "login-bug.md", issueText("Login fails", "open", "The OAuth callback drops the session.\n"))
	writeIssue(t, s, "other-task.md", issueText("Something else", "open", "unrelated\n"))
	x := openIndex(t, s)
	refresh(t, x)
	cases := map[string][]string{
		"callback":            {"login-bug"},
		"SESSION oauth":       {"login-bug"},
		"login":               {"login-bug"},
		"absent":              nil,
		`callback" OR "other`: nil,
	}

	for query, want := range cases {
		got, err := x.Search(query)

		if err != nil || !slices.Equal(got, want) {
			t.Errorf("Search(%q) = %v, %v; want %v", query, got, err, want)
		}
	}
}

func TestRebuild_ExistingIndex_ReparsesEverything(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "first-issue.md", issueText("First", "open", ""))
	x := openIndex(t, s, laterClock())
	refresh(t, x)

	report, err := x.refresh(true)

	if err != nil || !report.rebuilt || len(report.parsed) != 1 || report.Indexed != 1 {
		t.Errorf("Rebuild = %+v, %v; want everything parsed again", report, err)
	}
}

func TestOpen_TreeChangedSinceLastOpen_ReturnsAFreshIndex(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	openIndex(t, s, laterClock())
	writeIssue(t, s, "late-issue.md", issueText("Late", "open", ""))

	x := openIndex(t, s, laterClock())

	report := x.Report()
	if !isIndexed(t, x, "late-issue") || report.Indexed != 1 || len(report.parsed) != 1 {
		t.Errorf("Report = %+v; want late-issue indexed by Open alone", report)
	}
}

func TestOpen_ModeRebuild_RebuildsTheIndex(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "first-issue.md", issueText("First", "open", ""))
	openIndex(t, s, laterClock())

	x, err := Open(s, ModeRebuild, laterClock())

	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := x.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	report := x.Report()
	if !report.rebuilt || len(report.parsed) != 1 || report.Indexed != 1 {
		t.Errorf("Report = %+v; want everything parsed again", report)
	}
}

func TestOpen_DatabaseUnreadable_FailsWithoutAnIndex(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := os.MkdirAll(filepath.Dir(Path(s)), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	garbage := []byte(strings.Repeat("not an index ", 512))
	if err := os.WriteFile(Path(s), garbage, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	x, err := Open(s, ModeRefresh)

	if err == nil || x != nil {
		t.Errorf("Open = %v, %v; want a failure and no index", x, err)
	}
}

func TestReport_CallerChangesIt_IndexKeepsItsOwn(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "Bad_Name.md", issueText("Bad", "open", ""))
	x := openIndex(t, s)

	x.Report().Warnings[0].Path = "renamed.md"

	if got := x.Report().Warnings[0].Path; got != ".capcom/issues/Bad_Name.md" {
		t.Errorf("Report().Warnings[0].Path = %q; want the index's own path", got)
	}
}

// nodeFileLimit is the largest node file the store reads, in bytes: 1 MiB.
const nodeFileLimit = 1 << 20

// oversizeIssueText is a valid issue one byte larger than nodeFileLimit, padded in its body.
func oversizeIssueText(title string) string {
	text := issueText(title, "open", "")
	return text + strings.Repeat("x", nodeFileLimit+1-len(text))
}

// makeSymlink creates link pointing at target, skipping the test on Windows and as root.
func makeSymlink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%s): %v", link, err)
	}
}

func TestRefresh_NonRegularNodeFiles_WarnOnceEachAndIndexTheRest(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	makeSymlink(t, "/dev/zero", filepath.Join(s.IssuesDir(), "zero-issue.md"))
	makeFIFO(t, filepath.Join(s.IssuesDir(), "pipe-issue.md"))
	writeIssue(t, s, "huge-issue.md", oversizeIssueText("Huge"))
	writeIssue(t, s, "z.md/child-issue.md", issueText("Child", "open", ""))
	writeIssue(t, s, "good-issue.md", issueText("Good", "open", ""))
	x := openIndex(t, s)

	report := refresh(t, x)

	want := []string{
		".capcom/issues/huge-issue.md " + string(diagnose.NotRegular),
		".capcom/issues/pipe-issue.md " + string(diagnose.NotRegular),
		".capcom/issues/z.md " + string(diagnose.OrphanDirectory),
		".capcom/issues/zero-issue.md " + string(diagnose.NotRegular),
	}
	if got := warningKinds(report); !slices.Equal(got, want) {
		t.Errorf("warning kinds = %q\nwant %q", got, want)
	}
	if report.Indexed != 1 || !isIndexed(t, x, "good-issue") {
		t.Errorf("report = %+v, want only good-issue indexed", report)
	}
	for _, id := range []string{"zero-issue", "pipe-issue", "huge-issue", "child-issue"} {
		if isIndexed(t, x, id) {
			t.Errorf("%s is indexed, want it left out", id)
		}
	}
}

func TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	path := writeIssue(t, s, "huge-issue.md", oversizeIssueText("Huge"))
	x := openIndex(t, s)
	ctx := context.Background()
	tx, err := x.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	r := &refresher{ctx: ctx, tx: tx, config: s.Config, refreshed: time.Now().UnixNano()}
	file := foundFile{
		path:     ".capcom/issues/huge-issue.md",
		absolute: path,
		kind:     kindIssue,
		id:       "huge-issue",
		size:     10,
	}

	isGone, err := r.indexFile(file)

	if err != nil || isGone {
		t.Fatalf("indexFile = gone %v, err %v; want a warning instead", isGone, err)
	}
	want := []string{".capcom/issues/huge-issue.md: file is larger than 1 MiB"}
	if got := warningStrings(Report{Warnings: r.warnings}); !slices.Equal(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
	wantKinds := []string{".capcom/issues/huge-issue.md " + string(diagnose.NotRegular)}
	if got := warningKinds(Report{Warnings: r.warnings}); !slices.Equal(got, wantKinds) {
		t.Errorf("warning kinds = %q, want %q", got, wantKinds)
	}
}

func TestRefresh_BadNames_WarnAndSkip(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "Bad_Name.md", issueText("Bad", "open", ""))
	writeIssue(t, s, "Bad_Name/child-issue.md", issueText("Child", "open", ""))
	writeIssue(t, s, "orphan-dir/lost-issue.md", issueText("Lost", "open", ""))
	writeIssue(t, s, "good-issue.md", issueText("Good", "open", ""))
	x := openIndex(t, s)

	report := refresh(t, x)

	if report.Indexed != 1 || len(report.Warnings) != 3 {
		t.Errorf("report = %+v, want 1 indexed and 3 warnings", report)
	}
}

func TestRefresh_DirectoryWarnings_WordedAsTheTreeWalk(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "Bad_Name.md", issueText("Bad", "open", ""))
	writeIssue(t, s, "Bad_Name/child-issue.md", issueText("Child", "open", ""))
	writeIssue(t, s, "orphan-dir/lost-issue.md", issueText("Lost", "open", ""))
	x := openIndex(t, s)

	report := refresh(t, x)

	want := map[string]string{
		".capcom/issues/Bad_Name":   "directory left out: its node file Bad_Name.md has an invalid id",
		".capcom/issues/orphan-dir": "directory without its node file: no orphan-dir.md beside it; left out",
	}
	got := map[string]string{}
	for _, warning := range report.Warnings {
		if warning.Code == diagnose.OrphanDirectory {
			got[warning.Path] = warning.Message
		}
	}
	if !maps.Equal(got, want) {
		t.Errorf("orphan directory warnings = %q\nwant %q", got, want)
	}
}

func TestRefresh_EachProducer_SetsTheWarningKindAndReplaysIt(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "Bad_Name.md", issueText("Bad", "open", ""))
	writeIssue(t, s, "Bad_Name/child-issue.md", issueText("Child", "open", ""))
	writeIssue(t, s, "orphan-dir/lost-issue.md", issueText("Lost", "open", ""))
	writeIssue(t, s, "broken-issue.md", "---\ntitle: Broken\nstatus: open\nno closing fence\n")
	writeIssue(t, s, "bad-status.md", issueText("Bad status", "nonsense", ""))
	writeIssue(t, s, "bad-alias.md", relatedIssueText("Bad alias", "aliases: [Bad_Alias]\n"))
	writeIssue(t, s, "own-alias.md", relatedIssueText("Own alias", "aliases: [own-alias]\n"))
	writeIssue(t, s, "twice-alias.md", relatedIssueText("Twice alias", "aliases: [twin-name, twin-name]\n"))
	writeIssue(t, s, "top-issue.md", issueText("Top", "open", ""))
	writeIssue(t, s, "top-issue/dup-issue.md", issueText("Nested dup", "open", ""))
	writeIssue(t, s, "dup-issue.md", issueText("Top dup", "open", ""))
	writeIssue(t, s, "huge-issue.md", oversizeIssueText("Huge"))
	writeIssue(t, s, "z.md/child-issue.md", issueText("Child", "open", ""))
	x := openIndex(t, s, laterClock())

	first := refresh(t, x)
	second := refresh(t, x)

	want := []string{
		".capcom/issues/Bad_Name " + string(diagnose.OrphanDirectory),
		".capcom/issues/Bad_Name.md " + string(diagnose.InvalidID),
		".capcom/issues/bad-alias.md " + string(diagnose.InvalidAlias),
		".capcom/issues/bad-status.md " + string(diagnose.Field),
		".capcom/issues/broken-issue.md " + string(diagnose.Frontmatter),
		".capcom/issues/dup-issue.md " + string(diagnose.Duplicate),
		".capcom/issues/orphan-dir " + string(diagnose.OrphanDirectory),
		".capcom/issues/own-alias.md " + string(diagnose.RepeatedAlias),
		".capcom/issues/top-issue/dup-issue.md " + string(diagnose.Duplicate),
		".capcom/issues/twice-alias.md " + string(diagnose.RepeatedAlias),
		".capcom/issues/huge-issue.md " + string(diagnose.NotRegular),
		".capcom/issues/z.md " + string(diagnose.OrphanDirectory),
	}
	slices.Sort(want)
	for name, report := range map[string]Report{"first": first, "second": second} {
		if got := warningKinds(report); !slices.Equal(got, want) {
			t.Errorf("%s refresh: warning kinds = %q\nwant %q", name, got, want)
		}
	}
	// The second refresh parses nothing: its parse problems are the stored ones, replayed.
	if len(second.parsed) != 0 {
		t.Errorf("second refresh parsed %v, want nothing", second.parsed)
	}
}

// The check package pins the same fixture's problems in TestRun_FileProblems_PinCodesAndWording.
func TestRefresh_FileProblems_PinKindsAndWording(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "unknown-key.md", relatedIssueText("Some issue", "estimate: 3\n"))
	writeIssue(t, s, "bad-status.md", issueText("Some issue", "wip", ""))
	writeIssue(t, s, "bad-alias.md", relatedIssueText("Some issue", "aliases: [Bad_Alias]\n"))
	writeIssue(t, s, "own-alias.md", relatedIssueText("Some issue", "aliases: [own-alias]\n"))
	writeIssue(t, s, "top-issue.md", issueText("Some issue", "open", ""))
	writeIssue(t, s, "top-issue/dup-issue.md", issueText("Some issue", "open", ""))
	writeIssue(t, s, "dup-issue.md", issueText("Some issue", "open", ""))
	outside := filepath.Join(s.WorktreeRoot, "outside.md")
	if err := os.WriteFile(outside, []byte(issueText("Some issue", "open", "")), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	makeSymlink(t, outside, filepath.Join(s.IssuesDir(), "link-issue.md"))
	x := openIndex(t, s)

	report := refresh(t, x)

	const dir = ".capcom/issues/"
	want := []diagnose.Finding{
		{
			Code:    diagnose.InvalidAlias,
			Path:    dir + "bad-alias.md",
			Line:    7,
			Message: `invalid alias "Bad_Alias": must use only a-z, 0-9, dots and hyphens`,
		},
		{
			Code:    diagnose.Field,
			Path:    dir + "bad-status.md",
			Line:    3,
			Message: `status "wip" is not declared in config; use one of [open active parked done dropped]`,
		},
		{
			Code: diagnose.Duplicate,
			Path: dir + "dup-issue.md",
			Message: `duplicate id "dup-issue": also the id of ` + dir + "top-issue/dup-issue.md; " +
				"both are left out",
		},
		{Code: diagnose.NotRegular, Path: dir + "link-issue.md", Message: "not a regular file"},
		{
			Code:    diagnose.RepeatedAlias,
			Path:    dir + "own-alias.md",
			Line:    7,
			Message: `alias "own-alias" repeats the issue's own id`,
		},
		{
			Code: diagnose.Duplicate,
			Path: dir + "top-issue/dup-issue.md",
			Message: `duplicate id "dup-issue": also the id of ` + dir + "dup-issue.md; " +
				"both are left out",
		},
		{
			Code: diagnose.Frontmatter,
			Path: dir + "unknown-key.md",
			Line: 7,
			Message: `unknown key "estimate"; an issue has title, status, type, priority, labels, deps, ` +
				"related, plan, standing, aliases, created, closed and reason",
		},
	}
	if !slices.Equal(report.Warnings, want) {
		t.Errorf("warnings = %#v\nwant %#v", report.Warnings, want)
	}
}

// relatedIssueText is the content of a valid open issue whose frontmatter ends with extra lines.
func relatedIssueText(title, extra string) string {
	return strings.Replace(issueText(title, "open", ""), "\n---\n", "\n"+extra+"---\n", 1)
}

// mustRelatedBy returns RelatedBy(id) and fails the test on error.
func mustRelatedBy(t *testing.T, x *Index, id string) []string {
	t.Helper()
	holders, err := x.RelatedBy(id)
	if err != nil {
		t.Fatalf("RelatedBy(%q): %v", id, err)
	}
	return holders
}

func TestRefresh_RelatedLinks_FilledAfterRefreshAndAnEdit(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "target-issue.md", issueText("Target", "open", ""))
	writeIssue(t, s, "other-issue.md", issueText("Other", "open", ""))
	writeIssue(t, s, "holder-issue.md", relatedIssueText("Holder", "related: [target-issue, other-issue]\n"))
	x := openIndex(t, s, laterClock())
	refresh(t, x)

	related := mustIssue(t, x, "holder-issue").Related
	holders := mustRelatedBy(t, x, "target-issue")

	if want := []string{"target-issue", "other-issue"}; !slices.Equal(related, want) {
		t.Errorf("Related = %v, want %v", related, want)
	}
	if want := []string{"holder-issue"}; !slices.Equal(holders, want) {
		t.Errorf("RelatedBy(target-issue) = %v, want %v", holders, want)
	}

	writeIssue(t, s, "holder-issue.md", relatedIssueText("Holder, edited", "related: [other-issue]\n"))
	refresh(t, x)

	if related := mustIssue(t, x, "holder-issue").Related; !slices.Equal(related, []string{"other-issue"}) {
		t.Errorf("Related after the edit = %v, want [other-issue]", related)
	}
	if holders := mustRelatedBy(t, x, "target-issue"); len(holders) != 0 {
		t.Errorf("RelatedBy(target-issue) after the edit = %v, want none", holders)
	}
}

func TestRelatedBy_HolderNamesAnAlias_FindsTheHolder(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "target-issue.md", relatedIssueText("Target", "aliases: [old-target]\n"))
	writeIssue(t, s, "holder-issue.md", relatedIssueText("Holder", "related: [old-target]\n"))
	writeIssue(t, s, "both-issue.md", relatedIssueText("Both", "related: [target-issue, old-target]\n"))
	x := openIndex(t, s, laterClock())
	refresh(t, x)

	holders := mustRelatedBy(t, x, "target-issue")

	if want := []string{"both-issue", "holder-issue"}; !slices.Equal(holders, want) {
		t.Errorf("RelatedBy(target-issue) = %v, want %v", holders, want)
	}
}

func TestRelatedBy_DuplicateExcludedHolder_IsSkipped(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "target-issue.md", issueText("Target", "open", ""))
	writeIssue(t, s, "dup-issue.md", relatedIssueText("Top dup", "related: [target-issue]\n"))
	writeIssue(t, s, "target-issue/dup-issue.md", relatedIssueText("Nested dup", "related: [target-issue]\n"))
	writeIssue(t, s, "kept-issue.md", relatedIssueText("Kept", "related: [target-issue]\n"))
	x := openIndex(t, s, laterClock())
	refresh(t, x)

	holders := mustRelatedBy(t, x, "target-issue")

	if want := []string{"kept-issue"}; !slices.Equal(holders, want) {
		t.Errorf("RelatedBy(target-issue) = %v, want %v", holders, want)
	}
}

func TestRelatedBy_AliasAFileBelowARepeatedIDShares_NamesNoHolder(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "target-issue.md", relatedIssueText("Target", "aliases: [old-target]\n"))
	writeIssue(t, s, "holder-issue.md", relatedIssueText("Holder", "related: [old-target]\n"))
	writeIssue(t, s, "a.md", issueText("A", "open", ""))
	writeIssue(t, s, "a/x.md", issueText("X one", "open", ""))
	writeIssue(t, s, "b.md", issueText("B", "open", ""))
	writeIssue(t, s, "b/x.md", issueText("X two", "open", ""))
	writeIssue(t, s, "b/x/below-issue.md", relatedIssueText("Below", "aliases: [old-target]\n"))
	x := openIndex(t, s, laterClock())
	refresh(t, x)

	holders := mustRelatedBy(t, x, "target-issue")

	if len(holders) != 0 {
		t.Errorf("RelatedBy(target-issue) = %v, want none: a file below the repeated id x shares old-target", holders)
	}
}

func TestRefresh_IndexFromBeforeRelatedLinks_RebuildsWithThem(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "target-issue.md", issueText("Target", "open", ""))
	writeIssue(t, s, "holder-issue.md", relatedIssueText("Holder", "related: [target-issue]\n"))
	x := openIndex(t, s, laterClock())
	refresh(t, x)
	db, err := sql.Open(driverName, Path(s))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// An index of the previous schema version has no related table.
	_, err = db.Exec(`DROP TABLE related`)
	if err == nil {
		_, err = db.Exec(`UPDATE meta SET value = ? WHERE key = ?`, schemaVersion-1, metaSchemaVersion)
	}
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("downgrade the index: %v, %v", err, closeErr)
	}

	report := refresh(t, x)

	if !report.rebuilt {
		t.Errorf("report = %+v, want a rebuild", report)
	}
	if holders := mustRelatedBy(t, x, "target-issue"); !slices.Equal(holders, []string{"holder-issue"}) {
		t.Errorf("RelatedBy(target-issue) = %v, want [holder-issue]", holders)
	}
}

func TestRefresh_StandingIssue_ReadsBackTheMark(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "epic.md", relatedIssueText("Epic", "standing: true\n"))
	writeIssue(t, s, "epic/child-task.md", issueText("Child task", "open", ""))
	writeIssue(t, s, "lone-task.md", issueText("Lone task", "open", ""))
	x := openIndex(t, s, laterClock())

	refresh(t, x)

	for id, isStanding := range map[string]bool{"epic": true, "child-task": false, "lone-task": false} {
		entry := mustIssue(t, x, id)
		if entry.IsStanding != isStanding {
			t.Errorf("Issue(%q).IsStanding = %v, want %v", id, entry.IsStanding, isStanding)
		}
		if node := entry.Node(); node.IsStanding != isStanding {
			t.Errorf("Issue(%q).Node().IsStanding = %v, want %v", id, node.IsStanding, isStanding)
		}
	}
}

func TestRefresh_IndexFromBeforeTheStandingMark_RebuildsWithIt(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "epic.md", relatedIssueText("Epic", "standing: true\n"))
	x := openIndex(t, s, laterClock())
	refresh(t, x)
	db, err := sql.Open(driverName, Path(s))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// An index of the previous schema version has no standing column.
	_, err = db.Exec(`ALTER TABLE issues DROP COLUMN standing`)
	if err == nil {
		_, err = db.Exec(`UPDATE meta SET value = ? WHERE key = ?`, schemaVersion-1, metaSchemaVersion)
	}
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("downgrade the index: %v, %v", err, closeErr)
	}

	report := refresh(t, x)

	if !report.rebuilt {
		t.Errorf("report = %+v, want a rebuild", report)
	}
	if entry := mustIssue(t, x, "epic"); !entry.IsStanding {
		t.Errorf("Issue(epic).IsStanding = false after the rebuild, want true")
	}
}

func TestRefresh_IndexFromBeforeClaims_RebuildsWithThem(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "foo-bar.md", issueText("Foo bar", "open", ""))
	writeIssue(t, s, "foo-rework.md", issueWith("Rework", "bogus", "aliases: [foo]\n"))
	x := openIndex(t, s, laterClock())
	refresh(t, x)
	db, err := sql.Open(driverName, Path(s))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// An index of the previous schema version has no claims table.
	_, err = db.Exec(`DROP TABLE claims`)
	if err == nil {
		_, err = db.Exec(`UPDATE meta SET value = ? WHERE key = ?`, schemaVersion-1, metaSchemaVersion)
	}
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("downgrade the index: %v, %v", err, closeErr)
	}

	report := refresh(t, x)

	if !report.rebuilt {
		t.Errorf("report = %+v, want a rebuild", report)
	}
	if _, err := x.Resolve("foo"); !errors.Is(err, tree.ErrNotFound) ||
		!strings.Contains(err.Error(), ".capcom/issues/foo-rework.md is invalid") {
		t.Errorf("Resolve(foo) err = %v, want not found naming foo-rework.md", err)
	}
}

func TestRefresh_WalkRefusedFileFixed_DropsItsClaim(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "foo-bar.md", issueText("Foo bar", "open", ""))
	writeIssue(t, s, "foo.md", oversizeIssueText("Foo"))
	x := openIndex(t, s)
	if _, err := x.Resolve("foo"); !errors.Is(err, tree.ErrNotFound) {
		t.Fatalf("Resolve(foo) before the fix err = %v, want not found", err)
	}
	writeIssue(t, s, "foo.md", issueText("Foo", "open", ""))

	refresh(t, x)

	if entry, err := x.Resolve("foo"); err != nil || entry.ID != "foo" {
		t.Errorf("Resolve(foo) = %q, %v; want foo", entry.ID, err)
	}
}

// TestRefresh_CommitFails_ReportsOnlyTheCommitError swaps the package's commitTx seam, so it
// never runs in parallel.
func TestRefresh_CommitFails_ReportsOnlyTheCommitError(t *testing.T) {
	s := newStore(t)
	writeIssue(t, s, "an-issue.md", issueText("An issue", "open", ""))
	x := openIndex(t, s)
	errCommit := errors.New("disk I/O error")
	original := commitTx
	t.Cleanup(func() { commitTx = original })
	// The fake ends the transaction as a failed commit does, so the deferred rollback meets
	// sql.ErrTxDone for real.
	commitTx = func(tx *sql.Tx) error {
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Rollback: %v", err)
		}
		return errCommit
	}

	_, err := x.refresh(false)

	if !errors.Is(err, errCommit) {
		t.Fatalf("Refresh error = %v, want it to wrap the commit failure", err)
	}
	if errors.Is(err, sql.ErrTxDone) {
		t.Errorf("Refresh error = %v, want no rollback error after the failed commit", err)
	}
	if !strings.Contains(err.Error(), "commit refresh: disk I/O error") {
		t.Errorf("Refresh error = %q, want it to name the commit failure", err)
	}
}

// TestRefresh_FileGoneBeforeItsRead_IsDroppedAndNotParsed swaps the package's readNodeFile seam,
// so it never runs in parallel.
func TestRefresh_FileGoneBeforeItsRead_IsDroppedAndNotParsed(t *testing.T) {
	s := newStore(t)
	path := writeIssue(t, s, "gone-issue.md", issueText("Gone", "open", "unique word\n"))
	x := openIndex(t, s, laterClock())
	writeIssue(t, s, "gone-issue.md", issueText("Gone", "open", "unique word, longer now\n"))
	original := readNodeFile
	t.Cleanup(func() { readNodeFile = original })
	readNodeFile = func(name string) ([]byte, error) {
		if err := os.Remove(path); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		return original(name)
	}

	report := refresh(t, x)

	found, err := x.Search("unique")
	if isIndexed(t, x, "gone-issue") || err != nil || len(found) != 0 {
		t.Errorf("gone-issue still indexed: search = %v, err %v", found, err)
	}
	if len(report.parsed) != 0 {
		t.Errorf("Parsed = %v, want nothing", report.parsed)
	}
	if want := []string{".capcom/issues/gone-issue.md"}; !slices.Equal(report.removed, want) {
		t.Errorf("Removed = %v, want %v", report.removed, want)
	}
}

// TestRefresh_FileGoneBeforeItsStat_IsRemoved swaps the package's lstat seam, so it never runs in
// parallel.
func TestRefresh_FileGoneBeforeItsStat_IsRemoved(t *testing.T) {
	s := newStore(t)
	path := writeIssue(t, s, "gone-issue.md", issueText("Gone", "open", ""))
	writeIssue(t, s, "kept-issue.md", issueText("Kept", "open", ""))
	x := openIndex(t, s, laterClock())
	original := lstat
	t.Cleanup(func() { lstat = original })
	lstat = func(name string) (os.FileInfo, error) {
		if name == path {
			if err := os.Remove(path); err != nil {
				t.Fatalf("Remove: %v", err)
			}
		}
		return original(name)
	}

	report := refresh(t, x)

	if want := []string{".capcom/issues/gone-issue.md"}; !slices.Equal(report.removed, want) || report.Indexed != 1 {
		t.Errorf("report = %+v, want %v removed and 1 indexed", report, want)
	}
	if isIndexed(t, x, "gone-issue") {
		t.Errorf("gone-issue still indexed")
	}
}

func TestRefresh_BadStoredProblems_FailsNamingTheFile(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "broken-issue.md", "---\ntitle: Broken\nstatus: open\nno closing fence\n")
	x := openIndex(t, s, laterClock())
	db, err := sql.Open(driverName, Path(s))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = db.Exec(`UPDATE files SET problems = '{' WHERE path = ?`, ".capcom/issues/broken-issue.md")
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("corrupt the stored problems: %v, %v", err, closeErr)
	}

	_, err = x.refresh(false)

	if err == nil || !strings.Contains(err.Error(), ".capcom/issues/broken-issue.md") {
		t.Errorf("Refresh error = %v, want one naming .capcom/issues/broken-issue.md", err)
	}
}

// concurrentRounds is how many times each goroutine of the concurrent refresh test acts.
const concurrentRounds = 20

// editedIssueText is the edited issue's content after edit round: its body grows by one byte a
// round, so every edit changes the file's size and no refresh can take an edit for clean.
func editedIssueText(round int) string {
	return issueText(fmt.Sprintf("Edit %d", round), "open", strings.Repeat("x", round)+"\n")
}

// refreshRounds refreshes x concurrentRounds times and sends every failure to failures.
func refreshRounds(x *Index, failures chan<- error) {
	for range concurrentRounds {
		if _, err := x.refresh(false); err != nil {
			failures <- err
		}
	}
}

// editRounds rewrites the file at path concurrentRounds times in place, as an editor saves it,
// and sends every failure to failures.
func editRounds(path string, failures chan<- error) {
	for round := 1; round <= concurrentRounds; round++ {
		if err := os.WriteFile(path, []byte(editedIssueText(round)), 0o644); err != nil {
			failures <- err
		}
	}
}

func TestRefresh_ConcurrentIndexesAndAnEdit_NeitherFailsAndBothSeeTheSameRows(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "first-issue.md", issueText("First", "open", ""))
	writeIssue(t, s, "first-issue/child-issue.md", issueText("Child", "open", ""))
	writeIssue(t, s, "second-issue.md", issueText("Second", "open", ""))
	edited := writeIssue(t, s, "edited-issue.md", editedIssueText(0))
	first := openIndex(t, s)
	second := openIndex(t, s)
	var waitGroup sync.WaitGroup
	failures := make(chan error, 3*concurrentRounds)

	for _, act := range []func(){
		func() { refreshRounds(first, failures) },
		func() { refreshRounds(second, failures) },
		func() { editRounds(edited, failures) },
	} {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			act()
		}()
	}
	waitGroup.Wait()
	close(failures)
	firstReport := refresh(t, first)
	secondReport := refresh(t, second)

	// A busy or locked database is the failure this test exists for; any other is a failure too.
	for err := range failures {
		t.Errorf("concurrent refresh or edit failed: %v", err)
	}
	want := []string{"edited-issue", "first-issue", "child-issue", "second-issue"}
	for name, x := range map[string]*Index{"first": first, "second": second} {
		ids, err := x.ListIDs(Filter{IsClosedIncluded: true})
		if err != nil {
			t.Fatalf("%s index ListIDs: %v", name, err)
		}
		if !slices.Equal(ids, want) {
			t.Errorf("%s index ids = %v, want %v, each once", name, ids, want)
		}
		if title := mustIssue(t, x, "edited-issue").Title; title != fmt.Sprintf("Edit %d", concurrentRounds) {
			t.Errorf("%s index edited-issue title = %q, want the last edit's", name, title)
		}
	}
	if firstReport.Indexed != len(want) || secondReport.Indexed != len(want) {
		t.Errorf("final refreshes indexed %d and %d issues, want %d each",
			firstReport.Indexed, secondReport.Indexed, len(want))
	}
}
