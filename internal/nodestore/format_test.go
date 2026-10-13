package nodestore

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// unformattedIssue is canonicalIssue with its status line before its title.
var unformattedIssue = strings.Replace(
	canonicalIssue,
	"title: Fix the parser\nstatus: open\n",
	"status: open\ntitle: Fix the parser\n",
	1,
)

// unformattedNote is canonicalNote with its refs line before its labels.
var unformattedNote = strings.Replace(
	canonicalNote,
	"labels: [conventions]\nrefs: [internal/cli/errors.go]\n",
	"refs: [internal/cli/errors.go]\nlabels: [conventions]\n",
	1,
)

func TestFormat_UnformattedFiles_RewritesThemWithoutAResolver(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.IssuesDir(), "fix-parser.md", unformattedIssue)
	writeNodeFile(t, s.IssuesDir(), "login-flow.md", plainIssue)
	writeNodeFile(t, s.KnowledgeDir(), "release-flow.md", unformattedNote)

	issues, issuesErr := Issues(s, nil).Format(false)
	notes, notesErr := Notes(s, nil).Format(false)

	if issuesErr != nil || !slices.Equal(issues.Paths, []string{".capcom/issues/fix-parser.md"}) ||
		len(issues.Warnings) != 0 {
		t.Fatalf("issues Format = %+v, %v; want fix-parser.md rewritten and no warnings", issues, issuesErr)
	}
	if notesErr != nil || !slices.Equal(notes.Paths, []string{".capcom/knowledge/release-flow.md"}) {
		t.Fatalf("notes Format = %+v, %v; want release-flow.md rewritten", notes, notesErr)
	}
	if got := readFile(t, filepath.Join(s.IssuesDir(), "fix-parser.md")); got != canonicalIssue {
		t.Errorf("fix-parser.md =\n%s\nwant\n%s", got, canonicalIssue)
	}
	if got := readFile(t, filepath.Join(s.KnowledgeDir(), "release-flow.md")); got != canonicalNote {
		t.Errorf("release-flow.md =\n%s\nwant\n%s", got, canonicalNote)
	}
	if again, err := Issues(s, nil).Format(false); err != nil || len(again.Paths) != 0 {
		t.Errorf("second Format = %+v, %v; want nothing rewritten", again, err)
	}
}

func TestFormat_FileChangedBeforeItsRewrite_ConflictsAndKeepsTheConcurrentBytes(t *testing.T) {
	s := newStore(t)
	firstPath := writeNodeFile(t, s.IssuesDir(), "fix-parser.md", unformattedIssue)
	secondPath := writeNodeFile(t, s.IssuesDir(), "login-flow.md", unformattedIssue)
	concurrent := strings.Replace(unformattedIssue, "Fix the parser", "Fix the parser today", 1)
	setBeforeRecheck(t, func(path string) {
		if path == secondPath {
			writeNodeFile(t, s.IssuesDir(), "login-flow.md", concurrent)
		}
	})

	_, err := Issues(s, nil).Format(false)

	assertChangedConflict(t, err, ".capcom/issues/login-flow.md")
	if got := readFile(t, secondPath); got != concurrent {
		t.Errorf("login-flow.md =\n%s\nwant the concurrent bytes kept", got)
	}
	if got := readFile(t, firstPath); got != canonicalIssue {
		t.Errorf("fix-parser.md =\n%s\nwant it rewritten before the conflict", got)
	}
}

func TestFormat_Check_ListsTheFileAndWritesNothing(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	path := writeNodeFile(t, s.IssuesDir(), "fix-parser.md", unformattedIssue)

	formatted, err := Issues(s, nil).Format(true)

	if err != nil || !slices.Equal(formatted.Paths, []string{".capcom/issues/fix-parser.md"}) {
		t.Fatalf("Format(check) = %+v, %v; want fix-parser.md listed", formatted, err)
	}
	if got := readFile(t, path); got != unformattedIssue {
		t.Errorf("fix-parser.md changed under check:\n%s", got)
	}
}

func TestFormat_BrokenFiles_WarnedAboutAndLeftAlone(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	undeclaredStatus := strings.Replace(unformattedIssue, "status: open", "status: wip", 1)
	files := map[string]string{
		"a.b.md":        unformattedIssue,
		"bogus-key.md":  unparseable(unformattedIssue),
		"wip-status.md": undeclaredStatus,
	}
	for name, content := range files {
		writeNodeFile(t, s.IssuesDir(), name, content)
	}

	formatted, err := Issues(s, nil).Format(false)

	if err != nil || len(formatted.Paths) != 0 {
		t.Fatalf("Format = %+v, %v; want nothing rewritten", formatted, err)
	}
	var messages []string
	for _, warning := range formatted.Warnings {
		messages = append(messages, warning.Error())
	}
	var problem *tree.Error
	if len(formatted.Warnings) != 3 || !errors.As(formatted.Warnings[0], &problem) ||
		!strings.HasPrefix(messages[0], ".capcom/issues/a.b.md: invalid id") {
		t.Fatalf("warnings = %q; want the tree problem of a.b.md first, then one per broken file", messages)
	}
	for i, prefix := range []string{
		".capcom/issues/bogus-key.md:2: ",
		`.capcom/issues/wip-status.md:2: status "wip" is not declared in config`,
	} {
		var file *FileError
		if !errors.As(formatted.Warnings[i+1], &file) || !strings.HasPrefix(messages[i+1], prefix) {
			t.Errorf("warning %d = %q, want a *FileError starting %q", i+1, messages[i+1], prefix)
		}
	}
	for name, content := range files {
		if got := readFile(t, filepath.Join(s.IssuesDir(), name)); got != content {
			t.Errorf("%s changed:\n%s", name, got)
		}
	}
}
