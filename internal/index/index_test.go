package index

import (
	"database/sql"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
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
	return WithClock(func() time.Time { return time.Now().Add(time.Hour) })
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
	report, err := x.Refresh()
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
		lines = append(lines, warning.Path+" "+string(warning.Kind))
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

	if !report.Rebuilt || report.Indexed != 2 || len(report.Parsed) != 2 || len(report.Warnings) != 0 {
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

	if want := []string{".capcom/issues/second-issue.md"}; !slices.Equal(report.Parsed, want) || report.Rebuilt {
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
	x := openIndex(t, s, WithClock(func() time.Time { return tick.Add(200 * time.Millisecond) }))
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

	if want := []string{".capcom/issues/gone-issue.md"}; !slices.Equal(report.Removed, want) || report.Indexed != 1 {
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
	if len(second.Parsed) != 0 {
		t.Errorf("second refresh parsed %v, want nothing", second.Parsed)
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
		{
			name: "alias of another issue",
			files: map[string]string{
				"real-issue.md": issueText("Real", "open", ""),
				"other-issue.md": "---\ntitle: Other\nstatus: open\ntype: task\naliases: [real-issue]\n" +
					"created: 2026-10-11T09:00:00Z\n---\n",
			},
			excluded: []string{"real-issue", "other-issue"},
			rivals:   []string{"real-issue.md", "other-issue.md"},
			remove:   "other-issue.md",
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

	if !report.Rebuilt {
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

	if !report.Rebuilt || len(report.Parsed) != 1 || report.Indexed != 1 {
		t.Errorf("report = %+v, want a rebuild that parsed and indexed the one issue", report)
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

	report, err := x.Rebuild()

	if err != nil || !report.Rebuilt || len(report.Parsed) != 1 || report.Indexed != 1 {
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
	if !isIndexed(t, x, "late-issue") || report.Indexed != 1 || len(report.Parsed) != 1 {
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
	if !report.Rebuilt || len(report.Parsed) != 1 || report.Indexed != 1 {
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
		if warning.Kind == WarningOrphanDirectory {
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
	x := openIndex(t, s, laterClock())

	first := refresh(t, x)
	second := refresh(t, x)

	want := []string{
		".capcom/issues/Bad_Name " + string(WarningOrphanDirectory),
		".capcom/issues/Bad_Name.md " + string(WarningInvalidID),
		".capcom/issues/bad-alias.md " + string(WarningInvalidAlias),
		".capcom/issues/bad-status.md " + string(WarningField),
		".capcom/issues/broken-issue.md " + string(WarningFrontmatter),
		".capcom/issues/dup-issue.md " + string(WarningDuplicate),
		".capcom/issues/orphan-dir " + string(WarningOrphanDirectory),
		".capcom/issues/own-alias.md " + string(WarningRepeatedAlias),
		".capcom/issues/top-issue/dup-issue.md " + string(WarningDuplicate),
		".capcom/issues/twice-alias.md " + string(WarningRepeatedAlias),
	}
	slices.Sort(want)
	for name, report := range map[string]Report{"first": first, "second": second} {
		if got := warningKinds(report); !slices.Equal(got, want) {
			t.Errorf("%s refresh: warning kinds = %q\nwant %q", name, got, want)
		}
	}
	// The second refresh parses nothing: its parse problems are the stored ones, replayed.
	if len(second.Parsed) != 0 {
		t.Errorf("second refresh parsed %v, want nothing", second.Parsed)
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

	if !report.Rebuilt {
		t.Errorf("report = %+v, want a rebuild", report)
	}
	if holders := mustRelatedBy(t, x, "target-issue"); !slices.Equal(holders, []string{"holder-issue"}) {
		t.Errorf("RelatedBy(target-issue) = %v, want [holder-issue]", holders)
	}
}
