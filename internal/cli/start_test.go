package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/index"
)

// startIssue is the Ready Issue the start tests claim.
const startIssue = "login-flow"

// newStartStore creates a store holding the Issue files, plus a Ready login-flow unless files names
// one of its own.
func newStartStore(t *testing.T, files map[string]string) string {
	t.Helper()
	root := newIssueStore(t, nil)
	all := map[string]string{startIssue + ".md": readyIssue("Login flow", "open", 2, "")}
	for name, content := range files {
		all[name] = content
	}
	writeIssueFiles(t, root, all)
	return root
}

// readOptionalFile returns the content of the file at path, or "" when it does not exist.
func readOptionalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

// issueFilePath is the file of the Issue at a slash path below .capcom/issues/.
func issueFilePath(root, name string) string {
	return filepath.Join(root, ".capcom", "issues", filepath.FromSlash(name))
}

// lockRecordPath is the lock record of root's main worktree.
func lockRecordPath(root string) string {
	return filepath.Join(root, ".git", "capcom", "locks", "_main.json")
}

// setBeforeStartClaim installs hook as beforeStartClaim for the rest of the test.
func setBeforeStartClaim(t *testing.T, hook func(entry index.Entry)) {
	t.Helper()
	previous := beforeStartClaim
	beforeStartClaim = hook
	t.Cleanup(func() { beforeStartClaim = previous })
}

func TestStart_ReadyIssue_ClaimsItHoldsTheLockAndPrintsTheToken(t *testing.T) {
	root := newStartStore(t, nil)

	stdout, stderr, exitCode := runCLI(t, root, "start", startIssue)

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q\n%s", exitCode, stderr, stdout)
	}
	if !strings.Contains(stdout, "status: active") || strings.Contains(stdout, "\nready:") {
		t.Errorf("stdout does not show the claimed Issue without a ready line:\n%s", stdout)
	}
	if !strings.Contains(stdout, "_main: acquired by ") || !strings.Contains(stdout, "\ntoken ") {
		t.Errorf("stdout lacks the lock and token lines:\n%s", stdout)
	}
	status := mustRun(t, root, "lock", "status", "--kv")
	if kvValue(t, status, "LOCK") != lockHeld || kvValue(t, status, "PURPOSE") != startIssue {
		t.Errorf("lock status = %q, want held for %s", status, startIssue)
	}
}

func TestStart_KV_AddsLockAndToken(t *testing.T) {
	root := newStartStore(t, nil)

	stdout := mustRun(t, root, "start", startIssue, "--kv")

	if kvValue(t, stdout, "ID") != startIssue || kvValue(t, stdout, "ISSUE_STATUS") != "active" {
		t.Errorf("stdout = %q, want the claimed Issue's record", stdout)
	}
	if kvValue(t, stdout, "LOCK") != lockAcquired || kvValue(t, stdout, "TOKEN") == "" {
		t.Errorf("stdout = %q, want LOCK=acquired and a TOKEN", stdout)
	}
	if !strings.HasPrefix(lastLine(stdout), "STATUS=") {
		t.Errorf("stdout = %q, want a closing STATUS= line", stdout)
	}
}

func TestStart_JSON_CarriesTheIssueAndTheLockToken(t *testing.T) {
	root := newStartStore(t, nil)

	envelope := decodeEnvelope(t, mustRun(t, root, "start", startIssue, "--json"))

	data := envelope["data"].(map[string]any)
	held := data["lock"].(map[string]any)
	if envelope["kind"] != kindStart || data["id"] != startIssue || data["status"] != "active" {
		t.Errorf("envelope = %v, want kind start with the claimed Issue", envelope)
	}
	if held["lock"] != lockAcquired || held["token"] == "" || held["token"] == nil {
		t.Errorf("lock = %v, want acquired with a token", held)
	}
}

func TestStart_RefusedIssue_WritesNothingAndNamesTheResolver(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		arg      string
		file     string
		isLocked bool
		wantExit int
		want     string
	}{
		{
			name:     "closed",
			files:    map[string]string{"shipped.md": readyIssue("Shipped", "done", 2, "")},
			arg:      "shipped",
			wantExit: ExitError,
			want:     "capcom reopen shipped --reason <text>",
		},
		{
			name:     "standing",
			files:    map[string]string{"area.md": readyIssue("Area", "open", 2, "standing: true\n")},
			arg:      "area",
			wantExit: ExitError,
			want:     "capcom list --tree area",
		},
		{
			name:     "blocked",
			files:    map[string]string{"gated.md": readyIssue("Gated", "open", 2, "deps: [login-flow]\n")},
			arg:      "gated",
			wantExit: ExitError,
			want:     "capcom dep list gated",
		},
		{
			name:     "open children",
			files:    map[string]string{"epic.md": readyIssue("Epic", "open", 2, ""), "epic/part.md": readyIssue("Part", "open", 2, "")},
			arg:      "epic",
			wantExit: ExitError,
			want:     "capcom list --tree epic",
		},
		{
			name:     "held by an ancestor",
			files:    map[string]string{"epic.md": readyIssue("Epic", "parked", 2, ""), "epic/part.md": readyIssue("Part", "open", 2, "")},
			arg:      "part",
			file:     "epic/part.md",
			wantExit: ExitError,
			want:     "capcom show epic",
		},
		{
			name:     "waiting category",
			files:    map[string]string{"later.md": readyIssue("Later", "parked", 2, "")},
			arg:      "later",
			wantExit: ExitError,
			want:     "capcom update later --status open",
		},
		{
			name:     "doing with the lock free",
			files:    map[string]string{"busy.md": readyIssue("Busy", "active", 2, "")},
			arg:      "busy",
			wantExit: ExitError,
			want:     "capcom lock acquire --purpose busy",
		},
		{
			name:     "doing with the lock held",
			files:    map[string]string{"busy.md": readyIssue("Busy", "active", 2, "")},
			arg:      "busy",
			isLocked: true,
			wantExit: ExitError,
			want:     "capcom start busy --token <token>",
		},
		{
			name:     "lock held by another",
			arg:      startIssue,
			isLocked: true,
			wantExit: ExitNotFound,
			want:     "capcom lock status",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newStartStore(t, test.files)
			if test.isLocked {
				acquireToken(t, root, "agent-b")
			}
			file := test.file
			if file == "" {
				file = test.arg + ".md"
			}
			entry := issueFilePath(root, file)
			before := readOptionalFile(t, entry)
			lockBefore := readOptionalFile(t, lockRecordPath(root))

			stdout, stderr, exitCode := runCLI(t, root, "start", test.arg)

			if exitCode != test.wantExit || !strings.Contains(stderr, test.want) {
				t.Errorf("exit = %d, want %d; stderr = %q, want it to name %q\n%s",
					exitCode, test.wantExit, stderr, test.want, stdout)
			}
			if after := readOptionalFile(t, entry); after != before {
				t.Errorf("issue file changed:\n%s\nwant\n%s", after, before)
			}
			if after := readOptionalFile(t, lockRecordPath(root)); after != lockBefore {
				t.Errorf("lock record changed:\n%s\nwant\n%s", after, lockBefore)
			}
		})
	}
}

func TestStart_AgainWithToken_ResumesTheLockAndLeavesTheIssue(t *testing.T) {
	root := newStartStore(t, nil)
	token := kvValue(t, mustRun(t, root, "start", startIssue, "--kv"), "TOKEN")
	before := readOptionalFile(t, issueFilePath(root, startIssue+".md"))

	stdout, stderr, exitCode := runCLI(t, root, "start", startIssue, "--token", token, "--kv")

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q\n%s", exitCode, stderr, stdout)
	}
	if kvValue(t, stdout, "LOCK") != lockResumed || kvValue(t, stdout, "TOKEN") != token {
		t.Errorf("stdout = %q, want LOCK=resumed with the same token", stdout)
	}
	if after := readOptionalFile(t, issueFilePath(root, startIssue+".md")); after != before {
		t.Errorf("issue file changed:\n%s\nwant\n%s", after, before)
	}
}

func TestStart_ClaimFailsAfterAcquire_ReleasesTheLock(t *testing.T) {
	root := newStartStore(t, nil)
	setBeforeStartClaim(t, func(index.Entry) {
		closed := readyIssue("Login flow", "done", 2, "")
		if err := os.WriteFile(issueFilePath(root, startIssue+".md"), []byte(closed), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	})

	stdout, stderr, exitCode := runCLI(t, root, "start", startIssue)

	if exitCode == ExitOK || !strings.Contains(stderr, "reopen") {
		t.Errorf("exit = %d, want the claim's refusal\nstdout = %q stderr = %q", exitCode, stdout, stderr)
	}
	if record := readOptionalFile(t, lockRecordPath(root)); record != "" {
		t.Errorf("lock record = %q, want the lock released", record)
	}
}
