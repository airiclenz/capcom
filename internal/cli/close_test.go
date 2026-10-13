package cli

import (
	"os"
	"path/filepath"
	"reflect"
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
	stdout, _, jsonExitCode := runCLI(t, root, "reopen", "--json", "login-flow", "--reason", "again")

	wantStderr := "capcom: .capcom/issues/login-flow.md: status \"open\" is in the todo category, " +
		"so the issue is not closed; only a closed issue can be reopened\n"
	if exitCode != ExitError || stderr != wantStderr {
		t.Errorf("exit = %d\nstderr = %q\nwant     %q", exitCode, stderr, wantStderr)
	}
	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if jsonExitCode != ExitError || failure["code"] != string(CodeInvalid) {
		t.Errorf("--json exit = %d error = %v, want invalid", jsonExitCode, failure)
	}
}

func TestClose_AlreadyClosed_KeepsClosedTimeAndReason(t *testing.T) {
	root := newWriteStore(t)
	before := readStoreFile(t, root, "shipped-fix.md")

	text, textStderr, textExitCode := runCLI(t, root, "close", "shipped-fix", "--reason", "again")
	kv, kvStderr, kvExitCode := runCLI(t, root, "close", "--kv", "shipped-fix", "--reason", "again")

	if textExitCode != ExitOK || textStderr != "" || text != "shipped-fix  already_closed\n" {
		t.Errorf("text: exit = %d stderr = %q stdout = %q", textExitCode, textStderr, text)
	}
	if kvExitCode != ExitOK || kvStderr != "" || kv != "ID=shipped-fix\nSTATUS=already_closed\n" {
		t.Errorf("--kv: exit = %d stderr = %q stdout = %q", kvExitCode, kvStderr, kv)
	}
	if after := readStoreFile(t, root, "shipped-fix.md"); after != before {
		t.Errorf("shipped-fix.md was rewritten:\n%s\nwant\n%s", after, before)
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

func TestClose_AliasOfInvalidFile_FailsNotFoundAndLeavesThePrefixMatchOpen(t *testing.T) {
	files := map[string]string{
		"auth-rework.md": readIssue("Auth rework", "bogus", "aliases: [auth]\n", ""),
		"auth-tests.md":  readIssue("Auth tests", "open", "", ""),
	}
	root := newIssueStore(t, nil)
	writeIssueFiles(t, root, files)

	_, stderr, exitCode := runCLI(t, root, "close", "auth", "--reason", "x")

	want := `id "auth" not found: .capcom/issues/auth-rework.md is invalid`
	if exitCode != ExitNotFound || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want not found naming auth-rework.md", exitCode, stderr)
	}
	assertIssueFilesUnchanged(t, root, files)
}

func TestClose_IDOfOversizeFile_FailsNotFoundAndLeavesThePrefixMatchOpen(t *testing.T) {
	oversize := readIssue("Foo", "open", "", "")
	files := map[string]string{
		"foo.md":     oversize + strings.Repeat("x", 1<<20+1-len(oversize)),
		"foo-bar.md": readIssue("Foo bar", "open", "", ""),
	}
	root := newIssueStore(t, nil)
	writeIssueFiles(t, root, files)

	_, stderr, exitCode := runCLI(t, root, "close", "foo", "--reason", "x")

	want := `id "foo" not found: .capcom/issues/foo.md is invalid`
	if exitCode != ExitNotFound || !strings.Contains(stderr, want) {
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

func TestClose_LastDepThroughAlias_EndsWithNowReady(t *testing.T) {
	root := newDepStore(t)

	stdout := mustRun(t, root, "close", "db-schema", "--reason", "migrated")

	if want := "db-schema  closed\nnow ready: build-pipeline\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestClose_AsDropped_AlsoReadiesTheDependent(t *testing.T) {
	root := newDepStore(t)

	stdout := mustRun(t, root, "close", "db-schema", "--status", "dropped", "--reason", "not needed")

	if got := lastLine(stdout); got != "now ready: build-pipeline" {
		t.Errorf("last line = %q, want the now ready line", got)
	}
}

func TestClose_NothingReadied_PrintsNoNowReadyLine(t *testing.T) {
	root := newDepStore(t)

	stdout := mustRun(t, root, "close", "login-flow", "--reason", "done")

	if want := "login-flow  closed\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// newShipStore extends newDepStore with ship-it, which needs both db-schema and login-flow.
func newShipStore(t *testing.T) string {
	t.Helper()
	root := newDepStore(t)
	writeIssueFiles(t, root, map[string]string{
		"ship-it.md": readIssue("Ship it", "open", "deps: [db-schema, login-flow]\n", ""),
	})
	return root
}

func TestClose_SeveralIDs_ReportsEachReadiedIDOnce(t *testing.T) {
	root := newShipStore(t)

	stdout := mustRun(t, root, "close", "db-schema", "login-flow", "--reason", "shipped")

	want := "db-schema  closed\nlogin-flow  closed\nnow ready: build-pipeline, ship-it\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestClose_JSON_CarriesNowReadyPerArgument(t *testing.T) {
	root := newShipStore(t)

	stdout, _, exitCode := runCLI(t, root, "close", "--json", "db-schema", "no-such-issue", "login-flow",
		"release-notes", "--reason", "shipped")

	data, _ := decodeEnvelope(t, stdout)["data"].([]any)
	want := map[string][]any{
		"db-schema":     {"build-pipeline"},
		"no-such-issue": {},
		"login-flow":    {"ship-it"},
		"release-notes": {},
	}
	if exitCode != ExitNotFound || len(data) != len(want) {
		t.Fatalf("exit = %d data = %v", exitCode, data)
	}
	for _, item := range data {
		record, _ := item.(map[string]any)
		id, _ := record["id"].(string)
		nowReady, isList := record["now_ready"].([]any)
		if !isList || !reflect.DeepEqual(nowReady, want[id]) {
			t.Errorf("%s now_ready = %#v, want %v", id, record["now_ready"], want[id])
		}
	}
}

func TestClose_HiddenDependent_IsNotReported(t *testing.T) {
	root := newIssueStore(t, nil)
	if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(readyConfig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writeIssueFiles(t, root, map[string]string{
		"db-schema.md":      readIssue("Schema", "open", "", ""),
		"build-pipeline.md": readIssue("Build pipeline", "open", "deps: [db-schema]\n", ""),
		"deploy-prod.md":    readIssue("Deploy", "open", "labels: [owner-run]\ndeps: [db-schema]\n", ""),
	})

	stdout := mustRun(t, root, "close", "db-schema", "--reason", "migrated")

	if got := lastLine(stdout); got != "now ready: build-pipeline" {
		t.Errorf("last line = %q, want deploy-prod left out", got)
	}
}
