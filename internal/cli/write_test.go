package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// runCLIWith runs capcom like runCLI, with stdin as its standard input and env set on top of the
// hermetic git environment.
func runCLIWith(
	t *testing.T,
	dir string,
	stdin string,
	env map[string]string,
	args ...string,
) (stdout, stderr string, exitCode int) {
	t.Helper()
	t.Chdir(dir)
	setHermeticGitEnv(t)
	for key, value := range env {
		t.Setenv(key, value)
	}

	var stdoutBuffer, stderrBuffer bytes.Buffer
	exitCode = Run(args, strings.NewReader(stdin), &stdoutBuffer, &stderrBuffer)
	return stdoutBuffer.String(), stderrBuffer.String(), exitCode
}

// newWriteStore creates a store with one open issue, one done issue and one issue carrying an
// alias that is a valid slug.
func newWriteStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"login-flow.md":    readIssue("Login flow", "open", "labels: [auth]\n", "\nThe old text stays old.\n"),
		"shipped-fix.md":   readIssue("Shipped fix", "done", "", "\nDone.\n"),
		"renamed-issue.md": readIssue("Renamed", "open", "aliases: [former-name]\n", ""),
	})
}

// readStoreFile returns the content of the issue file at a slash path below .capcom/issues/.
func readStoreFile(t *testing.T, root string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".capcom", "issues", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

// mustRun runs capcom and fails the test unless it exits ok; it returns stdout.
func mustRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	stdout, stderr, exitCode := runCLI(t, root, args...)
	if exitCode != ExitOK {
		t.Fatalf("%v exit = %d\nstdout = %s\nstderr = %s", args, exitCode, stdout, stderr)
	}
	return stdout
}

// today is the date a Log entry written now carries.
func today() string {
	return time.Now().Format(time.DateOnly)
}

func TestCreate_UnderParent_CreatesParentDirectory(t *testing.T) {
	root := newWriteStore(t)

	stdout := mustRun(t, root, "create", "--kv", "--id", "session-refresh", "--title", "Refresh session",
		"--parent", "login", "--type", "bug", "--priority", "1", "--label", "auth,ui", "--plan", "login-plan")

	content := readStoreFile(t, root, "login-flow/session-refresh.md")
	wantFront := "---\ntitle: Refresh session\nstatus: open\ntype: bug\npriority: 1\nlabels: [auth, ui]\nplan: login-plan\ncreated: "
	if !strings.HasPrefix(content, wantFront) || !strings.Contains(stdout, "PATH=.capcom/issues/login-flow/session-refresh.md\n") {
		t.Errorf("content = %q\nstdout = %q", content, stdout)
	}
	if ids := listIDs(t, root, "--parent", "login-flow"); len(ids) != 1 || ids[0] != "session-refresh" {
		t.Errorf("children of login-flow = %v", ids)
	}
}

func TestCreate_Defaults_TaskOpenAndEmptyBody(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "create", "--id", "new-thing", "--title", "New thing")

	content := readStoreFile(t, root, "new-thing.md")
	if !strings.HasPrefix(content, "---\ntitle: New thing\nstatus: open\ntype: task\npriority: 2\ncreated: ") ||
		!strings.HasSuffix(content, "Z\n---\n") {
		t.Errorf("content = %q", content)
	}
}

func TestCreate_TakenIDOrAlias_FailsExists(t *testing.T) {
	root := newWriteStore(t)

	for _, id := range []string{"login-flow", "former-name"} {
		stdout, _, exitCode := runCLI(t, root, "create", "--json", "--id", id, "--title", "Again")

		envelope := decodeEnvelope(t, stdout)
		failure, _ := envelope["error"].(map[string]any)
		if exitCode != ExitError || failure["code"] != string(CodeExists) || failure["path"] == "" {
			t.Errorf("%s: exit = %d envelope = %v", id, exitCode, envelope)
		}
	}
}

func TestCreate_BadSlug_FailsInvalid(t *testing.T) {
	root := newWriteStore(t)

	for _, id := range []string{"Upper-Case", "a-b-c-d-e", "under_score-x"} {
		_, stderr, exitCode := runCLI(t, root, "create", "--id", id, "--title", "Bad")

		if exitCode != ExitError || !strings.Contains(stderr, "invalid id") {
			t.Errorf("%s: exit = %d stderr = %q", id, exitCode, stderr)
		}
	}
}

func TestCreate_MissingIDOrClosedStatus_IsRefused(t *testing.T) {
	root := newWriteStore(t)

	_, _, missingExit := runCLI(t, root, "create", "--title", "No id")
	_, stderr, closedExit := runCLI(t, root, "create", "--id", "born-done", "--title", "Done", "--status", "done")

	wantStderr := "capcom: status \"done\" is in the done category; create the issue open, then close it\nhint: capcom create --help\n"
	if missingExit != ExitUsage || closedExit != ExitError || stderr != wantStderr {
		t.Errorf("missing id exit = %d, closed status exit = %d\nstderr = %q\nwant     %q",
			missingExit, closedExit, stderr, wantStderr)
	}
}

func TestCreate_BodyFromStdin_WritesBody(t *testing.T) {
	root := newWriteStore(t)

	_, stderr, exitCode := runCLIWith(t, root, "## Acceptance\r\n\r\n- works\r\n", nil,
		"create", "--id", "from-stdin", "--title", "From stdin", "--body-file", "-")

	content := readStoreFile(t, root, "from-stdin.md")
	if exitCode != ExitOK || !strings.HasSuffix(content, "---\n\n## Acceptance\n\n- works\n") {
		t.Errorf("exit = %d stderr = %q content = %q", exitCode, stderr, content)
	}
}

func TestCreate_LabelWithEmptyEntry_DropsIt(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "create", "--id", "new-thing", "--title", "New thing", "--label", "x,,y")

	content := readStoreFile(t, root, "new-thing.md")
	if !strings.Contains(content, "\nlabels: [x, y]\n") {
		t.Errorf("content = %q", content)
	}
	mustRun(t, root, "show", "new-thing")
	mustRun(t, root, "check")
}

func TestCreate_DepByAliasAndRepeated_StoresEachCanonicalIDOnce(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "create", "--id", "new-thing", "--title", "New thing",
		"--dep", "former-name,login-flow", "--dep", "renamed-issue")

	if content := readStoreFile(t, root, "new-thing.md"); !strings.Contains(content, "\ndeps: [renamed-issue, login-flow]\n") {
		t.Errorf("content = %q", content)
	}
	stdout := mustRun(t, root, "dep", "list", "former-name")
	if !strings.Contains(stdout, "dependents:\n  new-thing  open  P2  New thing\n") {
		t.Errorf("dep list stdout = %q", stdout)
	}
}

func TestCreate_DepWithEmptyEntry_SkipsIt(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "create", "--id", "new-thing", "--title", "New thing", "--dep", "login-flow,,shipped-fix")

	if content := readStoreFile(t, root, "new-thing.md"); !strings.Contains(content, "\ndeps: [login-flow, shipped-fix]\n") {
		t.Errorf("content = %q", content)
	}
}

func TestCreate_TakenIDAsItsOwnDep_FailsExists(t *testing.T) {
	root := newWriteStore(t)
	before := readStoreFile(t, root, "login-flow.md")

	stdout, _, exitCode := runCLI(t, root, "create", "--json", "--id", "login-flow", "--title", "Again",
		"--dep", "login-flow")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeExists) {
		t.Errorf("exit = %d error = %v, want exists", exitCode, failure)
	}
	if after := readStoreFile(t, root, "login-flow.md"); after != before {
		t.Errorf("login-flow.md = %q, want it unchanged", after)
	}
}

func TestCreate_MissingDep_FailsLikeDepAddAndWritesNothing(t *testing.T) {
	root := newWriteStore(t)
	_, depAddStderr, depAddExit := runCLI(t, root, "dep", "add", "login-flow", "gone-issue")

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "new-thing", "--title", "New thing",
		"--dep", "login-flow", "--dep", "gone-issue")

	if exitCode != depAddExit || exitCode == ExitOK || stderr != depAddStderr {
		t.Errorf("exit = %d stderr = %q, want dep add's exit = %d stderr = %q", exitCode, stderr, depAddExit, depAddStderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "new-thing.md")); !os.IsNotExist(err) {
		t.Errorf("new-thing.md: Stat err = %v, want not exist", err)
	}
}

func TestCreate_EmptyOrWhitespaceBody_WritesAndWarnsInTextAndJSON(t *testing.T) {
	const want = `new-thing has no body: state what, why and a "## Acceptance" section with ` +
		`capcom update new-thing --body-append-file <file>`
	cases := []struct {
		name  string
		stdin string
		args  []string
	}{
		{name: "no body"},
		{name: "whitespace body", stdin: " \n\t\n", args: []string{"--body-file", "-"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			textRoot, jsonRoot := newWriteStore(t), newWriteStore(t)
			args := append([]string{"create", "--id", "new-thing", "--title", "New thing"}, tc.args...)

			textStdout, textStderr, textExit := runCLIWith(t, textRoot, tc.stdin, nil, args...)
			jsonStdout, _, jsonExit := runCLIWith(t, jsonRoot, tc.stdin, nil, append(args, "--json")...)

			if textExit != ExitOK || !strings.Contains(textStdout, "warning: "+want+"\n") {
				t.Errorf("text: exit = %d stdout = %q stderr = %q, want warning %q", textExit, textStdout, textStderr, want)
			}
			warnings, _ := decodeEnvelope(t, jsonStdout)["warnings"].([]any)
			if jsonExit != ExitOK || len(warnings) != 1 || warnings[0] != want {
				t.Errorf("json: exit = %d warnings = %v, want [%q]", jsonExit, warnings, want)
			}
			readStoreFile(t, textRoot, "new-thing.md")
		})
	}
}

func TestCreate_WithBody_DoesNotWarn(t *testing.T) {
	root := newWriteStore(t)

	stdout, stderr, exitCode := runCLIWith(t, root, "## Acceptance\n\n- works\n", nil,
		"create", "--json", "--id", "new-thing", "--title", "New thing", "--body-file", "-")

	warnings, _ := decodeEnvelope(t, stdout)["warnings"].([]any)
	if exitCode != ExitOK || len(warnings) != 0 {
		t.Errorf("exit = %d stderr = %q warnings = %v, want none", exitCode, stderr, warnings)
	}
}

func TestUpdate_Fields_ChangeTheFile(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "update", "login", "--title", "Login rework", "--status", "parked", "--type", "feature",
		"--priority", "0", "--add-label", "ui,epic", "--remove-label", "auth")

	content := readStoreFile(t, root, "login-flow.md")
	want := "---\ntitle: Login rework\nstatus: parked\ntype: feature\npriority: 0\nlabels: [ui, epic]\n"
	if !strings.HasPrefix(content, want) {
		t.Errorf("content = %q\nwant prefix %q", content, want)
	}
}

func TestUpdate_AddLabelWithEmptyEntry_DropsIt(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "update", "login-flow", "--add-label", "x,,y")

	content := readStoreFile(t, root, "login-flow.md")
	if !strings.Contains(content, "\nlabels: [auth, x, y]\n") {
		t.Errorf("content = %q", content)
	}
	mustRun(t, root, "show", "login-flow")
	mustRun(t, root, "check")
}

func TestUpdate_NoChangeFlag_ExitsUsage(t *testing.T) {
	root := newWriteStore(t)

	_, _, exitCode := runCLI(t, root, "update", "login-flow")

	if exitCode != ExitUsage {
		t.Errorf("exit = %d, want usage", exitCode)
	}
}

func TestUpdate_BodyReplace_MustMatchExactlyOnce(t *testing.T) {
	root := newWriteStore(t)

	// Ordered: "old" occurs twice only until "The old" has been replaced.
	cases := []struct {
		old      string
		wantExit int
	}{
		{"absent", ExitError},
		{"old", ExitError},
		{"The old", ExitOK},
	}
	for _, tc := range cases {
		_, stderr, exitCode := runCLI(t, root, "update", "login-flow",
			"--body-replace-old", tc.old, "--body-replace-new", "The new")

		if exitCode != tc.wantExit {
			t.Errorf("%q: exit = %d stderr = %q, want %d", tc.old, exitCode, stderr, tc.wantExit)
		}
	}
	if content := readStoreFile(t, root, "login-flow.md"); !strings.HasSuffix(content, "\nThe new text stays old.\n") {
		t.Errorf("content = %q", content)
	}
}

func TestUpdate_BodyAppend_IsLiteral(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "update", "login-flow", "--body-append", "@alice")

	if content := readStoreFile(t, root, "login-flow.md"); !strings.HasSuffix(content, "stays old.\n\n@alice\n") {
		t.Errorf("content = %q", content)
	}
}

func TestUpdate_BodyAppendFile_FromFileAndStdin(t *testing.T) {
	root := newWriteStore(t)
	notes := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(notes, []byte("From a file.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mustRun(t, root, "update", "login-flow", "--body-append-file", notes)
	_, stderr, exitCode := runCLIWith(t, root, "From stdin.\n", nil, "update", "login-flow", "--body-append-file", "-")

	content := readStoreFile(t, root, "login-flow.md")
	if exitCode != ExitOK || !strings.HasSuffix(content, "stays old.\n\nFrom a file.\n\nFrom stdin.\n") {
		t.Errorf("exit = %d stderr = %q content = %q", exitCode, stderr, content)
	}
}

// newBodyEditStore returns a store whose one issue, body-edits, has the body "alpha beta gamma".
func newBodyEditStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"body-edits.md": readIssue("Body edits", "open", "", "\nalpha beta gamma\n"),
	})
}

func TestUpdate_RepeatedBodyEdits_ApplyInOrder(t *testing.T) {
	cases := map[string]struct {
		args     []string
		wantBody string
	}{
		"two replacement pairs": {
			args: []string{"--body-replace-old", "alpha", "--body-replace-new", "A",
				"--body-replace-old", "gamma", "--body-replace-new", "G"},
			wantBody: "\nA beta G\n",
		},
		"a later pair matches text an earlier one inserted": {
			args: []string{"--body-replace-old", "alpha", "--body-replace-new", "omega",
				"--body-replace-old", "omega beta", "--body-replace-new", "Z"},
			wantBody: "\nZ gamma\n",
		},
		"two appends": {
			args:     []string{"--body-append", "one", "--body-append", "two"},
			wantBody: "\nalpha beta gamma\n\none\n\ntwo\n",
		},
		"an append keeps its comma": {
			args:     []string{"--body-append", "a, b"},
			wantBody: "\nalpha beta gamma\n\na, b\n",
		},
		"replacements before appends": {
			args: []string{"--body-append", "alpha again", "--body-replace-old", "alpha",
				"--body-replace-new", "A"},
			wantBody: "\nA beta gamma\n\nalpha again\n",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			root := newBodyEditStore(t)

			mustRun(t, root, append([]string{"update", "body-edits"}, testCase.args...)...)

			if content := readStoreFile(t, root, "body-edits.md"); !strings.HasSuffix(content, "---\n"+testCase.wantBody) {
				t.Errorf("content = %q, want the body %q", content, testCase.wantBody)
			}
		})
	}
}

func TestUpdate_RepeatedBodyEdits_AreRefusedWhole(t *testing.T) {
	cases := map[string]struct {
		args       []string
		wantExit   int
		wantStderr string
	}{
		"a later pair does not match": {
			args: []string{"--body-replace-old", "alpha", "--body-replace-new", "A",
				"--body-replace-old", "delta", "--body-replace-new", "D", "--body-append", "never"},
			wantExit:   ExitError,
			wantStderr: `--body-replace-old #2 "delta": `,
		},
		"two old texts and one new": {
			args: []string{"--body-replace-old", "alpha", "--body-replace-old", "gamma",
				"--body-replace-new", "A"},
			wantExit:   ExitUsage,
			wantStderr: "2 --body-replace-old and 1 --body-replace-new given; give them in pairs",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			root := newBodyEditStore(t)
			before := readStoreFile(t, root, "body-edits.md")

			_, stderr, exitCode := runCLI(t, root, append([]string{"update", "body-edits"}, testCase.args...)...)

			if exitCode != testCase.wantExit || !strings.Contains(stderr, testCase.wantStderr) {
				t.Errorf("exit = %d stderr = %q, want %d and %q", exitCode, stderr, testCase.wantExit, testCase.wantStderr)
			}
			if after := readStoreFile(t, root, "body-edits.md"); after != before {
				t.Errorf("file changed:\nbefore = %q\nafter  = %q", before, after)
			}
		})
	}
}

func TestUpdate_StatusIntoOrOutOfClosed_IsRefused(t *testing.T) {
	root := newWriteStore(t)

	const leavesClosed = "capcom: .capcom/issues/shipped-fix.md: " +
		"status \"done\" is closed; use reopen to move the issue out of it\nhint: capcom update --help\n"
	cases := map[string]struct {
		args       []string
		wantStderr string
	}{
		"into done": {
			args:       []string{"update", "login-flow", "--status", "done"},
			wantStderr: "capcom: .capcom/issues/login-flow.md: status \"done\" is in the done category; use close\nhint: capcom update --help\n",
		},
		"out of done":  {args: []string{"update", "shipped-fix", "--status", "open"}, wantStderr: leavesClosed},
		"done to drop": {args: []string{"update", "shipped-fix", "--status", "dropped"}, wantStderr: leavesClosed},
	}
	for name, tc := range cases {
		_, stderr, exitCode := runCLI(t, root, tc.args...)

		if exitCode != ExitError || stderr != tc.wantStderr {
			t.Errorf("%s: exit = %d\nstderr = %q\nwant     %q", name, exitCode, stderr, tc.wantStderr)
		}
	}
}

func TestUpdate_IfMatchFromShow_WritesOnlyOnTheCurrentEtag(t *testing.T) {
	root := newWriteStore(t)
	kvEtag := kvValue(t, mustRun(t, root, "show", "--kv", "login-flow"), "ETAG")
	jsonData, _ := decodeEnvelope(t, mustRun(t, root, "show", "--json", "login-flow"))["data"].([]any)
	jsonEtag, _ := jsonData[0].(map[string]any)["etag"].(string)
	textEtag := ""
	for _, line := range strings.Split(mustRun(t, root, "show", "login-flow"), "\n") {
		if value, isEtag := strings.CutPrefix(line, "etag: "); isEtag {
			textEtag = value
		}
	}
	if kvEtag == "" || jsonEtag != kvEtag || textEtag != kvEtag {
		t.Fatalf("etags differ: kv %q json %q text %q", kvEtag, jsonEtag, textEtag)
	}

	updated := mustRun(t, root, "update", "--kv", "login-flow", "--if-match", kvEtag, "--title", "First")
	stdout, _, exitCode := runCLI(t, root, "update", "--json", "login-flow", "--if-match", kvEtag, "--title", "Second")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	newEtag := kvValue(t, updated, "ETAG")
	if exitCode != ExitError || failure["code"] != string(CodeConflict) || newEtag == kvEtag {
		t.Errorf("exit = %d error = %v new etag = %q", exitCode, failure, newEtag)
	}
	if shown := kvValue(t, mustRun(t, root, "show", "--kv", "login-flow"), "ETAG"); shown != newEtag {
		t.Errorf("show etag = %q, update reported %q", shown, newEtag)
	}
}

// kvValue returns the value of key in --kv output.
func kvValue(t *testing.T, output string, key string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if value, found := strings.CutPrefix(line, key+"="); found {
			return value
		}
	}
	t.Fatalf("no %s= line in %q", key, output)
	return ""
}

func TestLog_CreatesSectionThenAppends(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "log", "login-flow", "started")
	mustRun(t, root, "log", "login", "finished")

	want := "\nThe old text stays old.\n\n## Log\n\n- " + today() + " Test Author: started\n- " +
		today() + " Test Author: finished\n"
	if content := readStoreFile(t, root, "login-flow.md"); !strings.HasSuffix(content, "---\n"+want) {
		t.Errorf("content = %q\nwant suffix %q", content, want)
	}
}

func TestLog_EmptyTextOrUnknownID_IsRefused(t *testing.T) {
	root := newWriteStore(t)

	_, _, emptyExit := runCLI(t, root, "log", "login-flow", "  ")
	_, _, unknownExit := runCLI(t, root, "log", "no-such-issue", "text")

	if emptyExit != ExitError || unknownExit != ExitNotFound {
		t.Errorf("empty exit = %d, unknown exit = %d", emptyExit, unknownExit)
	}
}

func TestLog_Author_FromEachSource(t *testing.T) {
	cases := []struct {
		name       string
		authorName string
		userName   string
		want       string
	}{
		{name: "GIT_AUTHOR_NAME", authorName: "Env Author", userName: "Config User", want: "Env Author"},
		{name: "git config user.name", userName: "Config User", want: "Config User"},
		{name: "USER", want: "login-name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newWriteStore(t)
			if tc.userName != "" {
				command := exec.Command("git", "config", "user.name", tc.userName)
				command.Dir = root
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("git config: %v\n%s", err, output)
				}
			}
			env := map[string]string{"GIT_AUTHOR_NAME": tc.authorName, "USER": "login-name"}

			_, stderr, exitCode := runCLIWith(t, root, "", env, "log", "login-flow", "noted")

			wantLine := "- " + today() + " " + tc.want + ": noted\n"
			if content := readStoreFile(t, root, "login-flow.md"); exitCode != ExitOK || !strings.HasSuffix(content, wantLine) {
				t.Errorf("exit = %d stderr = %q content = %q, want line %q", exitCode, stderr, content, wantLine)
			}
		})
	}
}

func TestCreate_UnrelatedBadFileName_WritesAndWarnsOnce(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{"Bad_Name.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "good-one.md")
	if count := strings.Count(stdout+stderr, ".capcom/issues/Bad_Name.md"); count != 1 {
		t.Errorf("Bad_Name.md named %d times, want once\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
}

func TestCreate_KV_BesideUnparseableFile_PrintsWarningBeforeRecord(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{"broken-one.md": brokenFrontmatter})

	stdout, stderr, exitCode := runCLIWith(t, root, "Body.\n", nil,
		"create", "--kv", "--id", "good-one", "--title", "Good one", "--body-file", "-")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	assertKVWarningFirst(t, stdout, "broken-one.md", "good-one")
}

func TestCreate_BesideConflictedFrontmatter_WarnsNamingTheMarker(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"merged-one.md": "---\n<<<<<<< HEAD\ntitle: Mine\n=======\ntitle: Theirs\n>>>>>>> other-branch\n" +
			"status: open\ntype: task\n---\n",
	})

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	want := `warning: .capcom/issues/merged-one.md:2: git conflict marker "<<<<<<< HEAD"; ` +
		"resolve the conflict and delete the markers\n"
	if !strings.Contains(stdout, want) || strings.Contains(stdout+stderr, "invalid YAML") {
		t.Errorf("stdout = %q\nstderr = %q\nwant the warning %q and no YAML error", stdout, stderr, want)
	}
}

// withConflictAroundTitle returns content, an issue file opening with its title line, with that
// line inside a git conflict hunk as a merge leaves it, so the file's frontmatter does not parse.
func withConflictAroundTitle(content string) string {
	title, rest, _ := strings.Cut(strings.TrimPrefix(content, "---\n"), "\n")
	return "---\n<<<<<<< HEAD\n" + title + "\n=======\ntitle: Theirs\n>>>>>>> other-branch\n" + rest
}

// breakIssueFile puts the title line of the issue file at a slash path below .capcom/issues/ inside
// a git conflict hunk, so the file does not parse.
func breakIssueFile(t *testing.T, root string, name string) {
	t.Helper()
	writeIssueFiles(t, root, map[string]string{name: withConflictAroundTitle(readStoreFile(t, root, name))})
}

func TestCreate_AliasOfUnparseableIssue_FailsExistsNamingTheFile(t *testing.T) {
	root := newWriteStore(t)
	mustRun(t, root, "create", "--id", "first-issue", "--title", "First")
	mustRun(t, root, "mv", "first-issue", "moved-issue")
	breakIssueFile(t, root, "moved-issue.md")

	stdout, _, exitCode := runCLI(t, root, "create", "--json", "--id", "first-issue", "--title", "Duplicate")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	wantMessage := `id "first-issue" may be an alias of issue moved-issue, whose file does not parse: fix it first`
	if exitCode != ExitError || failure["code"] != string(CodeExists) ||
		failure["path"] != ".capcom/issues/moved-issue.md" || failure["message"] != wantMessage {
		t.Errorf("exit = %d error = %v, want exists at moved-issue.md with %q", exitCode, failure, wantMessage)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "first-issue.md")); err == nil {
		t.Error("first-issue.md was written although an unparseable file lists it as an alias")
	}
}

func TestCreate_AliasOfUnparseableIssueInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"lost-dir/deeper-dir/other-id.md": withConflictAroundTitle(
			readIssue("Lost", "open", "aliases:\n  - dup-id\n", ""),
		),
	})

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "dup-id", "--title", "Duplicate")

	want := `.capcom/issues/lost-dir/deeper-dir/other-id.md: id "dup-id" may be an alias of issue other-id, ` +
		"whose file does not parse: fix it first"
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "dup-id.md")); err == nil {
		t.Error("dup-id.md was written although an unparseable orphaned file lists it as an alias")
	}
}

func TestCreate_BesideUnparseableIssueWithOtherAlias_WritesAndWarnsOnce(t *testing.T) {
	root := newWriteStore(t)
	breakIssueFile(t, root, "renamed-issue.md")

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "good-one.md")
	assertWarnedOnce(t, stdout+stderr, "warning: .capcom/issues/renamed-issue.md:")
}

func TestCreate_UnderParentInOrphanDirectory_Fails(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{"lost-dir/parent-one.md": validIssue})

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "child-one", "--title", "Child", "--parent", "parent-one")

	if exitCode == ExitOK {
		t.Errorf("exit = %d stderr = %q, want a failure", exitCode, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "lost-dir", "parent-one", "child-one.md")); err == nil {
		t.Error("child-one.md was written below the orphaned parent")
	}
}

func TestCreate_BesideOrphanChildrenDirectory_Fails(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{"foo-bar/stray-child.md": validIssue})

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "foo-bar", "--title", "Foo bar")

	want := ".capcom/issues/foo-bar: directory without its node file"
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "foo-bar.md")); err == nil {
		t.Error("foo-bar.md was written beside its orphan directory")
	}
}

func TestCreate_IDInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{name: "directly inside", file: "lost-dir/dup-id.md"},
		{name: "deeper inside", file: "lost-dir/deeper-dir/dup-id.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newWriteStore(t)
			writeIssueFiles(t, root, map[string]string{tc.file: validIssue})

			_, stderr, exitCode := runCLI(t, root, "create", "--id", "dup-id", "--title", "Duplicate")

			want := ".capcom/issues/" + tc.file + `: id "dup-id" is already taken by this file`
			if exitCode != ExitError || !strings.Contains(stderr, want) {
				t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
			}
			if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "dup-id.md")); err == nil {
				t.Error("dup-id.md was written beside the orphaned file of the same id")
			}
		})
	}
}

func TestCreate_IDInsideInvalidIDDirectory_FailsNamingTheFile(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"Bad_Name.md":             validIssue,
		"Bad_Name/child-issue.md": validIssue,
	})

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "child-issue", "--title", "Child")

	want := `.capcom/issues/Bad_Name/child-issue.md: id "child-issue" is already taken by this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "child-issue.md")); err == nil {
		t.Error("child-issue.md was written beside the left-out file of the same id")
	}
}

func TestCreate_UnrelatedOrphanDirectory_WritesAndWarns(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{"lost-dir/other-id.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "good-one.md")
	want := ".capcom/issues/lost-dir: directory without its node file"
	if count := strings.Count(stdout+stderr, want); count != 1 {
		t.Errorf("%q warned %d times, want once\nstdout = %q\nstderr = %q", want, count, stdout, stderr)
	}
}

// assertWarnedOnce fails the test unless each of wants appears exactly once in output.
func assertWarnedOnce(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if count := strings.Count(output, want); count != 1 {
			t.Errorf("%q warned %d times, want once\noutput = %q", want, count, output)
		}
	}
}

func TestCreate_IndexWarnedFileWithDuplicateID_WarnsAboutBoth(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"dup-id.md":           validIssue,
		"zz-parent.md":        validIssue,
		"zz-parent/dup-id.md": readIssue("Bad status", "bogus", "", ""),
	})

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "good-one.md")
	assertWarnedOnce(
		t,
		stdout+stderr,
		`status "bogus" is not declared`,
		`.capcom/issues/zz-parent/dup-id.md: duplicate id "dup-id"`,
	)
}

func TestCreate_IndexWarnedFileWithDuplicateKeyAndID_WarnsAboutBoth(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"dup-id.md":           validIssue,
		"zz-parent.md":        validIssue,
		"zz-parent/dup-id.md": readIssue("Duplicate key", "open", "type: task\n", ""),
	})

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "good-one.md")
	assertWarnedOnce(
		t,
		stdout+stderr,
		`.capcom/issues/zz-parent/dup-id.md:6: duplicate key "type"`,
		`.capcom/issues/zz-parent/dup-id.md: duplicate id "dup-id"`,
	)
}

func TestCreate_IndexWarnedFileDuplicatedElsewhere_WarnsOnce(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"dup-id.md":           validIssue,
		"zz-parent.md":        validIssue,
		"zz-parent/dup-id.md": validIssue,
	})

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "good-one.md")
	assertWarnedOnce(
		t,
		stdout+stderr,
		`.capcom/issues/dup-id.md: duplicate id "dup-id"`,
		`.capcom/issues/zz-parent/dup-id.md: duplicate id "dup-id"`,
	)
}

func TestCreate_IndexWarnedFileSelfAlias_WarnsOnce(t *testing.T) {
	cases := []struct {
		name    string
		aliases string
		want    string
	}{
		{
			name:    "alias repeats its own id",
			aliases: "aliases: [self-alias]\n",
			want:    `.capcom/issues/self-alias.md:6: alias "self-alias" repeats the issue's own id`,
		},
		{
			name:    "alias listed twice",
			aliases: "aliases: [old-name, old-name]\n",
			want:    `.capcom/issues/self-alias.md:6: alias "old-name" is listed twice`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newWriteStore(t)
			writeIssueFiles(t, root, map[string]string{
				"self-alias.md": readIssue("Self alias", "open", tc.aliases, ""),
			})

			stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

			if exitCode != ExitOK {
				t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
			}
			readStoreFile(t, root, "good-one.md")
			assertWarnedOnce(t, stdout+stderr, ".capcom/issues/self-alias.md", tc.want)

			// check reports the same alias problem exactly as the index warns it.
			checkOut, checkErr, checkCode := runCLI(t, root, "check")
			if checkCode != ExitError || checkOut != tc.want+"\n" || checkErr != "" {
				t.Errorf("check = %d, stdout %q, stderr %q; want 1 and %q", checkCode, checkOut, checkErr, tc.want)
			}
		})
	}
}

func TestCreate_AliasInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"lost-dir/deeper-dir/other-id.md": readIssue("Lost", "open", "aliases: [dup-id]\n", ""),
	})

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "dup-id", "--title", "Duplicate")

	want := `.capcom/issues/lost-dir/deeper-dir/other-id.md: id "dup-id" is already an alias of this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "dup-id.md")); err == nil {
		t.Error("dup-id.md was written although an orphaned file has it as an alias")
	}
}

func TestCreate_AliasOfDroppedDuplicateIDFile_FailsNamingTheFile(t *testing.T) {
	root := newWriteStore(t)
	files := map[string]string{
		"foo.md":           readIssue("Foo", "open", "", ""),
		"sub-issue.md":     readIssue("Sub issue", "open", "", ""),
		"sub-issue/foo.md": readIssue("Foo too", "open", "aliases: [old-bar]\n", ""),
	}
	writeIssueFiles(t, root, files)

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "old-bar", "--title", "Old bar")

	want := `.capcom/issues/sub-issue/foo.md: id "old-bar" is already an alias of this file, left out for the duplicate id "foo"`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "old-bar.md")); err == nil {
		t.Error("old-bar.md was written although a file left out for a duplicate id has it as an alias")
	}
	assertIssueFilesUnchanged(t, root, files)
}

func TestCreate_OrphanDirectoryWithOtherAlias_WritesAndWarns(t *testing.T) {
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"lost-dir/other-id.md": readIssue("Lost", "open", "aliases: [other-alias]\n", ""),
	})

	stdout, stderr, exitCode := runCLI(t, root, "create", "--id", "good-one", "--title", "Good one")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	readStoreFile(t, root, "good-one.md")
	if want := ".capcom/issues/lost-dir: directory without its node file"; !strings.Contains(stdout+stderr, want) {
		t.Errorf("stdout = %q stderr = %q, want a warning containing %q", stdout, stderr, want)
	}
}

func TestUpdate_DuplicatedID_FailsNamingBothFilesAndLeavesThePrefixMatchUnchanged(t *testing.T) {
	root, files := newDuplicateIssueStore(t)

	_, stderr, exitCode := runCLI(t, root, "update", "foo", "--title", "y")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/foo.md", ".capcom/issues/sub-issue/foo.md")
	assertIssueFilesUnchanged(t, root, files)
}

func TestCreate_ParentOnlyAnUnusableFileClaims_IsRefusedNamingIt(t *testing.T) {
	for _, fixture := range unusableClaimStores {
		t.Run(fixture.name, func(t *testing.T) {
			root, files := newUnusableClaimStore(t, fixture)

			_, stderr, exitCode := runCLI(t, root, "create", "--id", "new-child", "--title", "New", "--parent", "foo")

			assertUnusableClaimRefusal(t, root, fixture, files, stderr, exitCode)
			if issueFileExists(t, root, "foo-bar/new-child.md") || issueFileExists(t, root, "new-child.md") {
				t.Error("new-child.md was written")
			}
		})
	}
}

// standingLine is the frontmatter line of an issue marked standing.
const standingLine = "\nstanding: true\n"

// standingNotOpenText is the refusal of marking standing an issue whose status, the format's
// argument, is closed or in the doing category, as update and create word it.
const standingNotOpenText = "status %q is in the %s category; " +
	"only an issue that is neither closed nor in the doing category can be marked standing\n"

// standingRefusalText is the refusal of claiming or closing a standing issue, as close and update
// word it on stderr; its arguments are the issue's id, for its file and for the update to run.
const standingRefusalText = "capcom: .capcom/issues/%s.md: the issue is standing: it only groups its children " +
	"and is never claimed or closed; run capcom update %s --no-standing first\n"

// newStandingStore is newWriteStore with feature-requests, an open issue marked standing, and
// busy-work, an issue in the doing category.
func newStandingStore(t *testing.T) string {
	t.Helper()
	root := newWriteStore(t)
	writeIssueFiles(t, root, map[string]string{
		"feature-requests.md": readIssue("Feature requests", "open", "standing: true\n", ""),
		"busy-work.md":        readIssue("Busy work", "active", "", ""),
	})
	return root
}

// shownStanding runs show --json on id and returns its standing value, nil when left out.
func shownStanding(t *testing.T, root string, id string) any {
	t.Helper()
	shown := decodeEnvelope(t, mustRun(t, root, "show", "--json", id))["data"].([]any)[0].(map[string]any)
	return shown["standing"]
}

func TestCreate_Standing_WritesTheMarkThatShowJSONCarries(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "create", "--id", "feature-requests", "--title", "Feature requests", "--standing")

	content := readStoreFile(t, root, "feature-requests.md")
	if !strings.Contains(content, "priority: 2"+standingLine+"created: ") {
		t.Errorf("content = %q, want the standing line after priority", content)
	}
	if standing := shownStanding(t, root, "feature-requests"); standing != true {
		t.Errorf("show --json standing = %v, want true", standing)
	}
}

func TestCreate_StandingInTheDoingCategory_IsRefusedAndWritesNothing(t *testing.T) {
	root := newWriteStore(t)

	_, stderr, exitCode := runCLI(t, root, "create", "--id", "feature-requests", "--title", "Feature requests",
		"--standing", "--status", "active")

	want := "capcom: " + fmt.Sprintf(standingNotOpenText, "active", "doing") + "hint: capcom create --help\n"
	_, statErr := os.Stat(filepath.Join(root, ".capcom", "issues", "feature-requests.md"))
	if exitCode != ExitError || stderr != want || !os.IsNotExist(statErr) {
		t.Errorf("exit = %d stat = %v\nstderr = %q\nwant     %q", exitCode, statErr, stderr, want)
	}
}

func TestCreate_StandingBelowTheStandingFormat_RaisesTheFormatWithAWarning(t *testing.T) {
	root := newWriteStore(t)
	writeStoreConfig(t, root, "format: 4\n")

	stdout := mustRun(t, root, "create", "--id", "feature-requests", "--title", "Feature requests", "--standing")

	want := "warning: " + fmt.Sprintf(formatRaisedWarning, ".capcom/config.yaml", 5, 4) + "\n"
	if config := readStoreConfig(t, root); config != "format: 5\n" || strings.Count(stdout, want) != 1 {
		t.Errorf("config.yaml = %q\nstdout = %q\nwant the warning %q once", config, stdout, want)
	}
}

func TestUpdate_StandingThenNoStanding_SetsAndClearsTheMark(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "update", "login-flow", "--standing")
	marked := readStoreFile(t, root, "login-flow.md")
	mustRun(t, root, "update", "login-flow", "--no-standing")
	cleared := readStoreFile(t, root, "login-flow.md")

	if !strings.Contains(marked, standingLine) || strings.Contains(cleared, "standing:") {
		t.Errorf("marked = %q\ncleared = %q", marked, cleared)
	}
}

func TestUpdate_NoStandingFalse_LeavesTheMarkAlone(t *testing.T) {
	root := newStandingStore(t)

	mustRun(t, root, "update", "login-flow", "--no-standing=false")
	mustRun(t, root, "update", "feature-requests", "--no-standing=false")

	unmarked := readStoreFile(t, root, "login-flow.md")
	marked := readStoreFile(t, root, "feature-requests.md")
	if strings.Contains(unmarked, "standing:") || !strings.Contains(marked, standingLine) {
		t.Errorf("unmarked = %q\nmarked = %q, want both marks unchanged", unmarked, marked)
	}
}

func TestUpdate_StandingAndNoStandingTogether_ExitsUsage(t *testing.T) {
	root := newStandingStore(t)
	before := readStoreFile(t, root, "feature-requests.md")

	_, stderr, exitCode := runCLI(t, root, "update", "feature-requests", "--standing", "--no-standing")

	if exitCode != ExitUsage || !strings.Contains(stderr, "no-standing") ||
		readStoreFile(t, root, "feature-requests.md") != before {
		t.Errorf("exit = %d stderr = %q, want a usage failure that writes nothing", exitCode, stderr)
	}
}

func TestUpdate_StandingOnAClosedOrDoingIssue_IsRefused(t *testing.T) {
	root := newStandingStore(t)
	cases := map[string]struct {
		id       string
		status   string
		category string
	}{
		"closed": {id: "shipped-fix", status: "done", category: "done"},
		"doing":  {id: "busy-work", status: "active", category: "doing"},
	}
	for name, tc := range cases {
		before := readStoreFile(t, root, tc.id+".md")

		_, stderr, exitCode := runCLI(t, root, "update", tc.id, "--standing")

		want := "capcom: .capcom/issues/" + tc.id + ".md: " + fmt.Sprintf(standingNotOpenText, tc.status, tc.category) +
			"hint: capcom update --help\n"
		if exitCode != ExitError || stderr != want || readStoreFile(t, root, tc.id+".md") != before {
			t.Errorf("%s: exit = %d\nstderr = %q\nwant     %q", name, exitCode, stderr, want)
		}
	}
}

func TestUpdate_StandingWithADoingStatus_IsRefused(t *testing.T) {
	root := newWriteStore(t)
	before := readStoreFile(t, root, "login-flow.md")

	_, _, exitCode := runCLI(t, root, "update", "login-flow", "--standing", "--status", "active")

	if after := readStoreFile(t, root, "login-flow.md"); exitCode != ExitError || after != before {
		t.Errorf("exit = %d content = %q, want a refusal that writes nothing", exitCode, after)
	}
}

func TestUpdate_NoStandingWithStatusActive_ClaimsTheIssueInOneCall(t *testing.T) {
	root := newStandingStore(t)

	mustRun(t, root, "update", "feature-requests", "--no-standing", "--status", "active")

	content := readStoreFile(t, root, "feature-requests.md")
	if !strings.Contains(content, "\nstatus: active\n") || strings.Contains(content, "standing:") {
		t.Errorf("content = %q, want it claimed and unmarked", content)
	}
}

func TestUpdate_StatusActiveOnAStandingIssue_IsRefusedNamingTheMark(t *testing.T) {
	root := newStandingStore(t)
	before := readStoreFile(t, root, "feature-requests.md")

	_, stderr, exitCode := runCLI(t, root, "update", "feature-requests", "--status", "active")

	want := fmt.Sprintf(standingRefusalText, "feature-requests", "feature-requests") + "hint: capcom update --help\n"
	if exitCode != ExitError || stderr != want || readStoreFile(t, root, "feature-requests.md") != before {
		t.Errorf("exit = %d\nstderr = %q\nwant     %q", exitCode, stderr, want)
	}
}

// The valid values the default config declares, as an invalid flag value lists them.
var (
	declaredStatuses = []string{"open (todo)", "active (doing)", "parked (waiting)", "done (done)", "dropped (dropped)"}
	declaredTypes    = []string{"bug", "feature", "task", "chore"}
	validPriorities  = []string{"0", "1", "2", "3", "4"}
)

// envelopeValid returns the valid list of the --json error envelope in stdout.
func envelopeValid(t *testing.T, stdout string) []string {
	t.Helper()
	body, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	values, _ := body["valid"].([]any)
	valid := make([]string, 0, len(values))
	for _, value := range values {
		text, _ := value.(string)
		valid = append(valid, text)
	}
	return valid
}

func TestCreateUpdateCloseReopen_InvalidFlagValue_ListsTheValidValuesAndWritesNothing(t *testing.T) {
	tests := map[string]struct {
		args        []string
		wantMessage string
		wantValid   []string
	}{
		"create status": {
			args:        []string{"create", "--id", "new-one", "--title", "New", "--status", "doing"},
			wantMessage: `--status "doing": not a status declared in config.yaml`,
			wantValid:   declaredStatuses,
		},
		"create type": {
			args:        []string{"create", "--id", "new-one", "--title", "New", "--type", "bugg"},
			wantMessage: `--type "bugg": not a type declared in config.yaml`,
			wantValid:   declaredTypes,
		},
		"create priority": {
			args:        []string{"create", "--id", "new-one", "--title", "New", "--priority", "9"},
			wantMessage: "--priority 9: out of range",
			wantValid:   validPriorities,
		},
		"update status": {
			args:        []string{"update", "login-flow", "--status", "doing"},
			wantMessage: `--status "doing": not a status declared in config.yaml`,
			wantValid:   declaredStatuses,
		},
		"update type": {
			args:        []string{"update", "login-flow", "--type", "bugg"},
			wantMessage: `--type "bugg": not a type declared in config.yaml`,
			wantValid:   declaredTypes,
		},
		"update priority": {
			args:        []string{"update", "login-flow", "--priority", "-1"},
			wantMessage: "--priority -1: out of range",
			wantValid:   validPriorities,
		},
		"close status": {
			args:        []string{"close", "login-flow", "--reason", "shipped", "--status", "finished"},
			wantMessage: `--status "finished": not a status declared in config.yaml`,
			wantValid:   declaredStatuses,
		},
		"reopen status": {
			args:        []string{"reopen", "shipped-fix", "--reason", "regressed", "--status", "doing"},
			wantMessage: `--status "doing": not a status declared in config.yaml`,
			wantValid:   declaredStatuses,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			root := newWriteStore(t)
			loginBefore := readStoreFile(t, root, "login-flow.md")
			shippedBefore := readStoreFile(t, root, "shipped-fix.md")

			stdout, stderr, exitCode := runCLI(t, root, test.args...)
			jsonOut, _, jsonExitCode := runCLI(t, root, append(test.args, "--json")...)

			wantStderr := "capcom: " + test.wantMessage + "; use one of: " + strings.Join(test.wantValid, ", ") +
				"\nhint: capcom " + test.args[0] + " --help\n"
			if exitCode != ExitError || stdout != "" || stderr != wantStderr {
				t.Errorf("exit = %d stdout = %q stderr = %q\nwant exit %d and stderr %q",
					exitCode, stdout, stderr, ExitError, wantStderr)
			}
			if valid := envelopeValid(t, jsonOut); jsonExitCode != ExitError || !slices.Equal(valid, test.wantValid) {
				t.Errorf("--json exit = %d valid = %q, want %d and %q", jsonExitCode, valid, ExitError, test.wantValid)
			}
			if !strings.Contains(jsonOut, `"code":"invalid"`) {
				t.Errorf("--json = %s, want code invalid", jsonOut)
			}
			if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "new-one.md")); !os.IsNotExist(err) {
				t.Errorf("new-one.md exists (err = %v), want nothing created", err)
			}
			if readStoreFile(t, root, "login-flow.md") != loginBefore ||
				readStoreFile(t, root, "shipped-fix.md") != shippedBefore {
				t.Error("an issue file changed, want nothing written")
			}
		})
	}
}

func TestUpdate_InvalidValueAndUnknownID_RefusesTheValueFirst(t *testing.T) {
	root := newWriteStore(t)

	_, stderr, exitCode := runCLI(t, root, "update", "absent", "--type", "bugg")

	if exitCode != ExitError || !strings.Contains(stderr, `--type "bugg"`) {
		t.Errorf("exit = %d stderr = %q, want the --type refusal", exitCode, stderr)
	}
}

func TestCreateCloseReopen_DeclaredFlagValues_WriteAsBefore(t *testing.T) {
	root := newWriteStore(t)

	mustRun(t, root, "create", "--id", "new-one", "--title", "New", "--status", "active", "--type", "bug",
		"--priority", "4")
	mustRun(t, root, "close", "new-one", "--reason", "done", "--status", "dropped")
	mustRun(t, root, "reopen", "new-one", "--reason", "back", "--status", "parked")

	content := readStoreFile(t, root, "new-one.md")
	if !strings.Contains(content, "status: parked\ntype: bug\npriority: 4\n") {
		t.Errorf("new-one.md = %q, want status parked, type bug, priority 4", content)
	}
}
