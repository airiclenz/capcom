package cli

import (
	"database/sql"
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

func TestReindex_CorruptIndex_ListFailsWithReindexHintAndReindexRepairsIt(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue, "other-issue.md": validIssue})
	mustRun(t, root, "reindex")
	indexPath := filepath.Join(root, ".git", "capcom", "index.db")
	if err := os.WriteFile(indexPath, []byte(strings.Repeat("not a database ", 512)), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	assertFailureHint(t, root, "capcom reindex", "list")
	listJSON, _, listExit := runCLI(t, root, "list", "--json")
	_, stderr, reindexExit := runCLI(t, root, "reindex")
	listed := mustRun(t, root, "list")

	errorBody, isObject := decodeEnvelope(t, listJSON)["error"].(map[string]any)
	if listExit != ExitError || !isObject || errorBody["code"] != string(CodeInvalid) {
		t.Errorf("list --json = %d %s, want an invalid failure", listExit, listJSON)
	}
	if reindexExit != ExitOK {
		t.Fatalf("reindex = %d, stderr %q, want the corrupt index rebuilt", reindexExit, stderr)
	}
	for _, id := range []string{"valid-issue", "other-issue"} {
		if !strings.Contains(listed, id) {
			t.Errorf("list after reindex:\n%s\nwant %s", listed, id)
		}
	}
}

// holdIndexLock holds an EXCLUSIVE transaction on the index at indexPath through a second
// connection until the test ends, the way a capcom reindex rebuilding it does.
func holdIndexLock(t *testing.T, indexPath string) {
	t.Helper()
	holder, err := sql.Open("sqlite", indexPath+"?_txlock=exclusive")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	held, err := holder.Begin()
	if err != nil {
		t.Fatalf("BEGIN EXCLUSIVE: %v", err)
	}
	t.Cleanup(func() {
		if err := held.Rollback(); err != nil {
			t.Errorf("Rollback: %v", err)
		}
		if err := holder.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
}

// TestReindex_BusyIndex_ListFailsInvalidNamingReindexWithoutReindexHint pays the index's
// five-second busy timeout on each of its two runs: the CLI has no seam to shorten it.
func TestReindex_BusyIndex_ListFailsInvalidNamingReindexWithoutReindexHint(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	mustRun(t, root, "reindex")
	indexPath := filepath.Join(root, ".git", "capcom", "index.db")
	holdIndexLock(t, indexPath)
	wantMessage := indexPath + ": the index is busy: another capcom command, " +
		"such as capcom reindex, is rebuilding or writing it; retry when it finishes"

	listJSON, _, jsonExit := runCLI(t, root, "list", "--json")
	_, stderr, textExit := runCLI(t, root, "list")

	errorBody, isObject := decodeEnvelope(t, listJSON)["error"].(map[string]any)
	if jsonExit != ExitError || !isObject || errorBody["code"] != string(CodeInvalid) {
		t.Errorf("list --json = %d %s, want an invalid failure", jsonExit, listJSON)
	}
	if isObject && (errorBody["message"] != wantMessage || errorBody["hint"] == reindexHint) {
		t.Errorf("list --json error = %v, want message %q and no reindex hint", errorBody, wantMessage)
	}
	isReindexHinted := lastLine(stderr) == "hint: "+reindexHint
	if textExit != ExitError || !strings.Contains(stderr, wantMessage) || isReindexHinted {
		t.Errorf("list = %d, stderr %q; want %q and no reindex hint line", textExit, stderr, wantMessage)
	}
}
