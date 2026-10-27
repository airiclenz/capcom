package nodestore

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
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

// newRefStore writes issues and notes (slash paths to content) into the issue tree and the
// Knowledge tree of a fresh store and returns the store.
func newRefStore(t *testing.T, issues map[string]string, notes map[string]string) *store.Store {
	t.Helper()
	s := newStore(t)
	writeRefTree(t, s.IssuesDir(), issues)
	writeRefTree(t, s.KnowledgeDir(), notes)
	return s
}

// parseNoteAt parses the Note file at the slash path name below dir.
func parseNoteAt(t *testing.T, dir string, name string) knowledge.Note {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	parsed, err := knowledge.Parse(path, []byte(readFile(t, path)))
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return parsed
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

func TestMove_ReparentIssue_RewritesNoOtherIssueButTheNoteRefs(t *testing.T) {
	t.Parallel()
	s := newRefStore(t, map[string]string{
		"login-flow.md":   refIssue("aliases: [old-login]\n", ""),
		"other-issue.md":  refIssue("", ""),
		"session-work.md": refIssue("deps: [login-flow]\nrelated: [old-login]\n", ""),
	}, map[string]string{
		"login-notes.md": refNote("Login", "refs: [.capcom/issues/login-flow.md#Acceptance]\n", ""),
	})
	dir := s.IssuesDir()
	before := treeContent(t, dir)
	delete(before, "login-flow.md")

	moved := mustMove(t, Issues(s, nil), "login-flow", Destination{Parent: "other-issue", IsReparented: true})

	after := treeContent(t, dir)
	delete(after, filepath.Join("other-issue", "login-flow.md"))
	if !maps.Equal(after, before) {
		t.Errorf("other issues =\n%v\nwant them byte for byte as before:\n%v", after, before)
	}
	want := []string{".capcom/issues/other-issue/login-flow.md#Acceptance"}
	if got := parseNoteAt(t, s.KnowledgeDir(), "login-notes.md").Refs; !slices.Equal(got, want) {
		t.Errorf("login-notes refs = %v, want %v", got, want)
	}
	if want := []string{".capcom/knowledge/login-notes.md"}; !slices.Equal(moved.RefsRewritten, want) {
		t.Errorf("RefsRewritten = %v, want %v", moved.RefsRewritten, want)
	}
}

func TestMove_RenameIssue_RewritesNoteRefsToItsFileAndDirectory(t *testing.T) {
	t.Parallel()
	untouched := refNote("Untouched", "refs: [.capcom/issues/other.md]\n", "")
	s := newRefStore(t, map[string]string{
		"beta-new.md":       refIssue("", "\n## Acceptance\n"),
		"beta-new/child.md": refIssue("", ""),
		"other.md":          refIssue("", ""),
	}, map[string]string{
		"nref.md": refNote("Refs", "refs: [.capcom/issues/beta-new.md, .capcom/issues/beta-new.md#Acceptance, "+
			".capcom/issues/beta-new/child.md, .capcom/issues/beta-new, .capcom/issues/other.md]\n", "\nSee beta-new.\n"),
		"untouched.md": untouched,
	})

	moved := mustMove(t, Issues(s, nil), "beta-new", Destination{ID: "beta-final"})

	note := parseNoteAt(t, s.KnowledgeDir(), "nref.md")
	want := []string{
		".capcom/issues/beta-final.md",
		".capcom/issues/beta-final.md#Acceptance",
		".capcom/issues/beta-final/child.md",
		".capcom/issues/beta-final",
		".capcom/issues/other.md",
	}
	if !slices.Equal(note.Refs, want) || note.Body != "\nSee beta-new.\n" {
		t.Errorf("nref refs = %v body = %q, want refs %v and the body unchanged", note.Refs, note.Body, want)
	}
	if got := readFile(t, filepath.Join(s.KnowledgeDir(), "untouched.md")); got != untouched {
		t.Errorf("untouched.md =\n%s\nwant it byte for byte as before", got)
	}
	if want := []string{".capcom/knowledge/nref.md"}; !slices.Equal(moved.RefsRewritten, want) || moved.UnreadNotes != nil {
		t.Errorf("RefsRewritten = %v UnreadNotes = %v, want %v and none unread", moved.RefsRewritten, moved.UnreadNotes, want)
	}
	notes := []knowledge.NoteFile{{ID: "nref", Path: ".capcom/knowledge/nref.md", Note: note}}
	findings, err := knowledge.Verify(knowledge.Repository{WorktreeRoot: s.WorktreeRoot}, notes)
	if err != nil || len(findings) != 0 {
		t.Errorf("Verify = %v, %v; want every Ref to resolve after the move", findings, err)
	}
}

func TestMove_Issue_NoteThatCannotBeCheckedWarnsAndTheMoveLands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		isPermission bool
		lock         string
		want         string
	}{
		{name: "unparseable note", want: ".capcom/knowledge/locked.md"},
		{name: "unreadable note", isPermission: true, lock: "locked.md", want: ".capcom/knowledge/locked.md"},
		{name: "unwalkable knowledge tree", isPermission: true, lock: ".", want: ".capcom/knowledge"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.isPermission && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("file permission tests do not run on Windows or as root")
			}
			lockedNote := refNote("Locked", "refs: [.capcom/issues/alpha.md]\n", "")
			if !test.isPermission {
				lockedNote = unparseable(lockedNote)
			}
			s := newRefStore(t, map[string]string{"alpha.md": refIssue("", "")}, map[string]string{"locked.md": lockedNote})
			if test.isPermission {
				locked := filepath.Join(s.KnowledgeDir(), test.lock)
				if err := os.Chmod(locked, 0o000); err != nil {
					t.Fatalf("Chmod: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
			}

			moved, _, err := Issues(s, nil).Move("alpha", Destination{ID: "beta"}, "")

			if err != nil || moved.Path != ".capcom/issues/beta.md" {
				t.Fatalf("Move = %+v, %v; want the move to land at beta.md", moved, err)
			}
			if want := []string{test.want}; !slices.Equal(moved.UnreadNotes, want) || moved.RefsRewritten != nil {
				t.Errorf("UnreadNotes = %v RefsRewritten = %v, want %v and nothing rewritten",
					moved.UnreadNotes, moved.RefsRewritten, want)
			}
		})
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

func TestMove_ReparentNote_RewritesNoSuccessorEntry(t *testing.T) {
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

func TestMove_RenameNote_RewritesAnotherNotesRefsAndSuccessorsInOneWrite(t *testing.T) {
	t.Parallel()
	notes, dir := newRefNotes(t, map[string]string{
		"old-rule.md":        refNote("Old rule", "", ""),
		"old-rule/detail.md": refNote("Detail", "", ""),
		"superseded.md": refNote("Superseded", "refs: [.capcom/knowledge/old-rule.md#Steps, "+
			".capcom/knowledge/old-rule/detail.md]\nsuperseded-by: [old-rule]\n", ""),
	})

	moved := mustMove(t, notes, "old-rule", Destination{ID: "new-rule"})

	note := parseNoteAt(t, dir, "superseded.md")
	wantRefs := []string{".capcom/knowledge/new-rule.md#Steps", ".capcom/knowledge/new-rule/detail.md"}
	if !slices.Equal(note.Refs, wantRefs) || !slices.Equal(note.SupersededBy, []string{"new-rule"}) {
		t.Errorf("superseded refs = %v superseded-by = %v, want %v and [new-rule]", note.Refs, note.SupersededBy, wantRefs)
	}
	if want := []string{".capcom/knowledge/superseded.md"}; !slices.Equal(moved.RefsRewritten, want) {
		t.Errorf("RefsRewritten = %v, want %v", moved.RefsRewritten, want)
	}
}

func TestMove_ReparentNote_RewritesRefsInsideItsSubtreeAndReportsTheirNewPaths(t *testing.T) {
	t.Parallel()
	notes, dir := newRefNotes(t, map[string]string{
		"guide.md":        refNote("Guide", "refs: [.capcom/knowledge/guide/step.md]\n", ""),
		"guide/step.md":   refNote("Step", "refs: [.capcom/knowledge/guide.md#Intro]\n", ""),
		"guide/broken.md": unparseable(refNote("Broken", "refs: [.capcom/knowledge/guide.md]\n", "")),
		"parent.md":       refNote("Parent", "", ""),
	})

	moved := mustMove(t, notes, "guide", Destination{Parent: "parent", IsReparented: true})

	guide := parseNoteAt(t, dir, "parent/guide.md")
	if want := []string{".capcom/knowledge/parent/guide/step.md"}; !slices.Equal(guide.Refs, want) {
		t.Errorf("guide refs = %v, want %v", guide.Refs, want)
	}
	if want := nodefile.ETag([]byte(readFile(t, filepath.Join(dir, "parent", "guide.md")))); moved.ETag != want {
		t.Errorf("Moved.ETag = %s, want %s, the etag of the rewritten guide", moved.ETag, want)
	}
	step := parseNoteAt(t, dir, "parent/guide/step.md")
	if want := []string{".capcom/knowledge/parent/guide.md#Intro"}; !slices.Equal(step.Refs, want) {
		t.Errorf("step refs = %v, want %v", step.Refs, want)
	}
	wantRewritten := []string{".capcom/knowledge/parent/guide.md", ".capcom/knowledge/parent/guide/step.md"}
	wantUnread := []string{".capcom/knowledge/parent/guide/broken.md"}
	if !slices.Equal(moved.RefsRewritten, wantRewritten) || !slices.Equal(moved.UnreadNotes, wantUnread) {
		t.Errorf("RefsRewritten = %v UnreadNotes = %v, want %v and %v",
			moved.RefsRewritten, moved.UnreadNotes, wantRewritten, wantUnread)
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

func TestMove_RenameOntoDanglingEntry_KeepsOneEntry(t *testing.T) {
	t.Parallel()
	issues, dir := newRefIssues(t, map[string]string{
		"login-flow.md":   refIssue("", ""),
		"session-work.md": refIssue("deps: [login-flow, auth-flow]\nrelated: [login-flow, auth-flow]\n", ""),
	})

	mustMove(t, issues, "login-flow", Destination{ID: "auth-flow"})

	session := parseIssueAt(t, dir, "session-work.md")
	want := []string{"auth-flow"}
	if !slices.Equal(session.Deps, want) || !slices.Equal(session.Related, want) {
		t.Errorf("session-work deps = %v related = %v, want both %v", session.Deps, session.Related, want)
	}
}

func TestMove_RenameIssueOntoItsOwnEntry_KeepsTheSelfEntry(t *testing.T) {
	t.Parallel()
	issues, dir := newRefIssues(t, map[string]string{
		"login-flow.md": refIssue("deps: [auth-flow]\nrelated: [login-flow]\n", ""),
	})

	mustMove(t, issues, "login-flow", Destination{ID: "auth-flow"})

	moved := parseIssueAt(t, dir, "auth-flow.md")
	want := []string{"auth-flow"}
	if !slices.Equal(moved.Deps, want) || !slices.Equal(moved.Related, want) {
		t.Errorf("auth-flow deps = %v related = %v, want both %v", moved.Deps, moved.Related, want)
	}
}

func TestMove_NoteRenamedToItsOwnSuccessor_DropsTheEntryAndStaysValid(t *testing.T) {
	t.Parallel()
	notes, dir := newRefNotes(t, map[string]string{
		"auth-v1.md": refNote("Auth v1", "superseded-by: [auth, other-rule]\n", "\nBody.\n"),
	})

	mustMove(t, notes, "auth-v1", Destination{ID: "auth"})

	path := filepath.Join(dir, "auth.md")
	parsed, err := knowledge.Parse(path, []byte(readFile(t, path)))
	if err != nil {
		t.Fatalf("Parse(auth.md): %v", err)
	}
	if want := []string{"other-rule"}; !slices.Equal(parsed.SupersededBy, want) {
		t.Errorf("superseded-by = %v, want %v", parsed.SupersededBy, want)
	}
	if want := []string{"auth-v1"}; !slices.Equal(parsed.Aliases, want) || parsed.Body != "\nBody.\n" {
		t.Errorf("aliases = %v body = %q, want %v and the body unchanged", parsed.Aliases, parsed.Body, want)
	}
	if problems := knowledge.Validate(path, parsed); len(problems) != 0 {
		t.Errorf("Validate(auth.md) = %v, want no problem", problems)
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

func TestMove_RenameIssue_NamesEveryCommentedFileItRewritesAtItsPathAfterTheMove(t *testing.T) {
	t.Parallel()
	issues, _ := newRefIssues(t, map[string]string{
		"login-flow.md":             refIssue("# why: customer outage\n", ""),
		"login-flow/child-issue.md": refIssue("# pinned by the parent\nrelated: [login-flow]\n", ""),
		"session-work.md":           refIssue("deps: [login-flow] # first\n", ""),
		"plain-work.md":             refIssue("deps: [login-flow]\n", ""),
		"quiet-work.md":             refIssue("# never rewritten\n", ""),
	})

	moved := mustMove(t, issues, "login-flow", Destination{ID: "auth-flow"})

	got := slices.Sorted(slices.Values(moved.DroppedComments.Paths()))
	want := []string{
		".capcom/issues/auth-flow.md",
		".capcom/issues/auth-flow/child-issue.md",
		".capcom/issues/session-work.md",
	}
	if !slices.Equal(got, want) {
		t.Errorf("DroppedComments = %v, want %v", got, want)
	}
}

func TestMove_ReparentIssue_NamesACommentedNoteWhoseRefsItRewrites(t *testing.T) {
	t.Parallel()
	s := newRefStore(t, map[string]string{
		"login-flow.md": refIssue("# why: customer outage\n", ""),
		"parent.md":     refIssue("", ""),
	}, map[string]string{
		"auth-notes.md":  refNote("Auth notes", "# keep this\nrefs: [.capcom/issues/login-flow.md]\n", ""),
		"other-notes.md": refNote("Other notes", "# unrelated\n", ""),
	})

	moved := mustMove(t, Issues(s, nil), "login-flow", Destination{Parent: "parent", IsReparented: true})

	if got, want := moved.DroppedComments.Paths(), []string{".capcom/knowledge/auth-notes.md"}; !slices.Equal(got, want) {
		t.Errorf("DroppedComments = %v, want %v: the moved file itself is renamed, not rewritten", got, want)
	}
}

func TestMove_RenameIssueWithoutComments_NamesNoFile(t *testing.T) {
	t.Parallel()
	issues, _ := newRefIssues(t, map[string]string{
		"login-flow.md":   refIssue("", ""),
		"session-work.md": refIssue("deps: [login-flow]\n", ""),
	})

	moved := mustMove(t, issues, "login-flow", Destination{ID: "auth-flow"})

	if moved.DroppedComments != "" {
		t.Errorf("DroppedComments = %q, want none", moved.DroppedComments)
	}
}

func TestRemove_DropDeps_NamesTheCommentedFilesItRewrites(t *testing.T) {
	t.Parallel()
	issues, _ := newRefIssues(t, map[string]string{
		"login-flow.md":   refIssue("", ""),
		"session-work.md": refIssue("# waits for login\ndeps: [login-flow]\n", ""),
		"plain-work.md":   refIssue("related: [login-flow]\n", ""),
		"quiet-work.md":   refIssue("# never rewritten\n", ""),
	})

	removed, _, err := issues.Remove("login-flow", "", true)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if got, want := removed.DroppedComments.Paths(), []string{".capcom/issues/session-work.md"}; !slices.Equal(got, want) {
		t.Errorf("DroppedComments = %v, want %v", got, want)
	}
}
