package nodestore

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
)

// refIssue is the content of an issue file with the frontmatter lines front, each ending in a
// newline, between its fixed keys and its created time, followed by body.
func refIssue(front string, body string) string {
	return "---\ntitle: Moved\nstatus: open\ntype: task\npriority: 2\n" + front +
		"created: 2026-10-11T09:00:00Z\n---\n" + body
}

// refNote is the content of a Note file titled title with the frontmatter lines front, each ending
// in a newline, followed by body.
func refNote(title string, front string, body string) string {
	return "---\ntitle: " + title + "\n" + front + "---\n" + body
}

// writeRefTree writes files (slash paths to content) below dir.
func writeRefTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		writeNodeFile(t, filepath.Dir(path), filepath.Base(path), content)
	}
}

// newRefIssues writes files (slash paths to content) into the issue tree of a fresh store and
// returns the store's issue Kind and its issues directory.
func newRefIssues(t *testing.T, files map[string]string) (*Kind[issue.Issue], string) {
	t.Helper()
	s := newStore(t)
	writeRefTree(t, s.IssuesDir(), files)
	return Issues(s, nil), s.IssuesDir()
}

// newRefNotes writes files (slash paths to content) into the Knowledge tree of a fresh store and
// returns the store's Note Kind and its Knowledge directory.
func newRefNotes(t *testing.T, files map[string]string) (*Kind[knowledge.Note], string) {
	t.Helper()
	s := newStore(t)
	writeRefTree(t, s.KnowledgeDir(), files)
	return Notes(s, nil), s.KnowledgeDir()
}

// parseIssueAt parses the issue file at the slash path name below dir.
func parseIssueAt(t *testing.T, dir string, name string) issue.Issue {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	parsed, err := issue.Parse(path, []byte(readFile(t, path)))
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return parsed
}

// successorsAt parses the Note file at the slash path name below dir and returns its
// superseded-by entries.
func successorsAt(t *testing.T, dir string, name string) []string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	parsed, err := knowledge.Parse(path, []byte(readFile(t, path)))
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return parsed.SupersededBy
}

// mustMove moves ref as to says through kind, failing the test on any error.
func mustMove[T any](t *testing.T, kind *Kind[T], ref string, to Destination) Moved {
	t.Helper()
	moved, _, err := kind.Move(ref, to, "")
	if err != nil {
		t.Fatalf("Move(%s): %v", ref, err)
	}
	return moved
}

func TestMove_RenameIssue_RewritesOtherIssuesDepsNamingTheIdOrAnAlias(t *testing.T) {
	t.Parallel()
	untouchedContent := refIssue("deps: [other-issue]\n", "")
	issues, dir := newRefIssues(t, map[string]string{
		"login-flow.md":                  refIssue("aliases: [old-login]\n", ""),
		"session-work.md":                refIssue("deps: [other-issue, login-flow, old-login]\n", "\nNeeds login-flow first.\n"),
		"other-issue.md":                 refIssue("", ""),
		"other-issue/nested-child.md":    refIssue("deps: [old-login]\n", ""),
		"other-issue/untouched-child.md": untouchedContent,
	})

	mustMove(t, issues, "login-flow", Destination{ID: "auth-flow"})

	session := parseIssueAt(t, dir, "session-work.md")
	if want := []string{"other-issue", "auth-flow"}; !slices.Equal(session.Deps, want) {
		t.Errorf("session-work deps = %v, want %v", session.Deps, want)
	}
	if session.Body != "\nNeeds login-flow first.\n" {
		t.Errorf("session-work body = %q, want it unchanged", session.Body)
	}
	if nested := parseIssueAt(t, dir, "other-issue/nested-child.md"); !slices.Equal(nested.Deps, []string{"auth-flow"}) {
		t.Errorf("nested-child deps = %v, want [auth-flow]", nested.Deps)
	}
	if got := readFile(t, filepath.Join(dir, "other-issue", "untouched-child.md")); got != untouchedContent {
		t.Errorf("untouched-child.md =\n%s\nwant it byte for byte as before", got)
	}
}

func TestMove_RenameIssue_RewritesOtherIssuesRelatedNamingTheIdOrAnAlias(t *testing.T) {
	t.Parallel()
	relatedIssue := func(related string) string {
		return refIssue("related: ["+related+"]\n", "\nSee login-flow.\n")
	}
	untouchedContent := relatedIssue("other-issue")
	issues, dir := newRefIssues(t, map[string]string{
		"login-flow.md":      refIssue("aliases: [old-login]\n", ""),
		"by-id.md":           relatedIssue("other-issue, login-flow"),
		"by-alias.md":        relatedIssue("old-login"),
		"by-both.md":         relatedIssue("login-flow, old-login"),
		"other-issue.md":     refIssue("", ""),
		"untouched-issue.md": untouchedContent,
	})

	moved := mustMove(t, issues, "login-flow", Destination{ID: "auth-flow"})

	wants := map[string][]string{
		"by-id.md":    {"other-issue", "auth-flow"},
		"by-alias.md": {"auth-flow"},
		"by-both.md":  {"auth-flow"},
	}
	for name, want := range wants {
		parsed := parseIssueAt(t, dir, name)
		if !slices.Equal(parsed.Related, want) || parsed.Deps != nil || parsed.Body != "\nSee login-flow.\n" {
			t.Errorf("%s related = %v deps = %v body = %q, want related %v and the rest unchanged",
				name, parsed.Related, parsed.Deps, parsed.Body, want)
		}
	}
	if got := readFile(t, filepath.Join(dir, "untouched-issue.md")); got != untouchedContent {
		t.Errorf("untouched-issue.md =\n%s\nwant it byte for byte as before", got)
	}
	if len(moved.Unread) != 0 {
		t.Errorf("Unread = %v, want none", moved.Unread)
	}
}

func TestMove_RenameIssueTwice_KeepsTheAliasChain(t *testing.T) {
	t.Parallel()
	issues, dir := newRefIssues(t, map[string]string{"first-name.md": refIssue("", "\nBody.\n")})

	mustMove(t, issues, "first-name", Destination{ID: "second-name"})
	moved := mustMove(t, issues, "second-name", Destination{ID: "third-name"})

	parsed := parseIssueAt(t, dir, "third-name.md")
	if want := []string{"first-name", "second-name"}; !slices.Equal(parsed.Aliases, want) {
		t.Errorf("aliases = %v, want %v", parsed.Aliases, want)
	}
	if moved.Path != ".capcom/issues/third-name.md" || parsed.Body != "\nBody.\n" {
		t.Errorf("Moved.Path = %s, body = %q; want third-name.md with the body unchanged", moved.Path, parsed.Body)
	}
	if again := mustMove(t, issues, "first-name", Destination{ID: "fourth-name"}); again.OldID != "third-name" {
		t.Errorf("Move(first-name).OldID = %s, want the first id to still resolve to third-name", again.OldID)
	}
}

func TestMove_RenameIssueBackToItsOwnAlias_SwapsIt(t *testing.T) {
	t.Parallel()
	issues, dir := newRefIssues(t, map[string]string{"new-name.md": refIssue("aliases: [old-name, legacy-name]\n", "")})

	mustMove(t, issues, "new-name", Destination{ID: "old-name"})

	parsed := parseIssueAt(t, dir, "old-name.md")
	if want := []string{"legacy-name", "new-name"}; !slices.Equal(parsed.Aliases, want) {
		t.Errorf("aliases = %v, want %v", parsed.Aliases, want)
	}
}

func TestMove_ReparentIssue_RewritesNoOtherFile(t *testing.T) {
	t.Parallel()
	issues, dir := newRefIssues(t, map[string]string{
		"login-flow.md":   refIssue("aliases: [old-login]\n", ""),
		"other-issue.md":  refIssue("", ""),
		"session-work.md": refIssue("deps: [login-flow]\nrelated: [old-login]\n", ""),
	})
	before := treeContent(t, dir)
	delete(before, "login-flow.md")

	mustMove(t, issues, "login-flow", Destination{Parent: "other-issue", IsReparented: true})

	after := treeContent(t, dir)
	delete(after, filepath.Join("other-issue", "login-flow.md"))
	if !maps.Equal(after, before) {
		t.Errorf("other files =\n%v\nwant them byte for byte as before:\n%v", after, before)
	}
}

func TestMove_UnparseableMovedIssue_DestinationCheckedFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		to     Destination
		assert func(t *testing.T, err error)
	}{
		{
			name:   "invalid id",
			to:     Destination{ID: "Bad_Name"},
			assert: assertMoveRefFileError(false, "invalid id"),
		},
		{
			name:   "taken id",
			to:     Destination{ID: "taken-issue"},
			assert: assertExists(".capcom/issues/taken-issue.md", "taken-issue", "is already taken by issue"),
		},
		{
			name:   "own subtree",
			to:     Destination{Parent: "child-one", IsReparented: true},
			assert: assertMoveRefFileError(false, "below itself or its own descendants"),
		},
		{
			name:   "valid rename",
			to:     Destination{ID: "renamed-issue"},
			assert: assertMoveRefFileError(true, "bogus"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			issues, dir := newRefIssues(t, map[string]string{
				"moved-issue.md":           unparseable(refIssue("", "")),
				"moved-issue/child-one.md": refIssue("", ""),
				"taken-issue.md":           refIssue("", ""),
			})
			before := treeContent(t, dir)

			_, _, err := issues.Move("moved-issue", test.to, "")

			test.assert(t, err)
			if after := treeContent(t, dir); !maps.Equal(after, before) {
				t.Errorf("tree changed by the refusal:\n%v\nwant\n%v", after, before)
			}
		})
	}
}

func TestMove_RenameNote_RecordsTheOldIDAsAnAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		aliases string
		newID   string
		want    string
	}{
		{name: "new id", newID: "release-steps", want: "aliases: [release-flow]\n"},
		{
			name:    "former alias",
			aliases: "aliases: [release-steps, shipping]\n",
			newID:   "release-steps",
			want:    "aliases: [shipping, release-flow]\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			notes, dir := newRefNotes(t, map[string]string{
				"release-flow.md": refNote("Release flow", test.aliases, "\nSee release-flow.\n"),
			})

			moved := mustMove(t, notes, "release-flow", Destination{ID: test.newID})

			content := readFile(t, filepath.Join(dir, test.newID+".md"))
			if !strings.Contains(content, test.want) || !strings.HasSuffix(content, "\nSee release-flow.\n") {
				t.Errorf("moved note =\n%s\nwant %q and the body unchanged", content, test.want)
			}
			if moved.ETag != nodefile.ETag([]byte(content)) {
				t.Errorf("Moved.ETag = %s, want the etag of the moved note", moved.ETag)
			}
		})
	}
}

func TestMove_RenameNote_RewritesOtherNotesSuccessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		successors string
		want       []string
	}{
		{name: "by id", successors: "[old-rule, other]", want: []string{"new-rule", "other"}},
		{name: "by alias", successors: "[older-rule]", want: []string{"new-rule"}},
		{name: "beside the new id", successors: "[old-rule, new-rule]", want: []string{"new-rule"}},
		{name: "after the new id", successors: "[new-rule, old-rule]", want: []string{"new-rule"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			notes, dir := newRefNotes(t, map[string]string{
				"old-rule.md":   refNote("Old rule", "aliases: [older-rule]\n", ""),
				"superseded.md": refNote("Superseded", "superseded-by: "+test.successors+"\n", ""),
			})

			mustMove(t, notes, "old-rule", Destination{ID: "new-rule"})

			if got := successorsAt(t, dir, "superseded.md"); !slices.Equal(got, test.want) {
				t.Errorf("superseded-by = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMove_ReparentNote_LeavesTheFileBytes(t *testing.T) {
	t.Parallel()
	original := "---\ntitle:   Release flow\n---\n"
	notes, dir := newRefNotes(t, map[string]string{
		"release-flow.md":  original,
		"process-notes.md": refNote("Process", "", ""),
	})

	moved := mustMove(t, notes, "release-flow", Destination{Parent: "process-notes", IsReparented: true})

	content := readFile(t, filepath.Join(dir, "process-notes", "release-flow.md"))
	if content != original || moved.ETag != nodefile.ETag([]byte(original)) {
		t.Errorf("moved note =\n%s\netag %s; want the original bytes and their etag", content, moved.ETag)
	}
	if moved.Path != ".capcom/knowledge/process-notes/release-flow.md" {
		t.Errorf("Moved.Path = %s, want the note below process-notes", moved.Path)
	}
}

func TestMove_ReparentNote_RewritesNoOtherNote(t *testing.T) {
	t.Parallel()
	successor := refNote("Superseded", "superseded-by: [old-rule]\n", "")
	notes, dir := newRefNotes(t, map[string]string{
		"old-rule.md":   refNote("Old rule", "", ""),
		"parent.md":     refNote("Parent", "", ""),
		"superseded.md": successor,
	})

	mustMove(t, notes, "old-rule", Destination{Parent: "parent", IsReparented: true})

	if got := readFile(t, filepath.Join(dir, "superseded.md")); got != successor {
		t.Errorf("superseded.md =\n%s\nwant it byte for byte as before", got)
	}
}

func TestMove_RenameNote_UnparseableOtherNoteJoinsUnread(t *testing.T) {
	t.Parallel()
	broken := "no frontmatter\n"
	notes, dir := newRefNotes(t, map[string]string{
		"old-rule.md": refNote("Old rule", "", ""),
		"broken.md":   broken,
	})

	moved := mustMove(t, notes, "old-rule", Destination{ID: "new-rule"})

	if want := []string{".capcom/knowledge/broken.md"}; !slices.Equal(moved.Unread, want) {
		t.Errorf("Unread = %v, want %v", moved.Unread, want)
	}
	if got := readFile(t, filepath.Join(dir, "broken.md")); got != broken {
		t.Errorf("broken.md = %q, want it left as it was", got)
	}
}

// assertMoveRefFileError returns an assertion that err is a *FileError at moved-issue.md, citing a
// line when hasLine, whose message contains text.
func assertMoveRefFileError(hasLine bool, text string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var failure *FileError
		if !errors.As(err, &failure) || failure.Path != ".capcom/issues/moved-issue.md" ||
			(failure.Line > 0) != hasLine || !strings.Contains(failure.Message, text) {
			t.Errorf("err = %v, want a *FileError at moved-issue.md (line: %t) saying %q", err, hasLine, text)
		}
	}
}
