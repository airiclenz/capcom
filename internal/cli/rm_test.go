package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// newRmStore creates a store with login-flow and a child below it, renamed-issue known also by an
// alias, and session-work depending on login-flow by id.
func newRmStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"login-flow.md":    readIssue("Login flow", "open", "", ""),
		"renamed-issue.md": readIssue("Renamed", "open", "aliases: [former-name]\n", ""),
		"session-work.md":  readIssue("Session work", "open", "deps: [login-flow]\n", ""),
	})
	mustRun(t, root, "create", "--id", "child-issue", "--title", "Child", "--parent", "login-flow")
	return root
}

// issueFileExists reports whether the slash path name exists below .capcom/issues/.
func issueFileExists(t *testing.T, root string, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, ".capcom", "issues", filepath.FromSlash(name)))
	return err == nil
}

// assertCheckPasses fails the test unless capcom check exits ok.
func assertCheckPasses(t *testing.T, root string) {
	t.Helper()
	if stdout, _, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after rm = %d\n%s", exitCode, stdout)
	}
}

func TestRm_Leaf_RemovesTheIssue(t *testing.T) {
	root := newRmStore(t)

	stdout := mustRun(t, root, "rm", "renamed")

	if stdout != "id: renamed-issue\npath: .capcom/issues/renamed-issue.md\n" {
		t.Errorf("rm stdout = %q", stdout)
	}
	if issueFileExists(t, root, "renamed-issue.md") {
		t.Errorf("renamed-issue.md was not removed")
	}
	jsonOutput, _, exitCode := runCLI(t, root, "show", "--json", "former-name")
	if exitCode != ExitNotFound {
		t.Errorf("show of the removed issue's alias = %d\n%s", exitCode, jsonOutput)
	}
	assertCheckPasses(t, root)
}

func TestRm_LastChild_TakesTheEmptyDirectory(t *testing.T) {
	root := newRmStore(t)

	mustRun(t, root, "rm", "child-issue")

	if issueFileExists(t, root, "login-flow") {
		t.Errorf("the emptied login-flow directory was not removed")
	}
	if !issueFileExists(t, root, "login-flow.md") {
		t.Errorf("the parent was removed")
	}
	assertCheckPasses(t, root)
}

func TestRm_WithChildren_IsRefusedNamingTheFileAndCount(t *testing.T) {
	root := newRmStore(t)

	_, stderr, exitCode := runCLI(t, root, "rm", "login-flow", "--drop-deps")

	wantPrefix := "capcom: .capcom/issues/login-flow.md: issue login-flow has 1 child issue"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm with children = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !issueFileExists(t, root, "login-flow.md") || !issueFileExists(t, root, "login-flow/child-issue.md") {
		t.Errorf("files were removed")
	}
}

func TestRm_ChildrenDirectoryWithAStrayFile_IsRefused(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"renamed-issue/notes.txt": "stray"})

	_, stderr, exitCode := runCLI(t, root, "rm", "renamed-issue")

	wantPrefix := "capcom: .capcom/issues/renamed-issue: the issue's children directory still holds 1 entry"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !issueFileExists(t, root, "renamed-issue.md") || !issueFileExists(t, root, "renamed-issue/notes.txt") {
		t.Errorf("files were removed")
	}
}

func TestRm_WithDependent_IsRefusedNamingIt(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")

	_, stderr, exitCode := runCLI(t, root, "rm", "login-flow")

	wantPrefix := "capcom: .capcom/issues/login-flow.md: issue login-flow is a dependency of session-work; pass --drop-deps"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm with a dependent = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !issueFileExists(t, root, "login-flow.md") {
		t.Errorf("login-flow.md was removed")
	}
}

func TestRm_DropDeps_RemovesTheDepsEntries(t *testing.T) {
	cases := []struct {
		name string
		deps string
		want string
	}{
		{name: "by id", deps: "deps: [renamed-issue, login-flow]\n", want: "deps: [login-flow]\n"},
		{name: "by alias", deps: "deps: [former-name]\n", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRmStore(t)
			writeIssueFiles(t, root, map[string]string{"blocked-work.md": readIssue("Blocked", "open", tc.deps, "")})

			stdout := mustRun(t, root, "rm", "--kv", "renamed-issue", "--drop-deps")

			if !strings.Contains(stdout, "DEPS_DROPPED=[blocked-work]\n") {
				t.Errorf("stdout = %q", stdout)
			}
			content := readStoreFile(t, root, "blocked-work.md")
			if strings.Contains(content, "renamed-issue") || strings.Contains(content, "former-name") ||
				(tc.want != "" && !strings.Contains(content, tc.want)) || (tc.want == "" && strings.Contains(content, "deps:")) {
				t.Errorf("blocked-work.md = %q, want deps %q", content, tc.want)
			}
			if issueFileExists(t, root, "renamed-issue.md") {
				t.Errorf("renamed-issue.md was not removed")
			}
			assertCheckPasses(t, root)
		})
	}
}

func TestRm_StaleIfMatch_ConflictsAndKeepsTheFile(t *testing.T) {
	root := newRmStore(t)
	etag := textETag(mustRun(t, root, "show", "renamed-issue"))
	mustRun(t, root, "update", "renamed-issue", "--title", "Renamed again")

	stdout, _, exitCode := runCLI(t, root, "rm", "--json", "renamed-issue", "--if-match", etag)

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeConflict) || failure["path"] != ".capcom/issues/renamed-issue.md" {
		t.Errorf("stale --if-match = %d, error %v; want conflict at the issue file", exitCode, failure)
	}
	if !issueFileExists(t, root, "renamed-issue.md") {
		t.Errorf("the issue was removed")
	}
}

func TestRm_StaleIfMatchWithDependent_Conflicts(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")
	etag := textETag(mustRun(t, root, "show", "login-flow"))
	mustRun(t, root, "update", "login-flow", "--title", "Login flow again")

	stdout, _, exitCode := runCLI(t, root, "rm", "--json", "login-flow", "--if-match", etag)

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeConflict) || failure["path"] != ".capcom/issues/login-flow.md" {
		t.Errorf("stale --if-match on a dependency = %d, error %v; want conflict at the issue file", exitCode, failure)
	}
	if !issueFileExists(t, root, "login-flow.md") {
		t.Errorf("the issue was removed")
	}
}

func TestRm_OwnFileDoesNotParse_RemovesItAndDropsItsDeps(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{
		"broken-issue.md": "---\ntitle: [unclosed\naliases: [broken-name]\n---\n",
		"blocked-work.md": readIssue("Blocked", "open", "deps: [broken-name, login-flow]\n", ""),
	})

	stdout, stderr, exitCode := runCLI(t, root, "rm", "--kv", "broken-issue", "--drop-deps")

	if exitCode != ExitOK || issueFileExists(t, root, "broken-issue.md") {
		t.Fatalf("rm of an issue whose file does not parse = %d\n%s%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "DEPS_DROPPED=[blocked-work]\n") {
		t.Errorf("stdout = %q", stdout)
	}
	if blocked := readStoreFile(t, root, "blocked-work.md"); !strings.Contains(blocked, "deps: [login-flow]\n") {
		t.Errorf("blocked-work.md = %q", blocked)
	}
}

func TestRm_UnrelatedBadFileName_RemovesAndWarnsOnce(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"Bad_Name.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "rm", "renamed-issue")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if issueFileExists(t, root, "renamed-issue.md") {
		t.Error("renamed-issue.md was not removed")
	}
	assertWarnedOnce(t, stdout+stderr, ".capcom/issues/Bad_Name.md")
}

func TestRm_UnparseableIssue_WarnsDepsNotChecked(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"broken-issue.md": "---\ntitle: [unclosed\n---\n"})

	stdout, stderr, exitCode := runCLI(t, root, "rm", "renamed-issue")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	warning := "warning: .capcom/issues/broken-issue.md: deps not checked for renamed-issue: the file does not parse"
	if !slices.Contains(strings.Split(stdout, "\n"), warning) {
		t.Errorf("no deps-not-checked warning line %q\nstdout = %q\nstderr = %q", warning, stdout, stderr)
	}
}

func TestRm_JSON_ReportsKindAndData(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")

	envelope := decodeEnvelope(t, mustRun(t, root, "rm", "--json", "login-flow", "--drop-deps"))

	data, _ := envelope["data"].(map[string]any)
	dropped, _ := data["deps_dropped"].([]any)
	if envelope["kind"] != kindIssueRemove || data["id"] != "login-flow" || data["path"] != ".capcom/issues/login-flow.md" ||
		!slices.Equal(dropped, []any{"session-work"}) {
		t.Errorf("rm --json = %v", envelope)
	}
	if session := readStoreFile(t, root, "session-work.md"); strings.Contains(session, "deps:") {
		t.Errorf("session-work.md = %q", session)
	}
}

func TestRm_DependsOnItself_IsRemoved(t *testing.T) {
	root := newRmStore(t)
	writeIssueFiles(t, root, map[string]string{"loop-issue.md": readIssue("Loop", "open", "deps: [loop-issue]\n", "")})

	stdout, stderr, exitCode := runCLI(t, root, "rm", "--json", "loop-issue")

	data, _ := decodeEnvelope(t, stdout)["data"].(map[string]any)
	if exitCode != ExitOK || issueFileExists(t, root, "loop-issue.md") {
		t.Fatalf("rm of a self-dependent issue = %d\n%s%s", exitCode, stdout, stderr)
	}
	if dropped, _ := data["deps_dropped"].([]any); len(dropped) != 0 {
		t.Errorf("deps_dropped = %v, want none", dropped)
	}
}

func TestRm_RelatedHolders_DropsTheirEntriesWithoutRefusing(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "related", "add", "login-flow", "renamed-issue")
	writeIssueFiles(t, root, map[string]string{
		"linked-work.md": readIssue("Linked", "open", "related: [former-name, session-work]\n", ""),
	})

	stdout := mustRun(t, root, "rm", "renamed-issue")

	if !strings.HasSuffix(stdout, "related_dropped: [linked-work, login-flow]\n") {
		t.Errorf("rm stdout = %q", stdout)
	}
	if login := readStoreFile(t, root, "login-flow.md"); strings.Contains(login, "related:") {
		t.Errorf("login-flow.md = %q, want no related key", login)
	}
	if linked := readStoreFile(t, root, "linked-work.md"); !strings.Contains(linked, "related: [session-work]\n") {
		t.Errorf("linked-work.md = %q", linked)
	}
	if related := showRelated(t, root, "login-flow"); len(related) != 0 {
		t.Errorf("related of login-flow = %v, want none", related)
	}
	assertCheckPasses(t, root)
}

func TestRm_JSON_ReportsRelatedDropped(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "related", "add", "session-work", "renamed-issue")

	envelope := decodeEnvelope(t, mustRun(t, root, "rm", "--json", "renamed-issue"))

	data, _ := envelope["data"].(map[string]any)
	related, _ := data["related_dropped"].([]any)
	deps, isList := data["deps_dropped"].([]any)
	if !slices.Equal(related, []any{"session-work"}) || !isList || len(deps) != 0 {
		t.Errorf("rm --json = %v", envelope)
	}
}

func TestRm_DependentAndRelatedHolder_RefusesAndKeepsTheHolder(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")
	mustRun(t, root, "related", "add", "renamed-issue", "login-flow")
	holder := readStoreFile(t, root, "renamed-issue.md")

	_, stderr, exitCode := runCLI(t, root, "rm", "login-flow")

	wantPrefix := "capcom: .capcom/issues/login-flow.md: issue login-flow is a dependency of session-work"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("rm = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if got := readStoreFile(t, root, "renamed-issue.md"); got != holder {
		t.Errorf("renamed-issue.md = %q, want it byte for byte as before: %q", got, holder)
	}
	if !issueFileExists(t, root, "login-flow.md") {
		t.Errorf("login-flow.md was removed")
	}
}

func TestRm_DropDeps_FileHoldingDepAndRelated_LosesBoth(t *testing.T) {
	root := newRmStore(t)
	mustRun(t, root, "rm", "child-issue")
	mustRun(t, root, "related", "add", "session-work", "login-flow")
	mustRun(t, root, "related", "add", "session-work", "renamed-issue")

	envelope := decodeEnvelope(t, mustRun(t, root, "rm", "--json", "login-flow", "--drop-deps"))

	data, _ := envelope["data"].(map[string]any)
	deps, _ := data["deps_dropped"].([]any)
	related, _ := data["related_dropped"].([]any)
	if !slices.Equal(deps, []any{"session-work"}) || !slices.Equal(related, []any{"session-work"}) {
		t.Errorf("rm --json = %v", envelope)
	}
	if session := readStoreFile(t, root, "session-work.md"); session != readIssue("Session work", "open", "related: [renamed-issue]\n", "") {
		t.Errorf("session-work.md = %q", session)
	}
	assertCheckPasses(t, root)
}

// newDuplicateIssueStore creates an issue store where foo.md and sub-issue/foo.md share the id foo
// beside foo-bar, and returns its root and the content of every issue file by slash path.
func newDuplicateIssueStore(t *testing.T) (string, map[string]string) {
	t.Helper()
	files := map[string]string{
		"foo.md":           readIssue("Foo", "open", "", ""),
		"foo-bar.md":       readIssue("Foo bar", "open", "", ""),
		"sub-issue.md":     readIssue("Sub issue", "open", "", ""),
		"sub-issue/foo.md": readIssue("Foo too", "open", "", ""),
	}
	root := newIssueStore(t, nil)
	writeIssueFiles(t, root, files)
	return root, files
}

// assertDuplicateRefusal fails the test unless a command exited 1 with stderr naming every one of
// paths, relative to the working tree root, and no absolute path inside root.
func assertDuplicateRefusal(t *testing.T, root string, stderr string, exitCode int, paths ...string) {
	t.Helper()
	if exitCode != ExitError || !strings.Contains(stderr, "duplicate") {
		t.Errorf("exit = %d stderr = %q, want 1 and a duplicate refusal", exitCode, stderr)
	}
	for _, path := range paths {
		if !strings.Contains(stderr, path) {
			t.Errorf("stderr = %q, want it to name %s", stderr, path)
		}
	}
	if strings.Contains(stderr, root) {
		t.Errorf("stderr = %q names an absolute path", stderr)
	}
}

// assertIssueFilesUnchanged fails the test unless every issue file of files still holds its content.
func assertIssueFilesUnchanged(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if got := readStoreFile(t, root, name); got != content {
			t.Errorf("%s = %q, want it unchanged", name, got)
		}
	}
}

func TestRm_DuplicatedID_FailsNamingBothFiles(t *testing.T) {
	root, files := newDuplicateIssueStore(t)

	_, stderr, exitCode := runCLI(t, root, "rm", "foo")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/foo.md", ".capcom/issues/sub-issue/foo.md")
	assertIssueFilesUnchanged(t, root, files)
}

// unusableClaimStore is a store in which only an unusable file claims the name foo, beside a
// usable foo-bar the name is a prefix of.
type unusableClaimStore struct {
	name string
	// files are the store's issue files, by slash path below .capcom/issues/.
	files map[string]string
	// claimant is the file that claims foo, relative to the working tree root.
	claimant string
}

// unusableClaimStores are the stores of every kind of unusable file that can claim foo.
var unusableClaimStores = []unusableClaimStore{
	{
		name: "alias of a file that does not parse",
		files: map[string]string{
			"zzz.md": "---\ntitle: [unclosed\naliases: [foo]\n---\n",
		},
		claimant: ".capcom/issues/zzz.md",
	},
	{
		name: "id of a file that fails validation",
		files: map[string]string{
			"foo.md": readIssue("Foo", "bogus", "", ""),
		},
		claimant: ".capcom/issues/foo.md",
	},
	{
		name: "id of a node file below an orphan directory",
		files: map[string]string{
			"ghost/foo.md": readIssue("Foo", "open", "", ""),
		},
		claimant: ".capcom/issues/ghost/foo.md",
	},
	{
		name: "alias only the later file of a repeated id lists",
		files: map[string]string{
			"a.md":     readIssue("A", "open", "", ""),
			"b.md":     readIssue("B", "open", "", ""),
			"a/dup.md": readIssue("Dup", "open", "", ""),
			"b/dup.md": readIssue("Dup too", "open", "aliases: [foo]\n", ""),
		},
		claimant: ".capcom/issues/b/dup.md",
	},
}

// newUnusableClaimStore creates the store of fixture, with foo-bar.md beside its files, and returns
// its root and every issue file it wrote.
func newUnusableClaimStore(t *testing.T, fixture unusableClaimStore) (string, map[string]string) {
	t.Helper()
	files := map[string]string{"foo-bar.md": readIssue("Foo bar", "open", "", "")}
	for name, content := range fixture.files {
		files[name] = content
	}
	root := newIssueStore(t, nil)
	writeIssueFiles(t, root, files)
	return root, files
}

// assertUnusableClaimRefusal fails the test unless a command exited not found with stderr giving
// the refusal of the name foo that fixture's claimant claims, with no absolute path inside root,
// and left every issue file of the store as it was, and show foo, through the index, gives the
// same refusal.
func assertUnusableClaimRefusal(
	t *testing.T,
	root string,
	fixture unusableClaimStore,
	files map[string]string,
	stderr string,
	exitCode int,
) {
	t.Helper()
	refusal := `id "foo" not found: ` + fixture.claimant + " is invalid"
	if exitCode != ExitNotFound || !strings.Contains(stderr, refusal) {
		t.Errorf("exit = %d stderr = %q, want %d and %q", exitCode, stderr, ExitNotFound, refusal)
	}
	if strings.Contains(stderr, root) {
		t.Errorf("stderr = %q names an absolute path", stderr)
	}
	assertIssueFilesUnchanged(t, root, files)
	if _, showStderr, _ := runCLI(t, root, "show", "foo"); !strings.Contains(showStderr, refusal) {
		t.Errorf("show foo stderr = %q, want the same refusal %q", showStderr, refusal)
	}
}

func TestRm_NameOnlyAnUnusableFileClaims_IsRefusedNamingIt(t *testing.T) {
	for _, fixture := range unusableClaimStores {
		t.Run(fixture.name, func(t *testing.T) {
			root, files := newUnusableClaimStore(t, fixture)

			_, stderr, exitCode := runCLI(t, root, "rm", "foo")

			assertUnusableClaimRefusal(t, root, fixture, files, stderr, exitCode)
		})
	}
}
