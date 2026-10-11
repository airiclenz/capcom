package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/edit"
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

// fileETag returns the etag of the issue file at a slash path below .capcom/issues/.
func fileETag(t *testing.T, root string, name string) string {
	t.Helper()
	return edit.ETag([]byte(readStoreFile(t, root, name)))
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

// newerFormatMessage is the error every command but prime gives on a store of format 2, whose
// format key sits on line 2 of the config written by writeNewerFormatConfig.
const newerFormatMessage = ".capcom/config.yaml:2: store format 2 is newer than this capcom reads (up to 1); upgrade capcom"

// writeNewerFormatConfig replaces the store's config.yaml with one declaring format 2.
func writeNewerFormatConfig(t *testing.T, root string) {
	t.Helper()
	content := "# written by a newer capcom\nformat: 2\n"
	if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestList_NewerStoreFormat_FailsNamingTheFormatLine(t *testing.T) {
	root := newReadStore(t)
	writeNewerFormatConfig(t, root)

	stdout, stderr, exitCode := runCLI(t, root, "list")

	if exitCode == ExitOK || stdout != "" || stderr != "capcom: "+newerFormatMessage+"\n" {
		t.Errorf("list = %d, stdout %q, stderr %q; want a failure carrying %q", exitCode, stdout, stderr, newerFormatMessage)
	}
}

func TestCreate_NewerStoreFormat_FailsWithoutWriting(t *testing.T) {
	root := newReadStore(t)
	writeNewerFormatConfig(t, root)

	_, _, exitCode := runCLI(t, root, "create", "--id", "new-issue", "--title", "New issue")

	_, statErr := os.Stat(filepath.Join(root, ".capcom", "issues", "new-issue.md"))
	if exitCode == ExitOK || statErr == nil {
		t.Errorf("create = %d, file stat err %v; want a failure and no new issue file", exitCode, statErr)
	}
}

func TestCheck_NewerStoreFormat_ReportsAConfigProblem(t *testing.T) {
	root := newReadStore(t)
	writeNewerFormatConfig(t, root)

	stdout, stderr, exitCode := runCLI(t, root, "check")
	jsonOut, _, jsonExitCode := runCLI(t, root, "check", "--json")

	if exitCode != ExitError || stdout != newerFormatMessage+"\n" || stderr != "" {
		t.Errorf("check = %d, stdout %q, stderr %q; want 1 and the too-new config problem", exitCode, stdout, stderr)
	}
	problems, _ := decodeEnvelope(t, jsonOut)["data"].([]any)
	if jsonExitCode != ExitError || len(problems) != 1 || problems[0].(map[string]any)["code"] != "config" {
		t.Errorf("check --json = %d, %s; want 1 and one config problem", jsonExitCode, jsonOut)
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
		"etag: " + fileETag(t, root, "login-flow/session-refresh.md") + "\n" +
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

	want := "ID=release-checklist\nETAG=" + fileETag(t, root, "release-checklist.md") + "\nISSUE_STATUS=open\nCATEGORY=todo\nTITLE=Release checklist\nSTATUS=ok\n" +
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

// elsewhereLine is the text line reporting session-refresh closed on the fix-login branch.
const elsewhereLine = "elsewhere: session-refresh: done on fix-login (unmerged)\n"

// newDivergedStore commits the read store, then closes session-refresh on a fix-login branch that
// the checked-out branch has not merged.
func newDivergedStore(t *testing.T) string {
	t.Helper()
	root := newReadStore(t)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "issues")
	runGit(t, root, "checkout", "-q", "-b", "fix-login")
	writeIssueFiles(t, root, map[string]string{
		"login-flow/session-refresh.md": readIssue(longTitle, "done", "labels: [auth]\n", "\nFixed.\n"),
	})
	runGit(t, root, "commit", "-q", "-a", "-m", "close session-refresh")
	runGit(t, root, "checkout", "-q", "-")
	return root
}

func TestReady_DivergedBranch_ReportsElsewhereAfterTheResult(t *testing.T) {
	root := newDivergedStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "ready")

	if exitCode != ExitOK || stderr != "" || !strings.HasPrefix(stdout, "release-checklist  open  P0") ||
		!strings.HasSuffix(stdout, elsewhereLine) {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant the ready list, then %q", exitCode, stderr, stdout, elsewhereLine)
	}
}

func TestList_DivergedBranchJSON_PutsElsewhereInTheEnvelope(t *testing.T) {
	root := newDivergedStore(t)

	stdout, _, exitCode := runCLI(t, root, "list", "--json")

	reported, _ := decodeEnvelope(t, stdout)["elsewhere"].([]any)
	want := map[string]any{
		"id":     "session-refresh",
		"branch": "fix-login",
		"change": "modified",
		"status": "done",
		"path":   ".capcom/issues/login-flow/session-refresh.md",
	}
	if exitCode != ExitOK || len(reported) != 1 || fmt.Sprint(reported[0]) != fmt.Sprint(want) {
		t.Errorf("exit = %d elsewhere = %v, want [%v]", exitCode, reported, want)
	}
}

func TestList_NoElsewhere_SkipsTheScan(t *testing.T) {
	root := newDivergedStore(t)

	stdout, _, exitCode := runCLI(t, root, "list", "--no-elsewhere")

	if exitCode != ExitOK || strings.Contains(stdout, "elsewhere") {
		t.Errorf("exit = %d stdout = %q, want no elsewhere line", exitCode, stdout)
	}
}

func TestShow_DivergedBranch_ReportsOnlyTheShownIssues(t *testing.T) {
	root := newDivergedStore(t)

	shown, _, shownExit := runCLI(t, root, "show", "session-refresh")
	other, _, otherExit := runCLI(t, root, "show", "release-checklist")

	if shownExit != ExitOK || !strings.HasSuffix(shown, elsewhereLine) {
		t.Errorf("show session-refresh exit = %d stdout = %q, want it to end with %q", shownExit, shown, elsewhereLine)
	}
	if otherExit != ExitOK || strings.Contains(other, "elsewhere") {
		t.Errorf("show release-checklist exit = %d stdout = %q, want no elsewhere line", otherExit, other)
	}
}

func TestShow_KVWithDivergedBranch_KeepsStatusLast(t *testing.T) {
	root := newDivergedStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--kv", "session-refresh")

	if exitCode != ExitOK || lastLine(stdout) != "STATUS=ok" || strings.Contains(stdout, "elsewhere") {
		t.Errorf("exit = %d stdout = %q, want no Elsewhere and STATUS=ok last", exitCode, stdout)
	}
}

func TestReady_UnbornHead_WarnsAndPrintsTheResult(t *testing.T) {
	root := newDivergedStore(t)
	runGit(t, root, "checkout", "-q", "--orphan", "fresh")

	stdout, _, exitCode := runCLI(t, root, "ready")

	if exitCode != ExitOK || !strings.HasPrefix(stdout, "release-checklist  open  P0") ||
		!strings.Contains(stdout, "warning: elsewhere: HEAD has no commit yet") {
		t.Errorf("exit = %d stdout = %q, want the ready list and a warning about HEAD", exitCode, stdout)
	}
}
