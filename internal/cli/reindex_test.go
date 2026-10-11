package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newIssueStore creates a git repository with an initialised store and writes the given issue
// files, named by slash paths below .capcom/issues/.
func newIssueStore(t *testing.T, files map[string]string) string {
	t.Helper()
	root := newGitRepo(t)
	if _, _, exitCode := runCLI(t, root, "init"); exitCode != ExitOK {
		t.Fatalf("init exit = %d", exitCode)
	}
	for name, content := range files {
		path := filepath.Join(root, ".capcom", "issues", filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return root
}

const validIssue = "---\ntitle: Valid\nstatus: open\ntype: task\npriority: 2\ncreated: 2026-10-11T09:00:00Z\n---\n"

func TestReindex_AfterDeletingIndex_RebuildsIt(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	indexPath := filepath.Join(root, ".git", "capcom", "index.db")
	runCLI(t, root, "reindex")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("Remove index.db: %v", err)
	}

	stdout, _, exitCode := runCLI(t, root, "reindex", "--json")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d\n%s", exitCode, stdout)
	}
	data := decodeEnvelope(t, stdout)["data"].(map[string]any)
	if data["issues"] != float64(1) || data["path"] != indexPath {
		t.Errorf("data = %v, want 1 issue in %s", data, indexPath)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Errorf("index.db not recreated: %v", err)
	}
}

func TestReindex_BrokenFile_WarnsAfterResultAndIndexesTheRest(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"valid-issue.md":  validIssue,
		"broken-issue.md": "no frontmatter\n",
	})

	stdout, stderr, exitCode := runCLI(t, root, "reindex")

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	want := "warning: .capcom/issues/broken-issue.md:1: an issue file must start with a --- line"
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "reindexed 1 issues into ") || lines[1] != want {
		t.Errorf("stdout = %q, want the result line then %q", stdout, want)
	}
}

func TestReindex_NoStore_FailsNotInitialized(t *testing.T) {
	root := newGitRepo(t)

	stdout, _, exitCode := runCLI(t, root, "reindex", "--json")

	errorBody, isObject := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || !isObject || errorBody["code"] != string(CodeNotInitialized) {
		t.Errorf("exit = %d stdout = %s, want a not_initialized failure", exitCode, stdout)
	}
}
