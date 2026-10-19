package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// newNextStore creates a store whose Ready list starts with beta: alpha and beta share the top
// priority, and beta comes first because closing it makes gamma Ready. labelled is a lower-priority
// Ready Issue carrying the auth label, and done is closed.
func newNextStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"alpha.md":    readyIssue("Alpha", "open", 1, ""),
		"beta.md":     readyIssue("Beta", "open", 1, ""),
		"gamma.md":    readyIssue("Gamma", "open", 2, "deps: [beta]\n"),
		"labelled.md": readyIssue("Labelled", "open", 3, "labels: [auth]\n"),
		"done.md":     readyIssue("Done", "done", 0, ""),
	})
}

func TestNext_OneReadyIssue_PrintsItAsShowDoes(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"only.md": readyIssue("Only", "open", 2, ""),
		"done.md": readyIssue("Done", "done", 0, ""),
	})
	want, _, _ := runCLI(t, root, "show", "only")

	stdout, stderr, exitCode := runCLI(t, root, "next")

	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestNext_PriorityTie_PrintsReadysFirstLine(t *testing.T) {
	root := newNextStore(t)
	readyOut, _, _ := runCLI(t, root, "ready")
	firstID := strings.Fields(readyOut)[0]
	want, _, _ := runCLI(t, root, "show", firstID)

	stdout, _, exitCode := runCLI(t, root, "next")

	if exitCode != ExitOK || firstID != "beta" || stdout != want {
		t.Errorf("exit = %d, ready's first id = %q\nstdout = %q\nwant     %q", exitCode, firstID, stdout, want)
	}
}

func TestNext_Label_SelectsLowerPriorityIssue(t *testing.T) {
	root := newNextStore(t)
	want, _, _ := runCLI(t, root, "show", "labelled")

	stdout, _, exitCode := runCLI(t, root, "next", "--label", "auth")

	if exitCode != ExitOK || stdout != want {
		t.Errorf("exit = %d\nstdout = %q\nwant     %q", exitCode, stdout, want)
	}
}

func TestNext_JSON_GivesShowsData(t *testing.T) {
	root := newNextStore(t)
	showOut, _, _ := runCLI(t, root, "show", "beta", "--json")
	want := decodeEnvelope(t, showOut)

	stdout, _, exitCode := runCLI(t, root, "next", "--json")

	got := decodeEnvelope(t, stdout)
	if exitCode != ExitOK || got["kind"] != want["kind"] || !reflect.DeepEqual(got["data"], want["data"]) {
		t.Errorf("exit = %d\nnext --json = %s\nshow --json = %s", exitCode, stdout, showOut)
	}
}

func TestNext_NothingReady_PrintsSoAndExitsNothingReady(t *testing.T) {
	root := newIssueStore(t, map[string]string{"done.md": readyIssue("Done", "done", 0, "")})

	stdout, stderr, exitCode := runCLI(t, root, "next")

	want := "nothing is ready to work on\n"
	if exitCode != output.ExitNothingReady || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q stdout = %q, want exit %d stdout %q",
			exitCode, stderr, stdout, output.ExitNothingReady, want)
	}
}

func TestNext_NothingReadyJSON_GivesKindNextWithNullData(t *testing.T) {
	root := newIssueStore(t, map[string]string{"done.md": readyIssue("Done", "done", 0, "")})

	stdout, stderr, exitCode := runCLI(t, root, "next", "--json")

	envelope := decodeEnvelope(t, stdout)
	_, hasError := envelope["error"]
	isDataNull := strings.Contains(stdout, `"data":null`)
	if exitCode != output.ExitNothingReady || stderr != "" || envelope["kind"] != "next" || !isDataNull || hasError {
		t.Errorf("exit = %d stderr = %q\nstdout = %s", exitCode, stderr, stdout)
	}
}

func TestNext_NothingReadyKV_ExitsNothingReady(t *testing.T) {
	root := newIssueStore(t, map[string]string{"done.md": readyIssue("Done", "done", 0, "")})

	stdout, stderr, exitCode := runCLI(t, root, "next", "--kv")

	if exitCode != output.ExitNothingReady || stderr != "" || stdout != "" {
		t.Errorf("exit = %d stderr = %q stdout = %q", exitCode, stderr, stdout)
	}
}
