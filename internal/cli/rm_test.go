package cli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// newRmStore creates a store with login-flow and a child below it, renamed-issue known also by an
// alias, and session-work depending on login-flow by id.
func newRmStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"login-flow.md":    readIssue("Login flow", "open", "", ""),
		"renamed-issue.md": readIssue("Renamed", "open", "aliases: [former-name]\n", ""),
		"session-work.md":  readIssue("Session work", "open", "deps: [login-flow]\n", ""),
	})
	mustRun(t, root, "create", "--id", "child-issue", "--title", "Child", "--parent", "login-flow")
	return root
}

// issueFileExists reports whether the slash path name exists below .capcom/issues/.
func issueFileExists(t *testing.T, root string, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, ".capcom", "issues", filepath.FromSlash(name)))
	return err == nil
}

// assertCheckPasses fails the test unless capcom check exits ok.
func assertCheckPasses(t *testing.T, root string) {
	t.Helper()
	if stdout, _, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after rm = %d\n%s", exitCode, stdout)
	}
}

func TestRm_Leaf_RemovesTheIssue(t *testing.T) {
	root := newRmStore(t)

	stdout := mustRun(t, root, "rm", "renamed")

	if stdout != "id: renamed-issue\npath: .capcom/issues/renamed-issue.md\n" {
		t.Errorf("rm stdout = %q", stdout)
	}
	if issueFileExists(t, root, "renamed-issue.md") {
		t.Errorf("renamed-issue.md was not removed")
	}
	jsonOutput, _, exitCode := runCLI(t, root, "show", "--json", "former-name")
	if exitCode != ExitNotFound {
		t.Errorf("show of the removed issue's alias = %d\n%s", exitCode, jsonOutput)
	}
	assertCheckPasses(t, root)
}

func TestRm_LastChild_TakesTheEmptyDirectory(t *testing.T) {
	root := newRmStore(t)

	mustRun(t, root, "rm", "child-issue")

	if issueFileExists(t, root, "login-flow") {
		t.Errorf("the emptied login-flow directory was not removed")
	}
	if !issueFileExists(t, root, "login-flow.md") {
		t.Errorf("the parent was removed")
	}
	assertCheckPasses(t, root)
}

func TestRm_WithChildren_IsRefusedNamingTheFileAndCount(t *testing.T) {
	root := newRmStore(t)

	_, stderr, exitCode := runCLI(t, root, "rm", "login-flow", "--drop-deps")

	wantPrefix := "capcom: .capcom/issues/login-flow.md: issue login-flow has 1 child issue"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm with children = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !issueFileExists(t, root, "login-flow.md") || !issueFileExists(t, root, "login-flow/child-issue.md") {
		t.Errorf("files were removed")
	}
}

func TestRm_ChildrenDirectoryWithAStrayFile_IsRefused(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"renamed-issue/notes.txt": "stray"})

	_, stderr, exitCode := runCLI(t, root, "rm", "renamed-issue")

	wantPrefix := "capcom: .capcom/issues/renamed-issue: the issue's children directory still holds 1 entry"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !issueFileExists(t, root, "renamed-issue.md") || !issueFileExists(t, root, "renamed-issue/notes.txt") {
		t.Errorf("files were removed")
	}
}

func TestRm_WithDependent_IsRefusedNamingIt(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")

	_, stderr, exitCode := runCLI(t, root, "rm", "login-flow")

	wantPrefix := "capcom: .capcom/issues/login-flow.md: issue login-flow is a dependency of session-work; pass --drop-deps"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm with a dependent = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !issueFileExists(t, root, "login-flow.md") {
		t.Errorf("login-flow.md was removed")
	}
}

func TestRm_DropDeps_RemovesTheDepsEntries(t *testing.T) {
	cases := []struct {
		name string
		deps string
		want string
	}{
		{name: "by id", deps: "deps: [renamed-issue, login-flow]\n", want: "deps: [login-flow]\n"},
		{name: "by alias", deps: "deps: [former-name]\n", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRmStore(t)
			writeIssueFiles(t, root, map[string]string{"blocked-work.md": readIssue("Blocked", "open", tc.deps, "")})

			stdout := mustRun(t, root, "rm", "--kv", "renamed-issue", "--drop-deps")

			if !strings.Contains(stdout, "DEPS_DROPPED=[blocked-work]\n") {
				t.Errorf("stdout = %q", stdout)
			}
			content := readStoreFile(t, root, "blocked-work.md")
			if strings.Contains(content, "renamed-issue") || strings.Contains(content, "former-name") ||
				(tc.want != "" && !strings.Contains(content, tc.want)) || (tc.want == "" && strings.Contains(content, "deps:")) {
				t.Errorf("blocked-work.md = %q, want deps %q", content, tc.want)
			}
			if issueFileExists(t, root, "renamed-issue.md") {
				t.Errorf("renamed-issue.md was not removed")
			}
			assertCheckPasses(t, root)
		})
	}
}

func TestRm_StaleIfMatch_ConflictsAndKeepsTheFile(t *testing.T) {
	root := newRmStore(t)
	etag := textETag(mustRun(t, root, "show", "renamed-issue"))
	mustRun(t, root, "update", "renamed-issue", "--title", "Renamed again")

	stdout, _, exitCode := runCLI(t, root, "rm", "--json", "renamed-issue", "--if-match", etag)

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeConflict) || failure["path"] != ".capcom/issues/renamed-issue.md" {
		t.Errorf("stale --if-match = %d, error %v; want conflict at the issue file", exitCode, failure)
	}
	if !issueFileExists(t, root, "renamed-issue.md") {
		t.Errorf("the issue was removed")
	}
}

func TestRm_UnrelatedBadFileName_RemovesAndWarnsOnce(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"Bad_Name.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "rm", "renamed-issue")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if issueFileExists(t, root, "renamed-issue.md") {
		t.Error("renamed-issue.md was not removed")
	}
	assertWarnedOnce(t, stdout+stderr, ".capcom/issues/Bad_Name.md")
}

func TestRm_UnparseableIssue_WarnsDepsNotChecked(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"broken-issue.md": "---\ntitle: [unclosed\n---\n"})

	stdout, stderr, exitCode := runCLI(t, root, "rm", "renamed-issue")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if !strings.Contains(stdout+stderr, ".capcom/issues/broken-issue.md: deps not checked for renamed-issue") {
		t.Errorf("no deps-not-checked warning\nstdout = %q\nstderr = %q", stdout, stderr)
	}
}

func TestRm_JSON_ReportsKindAndData(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")

	envelope := decodeEnvelope(t, mustRun(t, root, "rm", "--json", "login-flow", "--drop-deps"))

	data, _ := envelope["data"].(map[string]any)
	dropped, _ := data["deps_dropped"].([]any)
	if envelope["kind"] != kindIssueRemove || data["id"] != "login-flow" || data["path"] != ".capcom/issues/login-flow.md" ||
		!slices.Equal(dropped, []any{"session-work"}) {
		t.Errorf("rm --json = %v", envelope)
	}
	if session := readStoreFile(t, root, "session-work.md"); strings.Contains(session, "deps:") {
		t.Errorf("session-work.md = %q", session)
	}
}

func TestRm_DependsOnItself_IsRemoved(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"loop-issue.md": readIssue("Loop", "open", "deps: [loop-issue]\n", "")})

	stdout, stderr, exitCode := runCLI(t, root, "rm", "--json", "loop-issue")

	data, _ := decodeEnvelope(t, stdout)["data"].(map[string]any)
	if exitCode != ExitOK || issueFileExists(t, root, "loop-issue.md") {
		t.Fatalf("rm of a self-dependent issue = %d\n%s%s", exitCode, stdout, stderr)
	}
	if dropped, _ := data["deps_dropped"].([]any); len(dropped) != 0 {
		t.Errorf("deps_dropped = %v, want none", dropped)
	}
}

func TestRm_RelatedHolders_DropsTheirEntriesWithoutRefusing(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "related", "add", "login-flow", "renamed-issue")
	writeIssueFiles(t, root, map[string]string{
		"linked-work.md": readIssue("Linked", "open", "related: [former-name, session-work]\n", ""),
	})

	stdout := mustRun(t, root, "rm", "renamed-issue")

	if !strings.HasSuffix(stdout, "related_dropped: [linked-work, login-flow]\n") {
		t.Errorf("rm stdout = %q", stdout)
	}
	if login := readStoreFile(t, root, "login-flow.md"); strings.Contains(login, "related:") {
		t.Errorf("login-flow.md = %q, want no related key", login)
	}
	if linked := readStoreFile(t, root, "linked-work.md"); !strings.Contains(linked, "related: [session-work]\n") {
		t.Errorf("linked-work.md = %q", linked)
	}
	if related := showRelated(t, root, "login-flow"); len(related) != 0 {
		t.Errorf("related of login-flow = %v, want none", related)
	}
	assertCheckPasses(t, root)
}

func TestRm_JSON_ReportsRelatedDropped(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "related", "add", "session-work", "renamed-issue")

	envelope := decodeEnvelope(t, mustRun(t, root, "rm", "--json", "renamed-issue"))

	data, _ := envelope["data"].(map[string]any)
	related, _ := data["related_dropped"].([]any)
	deps, isList := data["deps_dropped"].([]any)
	if !slices.Equal(related, []any{"session-work"}) || !isList || len(deps) != 0 {
		t.Errorf("rm --json = %v", envelope)
	}
}

func TestRm_DependentAndRelatedHolder_RefusesAndKeepsTheHolder(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")
	mustRun(t, root, "related", "add", "renamed-issue", "login-flow")
	holder := readStoreFile(t, root, "renamed-issue.md")

	_, stderr, exitCode := runCLI(t, root, "rm", "login-flow")

	wantPrefix := "capcom: .capcom/issues/login-flow.md: issue login-flow is a dependency of session-work"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if got := readStoreFile(t, root, "renamed-issue.md"); got != holder {
		t.Errorf("renamed-issue.md = %q, want it byte for byte as before: %q", got, holder)
	}
	if !issueFileExists(t, root, "login-flow.md") {
		t.Errorf("login-flow.md was removed")
	}
}

func TestRm_DropDeps_FileHoldingDepAndRelated_LosesBoth(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")
	mustRun(t, root, "related", "add", "session-work", "login-flow")
	mustRun(t, root, "related", "add", "session-work", "renamed-issue")

	envelope := decodeEnvelope(t, mustRun(t, root, "rm", "--json", "login-flow", "--drop-deps"))

	data, _ := envelope["data"].(map[string]any)
	deps, _ := data["deps_dropped"].([]any)
	related, _ := data["related_dropped"].([]any)
	if !slices.Equal(deps, []any{"session-work"}) || !slices.Equal(related, []any{"session-work"}) {
		t.Errorf("rm --json = %v", envelope)
	}
	if session := readStoreFile(t, root, "session-work.md"); session != readIssue("Session work", "open", "related: [renamed-issue]\n", "") {
		t.Errorf("session-work.md = %q", session)
	}
	assertCheckPasses(t, root)
}

func TestRm_IssueFileRemovalFails_RestoresRewriteBytesAndMode(t *testing.T) {
	root := newRmStore(t)
	original := strings.ReplaceAll(readIssue("Blocked", "open", "deps: [renamed-issue]\n", ""), "\n", "\r\n")
	writeIssueFiles(t, root, map[string]string{"blocked-work.md": original})
	dependent := filepath.Join(root, ".capcom", "issues", "blocked-work.md")
	if err := os.Chmod(dependent, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dependent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeFile = os.Remove })
	removeFile = func(path string) error {
		if filepath.Base(path) == "renamed-issue.md" {
			return errors.New("injected remove failure")
		}
		return os.Remove(path)
	}

	_, stderr, exitCode := runCLI(t, root, "rm", "renamed-issue", "--drop-deps")

	if exitCode == 0 || !strings.Contains(stderr, ".capcom/issues/renamed-issue.md") {
		t.Errorf("rm exit %d, stderr %q, want a failure naming renamed-issue.md", exitCode, stderr)
	}
	if content := readStoreFile(t, root, "blocked-work.md"); content != original {
		t.Errorf("blocked-work.md = %q, want %q", content, original)
	}
	after, err := os.Stat(dependent)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && after.Mode().Perm() != before.Mode().Perm() {
		t.Errorf("blocked-work.md mode = %v, want %v kept", after.Mode().Perm(), before.Mode().Perm())
	}
	if !issueFileExists(t, root, "renamed-issue.md") {
		t.Errorf("renamed-issue.md was removed")
	}
}
