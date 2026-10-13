package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// newMoveStore creates a store with login-flow, a child below it, and an issue depending on
// login-flow by id and on renamed-issue by its alias.
func newMoveStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"login-flow.md":    readIssue("Login flow", "open", "", "\nSee login-flow.\n"),
		"renamed-issue.md": readIssue("Renamed", "open", "aliases: [former-name]\n", ""),
		"session-work.md":  readIssue("Session work", "open", "deps: [login-flow, former-name]\n", ""),
	})
	mustRun(t, root, "create", "--id", "child-issue", "--title", "Child", "--parent", "login-flow")
	return root
}

func TestMv_Rename_OldIDResolvesAndDepsFollow(t *testing.T) {
	root := newMoveStore(t)

	stdout := mustRun(t, root, "mv", "--kv", "login", "auth-flow")

	if !strings.Contains(stdout, "ID=auth-flow\nPATH=.capcom/issues/auth-flow.md\n") {
		t.Errorf("stdout = %q", stdout)
	}
	shown := decodeEnvelope(t, mustRun(t, root, "show", "--json", "login-flow"))
	if data, _ := shown["data"].([]any); len(data) != 1 || data[0].(map[string]any)["id"] != "auth-flow" {
		t.Errorf("show login-flow = %v", shown)
	}
	content := readStoreFile(t, root, "auth-flow.md")
	if !strings.Contains(content, "aliases: [login-flow]\n") || !strings.HasSuffix(content, "\nSee login-flow.\n") {
		t.Errorf("auth-flow.md = %q", content)
	}
	if session := readStoreFile(t, root, "session-work.md"); !strings.Contains(session, "deps: [auth-flow, former-name]\n") {
		t.Errorf("session-work.md = %q", session)
	}
	if ids := listIDs(t, root, "--parent", "auth-flow"); !slices.Equal(ids, []string{"child-issue"}) {
		t.Errorf("children of auth-flow = %v", ids)
	}
}

func TestMv_Parent_MovesBelowThenToTop(t *testing.T) {
	root := newMoveStore(t)

	mustRun(t, root, "mv", "session-work", "--parent", "child-issue")
	below := listIDs(t, root, "--parent", "child-issue")
	mustRun(t, root, "mv", "session-work", "--parent", ".")

	if !slices.Equal(below, []string{"session-work"}) {
		t.Errorf("children of child-issue = %v", below)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "session-work.md")); err != nil {
		t.Errorf("session-work.md at the top: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "login-flow", "child-issue")); !os.IsNotExist(err) {
		t.Errorf("emptied child-issue directory: stat error = %v, want it removed", err)
	}
}

func TestMv_EmptiedOldParentNotRemovable_MovesAndWarns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not stop directory removal on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root removes from a read-only directory")
	}
	root := newIssueStore(t, nil)
	mustRun(t, root, "create", "--id", "old-parent", "--title", "Old parent")
	mustRun(t, root, "create", "--id", "moved-child", "--title", "Moved child", "--parent", "old-parent")
	mustRun(t, root, "create", "--id", "new-parent", "--title", "New parent")
	mustRun(t, root, "create", "--id", "kept-child", "--title", "Kept child", "--parent", "new-parent")
	readOnlyDir := filepath.Join(root, ".capcom", "issues")
	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	stdout, stderr, exitCode := runCLI(t, root, "mv", "--json", "moved-child", "--parent", "new-parent")

	if exitCode != ExitOK {
		t.Fatalf("mv = %d\n%s%s; want it to succeed", exitCode, stdout, stderr)
	}
	warnings, _ := decodeEnvelope(t, stdout)["warnings"].([]any)
	want := ".capcom/issues/old-parent: remove .capcom/issues/old-parent: permission denied; " +
		"the empty directory was not removed"
	if len(warnings) != 1 || warnings[0] != want {
		t.Errorf("warnings = %q; want [%q]", warnings, want)
	}
	if _, err := os.Stat(filepath.Join(readOnlyDir, "new-parent", "moved-child.md")); err != nil {
		t.Errorf("moved-child.md below new-parent: %v", err)
	}
}

func TestMv_EmptyParent_FailsUsageChangingNothing(t *testing.T) {
	for _, parent := range []string{"", " \t"} {
		root := newMoveStore(t)
		childPath := filepath.Join(root, ".capcom", "issues", "login-flow", "child-issue.md")
		before, err := os.ReadFile(childPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		_, stderr, exitCode := runCLI(t, root, "mv", "child-issue", "--parent", parent)

		after, err := os.ReadFile(childPath)
		if exitCode != ExitUsage || !strings.Contains(stderr, `--parent needs an id or "."`) {
			t.Errorf("--parent %q: exit = %d stderr = %q", parent, exitCode, stderr)
		}
		if err != nil || string(after) != string(before) {
			t.Errorf("--parent %q: child-issue.md changed or moved: %v", parent, err)
		}
	}
}

func TestMv_ParentDot_MovesChildToTop(t *testing.T) {
	root := newMoveStore(t)

	mustRun(t, root, "mv", "child-issue", "--parent", ".")

	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "child-issue.md")); err != nil {
		t.Errorf("child-issue.md at the top: %v", err)
	}
}

func TestMv_UnderOwnChild_FailsInvalid(t *testing.T) {
	root := newMoveStore(t)

	_, stderr, exitCode := runCLI(t, root, "mv", "login-flow", "--parent", "child-issue")

	if exitCode != ExitError || !strings.Contains(stderr, "below itself or its own descendants") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "login-flow.md")); err != nil {
		t.Errorf("login-flow.md: %v", err)
	}
}

func TestMv_OntoTakenIDOrAlias_FailsExists(t *testing.T) {
	root := newMoveStore(t)

	for _, newID := range []string{"renamed-issue", "former-name"} {
		stdout, _, exitCode := runCLI(t, root, "mv", "--json", "login-flow", newID)

		envelope := decodeEnvelope(t, stdout)
		failure, _ := envelope["error"].(map[string]any)
		if exitCode != ExitError || failure["code"] != string(CodeExists) || failure["path"] != ".capcom/issues/renamed-issue.md" {
			t.Errorf("%s: exit = %d envelope = %v", newID, exitCode, envelope)
		}
	}
}

func TestMv_UnknownIssueOrParent_FailsNotFound(t *testing.T) {
	root := newMoveStore(t)

	for _, args := range [][]string{{"mv", "no-such-issue", "new-name"}, {"mv", "login-flow", "--parent", "no-such-issue"}} {
		_, stderr, exitCode := runCLI(t, root, args...)

		if exitCode != ExitNotFound {
			t.Errorf("%v: exit = %d stderr = %q", args, exitCode, stderr)
		}
	}
}

func TestMv_NothingToDo_FailsUsage(t *testing.T) {
	root := newMoveStore(t)

	_, stderr, exitCode := runCLI(t, root, "mv", "login-flow")

	if exitCode != ExitUsage || !strings.Contains(stderr, "new id, --parent") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
}

func TestMv_SecondHolderOfDuplicateAlias_FailsNamingTheFirstRelative(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"export-csv.md": readIssue("Export CSV", "open", "aliases: [apogee-qi3]\n", ""),
		"login-flow.md": readIssue("Login flow", "open", "aliases: [apogee-qi3]\n", ""),
	})

	_, stderr, exitCode := runCLI(t, root, "mv", "login-flow", "auth-flow")

	want := `.capcom/issues/login-flow.md: duplicate alias "apogee-qi3": also an alias of .capcom/issues/export-csv.md`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if strings.Contains(stderr, root) {
		t.Errorf("stderr = %q names an absolute path", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "login-flow.md")); err != nil {
		t.Errorf("login-flow.md: %v", err)
	}
}

func TestMv_UnrelatedBadFileName_MovesAndWarnsOnce(t *testing.T) {
	root := newMoveStore(t)
	writeIssueFiles(t, root, map[string]string{"Bad_Name.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "mv", "login-flow", "auth-flow")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "auth-flow.md")
	if count := strings.Count(stdout+stderr, ".capcom/issues/Bad_Name.md"); count != 1 {
		t.Errorf("Bad_Name.md named %d times, want once\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
}

func TestMv_UnrelatedInvalidIDDirectory_MovesAndWarnsOnce(t *testing.T) {
	root := newMoveStore(t)
	writeIssueFiles(t, root, map[string]string{
		"Bad_Name.md":             validIssue,
		"Bad_Name/child-issue.md": validIssue,
	})

	stdout, stderr, exitCode := runCLI(t, root, "mv", "login-flow", "auth-flow")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "auth-flow.md")
	want := ".capcom/issues/Bad_Name: directory left out: its node file Bad_Name.md has an invalid id"
	if count := strings.Count(stdout+stderr, want); count != 1 {
		t.Errorf("%q warned %d times, want once\nstdout = %q\nstderr = %q", want, count, stdout, stderr)
	}
}

func TestMv_BadFileInMovedChildrenDir_WarnsWithNewPath(t *testing.T) {
	root := newMoveStore(t)
	writeIssueFiles(t, root, map[string]string{"login-flow/Bad_Name.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "mv", "login-flow", "auth-flow")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "auth-flow/Bad_Name.md")
	if count := strings.Count(stdout+stderr, ".capcom/issues/login-flow/Bad_Name.md"); count != 0 {
		t.Errorf("old path named %d times, want 0\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
	if count := strings.Count(stdout+stderr, ".capcom/issues/auth-flow/Bad_Name.md"); count != 1 {
		t.Errorf("new path named %d times, want once\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
}

func TestMv_DuplicateID_FailsNamingBothFiles(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"dup-issue.md":  readIssue("Duplicate", "open", "", ""),
		"login-flow.md": readIssue("Login flow", "open", "", ""),
	})
	writeIssueFiles(t, root, map[string]string{"login-flow/dup-issue.md": readIssue("Duplicate too", "open", "", "")})

	_, stderr, exitCode := runCLI(t, root, "mv", "dup-issue", "moved-issue")

	want := `.capcom/issues/login-flow/dup-issue.md: duplicate id "dup-issue", also at .capcom/issues/dup-issue.md`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "dup-issue.md")); err != nil {
		t.Errorf("dup-issue.md: %v", err)
	}
}

func TestMv_OntoIDInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newMoveStore(t)
	writeIssueFiles(t, root, map[string]string{"lost-dir/dup-id.md": validIssue})

	_, stderr, exitCode := runCLI(t, root, "mv", "renamed-issue", "dup-id")

	want := `.capcom/issues/lost-dir/dup-id.md: id "dup-id" is already taken by this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	readStoreFile(t, root, "renamed-issue.md")
}

func TestMv_OntoAliasInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newMoveStore(t)
	writeIssueFiles(t, root, map[string]string{
		"lost-dir/other-id.md": readIssue("Lost", "open", "aliases: [dup-id]\n", ""),
	})

	_, stderr, exitCode := runCLI(t, root, "mv", "renamed-issue", "dup-id")

	want := `.capcom/issues/lost-dir/other-id.md: id "dup-id" is already an alias of this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	readStoreFile(t, root, "renamed-issue.md")
}

func TestMv_OntoAliasOfUnparseableIssue_FailsExistsNamingTheFile(t *testing.T) {
	root := newMoveStore(t)
	mustRun(t, root, "create", "--id", "first-issue", "--title", "First")
	mustRun(t, root, "mv", "first-issue", "moved-issue")
	breakIssueFile(t, root, "moved-issue.md")

	stdout, _, exitCode := runCLI(t, root, "mv", "--json", "login-flow", "first-issue")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	wantMessage := `id "first-issue" may be an alias of issue moved-issue, whose file does not parse: fix it first`
	if exitCode != ExitError || failure["code"] != string(CodeExists) ||
		failure["path"] != ".capcom/issues/moved-issue.md" || failure["message"] != wantMessage {
		t.Errorf("exit = %d error = %v, want exists at moved-issue.md with %q", exitCode, failure, wantMessage)
	}
	readStoreFile(t, root, "login-flow.md")
}

func TestMv_OntoAliasOfUnparseableIssueInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newMoveStore(t)
	writeIssueFiles(t, root, map[string]string{
		"lost-dir/other-id.md": withConflictAroundTitle(readIssue("Lost", "open", "aliases: [dup-id]\n", "")),
	})

	_, stderr, exitCode := runCLI(t, root, "mv", "renamed-issue", "dup-id")

	want := `.capcom/issues/lost-dir/other-id.md: id "dup-id" may be an alias of issue other-id, ` +
		"whose file does not parse: fix it first"
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	readStoreFile(t, root, "renamed-issue.md")
}

func TestMv_Rename_RelatedLinksFollow(t *testing.T) {
	root := newMoveStore(t)
	mustRun(t, root, "related", "add", "session-work", "login-flow")
	mustRun(t, root, "related", "add", "renamed-issue", "session-work")

	mustRun(t, root, "mv", "login-flow", "auth-flow")
	mustRun(t, root, "mv", "session-work", "session-tasks")

	if session := readStoreFile(t, root, "session-tasks.md"); !strings.Contains(session, "related: [auth-flow]\n") {
		t.Errorf("session-tasks.md = %q", session)
	}
	if renamed := readStoreFile(t, root, "renamed-issue.md"); !strings.Contains(renamed, "related: [session-tasks]\n") {
		t.Errorf("renamed-issue.md = %q", renamed)
	}
	holder, other := showRelated(t, root, "session-tasks"), showRelated(t, root, "auth-flow")
	if !slices.Equal(holder, []string{"auth-flow", "renamed-issue"}) || !slices.Equal(other, []string{"session-tasks"}) {
		t.Errorf("related of session-tasks = %v, of auth-flow = %v", holder, other)
	}
	assertCheckPasses(t, root)
}

// assertWorktreeRelativeFailure checks that the --json failure in stdout is invalid at wantPath,
// with a line when hasLine, and that its message names no path by the worktree's absolute root.
func assertWorktreeRelativeFailure(t *testing.T, root string, stdout string, wantPath string, hasLine bool) {
	t.Helper()
	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	message, _ := failure["message"].(string)
	line, _ := failure["line"].(float64)
	if failure["code"] != string(CodeInvalid) || failure["path"] != wantPath || (line > 0) != hasLine ||
		strings.Contains(message, root) {
		t.Errorf("error = %v, want invalid at %s (line: %t) naming no path under %s", failure, wantPath, hasLine, root)
	}
}

func TestMv_UnparseableIssue_FailsInvalidAtItsRelativeFileAndLine(t *testing.T) {
	root := newMoveStore(t)
	breakIssueFile(t, root, "renamed-issue.md")

	stdout, _, exitCode := runCLI(t, root, "mv", "--json", "renamed-issue", "other-name")

	if exitCode != ExitError {
		t.Errorf("exit = %d, want %d", exitCode, ExitError)
	}
	assertWorktreeRelativeFailure(t, root, stdout, ".capcom/issues/renamed-issue.md", true)
}

func TestMv_IfMatchFromShow_Moves(t *testing.T) {
	root := newMoveStore(t)
	etag := textETag(mustRun(t, root, "show", "renamed-issue"))

	stdout, stderr, exitCode := runCLI(t, root, "mv", "renamed-issue", "other-name", "--if-match", etag)

	if exitCode != ExitOK || !issueFileExists(t, root, "other-name.md") {
		t.Errorf("mv --if-match %s = %d\n%s%s", etag, exitCode, stdout, stderr)
	}
}

func TestMv_StaleIfMatch_ConflictsAndMovesNothing(t *testing.T) {
	root := newMoveStore(t)
	etag := textETag(mustRun(t, root, "show", "login-flow"))
	mustRun(t, root, "update", "login-flow", "--title", "Login steps")
	before := readStoreFile(t, root, "login-flow.md")
	dependent := readStoreFile(t, root, "session-work.md")

	stdout, _, exitCode := runCLI(t, root, "mv", "--json", "login-flow", "auth-flow", "--if-match", etag)

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	message, _ := failure["message"].(string)
	if exitCode != ExitError || failure["code"] != string(CodeConflict) ||
		failure["path"] != ".capcom/issues/login-flow.md" || !strings.Contains(message, "show the issue again") {
		t.Errorf("stale --if-match = %d, error %v; want conflict at the issue file", exitCode, failure)
	}
	if issueFileExists(t, root, "auth-flow.md") || !issueFileExists(t, root, "login-flow/child-issue.md") ||
		readStoreFile(t, root, "login-flow.md") != before || readStoreFile(t, root, "session-work.md") != dependent {
		t.Errorf("the tree changed")
	}
}

func TestMv_BelowDuplicatedID_FailsNamingBothFiles(t *testing.T) {
	root, files := newDuplicateIssueStore(t)

	_, stderr, exitCode := runCLI(t, root, "mv", "foo-bar", "--parent", "foo")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/foo.md", ".capcom/issues/sub-issue/foo.md")
	assertIssueFilesUnchanged(t, root, files)
	if issueFileExists(t, root, "foo/foo-bar.md") {
		t.Error("foo-bar was moved below the duplicated id")
	}
}
