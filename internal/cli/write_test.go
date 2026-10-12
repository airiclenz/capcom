package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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

	for _, id := range []string{"single", "Upper-Case", "a-b-c-d-e", "under_score-x"} {
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

	if missingExit != ExitUsage || closedExit != ExitError || !strings.Contains(stderr, "close") {
		t.Errorf("missing id exit = %d, closed status exit = %d stderr = %q", missingExit, closedExit, stderr)
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

func TestUpdate_StatusIntoOrOutOfClosed_IsRefused(t *testing.T) {
	root := newWriteStore(t)

	cases := map[string][]string{
		"into done":    {"update", "login-flow", "--status", "done"},
		"out of done":  {"update", "shipped-fix", "--status", "open"},
		"done to drop": {"update", "shipped-fix", "--status", "dropped"},
	}
	for name, args := range cases {
		_, stderr, exitCode := runCLI(t, root, args...)

		if exitCode != ExitError || !strings.Contains(stderr, ".capcom/issues/") {
			t.Errorf("%s: exit = %d stderr = %q", name, exitCode, stderr)
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

	stdout, stderr, exitCode := runCLI(t, root, "create", "--kv", "--id", "good-one", "--title", "Good one")

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
			want:    `.capcom/issues/self-alias.md: alias "self-alias" repeats the issue's own id`,
		},
		{
			name:    "alias listed twice",
			aliases: "aliases: [old-name, old-name]\n",
			want:    `.capcom/issues/self-alias.md: alias "old-name" is listed twice`,
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
