package index

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// writeNote writes content to the Note file at name, a slash path below the Knowledge tree.
func writeNote(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	path := filepath.Join(s.KnowledgeDir(), filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// noteIDs returns the ids of entries, in order.
func noteIDs(entries []NoteEntry) []string {
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.ID)
	}
	return values
}

// noteFixture indexes a small Knowledge tree next to one issue sharing a Note's id.
func noteFixture(t *testing.T) (*store.Store, *Index) {
	t.Helper()
	s := newStore(t)
	writeNote(t, s, "testing-rules.md", "---\ntitle: Testing rules\nlabels: [invariant]\n---\nTests use real temp dirs.\n")
	writeNote(t, s, "testing-rules/no-parallel-cli.md",
		"---\ntitle: No parallel CLI tests\nlabels: [invariant, cli]\nrefs: [internal/cli/testharness_test.go#runCLI]\n"+
			"aliases: [old-cli-rule]\n---\nThe harness changes the working directory.\n")
	writeNote(t, s, "release-flow.md", "---\ntitle: Release flow\n---\nTag, then publish.\n")
	writeIssue(t, s, "release-flow.md", issueText("Release flow issue", "open", "Ship the tag.\n"))
	x := openIndex(t, s)
	refresh(t, x)
	return s, x
}

func TestNotes_KnowledgeTree_IndexedInTreeOrderWithParentsAndLists(t *testing.T) {
	t.Parallel()
	_, x := noteFixture(t)

	notes, err := x.Notes()

	if err != nil {
		t.Fatalf("Notes: %v", err)
	}
	if got := noteIDs(notes); !slices.Equal(got, []string{"release-flow", "testing-rules", "no-parallel-cli"}) {
		t.Fatalf("Notes ids = %v", got)
	}
	child := notes[2]
	if child.ParentID != "testing-rules" || child.Path != ".capcom/knowledge/testing-rules/no-parallel-cli.md" ||
		!slices.Equal(child.Labels, []string{"invariant", "cli"}) ||
		!slices.Equal(child.Refs, []string{"internal/cli/testharness_test.go#runCLI"}) ||
		!slices.Equal(child.Aliases, []string{"old-cli-rule"}) {
		t.Errorf("child note = %+v", child)
	}
}

func TestNotes_IssueAndNoteShareAnID_BothStayUsable(t *testing.T) {
	t.Parallel()
	_, x := noteFixture(t)

	issueEntry, issueErr := x.Resolve("release-flow")
	noteEntry, noteErr := x.ResolveNote("release-flow")

	if issueErr != nil || issueEntry.Title != "Release flow issue" {
		t.Errorf("Resolve = %+v, %v; want the issue", issueEntry, issueErr)
	}
	if noteErr != nil || noteEntry.Title != "Release flow" {
		t.Errorf("ResolveNote = %+v, %v; want the note", noteEntry, noteErr)
	}
}

func TestNotes_ResolveNote_IDAliasAndPrefix(t *testing.T) {
	t.Parallel()
	_, x := noteFixture(t)
	cases := map[string]string{"no-parallel-cli": "no-parallel-cli", "old-cli-rule": "no-parallel-cli", "testing": "testing-rules"}

	for arg, want := range cases {
		entry, err := x.ResolveNote(arg)

		if err != nil || entry.ID != want {
			t.Errorf("ResolveNote(%q) = %q, %v; want %q", arg, entry.ID, err, want)
		}
	}
	if _, err := x.ResolveNote("absent-note"); !errors.Is(err, tree.ErrNotFound) {
		t.Errorf("ResolveNote(absent) = %v, want not found", err)
	}
}

func TestNotes_SearchNotes_MatchesWordsAndLabelsButIssueSearchIgnoresNotes(t *testing.T) {
	t.Parallel()
	_, x := noteFixture(t)

	byWord, wordErr := x.SearchNotes("harness", nil)
	byLabel, labelErr := x.SearchNotes("temp", []string{"invariant"})
	wrongLabel, wrongErr := x.SearchNotes("harness", []string{"absent"})
	issues, issueErr := x.Search("harness")

	if wordErr != nil || !slices.Equal(noteIDs(byWord), []string{"no-parallel-cli"}) {
		t.Errorf("SearchNotes(harness) = %v, %v", noteIDs(byWord), wordErr)
	}
	if labelErr != nil || !slices.Equal(noteIDs(byLabel), []string{"testing-rules"}) {
		t.Errorf("SearchNotes(temp, invariant) = %v, %v", noteIDs(byLabel), labelErr)
	}
	if wrongErr != nil || len(wrongLabel) != 0 {
		t.Errorf("SearchNotes(harness, absent) = %v, %v; want none", noteIDs(wrongLabel), wrongErr)
	}
	if issueErr != nil || len(issues) != 0 {
		t.Errorf("Search(harness) = %v, %v; want no issue", issues, issueErr)
	}
}

func TestNotes_InvalidAndDuplicateNotes_WarnAndStayOut(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNote(t, s, "no-title.md", "---\nlabels: [x]\n---\n")
	writeNote(t, s, "first-note.md", "---\ntitle: First\naliases: [second-note]\n---\n")
	writeNote(t, s, "second-note.md", "---\ntitle: Second\n---\n")
	writeIssue(t, s, "no-title.md", issueText("An issue with the same id", "open", ""))
	x := openIndex(t, s)

	report := refresh(t, x)
	notes, err := x.Notes()

	if err != nil || len(notes) != 0 {
		t.Errorf("Notes = %v, %v; want none usable", noteIDs(notes), err)
	}
	if report.Indexed != 1 {
		t.Errorf("Indexed = %d, want the one issue", report.Indexed)
	}
	warnings := strings.Join(warningStrings(report), "\n")
	for _, want := range []string{".capcom/knowledge/no-title.md: title is required", "duplicate alias \"second-note\""} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings = %s\nwant one containing %q", warnings, want)
		}
	}
}

func TestNotes_EachProducer_SetsTheWarningKind(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNote(t, s, "no-title.md", "---\nlabels: [x]\n---\n")
	writeNote(t, s, "broken-note.md", "---\ntitle: Broken\nno closing fence\n")
	writeNote(t, s, "first-note.md", "---\ntitle: First\naliases: [second-note]\n---\n")
	writeNote(t, s, "second-note.md", "---\ntitle: Second\n---\n")
	x := openIndex(t, s)

	report := refresh(t, x)

	want := []string{
		".capcom/knowledge/broken-note.md " + string(WarningFrontmatter),
		".capcom/knowledge/first-note.md " + string(WarningDuplicate),
		".capcom/knowledge/no-title.md " + string(WarningField),
		".capcom/knowledge/second-note.md " + string(WarningDuplicate),
	}
	if got := warningKinds(report); !slices.Equal(got, want) {
		t.Errorf("warning kinds = %q\nwant %q", got, want)
	}
}

func TestNotes_OldSchemaIndex_RebuildsWithTheNotes(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNote(t, s, "release-flow.md", "---\ntitle: Release flow\n---\n")
	if err := os.MkdirAll(filepath.Dir(Path(s)), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	db, err := sql.Open(driverName, Path(s))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE files (path TEXT PRIMARY KEY, kind TEXT NOT NULL, size INTEGER NOT NULL,
			mtime INTEGER NOT NULL, refreshed INTEGER NOT NULL, problems TEXT)`,
		`CREATE TABLE labels (path TEXT NOT NULL, label TEXT NOT NULL)`,
		`CREATE VIRTUAL TABLE issues_fts USING fts5 (path UNINDEXED, title, body)`,
		`INSERT INTO meta (key, value) VALUES ('schema_version', '1')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create old schema: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	x := openIndex(t, s)

	report := refresh(t, x)
	_, found, err := x.Note("release-flow")

	if !report.Rebuilt || err != nil || !found {
		t.Errorf("report = %+v, note found %v, %v; want a rebuild indexing the note", report, found, err)
	}
}
