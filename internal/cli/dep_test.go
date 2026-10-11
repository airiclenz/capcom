package cli

import (
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
