package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/agents"
)

// noteFileExists reports whether a Note file or directory is at a slash path below
// .capcom/knowledge/.
func noteFileExists(t *testing.T, root string, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, ".capcom", "knowledge", filepath.FromSlash(name)))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("Stat: %v", err)
	}
	return err == nil
}

func TestKnowMv_Rename_KeepsTheOldIDAsAnAlias(t *testing.T) {
	root := newKnowStore(t)

	stdout := mustRun(t, root, "know", "mv", "release", "release-steps")

	if !strings.HasPrefix(stdout, "id: release-steps\npath: .capcom/knowledge/release-steps.md\netag: ") ||
		textETag(stdout) != noteFileETag(t, root, "release-steps.md") {
		t.Errorf("know mv stdout = %q", stdout)
	}
	want := "---\ntitle: Release flow\nlabels: [process]\naliases: [release-flow]\n---\n\nTag, then publish.\n"
	if got := readNoteFile(t, root, "release-steps.md"); got != want {
		t.Errorf("moved note = %q; want %q", got, want)
	}
	shown := decodeEnvelope(t, mustRun(t, root, "know", "show", "--json", "release-flow"))
	if data, _ := shown["data"].([]any); len(data) != 1 || data[0].(map[string]any)["id"] != "release-steps" {
		t.Errorf("know show release-flow = %v", shown)
	}
}

func TestKnowMv_Parent_MovesBelowThenToTop(t *testing.T) {
	root := newKnowStore(t)

	mustRun(t, root, "know", "mv", "release-flow", "--parent", "git-fixtures")
	below := mustRun(t, root, "know", "tree", "git-fixtures")
	stdout := mustRun(t, root, "know", "mv", "release-flow", "--parent", ".")

	if below != "git-fixtures  Git fixtures\n  release-flow  Release flow\n" {
		t.Errorf("know tree git-fixtures = %q", below)
	}
	if !strings.Contains(stdout, "path: .capcom/knowledge/release-flow.md\n") || !noteFileExists(t, root, "release-flow.md") {
		t.Errorf("know mv --parent . = %q; want release-flow.md at the top", stdout)
	}
	if noteFileExists(t, root, "testing-rules/git-fixtures") {
		t.Errorf("the emptied git-fixtures directory was not removed")
	}
}

func TestKnowMv_EmptyParent_FailsUsageChangingNothing(t *testing.T) {
	for _, parent := range []string{"", " "} {
		root := newKnowStore(t)
		before := readNoteFile(t, root, "testing-rules/git-fixtures.md")

		_, stderr, exitCode := runCLI(t, root, "know", "mv", "git-fixtures", "--parent", parent)

		if exitCode != ExitUsage || !strings.Contains(stderr, `--parent needs an id or "."`) {
			t.Errorf("--parent %q: exit = %d stderr = %q", parent, exitCode, stderr)
		}
		if !noteFileExists(t, root, "testing-rules/git-fixtures.md") || noteFileExists(t, root, "git-fixtures.md") {
			t.Errorf("--parent %q: git-fixtures.md moved", parent)
		}
		if after := readNoteFile(t, root, "testing-rules/git-fixtures.md"); after != before {
			t.Errorf("--parent %q: git-fixtures.md = %q, want %q", parent, after, before)
		}
	}
}

func TestKnowMv_RenameWithChildren_CarriesTheSubtree(t *testing.T) {
	root := newKnowStore(t)

	mustRun(t, root, "know", "mv", "cli-tests", "harness-rules", "--parent", ".")

	if !noteFileExists(t, root, "harness-rules/no-parallel.md") || noteFileExists(t, root, "testing-rules/cli-tests") {
		t.Errorf("the children directory did not move with the note")
	}
	want := "harness-rules  CLI tests\n  no-parallel  No parallel\n"
	if got := mustRun(t, root, "know", "tree", "harness-rules"); got != want {
		t.Errorf("know tree harness-rules = %q; want %q", got, want)
	}
}

func TestKnowMv_OntoTakenID_HintsShowingTheNoteHoldingIt(t *testing.T) {
	root := newKnowStore(t)

	assertFailureHint(t, root, "capcom know show testing-rules", "know", "mv", "git-fixtures", "testing-rules")
}

func TestKnowMv_OntoTakenIDOrAlias_FailsExistsNamingTheNote(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "know", "mv", "release-flow", "release-steps")
	cases := []struct {
		newID string
		want  string
	}{
		{"testing-rules", "already taken by Note testing-rules"},
		{"release-flow", "already an alias of Note release-steps"},
	}

	for _, current := range cases {
		stdout, _, exitCode := runCLI(t, root, "know", "mv", "--json", "git-fixtures", current.newID)

		failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
		message, _ := failure["message"].(string)
		if exitCode != ExitError || failure["code"] != string(CodeExists) ||
			!strings.Contains(message, current.want) || strings.Contains(message, "issue") {
			t.Errorf("know mv onto %s = %d, error %v; want exists with %q", current.newID, exitCode, failure, current.want)
		}
	}
	if !noteFileExists(t, root, "testing-rules/git-fixtures.md") {
		t.Errorf("git-fixtures.md moved")
	}
}

func TestKnowMv_OntoAliasOfUnparseableNote_FailsExistsNamingTheFile(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "know", "mv", "release-flow", "release-steps")
	breakNoteFile(t, root, "release-steps.md")

	stdout, _, exitCode := runCLI(t, root, "know", "mv", "--json", "git-fixtures", "release-flow")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	wantMessage := `id "release-flow" may be an alias of Note release-steps, whose file does not parse: fix it first`
	if exitCode != ExitError || failure["code"] != string(CodeExists) ||
		failure["path"] != ".capcom/knowledge/release-steps.md" || failure["message"] != wantMessage {
		t.Errorf("exit = %d error = %v, want exists at release-steps.md with %q", exitCode, failure, wantMessage)
	}
	if !noteFileExists(t, root, "testing-rules/git-fixtures.md") {
		t.Errorf("git-fixtures.md moved")
	}
}

func TestKnowMv_BelowItsOwnChild_FailsInvalid(t *testing.T) {
	root := newKnowStore(t)

	_, stderr, exitCode := runCLI(t, root, "know", "mv", "testing-rules", "--parent", "no-parallel")

	wantPrefix := "capcom: .capcom/knowledge/testing-rules.md: cannot move a Note below itself or its own descendants"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("know mv below its own child = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !noteFileExists(t, root, "testing-rules/cli-tests/no-parallel.md") {
		t.Errorf("the tree changed")
	}
}

func TestKnowMv_IfMatchFromShow_Moves(t *testing.T) {
	root := newKnowStore(t)
	etag := textETag(mustRun(t, root, "know", "show", "release-flow"))

	stdout, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "release-steps", "--if-match", etag)

	if exitCode != ExitOK || !noteFileExists(t, root, "release-steps.md") {
		t.Errorf("know mv --if-match %s = %d\n%s%s", etag, exitCode, stdout, stderr)
	}
}

func TestKnowMv_StaleIfMatch_ConflictsAndLeavesTheTree(t *testing.T) {
	root := newKnowStore(t)
	etag := textETag(mustRun(t, root, "know", "show", "release-flow"))
	mustRun(t, root, "know", "update", "release-flow", "--title", "Release steps")
	before := readNoteFile(t, root, "release-flow.md")

	stdout, _, exitCode := runCLI(t, root, "know", "mv", "--json", "release-flow", "release-steps", "--if-match", etag)

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeConflict) || failure["path"] != ".capcom/knowledge/release-flow.md" {
		t.Errorf("stale --if-match = %d, error %v; want conflict at the note file", exitCode, failure)
	}
	if noteFileExists(t, root, "release-steps.md") || readNoteFile(t, root, "release-flow.md") != before {
		t.Errorf("the tree changed")
	}
}

func TestKnowMv_NothingToDo_FailsUsage(t *testing.T) {
	root := newKnowStore(t)

	_, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow")

	if exitCode != ExitUsage || !strings.Contains(stderr, "new id, --parent") {
		t.Errorf("know mv without a destination = %d, stderr %q", exitCode, stderr)
	}
}

func TestKnowMv_TopLevelChange_RefreshesTheAgentsBlock(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")

	stdout := mustRun(t, root, "know", "mv", "release-flow", "--parent", "testing-rules")

	if !strings.HasSuffix(stdout, "refreshed: AGENTS.md\n") || strings.Contains(readAgentsGuide(t, root), "`release-flow`") {
		t.Errorf("know mv = %q, AGENTS.md =\n%s\nwant the block refreshed without release-flow", stdout, readAgentsGuide(t, root))
	}
	if output, _, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after know mv = %d\n%s", exitCode, output)
	}
}

func TestKnowMv_TangledAgentsMarkers_WarnsAndKeepsTheMove(t *testing.T) {
	root := newKnowStore(t)
	tangled := "# Guide\n" + agents.BeginMarker + "\n"
	writeAgentsGuide(t, root, tangled)

	stdout, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "release-steps")

	if exitCode != ExitOK || !strings.Contains(stdout, "warning: AGENTS.md:2: ") {
		t.Errorf("know mv = %d\n%s%s; want ok with a warning at AGENTS.md:2", exitCode, stdout, stderr)
	}
	if !noteFileExists(t, root, "release-steps.md") || readAgentsGuide(t, root) != tangled {
		t.Errorf("the move was not kept, or AGENTS.md changed to %q", readAgentsGuide(t, root))
	}
}

func TestKnowMv_UnrelatedBadFileName_MovesAndWarnsOnce(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"Bad_Name.md": "---\ntitle: Bad name\n---\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "release-steps")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if !noteFileExists(t, root, "release-steps.md") {
		t.Error("release-steps.md was not written")
	}
	if count := strings.Count(stdout+stderr, ".capcom/knowledge/Bad_Name.md"); count != 1 {
		t.Errorf("Bad_Name.md named %d times, want once\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
}

func TestKnowMv_BadFileInMovedChildrenDir_WarnsWithNewPath(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"release-flow/Bad_Name.md": "---\ntitle: Bad name\n---\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "release-steps")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if !noteFileExists(t, root, "release-steps/Bad_Name.md") {
		t.Error("release-steps/Bad_Name.md did not move")
	}
	if count := strings.Count(stdout+stderr, ".capcom/knowledge/release-flow/Bad_Name.md"); count != 0 {
		t.Errorf("old path named %d times, want 0\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
	if count := strings.Count(stdout+stderr, ".capcom/knowledge/release-steps/Bad_Name.md"); count != 1 {
		t.Errorf("new path named %d times, want once\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
}

func TestKnowMv_BelowDuplicateParent_Fails(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"zz-other.md": "---\ntitle: Other\n---\n", "zz-other/testing-rules.md": "---\ntitle: Testing rules too\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "--parent", "testing-rules")

	want := `duplicate id "testing-rules"`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if !noteFileExists(t, root, "release-flow.md") {
		t.Error("release-flow.md was moved below the duplicate parent")
	}
}

func TestKnowMv_OntoIDInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"lost-dir/dup-id.md": "---\ntitle: Lost note\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "dup-id")

	want := `.capcom/knowledge/lost-dir/dup-id.md: id "dup-id" is already taken by this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if !noteFileExists(t, root, "release-flow.md") {
		t.Error("release-flow.md was moved onto the orphaned id")
	}
}

func TestKnowMv_OntoAliasInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"lost-dir/other-id.md": "---\ntitle: Lost note\naliases: [dup-id]\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "dup-id")

	want := `.capcom/knowledge/lost-dir/other-id.md: id "dup-id" is already an alias of this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if !noteFileExists(t, root, "release-flow.md") {
		t.Error("release-flow.md was moved onto an orphaned Note's alias")
	}
}

func TestKnowMv_UnparseableNote_FailsInvalidAtItsRelativeFileAndLine(t *testing.T) {
	root := newKnowStore(t)
	breakNoteFile(t, root, "release-flow.md")

	stdout, _, exitCode := runCLI(t, root, "know", "mv", "--json", "release-flow", "release-steps")

	if exitCode != ExitError {
		t.Errorf("exit = %d, want %d", exitCode, ExitError)
	}
	assertWorktreeRelativeFailure(t, root, stdout, ".capcom/knowledge/release-flow.md", true)
}

func TestKnowMv_Rename_RewritesSupersededByInOtherNotes(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\nsuperseded-by: [release-flow, release-steps]\n---\n",
	})

	mustRun(t, root, "know", "mv", "release-flow", "release-steps")

	want := "---\ntitle: Git fixtures\nsuperseded-by: [release-steps]\n---\n"
	if got := readNoteFile(t, root, "testing-rules/git-fixtures.md"); got != want {
		t.Errorf("git-fixtures.md = %q; want %q", got, want)
	}
}

func TestKnowMv_UnparseableBystander_WarnsAndMoves(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"testing-rules/broken-note.md": "no frontmatter\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "release-steps")

	warning := ".capcom/knowledge/testing-rules/broken-note.md: superseded-by not checked for release-flow: " +
		"the file does not parse"
	if exitCode != ExitOK || !strings.Contains(stdout+stderr, warning) {
		t.Errorf("know mv = %d\n%s%s; want success warning %q", exitCode, stdout, stderr, warning)
	}
	if !noteFileExists(t, root, "release-steps.md") {
		t.Errorf("release-steps.md was not written")
	}
}

func TestKnowMv_EmptiedOldParentNotRemovable_MovesAndWarns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not stop directory removal on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root removes from a read-only directory")
	}
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"old-parent.md":            "---\ntitle: Old parent\n---\n",
		"old-parent/moved-note.md": "---\ntitle: Moved note\n---\n",
		"new-parent.md":            "---\ntitle: New parent\n---\n",
		"new-parent/kept-note.md":  "---\ntitle: Kept note\n---\n",
	})
	readOnlyDir := knowledgePath(root, "")
	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	stdout, stderr, exitCode := runCLI(t, root, "know", "mv", "--json", "moved-note", "--parent", "new-parent")

	if exitCode != ExitOK {
		t.Fatalf("know mv = %d\n%s%s; want it to succeed", exitCode, stdout, stderr)
	}
	warnings, _ := decodeEnvelope(t, stdout)["warnings"].([]any)
	want := ".capcom/knowledge/old-parent: remove .capcom/knowledge/old-parent: permission denied; " +
		"the empty directory was not removed"
	if len(warnings) != 1 || warnings[0] != want {
		t.Errorf("warnings = %q; want [%q]", warnings, want)
	}
	if !noteFileExists(t, root, "new-parent/moved-note.md") {
		t.Errorf("moved-note.md is not below new-parent")
	}
}

func TestKnowMv_DependentWriteFails_RollsEveryFileBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not stop file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only directory")
	}
	root := newKnowStore(t)
	dependent := "---\ntitle: Git fixtures\nsuperseded-by: [release-flow]\n---\n"
	writeNoteFiles(t, root, map[string]string{"testing-rules/git-fixtures.md": dependent})
	original := readNoteFile(t, root, "release-flow.md")
	readOnlyDir := knowledgePath(root, "testing-rules")
	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	stdout, stderr, exitCode := runCLI(t, root, "know", "mv", "release-flow", "release-steps")

	if exitCode == ExitOK {
		t.Errorf("know mv = %d\n%s%s; want a failure", exitCode, stdout, stderr)
	}
	if noteFileExists(t, root, "release-steps.md") || readNoteFile(t, root, "release-flow.md") != original {
		t.Errorf("release-flow.md was not rolled back")
	}
	if got := readNoteFile(t, root, "testing-rules/git-fixtures.md"); got != dependent {
		t.Errorf("git-fixtures.md = %q; want %q", got, dependent)
	}
}
