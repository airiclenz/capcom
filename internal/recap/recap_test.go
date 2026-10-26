package recap

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/store"
)

// Log entries the tests write.
const (
	entrySeen    = "- 2026-10-20 Bob: seen"
	entryStarted = "- 2026-10-22 Alice: started\n  on the login page"
)

// git runs a (possibly writing) git command for test setup and returns its trimmed output.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

// newStore creates a hermetic git repository (ADR 0007) with an initialised, uncommitted store.
// Tests using it set environment variables and so do not call t.Parallel.
func newStore(t *testing.T) *store.Store {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	environment := map[string]string{
		"GIT_CONFIG_GLOBAL":       os.DevNull,
		"GIT_CONFIG_NOSYSTEM":     "1",
		"GIT_CEILING_DIRECTORIES": filepath.Dir(root),
		"GIT_AUTHOR_NAME":         "Test Author",
		"GIT_AUTHOR_EMAIL":        "author@example.com",
		"GIT_COMMITTER_NAME":      "Test Committer",
		"GIT_COMMITTER_EMAIL":     "committer@example.com",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	git(t, root, "init", "-q")
	opened, _, err := store.Init(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	return opened
}

// commit writes files, named by slash paths below .capcom/ (an empty content deletes the file),
// commits them and returns the new commit's object name.
func commit(t *testing.T, s *store.Store, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(s.Root, filepath.FromSlash(name))
		if content == "" {
			if err := os.Remove(path); err != nil {
				t.Fatalf("Remove: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	git(t, s.WorktreeRoot, "add", "-A")
	git(t, s.WorktreeRoot, "commit", "-q", "--allow-empty", "-m", "change")
	return git(t, s.WorktreeRoot, "rev-parse", "HEAD")
}

// issueFile is an issue file; extra holds more frontmatter lines, and entries make up its Log.
func issueFile(title, status, extra string, entries ...string) string {
	content := "---\ntitle: " + title + "\nstatus: " + status + "\ntype: task\n" + extra +
		"created: 2026-10-11T09:00:00Z\n---\n\nBody.\n"
	if len(entries) > 0 {
		content += "\n## Log\n\n" + strings.Join(entries, "\n") + "\n"
	}
	return content
}

// noteFile is a Note file; extra holds more frontmatter lines.
func noteFile(title, extra, body string) string {
	return "---\ntitle: " + title + "\n" + extra + "---\n\n" + body + "\n"
}

// baseline is the store every scripted test starts from.
var baseline = map[string]string{
	"issues/to-close.md":       issueFile("To close", "open", ""),
	"issues/to-change.md":      issueFile("To change", "open", ""),
	"issues/to-log.md":         issueFile("To log", "open", "", entrySeen),
	"issues/to-rename.md":      issueFile("To rename", "open", ""),
	"issues/to-remove.md":      issueFile("To remove", "open", ""),
	"issues/untouched.md":      issueFile("Untouched", "done", "reason: shipped\n"),
	"knowledge/note-change.md": noteFile("Note to change", "", "Old."),
	"knowledge/note-retire.md": noteFile("Note to retire", "", "Holds."),
	"knowledge/note-remove.md": noteFile("Note to remove", "", "Gone soon."),
	"knowledge/note-rename.md": noteFile("Note to rename", "", "Renamed."),
	"knowledge/note-same.md":   noteFile("Note unchanged", "", "Same."),
}

func TestCompute_ScriptedCommits_ListEachChangeInItsGroup(t *testing.T) {
	s := newStore(t)
	boundary := commit(t, s, baseline)
	script := []map[string]string{
		{"issues/to-close.md": issueFile("To close", "done", "reason: fixed it\n")},
		{"issues/new-issue.md": issueFile("New issue", "open", "")},
		{"issues/filed-closed.md": issueFile("Filed and closed", "dropped", "reason: dup\n")},
		{"issues/to-log.md": issueFile("To log", "open", "", entrySeen, entryStarted)},
		{"issues/to-change.md": issueFile("To change", "active", "")},
		{
			"issues/to-rename.md":            "",
			"issues/to-close/renamed.md":     issueFile("Renamed", "open", "aliases: [to-rename]\n"),
			"issues/to-change/.scratch.md":   issueFile("Hidden", "open", ""),
			"issues/to-change/notes.txt":     "not an issue\n",
			"knowledge/note-rename.md":       "",
			"knowledge/area/note-renamed.md": noteFile("Note renamed", "aliases: [note-rename]\n", "Renamed."),
		},
		{"issues/to-remove.md": ""},
		{"knowledge/note-added.md": noteFile("Note added", "", "New.")},
		{"knowledge/note-change.md": noteFile("Note to change", "", "New.")},
		{"knowledge/note-retire.md": noteFile("Note to retire", "retired: true\n", "No longer.")},
		{"knowledge/note-remove.md": ""},
	}
	for _, files := range script {
		commit(t, s, files)
	}

	report, err := Compute(s, boundary)

	want := Report{
		Closed: []Entry{
			{ID: "filed-closed", Title: "Filed and closed", Status: "dropped", Reason: "dup"},
			{ID: "to-close", Title: "To close", Status: "done", Reason: "fixed it"},
		},
		Filed:         []Entry{{ID: "filed-closed", Title: "Filed and closed"}, {ID: "new-issue", Title: "New issue"}},
		StatusChanged: []Entry{{ID: "to-change", Title: "To change", Status: "active"}},
		Log:           []Entry{{ID: "to-log", Title: "To log", LogEntry: entryStarted}},
		NotesAdded:    []Entry{{ID: "note-added", Title: "Note added"}},
		NotesChanged: []Entry{
			{ID: "note-change", Title: "Note to change"},
			{ID: "note-renamed", Title: "Note renamed"},
		},
		NotesRetired: []Entry{
			{ID: "note-remove", Title: "Note to remove"},
			{ID: "note-retire", Title: "Note to retire"},
		},
	}
	if err != nil || !reflect.DeepEqual(report, want) {
		t.Errorf("Compute = %+v, %v;\nwant %+v", report, err, want)
	}
}

func TestCompute_NoBoundary_ListsEverythingAsNew(t *testing.T) {
	s := newStore(t)
	commit(t, s, map[string]string{
		"issues/open-one.md":   issueFile("Open one", "open", "", entrySeen),
		"issues/closed-one.md": issueFile("Closed one", "done", "reason: shipped\n"),
		"knowledge/a-note.md":  noteFile("A note", "", "Holds."),
	})

	report, err := Compute(s, "")

	want := Report{
		Closed:     []Entry{{ID: "closed-one", Title: "Closed one", Status: "done", Reason: "shipped"}},
		Filed:      []Entry{{ID: "closed-one", Title: "Closed one"}, {ID: "open-one", Title: "Open one"}},
		Log:        []Entry{{ID: "open-one", Title: "Open one", LogEntry: entrySeen}},
		NotesAdded: []Entry{{ID: "a-note", Title: "A note"}},
	}
	if err != nil || !reflect.DeepEqual(report, want) {
		t.Errorf("Compute = %+v, %v;\nwant %+v", report, err, want)
	}
}

func TestCompute_ReopenedIssue_IsAStatusChangeWithItsLogEntry(t *testing.T) {
	s := newStore(t)
	boundary := commit(t, s, map[string]string{
		"issues/back.md": issueFile("Back", "done", "reason: shipped\n"),
	})
	reopened := "- 2026-10-25 Bob: reopened: still broken"
	commit(t, s, map[string]string{"issues/back.md": issueFile("Back", "open", "", reopened)})

	report, err := Compute(s, boundary)

	want := Report{
		StatusChanged: []Entry{{ID: "back", Title: "Back", Status: "open"}},
		Log:           []Entry{{ID: "back", Title: "Back", LogEntry: reopened}},
	}
	if err != nil || !reflect.DeepEqual(report, want) {
		t.Errorf("Compute = %+v, %v;\nwant %+v", report, err, want)
	}
}

func TestCompute_UnparsableFile_IsAWarningAndLeftOut(t *testing.T) {
	s := newStore(t)
	boundary := commit(t, s, baseline)
	commit(t, s, map[string]string{
		"issues/to-close.md":       "no frontmatter\n",
		"knowledge/note-change.md": "---\ntitle: [unclosed\n---\n",
	})

	report, err := Compute(s, boundary)

	isEmpty := len(report.Closed)+len(report.StatusChanged)+len(report.NotesChanged) == 0
	if err != nil || !isEmpty || len(report.Warnings) != 2 {
		t.Fatalf("Compute = %+v, %v; want two warnings and nothing listed", report, err)
	}
	for _, warning := range report.Warnings {
		if !strings.HasPrefix(warning, "recap: ") {
			t.Errorf("warning %q lacks the recap: prefix", warning)
		}
	}
}

func TestCompute_UnknownBoundary_FailsWithErrUnknownBoundary(t *testing.T) {
	s := newStore(t)
	commit(t, s, baseline)

	_, err := Compute(s, "no-such-ref")

	if !errors.Is(err, ErrUnknownBoundary) {
		t.Errorf("Compute with an unknown boundary = %v; want ErrUnknownBoundary", err)
	}
}

func TestCompute_UnbornHead_IsEmptyWithoutError(t *testing.T) {
	s := newStore(t)

	report, err := Compute(s, "")

	if err != nil || !reflect.DeepEqual(report, Report{}) {
		t.Errorf("Compute on an unborn HEAD = %+v, %v; want an empty report", report, err)
	}
}
