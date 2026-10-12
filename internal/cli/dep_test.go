package cli

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// newDepStore creates a store with a chain build-pipeline -> db-schema (written by its alias)
// and two free issues.
func newDepStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"build-pipeline.md": readIssue("Build pipeline", "open", "deps: [apogee-kk0.4]\n", ""),
		"db-schema.md":      readIssue("Schema", "open", "aliases: [apogee-kk0.4]\n", ""),
		"login-flow.md":     readIssue("Login flow", "open", "", ""),
		"release-notes.md":  readIssue("Release notes", "done", "", ""),
	})
}

func TestDepAdd_ThroughAlias_StoresTheCanonicalID(t *testing.T) {
	root := newDepStore(t)

	mustRun(t, root, "dep", "add", "login-flow", "apogee-kk0.4")

	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "deps: [db-schema]\n") {
		t.Errorf("login-flow.md = %q", content)
	}
}

func TestDepAdd_AlreadyThereThroughAlias_LeavesTheDepsAsTheyAre(t *testing.T) {
	root := newDepStore(t)

	mustRun(t, root, "dep", "add", "build-pipeline", "db-schema")

	if content := readStoreFile(t, root, "build-pipeline.md"); !strings.Contains(content, "deps: [apogee-kk0.4]\n") {
		t.Errorf("build-pipeline.md = %q", content)
	}
}

func TestDepAdd_SelfDependency_FailsInvalid(t *testing.T) {
	root := newDepStore(t)

	_, stderr, exitCode := runCLI(t, root, "dep", "add", "db-schema", "apogee-kk0.4")

	if exitCode != ExitError || !strings.Contains(stderr, "db-schema cannot depend on itself") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
}

func TestDepAdd_CycleOfLengthThree_FailsNamingTheCycle(t *testing.T) {
	root := newDepStore(t)
	mustRun(t, root, "dep", "add", "db-schema", "login-flow")
	before := readStoreFile(t, root, "login-flow.md")

	stdout, _, exitCode := runCLI(t, root, "dep", "add", "--json", "login-flow", "build-pipeline")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	wantCycle := "login-flow -> build-pipeline -> db-schema -> login-flow"
	if exitCode != ExitError || failure["code"] != string(CodeInvalid) ||
		!strings.Contains(failure["message"].(string), wantCycle) ||
		failure["path"] != ".capcom/issues/login-flow.md" {
		t.Errorf("exit = %d error = %v, want invalid naming %q", exitCode, failure, wantCycle)
	}
	if after := readStoreFile(t, root, "login-flow.md"); after != before {
		t.Errorf("login-flow.md changed:\n%s", after)
	}
}

func TestDepList_Issue_ShowsDepsAndDependents(t *testing.T) {
	root := newDepStore(t)
	mustRun(t, root, "dep", "add", "db-schema", "release-notes")
	mustRun(t, root, "dep", "add", "login-flow", "db-schema")
	writeIssueFiles(t, root, map[string]string{
		"db-schema.md": readIssue("Schema", "open", "deps: [release-notes, gone-issue]\naliases: [apogee-kk0.4]\n", ""),
	})

	stdout := mustRun(t, root, "dep", "list", "apogee-kk0.4")

	want := "deps:\n" +
		"  release-notes  done  P2  Release notes\n" +
		"  gone-issue  missing\n" +
		"dependents:\n" +
		"  build-pipeline  open  P2  Build pipeline\n" +
		"  login-flow  open  P2  Login flow\n"
	if stdout != want {
		t.Errorf("stdout = %q\nwant     %q", stdout, want)
	}
}

// newDepChainStore creates a store with the chain chain-a -> chain-b -> chain-c -> chain-d.
func newDepChainStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"chain-a.md": readIssue("A", "open", "deps: [chain-b]\n", ""),
		"chain-b.md": readIssue("B", "open", "deps: [chain-c]\n", ""),
		"chain-c.md": readIssue("C", "open", "deps: [chain-d]\n", ""),
		"chain-d.md": readIssue("D", "open", "", ""),
	})
}

func TestDepList_Depth_WalksThatManyLevels(t *testing.T) {
	root := newDepChainStore(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "deps at depth 2 hint the cut level",
			args: []string{"chain-a", "--depth", "2"},
			want: "deps:\n" +
				"  chain-b  open  P2  B\n" +
				"    chain-c  open  P2  C\n" +
				"      ... 1 more (--depth 3)\n" +
				"dependents:\n",
		},
		{
			name: "deps at depth 0 show every level",
			args: []string{"chain-a", "--depth", "0"},
			want: "deps:\n" +
				"  chain-b  open  P2  B\n" +
				"    chain-c  open  P2  C\n" +
				"      chain-d  open  P2  D\n" +
				"dependents:\n",
		},
		{
			name: "dependents at depth 2 hint the cut level",
			args: []string{"chain-d", "--depth", "2"},
			want: "deps:\n" +
				"dependents:\n" +
				"  chain-c  open  P2  C\n" +
				"    chain-b  open  P2  B\n" +
				"      ... 1 more (--depth 3)\n",
		},
		{
			name: "explicit depth 1 hints the cut level",
			args: []string{"chain-b", "--depth", "1"},
			want: "deps:\n" +
				"  chain-c  open  P2  C\n" +
				"    ... 1 more (--depth 2)\n" +
				"dependents:\n" +
				"  chain-a  open  P2  A\n",
		},
		{
			name: "default prints the direct deps and dependents alone",
			args: []string{"chain-b"},
			want: "deps:\n" +
				"  chain-c  open  P2  C\n" +
				"dependents:\n" +
				"  chain-a  open  P2  A\n",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stdout := mustRun(t, root, append([]string{"dep", "list"}, testCase.args...)...)

			if stdout != testCase.want {
				t.Errorf("stdout = %q\nwant     %q", stdout, testCase.want)
			}
		})
	}
}

func TestDepList_Diamond_PrintsTheSharedIssueOnceThenSeeAbove(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"top.md":   readIssue("Top", "open", "deps: [left, right]\n", ""),
		"left.md":  readIssue("Left", "open", "deps: [base]\n", ""),
		"right.md": readIssue("Right", "open", "deps: [base]\n", ""),
		"base.md":  readIssue("Base", "open", "", ""),
	})

	stdout := mustRun(t, root, "dep", "list", "top", "--depth", "0")
	jsonOut := mustRun(t, root, "dep", "list", "--json", "top", "--depth", "0")

	want := "deps:\n" +
		"  left  open  P2  Left\n" +
		"    base  open  P2  Base\n" +
		"  right  open  P2  Right\n" +
		"    base  (see above)\n" +
		"dependents:\n"
	if stdout != want {
		t.Errorf("stdout = %q\nwant     %q", stdout, want)
	}
	data, _ := decodeEnvelope(t, jsonOut)["data"].(map[string]any)
	deps, _ := data["deps"].([]any)
	var got []string
	for _, item := range deps {
		fields := item.(map[string]any)
		got = append(got, fmt.Sprintf("%v@%v<%v", fields["id"], fields["depth"], fields["via"]))
	}
	wantItems := []string{"left@1<", "base@2<left", "right@1<", "base@2<right"}
	if !slices.Equal(got, wantItems) {
		t.Errorf("deps items = %v, want %v", got, wantItems)
	}
}

func TestDepList_NegativeDepth_FailsUsage(t *testing.T) {
	root := newDepChainStore(t)

	_, stderr, exitCode := runCLI(t, root, "dep", "list", "chain-a", "--depth", "-1")

	if exitCode != ExitUsage || !strings.Contains(stderr, "--depth must be 0 (every level) or more") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
}

func TestDepList_MissingDepBeforeResolved_PrintsInFileOrder(t *testing.T) {
	root := newDepChainStore(t)
	writeIssueFiles(t, root, map[string]string{
		"chain-b.md": readIssue("B", "open", "deps: [gone-issue, chain-c]\n", ""),
	})

	stdout := mustRun(t, root, "dep", "list", "chain-b", "--depth", "2")

	want := "deps:\n" +
		"  gone-issue  missing\n" +
		"  chain-c  open  P2  C\n" +
		"    chain-d  open  P2  D\n" +
		"dependents:\n" +
		"  chain-a  open  P2  A\n"
	if stdout != want {
		t.Errorf("stdout = %q\nwant     %q", stdout, want)
	}
}

func TestDepList_DuplicateAndSelfDep_PrintVerbatimAtDepthOne(t *testing.T) {
	root := newDepChainStore(t)
	writeIssueFiles(t, root, map[string]string{
		"chain-b.md": readIssue("B", "open", "deps: [chain-c, chain-b, chain-c]\n", ""),
	})

	stdout := mustRun(t, root, "dep", "list", "chain-b", "--depth", "0")

	want := "deps:\n" +
		"  chain-c  open  P2  C\n" +
		"    chain-d  open  P2  D\n" +
		"  chain-b  open  P2  B\n" +
		"  chain-c  open  P2  C\n" +
		"dependents:\n" +
		"  chain-a  open  P2  A\n" +
		"  chain-b  open  P2  B\n"
	if stdout != want {
		t.Errorf("stdout = %q\nwant     %q", stdout, want)
	}
}

func TestDepList_OtherCommandsJSON_CarryNoDepthOrVia(t *testing.T) {
	root := newDepChainStore(t)

	listOut := mustRun(t, root, "list", "--json")
	showOut := mustRun(t, root, "show", "--json", "chain-a")

	listed, _ := decodeEnvelope(t, listOut)["data"].([]any)
	shown, _ := decodeEnvelope(t, showOut)["data"].(map[string]any)
	items := []map[string]any{shown}
	for _, item := range listed {
		items = append(items, item.(map[string]any))
	}
	if len(items) != 5 {
		t.Fatalf("got %d items, want 4 listed and 1 shown", len(items))
	}
	for _, item := range items {
		if _, hasDepth := item["depth"]; hasDepth {
			t.Errorf("%v carries depth", item["id"])
		}
		if _, hasVia := item["via"]; hasVia {
			t.Errorf("%v carries via", item["id"])
		}
	}
}

func TestDepRm_DepWrittenAsAlias_RemovesIt(t *testing.T) {
	root := newDepStore(t)

	mustRun(t, root, "dep", "rm", "build-pipeline", "db-schema")

	if content := readStoreFile(t, root, "build-pipeline.md"); strings.Contains(content, "deps:") {
		t.Errorf("build-pipeline.md = %q", content)
	}
}

func TestDepRm_NotADep_FailsNotFound(t *testing.T) {
	root := newDepStore(t)

	_, stderr, exitCode := runCLI(t, root, "dep", "rm", "login-flow", "db-schema")

	if exitCode != ExitNotFound || !strings.Contains(stderr, "login-flow does not depend on db-schema") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
}

func TestDepRm_DanglingDep_RemovesItByItsText(t *testing.T) {
	root := newDepStore(t)
	writeIssueFiles(t, root, map[string]string{
		"login-flow.md": readIssue("Login flow", "open", "deps: [gone-issue, db-schema]\n", ""),
	})

	mustRun(t, root, "dep", "rm", "login-flow", "gone-issue")

	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "deps: [db-schema]\n") {
		t.Errorf("login-flow.md = %q", content)
	}
}

func TestDepRm_IDOfInvalidFile_RemovesTheExactEntry(t *testing.T) {
	root, _ := newInvalidFooStore(t)
	writeIssueFiles(t, root, map[string]string{
		"dependent.md": readIssue("Dependent", "open", "deps: [foo, foo-bar]\n", ""),
	})

	mustRun(t, root, "dep", "rm", "dependent", "foo")

	if content := readStoreFile(t, root, "dependent.md"); !strings.Contains(content, "deps: [foo-bar]\n") {
		t.Errorf("dependent.md = %q", content)
	}
}

func TestDepRm_DuplicatedDep_FailsNamingBothFilesAndWritesNothing(t *testing.T) {
	root, files := newDuplicateIssueStore(t)
	files["dependent.md"] = readIssue("Dependent", "open", "deps: [foo, foo-bar]\n", "")
	writeIssueFiles(t, root, files)

	_, stderr, exitCode := runCLI(t, root, "dep", "rm", "dependent", "foo")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/foo.md", ".capcom/issues/sub-issue/foo.md")
	assertIssueFilesUnchanged(t, root, files)
}
