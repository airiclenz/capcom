package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// closedPrefix matches the closed time capcom writes: RFC3339 UTC to the second.
const closedPrefix = "closed: 20"

// shippedConfig names no status done: its done category is "shipped" and its todo one "triage".
const shippedConfig = `statuses:
  triage: todo
  open: todo
  shipped: done
  binned: dropped
`

func TestClose_SeveralIDsOneAlreadyClosed_ClosesTheRestAndReportsEach(t *testing.T) {
	root := newWriteStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "close", "--kv", "login-flow", "shipped-fix", "former-name",
		"--reason", "merged in abc123")

	want := "ID=login-flow\nSTATUS=closed\nID=shipped-fix\nSTATUS=already_closed\nID=renamed-issue\nSTATUS=closed\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Fatalf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
	login := readStoreFile(t, root, "login-flow.md")
	if !strings.Contains(login, "status: done\n") || !strings.Contains(login, closedPrefix) ||
		!strings.Contains(login, "reason: merged in abc123\n") {
		t.Errorf("login-flow.md = %q", login)
	}
	if shipped := readStoreFile(t, root, "shipped-fix.md"); !strings.Contains(shipped, "reason: finished\n") {
		t.Errorf("already closed shipped-fix.md was rewritten: %q", shipped)
	}
}

func TestClose_OneIDMissing_ClosesTheOthersAndExitsNotFound(t *testing.T) {
	root := newWriteStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "close", "--kv", "no-such-issue", "login-flow", "--reason", "done")

	want := "ID=no-such-issue\nSTATUS=not_found\nID=login-flow\nSTATUS=closed\n"
	if exitCode != ExitNotFound || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestClose_WithoutReason_FailsAndWritesNothing(t *testing.T) {
	cases := map[string][]string{
		"no flag":     {"close", "login-flow"},
		"blank flag":  {"close", "login-flow", "--reason", "  "},
		"reopen flag": {"reopen", "shipped-fix"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			root := newWriteStore(t)
			before := readStoreFile(t, root, "login-flow.md")

			_, stderr, exitCode := runCLI(t, root, args...)

			if exitCode == ExitOK || !strings.Contains(stderr, "--reason") {
				t.Errorf("exit = %d stderr = %q, want a failure naming --reason", exitCode, stderr)
			}
			if after := readStoreFile(t, root, "login-flow.md"); after != before {
				t.Errorf("login-flow.md changed:\n%s", after)
			}
		})
	}
}

func TestClose_AsDropped_SetsTheDroppedStatus(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "close", "login-flow", "--status", "dropped", "--reason", "superseded")

	content := readStoreFile(t, root, "login-flow.md")
	if !strings.Contains(content, "status: dropped\n") || !strings.Contains(content, "reason: superseded\n") {
		t.Errorf("login-flow.md = %q", content)
	}
}

func TestClose_StatusNotClosed_FailsInvalid(t *testing.T) {
	root := newWriteStore(t)

	stdout, _, exitCode := runCLI(t, root, "close", "--json", "login-flow", "--status", "parked", "--reason", "x")

	envelope := decodeEnvelope(t, stdout)
	if code := envelope["error"].(map[string]any)["code"]; exitCode != ExitError || code != string(CodeInvalid) {
		t.Errorf("exit = %d envelope = %v, want invalid", exitCode, envelope)
	}
}

func TestClose_ConfigWithoutDoneStatus_UsesTheFirstDoneCategoryStatus(t *testing.T) {
	root := newIssueStore(t, nil)
	if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(shippedConfig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writeIssueFiles(t, root, map[string]string{"login-flow.md": readIssue("Login flow", "open", "", "")})

	mustRun(t, root, "close", "login-flow", "--reason", "shipped in 1.2")

	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "status: shipped\n") {
		t.Fatalf("login-flow.md = %q", content)
	}
	mustRun(t, root, "reopen", "login-flow", "--reason", "regressed")
	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "status: triage\n") {
		t.Errorf("reopened login-flow.md = %q, want the first todo status", content)
	}
}

func TestReopen_ClosedIssue_ClearsClosingFieldsAndLogsTheReason(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "reopen", "shipped-fix", "--reason", "the fix regressed")

	content := readStoreFile(t, root, "shipped-fix.md")
	want := "---\ntitle: Shipped fix\nstatus: open\ntype: task\npriority: 2\ncreated: 2026-10-11T09:00:00Z\n---\n" +
		"\nDone.\n\n## Log\n\n- " + today() + " Test Author: reopened: the fix regressed\n"
	if content != want {
		t.Errorf("shipped-fix.md = %q\nwant            %q", content, want)
	}
}

func TestReopen_OpenIssue_FailsInvalid(t *testing.T) {
	root := newWriteStore(t)

	_, stderr, exitCode := runCLI(t, root, "reopen", "login-flow", "--reason", "again")

	if exitCode != ExitError || !strings.Contains(stderr, "login-flow.md: ") || !strings.Contains(stderr, "not closed") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
}

// newInvalidFooStore creates an issue store where foo.md is invalid beside a valid foo-bar, and
// returns its root and the content of every issue file by slash path.
func newInvalidFooStore(t *testing.T) (string, map[string]string) {
	t.Helper()
	files := map[string]string{
		"foo.md":     "---\nstatus: open\n---\n",
		"foo-bar.md": readIssue("Foo bar", "open", "", ""),
	}
	root := newIssueStore(t, nil)
	writeIssueFiles(t, root, files)
	return root, files
}

func TestClose_IDOfInvalidFile_FailsNotFoundAndLeavesThePrefixMatchOpen(t *testing.T) {
	root, files := newInvalidFooStore(t)

	_, stderr, exitCode := runCLI(t, root, "close", "foo", "--reason", "x")

	if exitCode != ExitNotFound || !strings.Contains(stderr, `id "foo" not found: .capcom/issues/foo.md`) {
		t.Errorf("exit = %d stderr = %q, want not found naming foo.md", exitCode, stderr)
	}
	assertIssueFilesUnchanged(t, root, files)
}

func TestClose_DuplicatedID_FailsNamingBothFilesAndLeavesThePrefixMatchOpen(t *testing.T) {
	root, files := newDuplicateIssueStore(t)

	_, stderr, exitCode := runCLI(t, root, "close", "foo", "--reason", "x")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/foo.md", ".capcom/issues/sub-issue/foo.md")
	assertIssueFilesUnchanged(t, root, files)
}
