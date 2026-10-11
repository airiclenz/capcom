package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// unformattedIssue is a valid issue in non-canonical form: keys out of order and plan quoted.
const unformattedIssue = "---\nstatus: open\ntitle: Valid\nplan: \"x\"\ntype: task\npriority: 2\n" +
	"created: 2026-10-11T09:00:00Z\n---\nBody.\n"

// formattedIssue is unformattedIssue in canonical form.
const formattedIssue = "---\ntitle: Valid\nstatus: open\ntype: task\npriority: 2\nplan: x\n" +
	"created: 2026-10-11T09:00:00Z\n---\nBody.\n"

// unformattedNote is a valid hand-written Note in non-canonical form.
const unformattedNote = "---\nlabels:\n  - testing\ntitle: \"Hand written\"\n---\nBody.\n"

// formattedNote is unformattedNote in canonical form.
const formattedNote = "---\ntitle: Hand written\nlabels: [testing]\n---\nBody.\n"

// readWorktreeFile returns the content of the file at a slash path below the working tree root.
func readWorktreeFile(t *testing.T, root string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

// newUnformattedStore returns a store holding one unformatted issue and one unformatted Note.
func newUnformattedStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{"plan-issue.md": unformattedIssue})
	writeNoteFiles(t, root, map[string]string{"hand-note.md": unformattedNote})
	return root
}

func TestFmt_UnformattedFiles_RewritesThemAndSecondRunChangesNothing(t *testing.T) {
	root := newUnformattedStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "fmt")
	again, _, againExitCode := runCLI(t, root, "fmt")

	want := ".capcom/issues/plan-issue.md\n.capcom/knowledge/hand-note.md\nformatted 2 files\n"
	if exitCode != ExitOK || stdout != want || stderr != "" {
		t.Fatalf("fmt = %d, stdout:\n%s\nstderr %q; want 0 and\n%s", exitCode, stdout, stderr, want)
	}
	if got := readWorktreeFile(t, root, ".capcom/issues/plan-issue.md"); got != formattedIssue {
		t.Errorf("issue file:\n%s\nwant\n%s", got, formattedIssue)
	}
	if got := readWorktreeFile(t, root, ".capcom/knowledge/hand-note.md"); got != formattedNote {
		t.Errorf("note file:\n%s\nwant\n%s", got, formattedNote)
	}
	if againExitCode != ExitOK || again != "formatted 0 files\n" {
		t.Errorf("second fmt = %d, stdout %q; want 0 and formatted 0 files", againExitCode, again)
	}
}

func TestFmtCheck_DirtyStore_ListsFilesExitsOneAndWritesNothing(t *testing.T) {
	root := newUnformattedStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "fmt", "--check")

	want := ".capcom/issues/plan-issue.md\n.capcom/knowledge/hand-note.md\nwould format 2 files\n"
	if exitCode != ExitError || stdout != want || stderr != "" {
		t.Fatalf("fmt --check = %d, stdout:\n%s\nstderr %q; want 1 and\n%s", exitCode, stdout, stderr, want)
	}
	if got := readWorktreeFile(t, root, ".capcom/issues/plan-issue.md"); got != unformattedIssue {
		t.Errorf("issue file changed:\n%s", got)
	}
	if got := readWorktreeFile(t, root, ".capcom/knowledge/hand-note.md"); got != unformattedNote {
		t.Errorf("note file changed:\n%s", got)
	}
}

func TestFmtCheck_FormattedStore_ExitsZero(t *testing.T) {
	root := newIssueStore(t, map[string]string{"plan-issue.md": formattedIssue})

	stdout, _, exitCode := runCLI(t, root, "fmt", "--check")

	if exitCode != ExitOK || stdout != "would format 0 files\n" {
		t.Fatalf("fmt --check = %d, stdout %q; want 0 and would format 0 files", exitCode, stdout)
	}
}

func TestFmt_JSON_IsKindFmtWithThePaths(t *testing.T) {
	root := newUnformattedStore(t)

	stdout, _, exitCode := runCLI(t, root, "fmt", "--check", "--json")

	envelope := decodeEnvelope(t, stdout)
	if exitCode != ExitError || envelope["kind"] != "fmt" || envelope["error"] != nil {
		t.Fatalf("fmt --check --json = %d, %v; want exit 1, kind fmt, no error member", exitCode, envelope)
	}
	want := map[string]any{"paths": []any{".capcom/issues/plan-issue.md", ".capcom/knowledge/hand-note.md"}}
	if !reflect.DeepEqual(envelope["data"], want) {
		t.Errorf("data = %v, want %v", envelope["data"], want)
	}
}

func TestFmt_BrokenFile_WarnsAndLeavesItUntouched(t *testing.T) {
	broken := "no frontmatter here\n"
	root := newIssueStore(t, map[string]string{"broken-issue.md": broken})

	stdout, _, exitCode := runCLI(t, root, "fmt")

	want := "formatted 0 files\nwarning: .capcom/issues/broken-issue.md:1: an issue file must start with a --- line\n"
	if exitCode != ExitOK || stdout != want {
		t.Fatalf("fmt = %d, stdout:\n%s\nwant 0 and\n%s", exitCode, stdout, want)
	}
	if got := readWorktreeFile(t, root, ".capcom/issues/broken-issue.md"); got != broken {
		t.Errorf("broken file changed:\n%s", got)
	}
}

func TestFmt_InvalidFiles_WarnedAboutAndUntouchedWhileOthersAreFormatted(t *testing.T) {
	undeclaredStatus := strings.Replace(unformattedIssue, "status: open", "status: wip", 1)
	root := newIssueStore(t, map[string]string{
		"wip-issue.md":  undeclaredStatus,
		"a.b.md":        unformattedIssue,
		"plan-issue.md": unformattedIssue,
	})

	stdout, _, exitCode := runCLI(t, root, "fmt")

	if exitCode != ExitOK || !strings.HasPrefix(stdout, ".capcom/issues/plan-issue.md\nformatted 1 files\n") {
		t.Fatalf("fmt = %d, stdout:\n%s\nwant 0, formatting only plan-issue.md", exitCode, stdout)
	}
	for _, warning := range []string{
		"warning: .capcom/issues/a.b.md: invalid id \"a.b\"",
		"warning: .capcom/issues/wip-issue.md:2: status \"wip\" is not declared in config",
	} {
		if !strings.Contains(stdout, warning) {
			t.Errorf("stdout lacks %q:\n%s", warning, stdout)
		}
	}
	untouched := map[string]string{"a.b.md": unformattedIssue, "wip-issue.md": undeclaredStatus}
	for name, content := range untouched {
		if got := readWorktreeFile(t, root, ".capcom/issues/"+name); got != content {
			t.Errorf("%s changed:\n%s", name, got)
		}
	}
}
