package cli

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/airiclenz/capcom/internal/config"
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

// newerFormat is a store format one past the newest this capcom reads.
const newerFormat = config.FormatVersion + 1

// newerFormatMessage is the error every command but prime gives on a store of newerFormat, whose
// format key sits on line 2 of the config written by writeNewerFormatConfig.
var newerFormatMessage = fmt.Sprintf(
	".capcom/config.yaml:2: store format %d is newer than this capcom reads (up to %d); upgrade capcom",
	newerFormat,
	config.FormatVersion,
)

// writeNewerFormatConfig replaces the store's config.yaml with one declaring newerFormat.
func writeNewerFormatConfig(t *testing.T, root string) {
	t.Helper()
	content := fmt.Sprintf("# written by a newer capcom\nformat: %d\n", newerFormat)
	if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestList_NewerStoreFormat_FailsNamingTheFormatLine(t *testing.T) {
	root := newReadStore(t)
	writeNewerFormatConfig(t, root)

	stdout, stderr, exitCode := runCLI(t, root, "list")

	if exitCode == ExitOK || stdout != "" || stderr != "capcom: "+newerFormatMessage+"\nhint: capcom list --help\n" {
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

	if want := "capcom: .capcom/config.yaml: file is larger than 1 MiB\nhint: capcom list --help\n"; exitCode != ExitError || stdout != "" || stderr != want {
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

func TestList_InvalidFlagValue_IsUsageListingTheValidValues(t *testing.T) {
	tests := map[string]struct {
		args        []string
		wantMessage string
		wantValid   []string
	}{
		"status": {
			args:        []string{"--status", "closed"},
			wantMessage: `--status "closed": not a status declared in config.yaml`,
			wantValid:   declaredStatuses,
		},
		"category": {
			args:        []string{"--category", "closed"},
			wantMessage: `--category "closed": not a status category`,
			wantValid:   []string{"todo", "doing", "waiting", "done", "dropped"},
		},
		"type": {
			args:        []string{"--type", "bugg"},
			wantMessage: `--type "bugg": not a type declared in config.yaml`,
			wantValid:   declaredTypes,
		},
		"priority": {
			args:        []string{"--priority", "5"},
			wantMessage: "--priority 5: out of range",
			wantValid:   validPriorities,
		},
	}
	root := newReadStore(t)
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			args := append([]string{"list"}, test.args...)

			stdout, stderr, exitCode := runCLI(t, root, args...)
			jsonOut, _, jsonExitCode := runCLI(t, root, append(args, "--json")...)

			wantStderr := "capcom: " + test.wantMessage + "; use one of: " + strings.Join(test.wantValid, ", ") +
				"\nhint: capcom list --help\n"
			if exitCode != ExitUsage || stdout != "" || stderr != wantStderr {
				t.Errorf("exit = %d stdout = %q stderr = %q\nwant exit %d and stderr %q",
					exitCode, stdout, stderr, ExitUsage, wantStderr)
			}
			if valid := envelopeValid(t, jsonOut); jsonExitCode != ExitUsage || !slices.Equal(valid, test.wantValid) {
				t.Errorf("--json exit = %d valid = %q, want %d and %q", jsonExitCode, valid, ExitUsage, test.wantValid)
			}
		})
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

func TestListTree_ChildOfUnparseableParent_PrintsAsRoot(t *testing.T) {
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

func TestListTree_ChildOfLeftOutParent_LeftOutWithWarning(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		want    string
		warning string
	}{
		{
			"below a repeated id",
			map[string]string{"release-checklist/login-flow.md": validIssue},
			"release-checklist  open  P0  Release checklist\n",
			".capcom/issues/login-flow: directory left out: its node file login-flow.md has a duplicate id\n",
		},
		{
			"below an invalid id",
			map[string]string{
				"Bad_Name.md":             validIssue,
				"Bad_Name/child-issue.md": validIssue,
			},
			readStoreTree,
			".capcom/issues/Bad_Name: directory left out: its node file Bad_Name.md has an invalid id\n",
		},
	}

	for _, current := range cases {
		root := newReadStore(t)
		writeIssueFiles(t, root, current.files)

		stdout, stderr, exitCode := runCLI(t, root, "list", "--tree")

		// The warnings follow the outline, path-sorted, so the directory's may not be the first.
		if exitCode != ExitOK || !strings.HasPrefix(stdout, current.want+"warning: ") ||
			!strings.Contains(stdout, current.warning) {
			t.Errorf("%s: list --tree = %d\nstdout = %q\nstderr = %q\nwant %q\nand %q",
				current.name, exitCode, stdout, stderr, current.want+"warning: ...", current.warning)
		}
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

func TestListTree_JSONCutTree_CarriesTheTextsCounts(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"login-flow/token-expiry.md": readIssue("Token expiry", "open", "", ""),
	})

	text := mustRun(t, root, "list", "--tree", "--depth", "1")
	jsonOut := mustRun(t, root, "list", "--tree", "--depth", "1", "--json")

	wantText := "login-flow  open  P2  Login flow  (0/2 closed)\n" +
		"  ... 2 more (--depth 2)\n" +
		"release-checklist  open  P0  Release checklist\n"
	if text != wantText {
		t.Errorf("list --tree --depth 1 = %q\nwant %q", text, wantText)
	}
	var got []string
	for _, item := range decodeEnvelope(t, jsonOut)["data"].([]any) {
		issue := item.(map[string]any)
		got = append(got, fmt.Sprintf("%v children=%v more=%v", issue["id"], issue["children"], issue["more"]))
	}
	want := []string{
		"login-flow children=map[closed:0 total:2] more=2",
		"release-checklist children=<nil> more=<nil>",
	}
	if !slices.Equal(got, want) {
		t.Errorf("list --tree --depth 1 --json items = %v\nwant %v", got, want)
	}
}

func TestListTree_BadFlags_UsageErrorNamesTheFlag(t *testing.T) {
	root := newReadStore(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--tree", "--limit", "2"}, "--limit cannot be combined with --tree"},
		{[]string{"--tree", "--depth", "-1"}, "--depth"},
		{[]string{"--depth", "1"}, "--depth"},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"list"}, current.args...)...)

		if exitCode != ExitUsage || stdout != "" || !strings.Contains(stderr, current.want) {
			t.Errorf("list %v exit = %d stdout = %q stderr = %q, want a usage failure saying %q",
				current.args, exitCode, stdout, stderr, current.want)
		}
	}
}

func TestListTree_BadFilterValue_ListsTheDeclaredValues(t *testing.T) {
	root := newReadStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "list", "--tree", "--status", "bogus")

	want := "use one of: " + strings.Join(declaredStatuses, ", ")
	if exitCode != ExitUsage || stdout != "" || !strings.Contains(stderr, want) {
		t.Errorf("list --tree --status bogus exit = %d stdout = %q stderr = %q, want a usage failure with %q",
			exitCode, stdout, stderr, want)
	}
}

func TestListTree_RowFilter_PrintsMatchesWithTheirAncestors(t *testing.T) {
	root := newReadStore(t)
	loginFlow := "login-flow  open  P2  Login flow  (0/1 closed)\n"
	sessionRefresh := "  session-refresh  parked  P2  " + longTitle + "\n"
	releaseChecklist := "release-checklist  open  P0  Release checklist\n"
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--label", "auth"}, loginFlow + sessionRefresh},
		{[]string{"--status", "open"}, loginFlow + releaseChecklist},
		{[]string{"--category", "todo"}, loginFlow + releaseChecklist},
		{[]string{"--no-label", "auth"}, releaseChecklist},
		{[]string{"--type", "task"}, loginFlow + sessionRefresh},
		{[]string{"--priority", "2"}, loginFlow + sessionRefresh},
		{[]string{"--plan", "login"}, loginFlow},
		{[]string{"--planned"}, loginFlow},
		{[]string{"--unplanned"}, strings.TrimSuffix(loginFlow, "\n") + "  [context]\n" + sessionRefresh + releaseChecklist},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"list", "--tree"}, current.args...)...)

		if exitCode != ExitOK || stderr != "" || stdout != current.want {
			t.Errorf("list --tree %v exit = %d stderr = %q\nstdout = %q\nwant     %q",
				current.args, exitCode, stderr, stdout, current.want)
		}
	}
}

// writeDeepLoginFlow adds to newReadStore's login-flow open Issues at every depth below it, each
// under an ancestor that is not open: a dropped child, the parked session-refresh, and an open child
// whose own child is parked.
func writeDeepLoginFlow(t *testing.T, root string) {
	t.Helper()
	writeIssueFiles(t, root, map[string]string{
		"login-flow/old-attempt.md":                   readIssue("Old attempt", "dropped", "", ""),
		"login-flow/old-attempt/revived-step.md":      readIssue("Revived step", "open", "", ""),
		"login-flow/session-refresh/retry-backoff.md": readIssue("Retry backoff", "open", "", ""),
		"login-flow/token-expiry.md":                  readIssue("Token expiry", "open", "", ""),
		"login-flow/token-expiry/cache-warmup.md":     readIssue("Cache warmup", "parked", "", ""),
	})
}

func TestListTree_StatusUnderParent_MarksTheAncestorsAsContext(t *testing.T) {
	root := newReadStore(t)
	writeDeepLoginFlow(t, root)

	stdout, stderr, exitCode := runCLI(t, root, "list", "--tree", "--parent", "login-flow", "--status", "open")

	want := "login-flow  open  P2  Login flow  (1/3 closed)\n" +
		"  old-attempt  dropped  P2  Old attempt  (0/1 closed)  [context]\n" +
		"    revived-step  open  P2  Revived step\n" +
		"  session-refresh  parked  P2  " + longTitle + "  (0/1 closed)  [context]\n" +
		"    retry-backoff  open  P2  Retry backoff\n" +
		"  token-expiry  open  P2  Token expiry  (0/1 closed)\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("list --tree --parent login-flow --status open exit = %d stderr = %q\nstdout = %q\nwant     %q",
			exitCode, stderr, stdout, want)
	}
}

func TestListTree_StatusUnderParentJSON_CarriesContext(t *testing.T) {
	root := newReadStore(t)
	writeDeepLoginFlow(t, root)

	stdout := mustRun(t, root, "list", "--tree", "--parent", "login-flow", "--status", "open", "--json")

	var got []string
	for _, item := range decodeEnvelope(t, stdout)["data"].([]any) {
		issue := item.(map[string]any)
		got = append(got, fmt.Sprintf("%v context=%v children=%v", issue["id"], issue["context"], issue["children"]))
	}
	want := []string{
		"login-flow context=<nil> children=map[closed:1 total:3]",
		"old-attempt context=true children=map[closed:0 total:1]",
		"revived-step context=<nil> children=<nil>",
		"session-refresh context=true children=map[closed:0 total:1]",
		"retry-backoff context=<nil> children=<nil>",
		"token-expiry context=<nil> children=map[closed:0 total:1]",
	}
	if !slices.Equal(got, want) {
		t.Errorf("list --tree --parent login-flow --status open --json items = %v\nwant %v", got, want)
	}
}

func TestListTree_LabelWithoutAll_LeavesOutClosedMatches(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"login-flow/old-auth.md": readIssue("Old auth", "dropped", "labels: [auth]\n", ""),
	})

	open := listTreeIDs(t, root, "--label", "auth")
	all := listTreeIDs(t, root, "--label", "auth", "--all")

	if want := []string{"login-flow", "session-refresh"}; !slices.Equal(open, want) {
		t.Errorf("list --tree --label auth ids = %v, want %v", open, want)
	}
	if want := []string{"login-flow", "old-auth", "session-refresh"}; !slices.Equal(all, want) {
		t.Errorf("list --tree --label auth --all ids = %v, want %v", all, want)
	}
}

// listTreeIDs runs capcom list --tree --json with args and returns the ids of the printed rows.
func listTreeIDs(t *testing.T, root string, args ...string) []string {
	t.Helper()
	return envelopeIDs(t, mustRun(t, root, append([]string{"list", "--tree", "--json"}, args...)...))
}

func TestListTree_FilterWithDepth_CountsKeptRowsOnly(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"login-flow/token-expiry.md": readIssue("Token expiry", "open", "", ""),
	})

	stdout := mustRun(t, root, "list", "--tree", "--depth", "1", "--status", "open")

	want := "login-flow  open  P2  Login flow  (0/2 closed)\n" +
		"  ... 1 more (--depth 2)\n" +
		"release-checklist  open  P0  Release checklist\n"
	if stdout != want {
		t.Errorf("list --tree --depth 1 --status open = %q\nwant %q", stdout, want)
	}
}

func TestListTree_TypeUnderClosedParent_PrintsTheParentAsContext(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"old-idea/old-step.md": readIssue("Old step", "open", "", ""),
	})

	stdout := mustRun(t, root, "list", "--tree", "--type", "task")

	want := "login-flow  open  P2  Login flow  (0/1 closed)\n" +
		"  session-refresh  parked  P2  " + longTitle + "\n" +
		"old-idea  dropped  P2  Old idea  (0/1 closed)  [context]\n" +
		"  old-step  open  P2  Old step\n"
	if stdout != want {
		t.Errorf("list --tree --type task = %q\nwant %q", stdout, want)
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

func TestShow_StandingIssue_PrintsStandingAfterPlanInText(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"feature-requests.md": readIssue("Feature requests", "open", "plan: intake\nstanding: true\n", ""),
	})

	stdout, stderr, exitCode := runCLI(t, root, "show", "feature-requests")

	want := "\nplan: intake\nstanding: true\ncreated: 2026-10-11T09:00:00Z\n"
	if exitCode != ExitOK || stderr != "" || !strings.HasSuffix(stdout, want) {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant the suffix %q", exitCode, stderr, stdout, want)
	}
}

func TestShowAndList_JSON_CarryStandingOnlyWhenMarked(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"feature-requests.md": readIssue("Feature requests", "open", "standing: true\n", ""),
		"login-flow.md":       readIssue("Login flow", "open", "", ""),
	})
	standingOf := func(data []any) map[string]any {
		values := map[string]any{}
		for _, item := range data {
			issue := item.(map[string]any)
			values[issue["id"].(string)] = issue["standing"]
		}
		return values
	}

	shown := standingOf(decodeEnvelope(t, mustRun(t, root, "show", "--json", "feature-requests"))["data"].([]any))
	listed := standingOf(decodeEnvelope(t, mustRun(t, root, "list", "--json"))["data"].([]any))

	if shown["feature-requests"] != true || listed["feature-requests"] != true {
		t.Errorf("standing: show = %v, list = %v, want true", shown, listed)
	}
	if value, isPresent := listed["login-flow"]; !isPresent || value != nil {
		t.Errorf("list login-flow standing = %v (listed %v), want it left out", value, isPresent)
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
		!strings.Contains(stdout, "\nid: old-idea\n") || stderr != "capcom: id \"absent\" not found\nhint: capcom search <text>\n" {
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

func TestShow_UnknownID_SuggestsTheClosestIDs(t *testing.T) {
	root := newReadStore(t)
	cases := []struct {
		name string
		arg  string
		want string
	}{
		{"an id", "login-flwo", "capcom: id \"login-flwo\" not found; did you mean login-flow?\nhint: capcom search <text>\n"},
		{"an alias", "apogee-kk0.5", "capcom: id \"apogee-kk0.5\" not found; did you mean apogee-kk0.4?\nhint: capcom search <text>\n"},
		{"a shared word", "session-timeout", "capcom: id \"session-timeout\" not found; did you mean session-refresh?\nhint: capcom search <text>\n"},
		{"nothing close", "zzzzzz", "capcom: id \"zzzzzz\" not found\nhint: capcom search <text>\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, stderr, exitCode := runCLI(t, root, "show", testCase.arg)

			if exitCode != ExitNotFound || stderr != testCase.want {
				t.Errorf("show %s exit = %d stderr = %q, want %q", testCase.arg, exitCode, stderr, testCase.want)
			}
		})
	}
}

func TestShow_UnknownIDJSON_CarriesSuggestions(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--json", "login-flwo")

	errorBody, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	suggestions, _ := errorBody["suggestions"].([]any)
	if exitCode != ExitNotFound || len(suggestions) != 1 || suggestions[0] != "login-flow" {
		t.Errorf("exit = %d error = %v, want suggestions [login-flow]", exitCode, errorBody)
	}
}

func TestShow_KVUnknownID_RecordCarriesSuggestions(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--kv", "login-flwo")

	want := "ID=login-flwo\nMESSAGE=id \"login-flwo\" not found\nSUGGESTIONS=login-flow\nHINT=capcom search <text>\nSTATUS=not_found\n"
	if exitCode != ExitNotFound || stdout != want {
		t.Errorf("exit = %d\nstdout = %q\nwant     %q", exitCode, stdout, want)
	}
}

func TestShow_SeveralUnknownIDs_NamesEachIDsSuggestions(t *testing.T) {
	root := newReadStore(t)

	_, stderr, exitCode := runCLI(t, root, "show", "login-flwo", "session-timeout", "zzzzzz")

	want := "capcom: id \"login-flwo\" not found, did you mean login-flow?; " +
		"id \"session-timeout\" not found, did you mean session-refresh?; id \"zzzzzz\" not found\n" +
		"hint: capcom search <text>\n"
	if exitCode != ExitNotFound || stderr != want {
		t.Errorf("exit = %d\nstderr = %q\nwant     %q", exitCode, stderr, want)
	}
}

func TestShow_SeveralUnknownIDsJSON_MessageNamesEachIDsSuggestions(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--json", "login-flwo", "session-timeout")

	errorBody, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	want := "id \"login-flwo\" not found, did you mean login-flow?; " +
		"id \"session-timeout\" not found, did you mean session-refresh?"
	_, hasSuggestions := errorBody["suggestions"]
	if exitCode != ExitNotFound || errorBody["message"] != want || hasSuggestions {
		t.Errorf("exit = %d error = %v, want message %q and no suggestions", exitCode, errorBody, want)
	}
}

func TestShow_KVSeveralUnknownIDs_EachRecordCarriesItsSuggestions(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--kv", "login-flwo", "session-timeout")

	want := "ID=login-flwo\nMESSAGE=id \"login-flwo\" not found\nSUGGESTIONS=login-flow\nHINT=capcom search <text>\nSTATUS=not_found\n" +
		"ID=session-timeout\nMESSAGE=id \"session-timeout\" not found\nSUGGESTIONS=session-refresh\n" +
		"HINT=capcom search <text>\nSTATUS=not_found\n"
	if exitCode != ExitNotFound || stdout != want {
		t.Errorf("exit = %d\nstdout = %q\nwant     %q", exitCode, stdout, want)
	}
}

func TestShow_KV_ClosesEachIDWithItsStatus(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "--kv", "rel", "absent")

	want := "ID=release-checklist\nETAG=" + fileETag(t, root, "release-checklist.md") + "\nISSUE_STATUS=open\nCATEGORY=todo\nTITLE=Release checklist\n" +
		"TYPE=chore\nPRIORITY=0\nLABELS=\nPARENT=\nDEPS=\nPLAN=\nSTATUS=ok\n" +
		"ID=absent\nMESSAGE=id \"absent\" not found\nHINT=capcom search <text>\nSTATUS=not_found\n"
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

func TestShow_Text_StandingParent_ReadyLineGivesTheReason(t *testing.T) {
	root := newStandingParentStore(t)

	area := mustRun(t, root, "show", "area")
	epic := mustRun(t, root, "show", "epic")

	if !strings.Contains(area, "\nready: no (standing: holds children)\n") ||
		!strings.Contains(epic, "\nready: no (standing: holds children; open children: child-task)\n") {
		t.Errorf("show area = %q\nshow epic = %q", area, epic)
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

	if exitCode != ExitNotFound || stdout != "" || stderr != "capcom: no issue matches \"absent\"\nhint: capcom search <text>\n" {
		t.Errorf("exit = %d stdout = %q stderr = %q, want not_found", exitCode, stdout, stderr)
	}
}

func TestSearch_OneWordInNoIssue_ListsAnyWordHitsUnderTheHeading(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"refresh-twice.md": readIssue("Refresh refresh", "open", "", "\nrefresh refresh refresh\n"),
	})

	stdout, stderr, exitCode := runCLI(t, root, "search", "refresh", "absent")
	jsonOut, _, jsonExit := runCLI(t, root, "search", "--json", "refresh", "absent")

	want := "No item holds every word; these hold some of them:\n" +
		"refresh-twice  open  P2  Refresh refresh\nsession-refresh  parked  P2  " + longTitle + "\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
	envelope := decodeEnvelope(t, jsonOut)
	if got := envelopeIDs(t, jsonOut); jsonExit != ExitOK || envelope["match"] != "any" ||
		!slices.Equal(got, []string{"refresh-twice", "session-refresh"}) {
		t.Errorf("search --json exit = %d envelope = %v, want match any and both issues", jsonExit, envelope)
	}
}

func TestSearch_EveryWordHeld_CarriesMatchAll(t *testing.T) {
	root := newReadStore(t)

	stdout, _, exitCode := runCLI(t, root, "search", "--json", "refresh", "config.yaml")

	envelope := decodeEnvelope(t, stdout)
	if got := envelopeIDs(t, stdout); exitCode != ExitOK || envelope["match"] != "all" ||
		!slices.Equal(got, []string{"session-refresh"}) {
		t.Errorf("search --json exit = %d envelope = %v, want match all and session-refresh", exitCode, envelope)
	}
}

func TestSearch_OperatorWordInNoIssue_StaysLiteralInTheFallback(t *testing.T) {
	root := newReadStore(t)

	for _, operator := range []string{"NOT", "OR", "NEAR"} {
		stdout, stderr, exitCode := runCLI(t, root, "search", "--json", operator, "epic")

		envelope := decodeEnvelope(t, stdout)
		if got := envelopeIDs(t, stdout); exitCode != ExitOK || envelope["match"] != "any" ||
			!slices.Equal(got, []string{"login-flow"}) {
			t.Errorf("search %s epic exit = %d envelope = %v stderr = %q, want match any and login-flow",
				operator, exitCode, envelope, stderr)
		}
	}
}

// hitExcerpts returns, by id, the excerpt of each hit in a search envelope's data list; a hit
// carrying no excerpt member is left out.
func hitExcerpts(t *testing.T, stdout string) map[string][]string {
	t.Helper()
	hits, _ := decodeEnvelope(t, stdout)["data"].([]any)
	excerpts := map[string][]string{}
	for _, item := range hits {
		hit := item.(map[string]any)
		lines, isPresent := hit["excerpt"].([]any)
		if !isPresent {
			continue
		}
		excerpt := make([]string, 0, len(lines))
		for _, line := range lines {
			excerpt = append(excerpt, line.(string))
		}
		excerpts[hit["id"].(string)] = excerpt
	}
	return excerpts
}

func TestSearch_Excerpt_PrintsTheMatchingBodyLinesUnderEachHit(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"refresh-twice.md": readIssue("Refresh refresh", "open", "", "\nrefresh refresh refresh\n"),
	})

	stdout, stderr, exitCode := runCLI(t, root, "search", "refresh", "--excerpt")
	jsonOut := mustRun(t, root, "search", "refresh", "--excerpt", "--json")

	want := "refresh-twice  open  P2  Refresh refresh\n    \n    refresh refresh refresh\n" +
		"session-refresh  parked  P2  " + longTitle + "\n    \n" +
		"    Blocked by login-timeout-race; the refresh doesn't wait for config.yaml.\n    \n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
	wantJSON := map[string][]string{
		"refresh-twice":   {"", "refresh refresh refresh"},
		"session-refresh": {"", "Blocked by login-timeout-race; the refresh doesn't wait for config.yaml.", ""},
	}
	if got := hitExcerpts(t, jsonOut); !maps.EqualFunc(got, wantJSON, slices.Equal) {
		t.Errorf("search --excerpt --json excerpts = %q, want %q", got, wantJSON)
	}
}

func TestSearch_ExcerptOfAWordWithDiacritics_PrintsTheLineHoldingIt(t *testing.T) {
	root := newReadStore(t)
	writeIssueFiles(t, root, map[string]string{
		"menu-cafe.md": readIssue("Menu screen", "open", "", "\nOpen the café menu.\n"),
	})

	stdout, stderr, exitCode := runCLI(t, root, "search", "cafe", "--excerpt")

	want := "menu-cafe  open  P2  Menu screen\n    \n    Open the café menu.\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestSearch_NoExcerptFlag_LeavesTheExcerptOut(t *testing.T) {
	root := newReadStore(t)

	stdout := mustRun(t, root, "search", "refresh", "--json")

	if got := hitExcerpts(t, stdout); len(got) != 0 {
		t.Errorf("search --json excerpts = %q, want none without --excerpt", got)
	}
}

func TestSearch_ExcerptOnTheAnyWordFallback_ShowsTheLinesOfTheWordsHeld(t *testing.T) {
	root := newReadStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "search", "epic", "absent", "--excerpt")
	jsonOut := mustRun(t, root, "search", "epic", "absent", "--excerpt", "--json")

	want := "No item holds every word; these hold some of them:\n" +
		"login-flow  open  P2  Login flow\n    \n    The login epic.\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
	wantJSON := map[string][]string{"login-flow": {"", "The login epic."}}
	if got := hitExcerpts(t, jsonOut); !maps.EqualFunc(got, wantJSON, slices.Equal) {
		t.Errorf("search --excerpt --json excerpts = %q, want %q", got, wantJSON)
	}
}

func TestSearch_ExcerptOfALongBody_HoldsAtMostEightLines(t *testing.T) {
	root := newReadStore(t)
	var body strings.Builder
	for line := range 40 {
		if line%4 == 0 {
			body.WriteString("alpha " + strings.Repeat("long ", 40) + "\n")
			continue
		}
		body.WriteString("filler\n")
	}
	writeIssueFiles(t, root, map[string]string{
		"long-body.md": readIssue("Long body", "open", "", "\n"+body.String()),
	})

	stdout := mustRun(t, root, "search", "alpha", "--excerpt")
	excerpt := hitExcerpts(t, mustRun(t, root, "search", "alpha", "--excerpt", "--json"))["long-body"]

	excerptLines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")[1:]
	if len(excerptLines) == 0 || len(excerptLines) > 8 || len(excerpt) != len(excerptLines) {
		t.Errorf("search alpha --excerpt = %d text lines and %d --json lines, want 1 to 8 each\n%s",
			len(excerptLines), len(excerpt), stdout)
	}
	for _, line := range excerpt {
		if utf8.RuneCountInString(line) > 120 {
			t.Errorf("excerpt line of %d runes, want at most 120: %q", utf8.RuneCountInString(line), line)
		}
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

// showKVKeys is every key of a show --kv record, in order.
var showKVKeys = []string{
	"ID", "ETAG", "ISSUE_STATUS", "CATEGORY", "TITLE",
	"TYPE", "PRIORITY", "LABELS", "PARENT", "DEPS", "PLAN", "STATUS",
}

func TestShow_KV_PrintsEveryKeyEvenWhenEmpty(t *testing.T) {
	root := newReadStore(t)

	stdout := mustRun(t, root, "show", "release-checklist", "--kv")

	if keys := kvKeys(stdout); !slices.Equal(keys, showKVKeys) {
		t.Fatalf("keys = %v, want %v", keys, showKVKeys)
	}
	for _, key := range []string{"LABELS", "PARENT", "DEPS", "PLAN"} {
		if value := kvValue(t, stdout, key); value != "" {
			t.Errorf("%s = %q, want empty", key, value)
		}
	}
}

func TestShow_KV_AfterUpdate_PrintsTypePriorityAndLabels(t *testing.T) {
	root := newReadStore(t)
	mustRun(t, root, "update", "release-checklist", "--add-label", "alpha", "--type", "bug", "--priority", "3")

	stdout := mustRun(t, root, "show", "release-checklist", "--kv")

	want := map[string]string{"TYPE": "bug", "PRIORITY": "3", "LABELS": "alpha"}
	for key, value := range want {
		if got := kvValue(t, stdout, key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
}

func TestShow_KV_PrintsParentDepsAndPlan(t *testing.T) {
	root := newReadStore(t)
	mustRun(t, root, "dep", "add", "session-refresh", "release-checklist")
	mustRun(t, root, "dep", "add", "session-refresh", "old-idea")

	child := mustRun(t, root, "show", "session-refresh", "--kv")
	parent := mustRun(t, root, "show", "login-flow", "--kv")

	if got := kvValue(t, child, "PARENT"); got != "login-flow" {
		t.Errorf("PARENT = %q, want login-flow", got)
	}
	if got := kvValue(t, child, "DEPS"); got != "old-idea,release-checklist" &&
		got != "release-checklist,old-idea" {
		t.Errorf("DEPS = %q, want old-idea and release-checklist comma-separated", got)
	}
	if got := kvValue(t, parent, "LABELS"); got != "auth,epic" {
		t.Errorf("LABELS = %q, want auth,epic", got)
	}
	if got := kvValue(t, parent, "PLAN"); got != "2026-10-11 - 00 - login-plan" {
		t.Errorf("PLAN = %q, want 2026-10-11 - 00 - login-plan", got)
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
