package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/nodefile"
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
	return nodefile.ETag([]byte(readStoreFile(t, root, name)))
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

// newerFormatMessage is the error every command but prime gives on a store of format 5, whose
// format key sits on line 2 of the config written by writeNewerFormatConfig.
const newerFormatMessage = ".capcom/config.yaml:2: store format 5 is newer than this capcom reads (up to 4); upgrade capcom"

// writeNewerFormatConfig replaces the store's config.yaml with one declaring format 5.
func writeNewerFormatConfig(t *testing.T, root string) {
	t.Helper()
	content := "# written by a newer capcom\nformat: 5\n"
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

func TestList_OversizedConfig_FailsNamingIt(t *testing.T) {
	root := newReadStore(t)
	const line = "# a comment line padding config.yaml past the guarded reader's limit\n"
	const size = 1<<20 + 1
	content := strings.Repeat(line, size/len(line)+1)[:size]
	if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "list")

	if want := "capcom: .capcom/config.yaml: file is larger than 1 MiB\n"; exitCode != ExitError || stdout != "" || stderr != want {
		t.Errorf("list = %d, stdout %q, stderr %q; want %d and stderr %q", exitCode, stdout, stderr, ExitError, want)
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

// readStoreTree is the --tree outline of newReadStore without --all: the dropped old-idea is left
// out, login-flow carries its one open child.
var readStoreTree = "login-flow  open  P2  Login flow  (0/1 closed)\n" +
	"  session-refresh  parked  P2  " + longTitle + "\n" +
	"release-checklist  open  P0  Release checklist\n"

func TestListTree_Flags_PrintTheOutline(t *testing.T) {
	root := newReadStore(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{}, readStoreTree},
		{[]string{"--depth", "0"}, readStoreTree},
		{[]string{"--depth", "2"}, readStoreTree},
		{[]string{"--depth", "1"}, "login-flow  open  P2  Login flow  (0/1 closed)\n" +
			"  ... 1 more (--depth 2)\n" +
			"release-checklist  open  P0  Release checklist\n"},
		{[]string{"--all"}, "login-flow  open  P2  Login flow  (0/1 closed)\n" +
			"  session-refresh  parked  P2  " + longTitle + "\n" +
			"old-idea  dropped  P2  Old idea\n" +
			"release-checklist  open  P0  Release checklist\n"},
		{[]string{"--parent", "login"}, "login-flow  open  P2  Login flow  (0/1 closed)\n" +
			"  session-refresh  parked  P2  " + longTitle + "\n"},
		{[]string{"--parent", "login-flow", "--depth", "1"}, "login-flow  open  P2  Login flow  (0/1 closed)\n" +
			"  session-refresh  parked  P2  " + longTitle + "\n"},
		{[]string{"--parent", "session-refresh"}, "session-refresh  parked  P2  " + longTitle + "\n"},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"list", "--tree"}, current.args...)...)

		if exitCode != ExitOK || stderr != "" || stdout != current.want {
			t.Errorf("list --tree %v exit = %d stderr = %q\nstdout = %q\nwant     %q",
				current.args, exitCode, stderr, stdout, current.want)
		}
	}
}

func TestListTree_ClosedChild_CountedButLeftOut(t *testing.T) {
	root := newReadStore(t)
	mustRun(t, root, "close", "session-refresh", "--reason", "done")

	stdout := mustRun(t, root, "list", "--tree")
	all := mustRun(t, root, "list", "--tree", "--all", "--parent", "login-flow")

	want := "login-flow  open  P2  Login flow  (1/1 closed)\n" +
		"release-checklist  open  P0  Release checklist\n"
	if stdout != want {
		t.Errorf("list --tree = %q, want %q", stdout, want)
	}
	if !strings.HasPrefix(all, "login-flow  open  P2  Login flow  (1/1 closed)\n  session-refresh  done  P2  ") {
		t.Errorf("list --tree --all --parent login-flow = %q, want the closed child shown under its parent", all)
	}
}

func TestListTree_ClosedParent_LeavesOutItsSubtree(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"old-idea/old-step.md": readIssue("Old step", "open", "", "\nA step of the old idea.\n"),
	})

	stdout := mustRun(t, root, "list", "--tree")
	all := mustRun(t, root, "list", "--tree", "--all")

	if stdout != readStoreTree {
		t.Errorf("list --tree = %q, want %q", stdout, readStoreTree)
	}
	if !strings.Contains(all, "\nold-idea  dropped  P2  Old idea  (0/1 closed)\n  old-step  open  P2  Old step\n") {
		t.Errorf("list --tree --all = %q, want old-step under old-idea", all)
	}
}

func TestListTree_ChildOfInvalidParent_PrintsAsRoot(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{"login-flow.md": "no frontmatter here\n"})

	stdout, _, exitCode := runCLI(t, root, "list", "--tree")

	// The unusable file's warning follows the outline.
	want := "session-refresh  parked  P2  " + longTitle + "\n" +
		"release-checklist  open  P0  Release checklist\nwarning: "
	if exitCode != ExitOK || !strings.HasPrefix(stdout, want) {
		t.Errorf("list --tree exit = %d\nstdout = %q\nwant     %q", exitCode, stdout, want)
	}
}

func TestListTree_JSON_ItemsCarryDepth(t *testing.T) {
	root := newReadStore(t)

	tree := mustRun(t, root, "list", "--tree", "--json")
	flat := mustRun(t, root, "list", "--json")

	envelope := decodeEnvelope(t, tree)
	var depths []string
	for _, item := range envelope["data"].([]any) {
		issue := item.(map[string]any)
		depths = append(depths, fmt.Sprintf("%s:%v", issue["id"], issue["depth"]))
	}
	want := []string{"login-flow:0", "session-refresh:1", "release-checklist:0"}
	if envelope["kind"] != "issue-list" || !slices.Equal(depths, want) {
		t.Errorf("list --tree --json = kind %v, depths %v; want issue-list, %v", envelope["kind"], depths, want)
	}
	for _, item := range decodeEnvelope(t, flat)["data"].([]any) {
		if _, hasDepth := item.(map[string]any)["depth"]; hasDepth {
			t.Errorf("list --json item = %v, want no depth", item)
		}
	}
}

func TestListTree_BadFlags_UsageErrorNamesTheFlag(t *testing.T) {
	root := newReadStore(t)
	cases := []struct {
		args []string
		flag string
	}{
		{[]string{"--tree", "--label", "x"}, "--label"},
		{[]string{"--tree", "--status", "open"}, "--status"},
		{[]string{"--tree", "--category", "todo"}, "--category"},
		{[]string{"--tree", "--no-label", "auth"}, "--no-label"},
		{[]string{"--tree", "--type", "task"}, "--type"},
		{[]string{"--tree", "--priority", "2"}, "--priority"},
		{[]string{"--tree", "--plan", "login"}, "--plan"},
		{[]string{"--tree", "--planned"}, "--planned"},
		{[]string{"--tree", "--unplanned"}, "--unplanned"},
		{[]string{"--tree", "--limit", "1"}, "--limit"},
		{[]string{"--tree", "--depth", "-1"}, "--depth"},
		{[]string{"--depth", "1"}, "--depth"},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"list"}, current.args...)...)

		if exitCode != ExitUsage || stdout != "" || !strings.Contains(stderr, current.flag) {
			t.Errorf("list %v exit = %d stdout = %q stderr = %q, want a usage failure naming %s",
				current.args, exitCode, stdout, stderr, current.flag)
		}
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

// rewriteKeepingTimes writes content over the file at path and sets its access and modification
// times to at, so a size-and-mtime check cannot tell the new content from the old.
func rewriteKeepingTimes(t *testing.T, path string, content string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
}

func TestShow_FileRewrittenAtSameSizeAndMtime_PrintsTheBytesItsEtagNames(t *testing.T) {
	root := newReadStore(t)
	path := filepath.Join(root, ".capcom", "issues", "stale-view.md")
	past := time.Now().Add(-time.Hour)
	rewriteKeepingTimes(t, path, readIssue("Old title", "open", "", "\nOld body.\n"), past)
	mustRun(t, root, "show", "stale-view")
	newContent := readIssue("New title", "open", "", "\nNew body.\n")
	rewriteKeepingTimes(t, path, newContent, past)
	want := nodefile.ETag([]byte(newContent))

	textOutput := mustRun(t, root, "show", "stale-view")
	jsonData, _ := decodeEnvelope(t, mustRun(t, root, "show", "stale-view", "--json"))["data"].([]any)

	if !strings.Contains(textOutput, "title: New title\n") || !strings.HasSuffix(textOutput, "\nNew body.\n") ||
		textETag(textOutput) != want {
		t.Errorf("show text = %q; want title New title, body New body. and etag %q", textOutput, want)
	}
	shown, _ := jsonData[0].(map[string]any)
	if shown["title"] != "New title" || shown["body"] != "\nNew body.\n" || shown["etag"] != want {
		t.Errorf("show --json data = %v; want title New title, body New body. and etag %q", shown, want)
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

func TestShow_CitationSpelling_ResolvesTheIssue(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"login-timeout-race.md": readIssue("Login timeout race", "open", "", "\nThe race.\n"),
	})

	stdout, stderr, exitCode := runCLI(t, root, "show", "--json", "Login Timeout Race")

	if got := envelopeIDs(t, stdout); exitCode != ExitOK || !slices.Equal(got, []string{"login-timeout-race"}) {
		t.Errorf("show \"Login Timeout Race\" exit = %d ids = %v\n%s", exitCode, got, stderr)
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

func TestShow_KV_BesideUnparseableFile_PrintsWarningBeforeRecord(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{"broken-one.md": brokenFrontmatter})

	stdout, stderr, exitCode := runCLI(t, root, "show", "--kv", "release-checklist")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	assertKVWarningFirst(t, stdout, "broken-one.md", "release-checklist")
}

// brokenFrontmatter is the content of an issue file whose frontmatter is not valid YAML.
const brokenFrontmatter = "---\ntitle: [unclosed\nstatus: open\ntype: task\n---\n"

// assertKVWarningFirst checks that --kv output opens with one WARNING= line naming the issue file
// at the slash path below .capcom/issues/, follows it with the record for id and ends in STATUS=ok.
func assertKVWarningFirst(t *testing.T, stdout, name, id string) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	isWarningFirst := len(lines) > 2 && strings.HasPrefix(lines[0], "WARNING=.capcom/issues/"+name+":")
	if !isWarningFirst || lines[1] != "ID="+id || lastLine(stdout) != "STATUS=ok" {
		t.Errorf("stdout = %q\nwant a WARNING= line naming %s, then ID=%s, last line STATUS=ok", stdout, name, id)
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

// newShowDepStore creates a store where api-client depends on db-schema, and the done
// write-docs depends on api-client.
func newShowDepStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"db-schema.md":  readIssue("Schema", "open", "", ""),
		"api-client.md": readIssue("API client", "open", "deps: [db-schema]\n", ""),
		"write-docs.md": readIssue("Write docs", "done", "deps: [api-client]\n", ""),
	})
}

func TestShow_Text_PrintsReadinessAndDependents(t *testing.T) {
	root := newShowDepStore(t)

	tests := []struct {
		id      string
		want    []string
		notWant []string
	}{
		{
			id:   "api-client",
			want: []string{"\ncategory: todo\nready: no (blocked by db-schema (todo))\ntype: task\n", "\ndeps: [db-schema]\ndependents: [write-docs]\n"},
		},
		{
			id:   "db-schema",
			want: []string{"\ncategory: todo\nready: yes\ntype: task\n", "\npriority: 2\ndependents: [api-client]\ncreated: "},
		},
		{
			id:      "write-docs",
			want:    []string{"\ncategory: done\ntype: task\n", "\ndeps: [api-client]\ncreated: "},
			notWant: []string{"ready:", "dependents:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			text := mustRun(t, root, "show", tt.id)

			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Errorf("show %s = %q, want it to contain %q", tt.id, text, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(text, notWant) {
					t.Errorf("show %s = %q, want no %q", tt.id, text, notWant)
				}
			}
		})
	}
}

func TestShow_Text_OpenChildren_WordedAsReadyAll(t *testing.T) {
	root := newReadStore(t)

	text := mustRun(t, root, "show", "login-flow")
	readyAll := mustRun(t, root, "ready", "--all")

	if !strings.Contains(text, "\ncategory: todo\nready: no (open children: session-refresh)\n") ||
		!strings.Contains(readyAll, "-- not ready: open children: session-refresh\n") {
		t.Errorf("show = %q\nready --all = %q", text, readyAll)
	}
}

func TestShow_JSON_CarriesReadyBlockingAndDependents(t *testing.T) {
	root := newShowDepStore(t)

	stdout := mustRun(t, root, "show", "--json", "api-client", "db-schema", "write-docs")

	shown := map[string]map[string]any{}
	for _, item := range decodeEnvelope(t, stdout)["data"].([]any) {
		issue := item.(map[string]any)
		shown[issue["id"].(string)] = issue
	}
	api, schema, docs := shown["api-client"], shown["db-schema"], shown["write-docs"]
	blocking := fmt.Sprint(api["blocking"])
	if api["ready"] != false || blocking != "[map[dep:db-schema reason:todo]]" ||
		fmt.Sprint(api["dependents"]) != "[write-docs]" {
		t.Errorf("api-client = %v, want ready false, blocked by db-schema, write-docs dependent", api)
	}
	if _, hasBlocking := schema["blocking"]; schema["ready"] != true || hasBlocking ||
		fmt.Sprint(schema["dependents"]) != "[api-client]" {
		t.Errorf("db-schema = %v, want ready true, no blocking, api-client dependent", schema)
	}
	if _, hasReady := docs["ready"]; hasReady || docs["dependents"] == nil || len(docs["dependents"].([]any)) != 0 {
		t.Errorf("write-docs = %v, want no ready and an empty dependents list", docs)
	}
}

func TestShow_Text_ChildProgress(t *testing.T) {
	root := newReadStore(t)

	before := mustRun(t, root, "show", "login-flow")
	leaf := mustRun(t, root, "show", "session-refresh")
	mustRun(t, root, "close", "session-refresh", "--reason", "done")
	after := mustRun(t, root, "show", "login-flow")

	if !strings.Contains(before, "\nlabels: [auth, epic]\nchildren: 0/1 closed\nplan: ") {
		t.Errorf("show login-flow with an open child = %q, want children: 0/1 closed after labels", before)
	}
	if !strings.Contains(after, "\nchildren: 1/1 closed\n") {
		t.Errorf("show login-flow with a closed child = %q, want children: 1/1 closed", after)
	}
	if strings.Contains(leaf, "children:") {
		t.Errorf("show session-refresh = %q, want no children line on a leaf", leaf)
	}
}

func TestShow_JSON_ChildrenCountedOnlyWhenPresent(t *testing.T) {
	root := newReadStore(t)
	mustRun(t, root, "close", "session-refresh", "--reason", "done")

	stdout := mustRun(t, root, "show", "--json", "login-flow", "session-refresh")

	shown := map[string]map[string]any{}
	for _, item := range decodeEnvelope(t, stdout)["data"].([]any) {
		issue := item.(map[string]any)
		shown[issue["id"].(string)] = issue
	}
	if children := fmt.Sprint(shown["login-flow"]["children"]); children != "map[closed:1 total:1]" {
		t.Errorf("login-flow children = %s, want map[closed:1 total:1]", children)
	}
	if _, hasChildren := shown["session-refresh"]["children"]; hasChildren {
		t.Errorf("session-refresh = %v, want no children key on a leaf", shown["session-refresh"])
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

func TestList_KV_FailsAsUnknownFlag(t *testing.T) {
	root := newReadStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "list", "--kv")

	// scanFormat still routes the failure to stdout as KV lines.
	if exitCode != ExitUsage || stderr != "" {
		t.Fatalf("exit = %d stderr = %q, want %d and empty", exitCode, stderr, ExitUsage)
	}
	if !strings.Contains(stdout, "MESSAGE=unknown flag: --kv\n") || lastLine(stdout) != "STATUS=invalid" {
		t.Errorf("stdout = %q, want MESSAGE=unknown flag: --kv and a closing STATUS=invalid", stdout)
	}
}

func TestShow_KV_EndsWithStatus(t *testing.T) {
	root := newReadStore(t)

	stdout := mustRun(t, root, "show", "login-flow", "--kv")

	if !strings.HasPrefix(lastLine(stdout), "STATUS=") {
		t.Errorf("stdout = %q, want a closing STATUS= line", stdout)
	}
}

func TestHelp_CommandsWithoutKV_OmitKV(t *testing.T) {
	invocations := [][]string{
		{"--help"},
		{"list", "--help"},
		{"know", "mv", "--help"},
		{"know", "rm", "--help"},
		{"know", "add", "--help"},
	}

	for _, args := range invocations {
		stdout := mustRun(t, "", args...)

		if strings.Contains(stdout, "--kv") {
			t.Errorf("%v output mentions --kv:\n%s", args, stdout)
		}
	}
}

func TestKV_SupportingCommands_ListKVAndEndWithStatus(t *testing.T) {
	cases := map[string]struct {
		setup [][]string
		args  []string
	}{
		"show":    {args: []string{"show", "login-flow"}},
		"close":   {args: []string{"close", "login-flow", "--reason", "shipped"}},
		"reopen":  {args: []string{"reopen", "old-idea", "--reason", "wanted again"}},
		"version": {args: []string{"version"}},
		"create":  {args: []string{"create", "--id", "new-issue", "--title", "New issue"}},
		"update":  {args: []string{"update", "login-flow", "--title", "Login"}},
		"log":     {args: []string{"log", "login-flow", "seen again"}},
		"mv":      {args: []string{"mv", "release-checklist", "release-list"}},
		"rm":      {args: []string{"rm", "release-checklist"}},
		"dep add": {args: []string{"dep", "add", "login-flow", "release-checklist"}},
		"dep rm": {
			setup: [][]string{{"dep", "add", "login-flow", "release-checklist"}},
			args:  []string{"dep", "rm", "login-flow", "release-checklist"},
		},
		"related add": {args: []string{"related", "add", "login-flow", "release-checklist"}},
		"related rm": {
			setup: [][]string{{"related", "add", "login-flow", "release-checklist"}},
			args:  []string{"related", "rm", "login-flow", "release-checklist"},
		},
		"lock acquire": {args: []string{"lock", "acquire", "--purpose", "kv"}},
		"lock check":   {args: []string{"lock", "check"}},
		"lock status":  {args: []string{"lock", "status"}},
		"lock break":   {args: []string{"lock", "break"}},
		"lock release": {args: []string{"lock", "release", "--token", "any"}},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			root := newReadStore(t)
			for _, step := range testCase.setup {
				mustRun(t, root, step...)
			}

			help := mustRun(t, root, append(slices.Clone(testCase.args), "--help")...)
			stdout := mustRun(t, root, append(slices.Clone(testCase.args), "--kv")...)

			if !strings.Contains(help, "--kv") {
				t.Errorf("help does not list --kv:\n%s", help)
			}
			if !strings.HasPrefix(lastLine(stdout), "STATUS=") {
				t.Errorf("stdout = %q, want KV records closed by a STATUS= line", stdout)
			}
		})
	}
}

func TestShow_NodeBelowDuplicatedID_FailsNamingBothFiles(t *testing.T) {
	root := newIssueStore(t, nil)
	writeIssueFiles(t, root, map[string]string{
		"a.md":      readIssue("A", "open", "", ""),
		"a/x.md":    readIssue("X one", "open", "", ""),
		"b.md":      readIssue("B", "open", "", ""),
		"b/x.md":    readIssue("X two", "open", "", ""),
		"b/x/c2.md": readIssue("C2", "open", "", ""),
	})

	stdout, stderr, exitCode := runCLI(t, root, "show", "c2")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/a/x.md", ".capcom/issues/b/x.md")
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing shown", stdout)
	}
}

func TestShow_DuplicatedID_FailsNamingBothFiles(t *testing.T) {
	root, _ := newDuplicateIssueStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "show", "foo")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/foo.md", ".capcom/issues/sub-issue/foo.md")
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing shown", stdout)
	}
}

func TestShow_AliasClash_ShowsEitherNodeAndRefusesTheSharedAlias(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"one-issue.md": readIssue("One", "open", "aliases: [shared]\n", ""),
		"two-issue.md": readIssue("Two", "open", "aliases: [shared]\n", ""),
		"linker.md":    readIssue("Linker", "open", "related: [shared]\n", ""),
	})

	for _, id := range []string{"one-issue", "two-issue"} {
		stdout, _, exitCode := runCLI(t, root, "show", "--json", id)

		issues, _ := decodeEnvelope(t, stdout)["data"].([]any)
		if exitCode != ExitOK || !slices.Equal(envelopeIDs(t, stdout), []string{id}) {
			t.Fatalf("show %s exit = %d stdout = %q, want the issue shown", id, exitCode, stdout)
		}
		if related := issues[0].(map[string]any)["related"]; len(related.([]any)) != 0 {
			t.Errorf("show %s related = %v, want no link through the refused alias", id, related)
		}
	}
	stdout, stderr, exitCode := runCLI(t, root, "show", "shared")
	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/one-issue.md", ".capcom/issues/two-issue.md")
	if stdout != "" {
		t.Errorf("show shared stdout = %q, want nothing shown", stdout)
	}
}

func TestShow_IDAnotherIssueAliases_RefusesTheIDAndShowsTheOther(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"foo.md": readIssue("Foo", "open", "", ""),
		"baz.md": readIssue("Baz", "open", "aliases: [foo]\n", ""),
	})

	stdout, stderr, exitCode := runCLI(t, root, "show", "foo")
	shown, _, shownExit := runCLI(t, root, "show", "--json", "baz")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/baz.md", ".capcom/issues/foo.md")
	if stdout != "" {
		t.Errorf("show foo stdout = %q, want nothing shown", stdout)
	}
	if shownExit != ExitOK || !slices.Equal(envelopeIDs(t, shown), []string{"baz"}) {
		t.Errorf("show baz exit = %d stdout = %q, want baz shown", shownExit, shown)
	}
}

func TestShow_PrefixOfRepeatedIDAndUsableID_NamesBothCandidates(t *testing.T) {
	root := newIssueStore(t, nil)
	writeIssueFiles(t, root, map[string]string{
		"a.md":       readIssue("A", "open", "", ""),
		"b.md":       readIssue("B", "open", "", ""),
		"a/dup-x.md": readIssue("Dup x", "open", "", ""),
		"b/dup-x.md": readIssue("Dup x too", "open", "", ""),
		"dup-y.md":   readIssue("Dup y", "open", "", ""),
	})

	_, stderr, exitCode := runCLI(t, root, "show", "dup")

	if exitCode != ExitError || !strings.Contains(stderr, "dup-x, dup-y") {
		t.Errorf("exit = %d stderr = %q, want an ambiguity naming dup-x and dup-y", exitCode, stderr)
	}
}

func TestShow_IDAnUnusableFileAlsoClaims_FailsNamingBothFiles(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		// claimant is the unusable file that claims foo, relative to the working tree root.
		claimant string
	}{
		{
			name:     "alias of a file that fails validation",
			files:    map[string]string{"bad.md": readIssue("Bad", "bogus", "aliases: [foo]\n", "")},
			claimant: ".capcom/issues/bad.md",
		},
		{
			name:     "alias of a file that does not parse",
			files:    map[string]string{"zzz.md": "---\ntitle: [unclosed\naliases: [foo]\n---\n"},
			claimant: ".capcom/issues/zzz.md",
		},
		{
			name:     "id of a node file below an orphan directory",
			files:    map[string]string{"ghost/foo.md": readIssue("Foo too", "open", "", "")},
			claimant: ".capcom/issues/ghost/foo.md",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newIssueStore(t, nil)
			writeIssueFiles(t, root, map[string]string{"foo.md": readIssue("Foo", "open", "", "")})
			writeIssueFiles(t, root, tc.files)

			stdout, stderr, exitCode := runCLI(t, root, "show", "foo")

			assertDuplicateRefusal(t, root, stderr, exitCode, tc.claimant, ".capcom/issues/foo.md")
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing shown", stdout)
			}
		})
	}
}
