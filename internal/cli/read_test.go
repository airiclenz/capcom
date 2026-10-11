package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// readIssue is the content of a valid issue file with the given extra frontmatter lines and body.
func readIssue(title, status, extra, body string) string {
	closing := ""
	if status == "done" || status == "dropped" {
		closing = "closed: 2026-10-12T09:00:00Z\nreason: finished\n"
	}
	return fmt.Sprintf(
		"---\ntitle: %s\nstatus: %s\ntype: task\npriority: 2\n%screated: 2026-10-11T09:00:00Z\n%s---\n%s",
		title, status, extra, closing, body,
	)
}

// longTitle is longer than any terminal, to prove output is never wrapped.
var longTitle = "Session refresh " + strings.Repeat("races the login timeout ", 8) + "end"

// newReadStore creates a store holding a small issue tree for the read commands.
func newReadStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"login-flow.md": readIssue("Login flow", "open",
			"labels: [auth, epic]\nplan: 2026-10-11 - 00 - login-plan\n", "\nThe login epic.\n"),
		"release-checklist.md": "---\ntitle: Release checklist\nstatus: open\ntype: chore\npriority: 0\n" +
			"created: 2026-10-11T09:00:00Z\n---\n",
		"old-idea.md": readIssue("Old idea", "dropped", "aliases: [apogee-kk0.4]\n", "\nShelved.\n"),
	})
	nested := map[string]string{
		"login-flow/session-refresh.md": readIssue(longTitle, "parked", "labels: [auth]\n",
			"\nBlocked by login-timeout-race; the refresh doesn't wait for config.yaml.\n\n## Log\n\n- 2026-10-11 airic: seen\n"),
	}
	writeIssueFiles(t, root, nested)
	return root
}

// writeIssueFiles writes issue files, named by slash paths below .capcom/issues/, creating their
// directories.
func writeIssueFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, ".capcom", "issues", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
}

// listIDs runs capcom list --json with args and returns the listed ids.
func listIDs(t *testing.T, root string, args ...string) []string {
	t.Helper()
	stdout, _, exitCode := runCLI(t, root, append([]string{"list", "--json"}, args...)...)
	if exitCode != ExitOK {
		t.Fatalf("list %v exit = %d\n%s", args, exitCode, stdout)
	}
	return envelopeIDs(t, stdout)
}

// envelopeIDs returns the ids of the issues in an envelope's data list.
func envelopeIDs(t *testing.T, stdout string) []string {
	t.Helper()
	issues, _ := decodeEnvelope(t, stdout)["data"].([]any)
	ids := make([]string, 0, len(issues))
	for _, item := range issues {
		ids = append(ids, item.(map[string]any)["id"].(string))
	}
	return ids
}

func TestList_NoArgs_ListsOpenIssuesIndentedByDepth(t *testing.T) {
	root := newReadStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "list")

	want := "login-flow  open  P2  Login flow\n" +
		"  session-refresh  parked  P2  " + longTitle + "\n" +
		"release-checklist  open  P0  Release checklist\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestList_Filters_SelectMatchingIssues(t *testing.T) {
	root := newReadStore(t)
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"--all"}, []string{"login-flow", "session-refresh", "old-idea", "release-checklist"}},
		{[]string{"--status", "dropped"}, []string{"old-idea"}},
		{[]string{"--category", "todo,waiting"}, []string{"login-flow", "session-refresh", "release-checklist"}},
		{[]string{"--label", "auth", "--label", "epic"}, []string{"login-flow"}},
		{[]string{"--no-label", "auth"}, []string{"release-checklist"}},
		{[]string{"--type", "chore"}, []string{"release-checklist"}},
		{[]string{"--priority", "0"}, []string{"release-checklist"}},
		{[]string{"--parent", "login"}, []string{"session-refresh"}},
		{[]string{"--plan", "login-plan"}, []string{"login-flow"}},
		{[]string{"--planned"}, []string{"login-flow"}},
		{[]string{"--unplanned"}, []string{"session-refresh", "release-checklist"}},
		{[]string{"--limit", "1"}, []string{"login-flow"}},
		{[]string{"--label", "absent"}, []string{}},
	}

	for _, current := range cases {
		got := listIDs(t, root, current.args...)

		if !slices.Equal(got, current.want) {
			t.Errorf("list %v = %v, want %v", current.args, got, current.want)
		}
	}
}

func TestList_JSON_CarriesIssueFields(t *testing.T) {
	root := newReadStore(t)

	stdout, _, _ := runCLI(t, root, "list", "--json", "--planned")

	envelope := decodeEnvelope(t, stdout)
	issue := envelope["data"].([]any)[0].(map[string]any)
	if envelope["kind"] != "issue-list" || issue["category"] != "todo" || issue["plan"] != "2026-10-11 - 00 - login-plan" ||
		fmt.Sprint(issue["labels"]) != "[auth epic]" || issue["body"] != nil {
		t.Errorf("envelope = %v, want an issue-list with login-flow's fields and no body", envelope)
	}
}

func TestList_BadFlags_ExitUsage(t *testing.T) {
	root := newReadStore(t)
	invocations := [][]string{
		{"--status", "closed"},
		{"--category", "finished"},
		{"--type", "epic"},
		{"--priority", "5"},
		{"--limit", "-1"},
		{"--planned", "--unplanned"},
		{"--kv"},
	}

	for _, args := range invocations {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"list"}, args...)...)

		// --kv reports its failure on stdout; text mode on stderr.
		if exitCode != ExitUsage || stdout+stderr == "" {
			t.Errorf("list %v exit = %d output = %q, want a usage failure", args, exitCode, stdout+stderr)
		}
	}
}

func TestList_UnknownParent_IsNotFound(t *testing.T) {
	root := newReadStore(t)

	_, stderr, exitCode := runCLI(t, root, "list", "--parent", "absent")

	if exitCode != ExitNotFound || !strings.Contains(stderr, "not found") {
		t.Errorf("exit = %d stderr = %q, want not_found", exitCode, stderr)
	}
}

func TestShow_Text_PrintsFrontmatterThenBodyVerbatim(t *testing.T) {
	root := newReadStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "show", "session-refresh")

	want := "id: session-refresh\n" +
		"path: .capcom/issues/login-flow/session-refresh.md\n" +
		"parent: login-flow\n" +
		"title: " + longTitle + "\n" +
		"status: parked\ncategory: waiting\ntype: task\npriority: 2\nlabels: [auth]\n" +
		"created: 2026-10-11T09:00:00Z\n" +
		"\nBlocked by login-timeout-race; the refresh doesn't wait for config.yaml.\n\n## Log\n\n- 2026-10-11 airic: seen\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestShow_AliasAndPrefix_ResolveTheIssue(t *testing.T) {
	root := newReadStore(t)

	for arg, want := range map[string]string{"apogee-kk0.4": "old-idea", "rel": "release-checklist"} {
		stdout, _, exitCode := runCLI(t, root, "show", "--json", arg)

		if got := envelopeIDs(t, stdout); exitCode != ExitOK || !slices.Equal(got, []string{want}) {
			t.Errorf("show %s exit = %d ids = %v, want %s", arg, exitCode, got, want)
		}
	}
}

func TestShow_OneOfSeveralMissing_PrintsFoundAndExitsNotFound(t *testing.T) {
	root := newReadStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "show", "login-flow", "absent", "old-idea")

	if exitCode != ExitNotFound || !strings.HasPrefix(stdout, "id: login-flow\n") ||
		!strings.Contains(stdout, "\nid: old-idea\n") || stderr != "capcom: id \"absent\" not found\n" {
		t.Errorf("exit = %d stderr = %q\nstdout = %q", exitCode, stderr, stdout)
	}
}

func TestShow_OneOfSeveralMissingJSON_CarriesDataAndError(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--json", "login-flow", "absent")

	envelope := decodeEnvelope(t, stdout)
	errorBody, _ := envelope["error"].(map[string]any)
	issue := envelope["data"].([]any)[0].(map[string]any)
	if exitCode != ExitNotFound || envelope["kind"] != "issue" || errorBody["code"] != string(CodeNotFound) ||
		issue["body"] != "\nThe login epic.\n" {
		t.Errorf("exit = %d envelope = %v, want login-flow with its body and a not_found error", exitCode, envelope)
	}
}

func TestShow_KV_ClosesEachIDWithItsStatus(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--kv", "rel", "absent")

	want := "ID=release-checklist\nISSUE_STATUS=open\nCATEGORY=todo\nTITLE=Release checklist\nSTATUS=ok\n" +
		"ID=absent\nMESSAGE=id \"absent\" not found\nSTATUS=not_found\n"
	if exitCode != ExitNotFound || stdout != want {
		t.Errorf("exit = %d\nstdout = %q\nwant     %q", exitCode, stdout, want)
	}
}

func TestShow_AmbiguousPrefix_NamesCandidates(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{"login-bug.md": readIssue("Login bug", "open", "", "")})

	_, stderr, exitCode := runCLI(t, root, "show", "login")

	if exitCode != ExitError || !strings.Contains(stderr, "login-bug, login-flow") {
		t.Errorf("exit = %d stderr = %q, want an ambiguity naming both ids", exitCode, stderr)
	}
}

func TestSearch_Words_RankTheBestMatchFirst(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"refresh-twice.md": readIssue("Refresh refresh", "open", "", "\nrefresh refresh refresh\n"),
	})

	stdout, stderr, exitCode := runCLI(t, root, "search", "refresh")

	want := "refresh-twice  open  P2  Refresh refresh\nsession-refresh  parked  P2  " + longTitle + "\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestSearch_Punctuation_FindsTheIssue(t *testing.T) {
	root := newReadStore(t)

	for _, words := range []string{"login-timeout-race", "doesn't", "config.yaml"} {
		stdout, _, exitCode := runCLI(t, root, "search", "--json", words)

		envelope := decodeEnvelope(t, stdout)
		if got := envelopeIDs(t, stdout); exitCode != ExitOK || envelope["kind"] != "search" ||
			!slices.Equal(got, []string{"session-refresh"}) {
			t.Errorf("search %q exit = %d envelope = %v, want session-refresh", words, exitCode, envelope)
		}
	}
}

func TestSearch_NoMatch_IsNotFound(t *testing.T) {
	root := newReadStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "search", "absent")

	if exitCode != ExitNotFound || stdout != "" || stderr != "capcom: no issue matches \"absent\"\n" {
		t.Errorf("exit = %d stdout = %q stderr = %q, want not_found", exitCode, stdout, stderr)
	}
}
