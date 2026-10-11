package cli

import (
	"os"
	"path/filepath"
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
