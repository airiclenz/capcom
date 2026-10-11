package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheck_CleanStore_ExitsZeroWithNoOutput(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "check")

	if exitCode != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("check = %d, stdout %q, stderr %q; want 0 and no output", exitCode, stdout, stderr)
	}
}

func TestCheck_Problems_ListedOnePerLineAndExitOne(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"valid-issue.md": strings.Replace(validIssue, "type: task", "type: epic", 1),
		"Bad_Name.md":    validIssue,
		"other-issue.md": strings.Replace(validIssue, "priority: 2\n", "priority: 2\ndeps: [no-such-issue]\n", 1),
	})

	stdout, stderr, exitCode := runCLI(t, root, "check")

	want := ".capcom/issues/Bad_Name.md: invalid id \"Bad_Name\": must be 2 to 4 words of a-z and 0-9 joined by single hyphens\n" +
		".capcom/issues/other-issue.md:6: dependency \"no-such-issue\" names no issue by id or by an alias only one issue has\n" +
		".capcom/issues/valid-issue.md:4: type \"epic\" is not declared in config; use one of [bug feature task chore]\n"
	if exitCode != ExitError || stdout != want || stderr != "" {
		t.Fatalf("check = %d, stdout:\n%s\nstderr %q; want 1 and\n%s", exitCode, stdout, stderr, want)
	}
}

func TestCheck_JSONWithProblems_IsKindCheckWithProblemList(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"valid-issue.md": strings.Replace(validIssue, "status: open", "status: wip", 1),
	})

	stdout, _, exitCode := runCLI(t, root, "check", "--json")

	envelope := decodeEnvelope(t, stdout)
	if exitCode != ExitError || envelope["kind"] != "check" || envelope["error"] != nil {
		t.Fatalf("check --json = %d, %v; want exit 1, kind check, no error member", exitCode, envelope)
	}
	want := []any{map[string]any{
		"path":    ".capcom/issues/valid-issue.md",
		"line":    float64(3),
		"code":    "field",
		"message": "status \"wip\" is not declared in config; use one of [open active parked done dropped]",
	}}
	if !reflect.DeepEqual(envelope["data"], want) {
		t.Errorf("data = %v, want %v", envelope["data"], want)
	}
}

func TestCheck_JSONCleanStore_IsAnEmptyList(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})

	stdout, _, exitCode := runCLI(t, root, "check", "--json")

	envelope := decodeEnvelope(t, stdout)
	if data, isList := envelope["data"].([]any); exitCode != ExitOK || !isList || len(data) != 0 {
		t.Fatalf("check --json = %d, data %v; want 0 and []", exitCode, envelope["data"])
	}
}

func TestCheck_WithoutStore_FailsNotInitialized(t *testing.T) {
	root := newGitRepo(t)

	_, stderr, exitCode := runCLI(t, root, "check")

	if exitCode != ExitError || !strings.Contains(stderr, "capcom init") {
		t.Fatalf("check = %d, stderr %q; want 1 naming capcom init", exitCode, stderr)
	}
}

func TestCheck_StaleAgentsBlock_ReportedAtTheMarkerLine(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")
	writeNoteFiles(t, root, map[string]string{"api-design.md": "---\ntitle: API design\n---\n"})

	stdout, _, exitCode := runCLI(t, root, "check")

	want := "AGENTS.md:1: the capcom block is out of date; run capcom agents to refresh it\n"
	if exitCode != ExitError || stdout != want {
		t.Errorf("check = %d, stdout %q; want 1 and %q", exitCode, stdout, want)
	}
}

func TestCheck_WithoutAgentsGuide_ExitsZero(t *testing.T) {
	root := newKnowStore(t)

	stdout, _, exitCode := runCLI(t, root, "check")

	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); exitCode != ExitOK || stdout != "" || !os.IsNotExist(err) {
		t.Errorf("check = %d, stdout %q, AGENTS.md stat %v; want 0, no output, no AGENTS.md", exitCode, stdout, err)
	}
}
