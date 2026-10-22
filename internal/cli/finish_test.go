package cli

import (
	"strings"
	"testing"
)

// finishFollowUp is the Issue that waits on startIssue, made Ready when finish closes it.
const finishFollowUp = "follow-up"

// newFinishStore creates a store holding a Ready login-flow and a follow-up that depends on it.
func newFinishStore(t *testing.T) string {
	t.Helper()
	return newStartStore(t, map[string]string{
		finishFollowUp + ".md": readyIssue("Follow up", "open", 2, "deps: [login-flow]\n"),
	})
}

// startToken starts startIssue in root and returns the Lock token start printed.
func startToken(t *testing.T, root string) string {
	t.Helper()
	return kvValue(t, mustRun(t, root, "start", startIssue, "--kv"), "TOKEN")
}

func TestFinish_StartedIssue_ClosesItFreesTheLockAndNamesTheNext(t *testing.T) {
	root := newFinishStore(t)
	token := startToken(t, root)

	stdout, stderr, exitCode := runCLI(t, root, "finish", startIssue, "--reason", "shipped", "--token", token)

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q\n%s", exitCode, stderr, stdout)
	}
	want := "login-flow  closed\nnow ready: follow-up\n_main: released\nnext: follow-up  Follow up\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if record := readOptionalFile(t, lockRecordPath(root)); record != "" {
		t.Errorf("lock record = %q, want the lock released", record)
	}
}

func TestFinish_KeepLock_LeavesTheLockHeld(t *testing.T) {
	root := newFinishStore(t)
	startToken(t, root)
	lockBefore := readOptionalFile(t, lockRecordPath(root))

	stdout, stderr, exitCode := runCLI(t, root, "finish", startIssue, "--reason", "shipped", "--keep-lock")

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q\n%s", exitCode, stderr, stdout)
	}
	if !strings.Contains(stdout, "_main: held by ") || !strings.Contains(stdout, "kept (--keep-lock)") {
		t.Errorf("stdout lacks the kept lock line:\n%s", stdout)
	}
	if after := readOptionalFile(t, lockRecordPath(root)); after != lockBefore {
		t.Errorf("lock record changed:\n%s\nwant\n%s", after, lockBefore)
	}
}

func TestFinish_TokenNotHoldingTheLock_LeavesItAndSaysSo(t *testing.T) {
	tests := []struct {
		name  string
		token []string
		want  string
	}{
		{name: "no token", want: "left alone: no --token given; run capcom lock status"},
		{name: "wrong token", token: []string{"--token", "not-the-token"}, want: "left alone: --token does not hold it; run capcom lock status"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newFinishStore(t)
			startToken(t, root)
			lockBefore := readOptionalFile(t, lockRecordPath(root))

			args := append([]string{"finish", startIssue, "--reason", "shipped"}, test.token...)
			stdout, stderr, exitCode := runCLI(t, root, args...)

			if exitCode != ExitOK || stderr != "" {
				t.Fatalf("exit = %d stderr = %q\n%s", exitCode, stderr, stdout)
			}
			if !strings.Contains(stdout, "login-flow  closed\n") || !strings.Contains(stdout, test.want) {
				t.Errorf("stdout = %q, want the close and %q", stdout, test.want)
			}
			if after := readOptionalFile(t, lockRecordPath(root)); after != lockBefore {
				t.Errorf("lock record changed:\n%s\nwant\n%s", after, lockBefore)
			}
		})
	}
}

func TestFinish_AlreadyClosed_ReportsItAndStillReleasesTheLock(t *testing.T) {
	root := newStartStore(t, map[string]string{"shipped.md": readyIssue("Shipped", "done", 2, "")})
	token := acquireToken(t, root, "agent-a")

	stdout := mustRun(t, root, "finish", "shipped", "--reason", "again", "--token", token)

	if !strings.HasPrefix(stdout, "shipped  already_closed\n") || !strings.Contains(stdout, "_main: released\n") {
		t.Errorf("stdout = %q, want already_closed and the lock released", stdout)
	}
	if record := readOptionalFile(t, lockRecordPath(root)); record != "" {
		t.Errorf("lock record = %q, want the lock released", record)
	}
}

func TestFinish_NothingReady_PrintsTheMessageAndExitsZero(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		root := newStartStore(t, nil)

		stdout, stderr, exitCode := runCLI(t, root, "finish", startIssue, "--reason", "shipped")

		if exitCode != ExitOK || stderr != "" {
			t.Fatalf("exit = %d stderr = %q\n%s", exitCode, stderr, stdout)
		}
		if lastLine(stdout) != nothingReadyMessage {
			t.Errorf("stdout = %q, want it to end with %q", stdout, nothingReadyMessage)
		}
	})
	t.Run("json", func(t *testing.T) {
		root := newStartStore(t, nil)

		envelope := decodeEnvelope(t, mustRun(t, root, "finish", startIssue, "--reason", "shipped", "--json"))

		data := envelope["data"].(map[string]any)
		if next, isPresent := data["next"]; !isPresent || next != nil {
			t.Errorf("data = %v, want next null", data)
		}
	})
}

func TestFinish_CloseFails_FailsAndLeavesTheLock(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "standing", arg: "area", want: "standing"},
		{name: "no such issue", arg: "no-such-issue", want: "no-such-issue"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newStartStore(t, map[string]string{"area.md": readyIssue("Area", "open", 2, "standing: true\n")})
			token := acquireToken(t, root, "agent-a")
			lockBefore := readOptionalFile(t, lockRecordPath(root))

			stdout, stderr, exitCode := runCLI(t, root, "finish", test.arg, "--reason", "shipped", "--token", token)

			if exitCode == ExitOK || !strings.Contains(stderr, test.want) {
				t.Errorf("exit = %d, want a failure naming %q\nstdout = %q stderr = %q", exitCode, test.want, stdout, stderr)
			}
			if after := readOptionalFile(t, lockRecordPath(root)); after != lockBefore {
				t.Errorf("lock record changed:\n%s\nwant\n%s", after, lockBefore)
			}
		})
	}
}

func TestFinish_CorruptLockRecord_ReportsItAndExitsZero(t *testing.T) {
	root := newFinishStore(t)
	writeLockRecord(t, root, "_main", "{")

	stdout, stderr, exitCode := runCLI(t, root, "finish", startIssue, "--reason", "shipped", "--token", "any")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d, want 0\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "login-flow  closed\n") || !strings.Contains(stdout, "_main: invalid; left alone") ||
		!strings.Contains(stdout, "capcom lock status, then capcom lock break") {
		t.Errorf("stdout = %q, want the close and the invalid lock line", stdout)
	}
	if !strings.Contains(stdout, "_main.json") {
		t.Errorf("stdout = %q, want a warning naming the lock record", stdout)
	}
	if record := readOptionalFile(t, lockRecordPath(root)); record != "{" {
		t.Errorf("lock record = %q, want it left in place", record)
	}
}

func TestFinish_KV_GivesOneRecord(t *testing.T) {
	root := newFinishStore(t)
	token := startToken(t, root)

	stdout := mustRun(t, root, "finish", startIssue, "--reason", "shipped", "--token", token, "--kv")

	want := "ID=login-flow\nLOCK=released\nNEXT=follow-up\nSTATUS=closed\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestFinish_JSON_CarriesTheCloseTheLockAndTheNext(t *testing.T) {
	root := newFinishStore(t)
	token := startToken(t, root)

	envelope := decodeEnvelope(t, mustRun(t, root, "finish", startIssue, "--reason", "shipped", "--token", token, "--json"))

	data := envelope["data"].(map[string]any)
	closed := data["close"].(map[string]any)
	held := data["lock"].(map[string]any)
	next := data["next"].(map[string]any)
	if envelope["kind"] != kindFinish || closed["id"] != startIssue || closed["outcome"] != outcomeClosed {
		t.Errorf("envelope = %v, want kind finish with the close", envelope)
	}
	if nowReady := closed["now_ready"].([]any); len(nowReady) != 1 || nowReady[0] != finishFollowUp {
		t.Errorf("now_ready = %v, want [%s]", closed["now_ready"], finishFollowUp)
	}
	if _, hasToken := held["token"]; held["lock"] != lockReleased || hasToken {
		t.Errorf("lock = %v, want released without a token", held)
	}
	if next["id"] != finishFollowUp || next["title"] != "Follow up" {
		t.Errorf("next = %v, want %s", next, finishFollowUp)
	}
}
