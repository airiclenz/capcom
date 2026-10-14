package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/agents"
)

// knowledgePath returns the absolute path of a slash path below .capcom/knowledge/.
func knowledgePath(root string, name string) string {
	return filepath.Join(root, ".capcom", "knowledge", filepath.FromSlash(name))
}

func TestKnowRm_Leaf_RemovesTheNote(t *testing.T) {
	root := newKnowStore(t)

	stdout := mustRun(t, root, "know", "rm", "release-flow")

	if stdout != "id: release-flow\npath: .capcom/knowledge/release-flow.md\n" {
		t.Errorf("know rm stdout = %q", stdout)
	}
	if noteFileExists(t, root, "release-flow.md") {
		t.Errorf("release-flow.md was not removed")
	}
	jsonOutput, _, exitCode := runCLI(t, root, "know", "show", "--json", "release-flow")
	failure, _ := decodeEnvelope(t, jsonOutput)["error"].(map[string]any)
	if exitCode != ExitNotFound || failure["code"] != string(CodeNotFound) {
		t.Errorf("know show after rm = %d, error %v; want not_found", exitCode, failure)
	}
}

func TestKnowRm_LastChild_TakesTheEmptyDirectory(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "know", "rm", "no-parallel")

	mustRun(t, root, "know", "rm", "cli-tests")

	if noteFileExists(t, root, "testing-rules/cli-tests") || noteFileExists(t, root, "testing-rules/cli-tests.md") {
		t.Errorf("cli-tests or its children directory is left")
	}
	if !noteFileExists(t, root, "testing-rules/git-fixtures.md") {
		t.Errorf("a sibling was removed")
	}
	mustRun(t, root, "know", "rm", "git-fixtures")
	if noteFileExists(t, root, "testing-rules") {
		t.Errorf("the emptied testing-rules directory was not removed")
	}
	if stdout, _, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after know rm = %d\n%s", exitCode, stdout)
	}
}

func TestKnowRm_WithChildren_IsRefusedNamingTheFileAndCount(t *testing.T) {
	root := newKnowStore(t)

	_, stderr, exitCode := runCLI(t, root, "know", "rm", "testing-rules")

	wantPrefix := "capcom: .capcom/knowledge/testing-rules.md: Note testing-rules has 2 child Notes"
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("know rm with children = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !noteFileExists(t, root, "testing-rules.md") || !noteFileExists(t, root, "testing-rules/cli-tests/no-parallel.md") {
		t.Errorf("files were removed")
	}
}

func TestKnowRm_EmptyOwnChildrenDirectory_RemovesBoth(t *testing.T) {
	root := newKnowStore(t)
	if err := os.Mkdir(knowledgePath(root, "release-flow"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	mustRun(t, root, "know", "rm", "release-flow")

	if noteFileExists(t, root, "release-flow.md") || noteFileExists(t, root, "release-flow") {
		t.Errorf("the note or its empty children directory is left")
	}
}

func TestKnowRm_ChildrenDirectoryWithANonNoteFile_IsRefused(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"release-flow/diagram.png": "png"})

	_, stderr, exitCode := runCLI(t, root, "know", "rm", "release-flow")

	wantPrefix := "capcom: .capcom/knowledge/release-flow: "
	if exitCode != ExitError || !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("know rm = %d, stderr %q; want %q", exitCode, stderr, wantPrefix)
	}
	if !noteFileExists(t, root, "release-flow.md") || !noteFileExists(t, root, "release-flow/diagram.png") {
		t.Errorf("files were removed")
	}
}

func TestKnowRm_IfMatchFromShow_Removes(t *testing.T) {
	root := newKnowStore(t)
	etag := textETag(mustRun(t, root, "know", "show", "release-flow"))

	stdout, stderr, exitCode := runCLI(t, root, "know", "rm", "release-flow", "--if-match", etag)

	if exitCode != ExitOK || noteFileExists(t, root, "release-flow.md") {
		t.Errorf("know rm --if-match %s = %d\n%s%s", etag, exitCode, stdout, stderr)
	}
}

func TestKnowRm_StaleIfMatch_ConflictsAndKeepsTheFile(t *testing.T) {
	root := newKnowStore(t)
	etag := textETag(mustRun(t, root, "know", "show", "release-flow"))
	mustRun(t, root, "know", "update", "release-flow", "--title", "Release steps")

	stdout, _, exitCode := runCLI(t, root, "know", "rm", "--json", "release-flow", "--if-match", etag)

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeConflict) || failure["path"] != ".capcom/knowledge/release-flow.md" {
		t.Errorf("stale --if-match = %d, error %v; want conflict at the note file", exitCode, failure)
	}
	if !noteFileExists(t, root, "release-flow.md") {
		t.Errorf("the note was removed")
	}
}

func TestKnowRm_TopLevel_RefreshesTheAgentsBlock(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")

	stdout := mustRun(t, root, "know", "rm", "--json", "release-flow")

	data, _ := decodeEnvelope(t, stdout)["data"].(map[string]any)
	if data["agents"] != blockRefreshed || strings.Contains(readAgentsGuide(t, root), "`release-flow`") {
		t.Errorf("know rm = %s, AGENTS.md =\n%s\nwant the block refreshed without release-flow", stdout, readAgentsGuide(t, root))
	}
	if output, _, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after know rm = %d\n%s", exitCode, output)
	}
}

func TestKnowRm_TangledAgentsMarkers_WarnsAndKeepsTheRemoval(t *testing.T) {
	root := newKnowStore(t)
	tangled := "# Guide\n" + agents.BeginMarker + "\n"
	writeAgentsGuide(t, root, tangled)

	stdout, stderr, exitCode := runCLI(t, root, "know", "rm", "release-flow")

	if exitCode != ExitOK || !strings.Contains(stdout, "warning: AGENTS.md:2: ") {
		t.Errorf("know rm = %d\n%s%s; want ok with a warning at AGENTS.md:2", exitCode, stdout, stderr)
	}
	if noteFileExists(t, root, "release-flow.md") || readAgentsGuide(t, root) != tangled {
		t.Errorf("the note was kept, or AGENTS.md changed to %q", readAgentsGuide(t, root))
	}
}

func TestKnowRm_UnknownID_FailsNotFound(t *testing.T) {
	root := newKnowStore(t)

	stdout, _, exitCode := runCLI(t, root, "know", "rm", "--json", "no-such-note")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitNotFound || failure["code"] != string(CodeNotFound) {
		t.Errorf("know rm of an unknown id = %d, error %v; want not_found", exitCode, failure)
	}
}

func TestKnowRm_UnrelatedBadFileName_RemovesAndWarnsOnce(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"Bad_Name.md": "---\ntitle: Bad name\n---\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "rm", "release-flow")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if noteFileExists(t, root, "release-flow.md") {
		t.Error("release-flow.md was not removed")
	}
	if count := strings.Count(stdout+stderr, ".capcom/knowledge/Bad_Name.md"); count != 1 {
		t.Errorf("Bad_Name.md named %d times, want once\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
}

func TestKnowRm_ChildOfDuplicateParent_Fails(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"zz-other.md": "---\ntitle: Other\n---\n", "zz-other/testing-rules.md": "---\ntitle: Testing rules too\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "rm", "git-fixtures")

	want := `duplicate id "testing-rules"`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if !noteFileExists(t, root, "testing-rules/git-fixtures.md") {
		t.Error("git-fixtures.md was removed below the duplicate parent")
	}
}

func TestKnowRm_DuplicatedID_FailsNamingBothFiles(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"testing-rules/release-flow.md": "---\ntitle: Release flow too\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "rm", "release-flow")

	assertDuplicateRefusal(t, root, stderr, exitCode,
		".capcom/knowledge/release-flow.md", ".capcom/knowledge/testing-rules/release-flow.md")
	for _, name := range []string{"release-flow.md", "testing-rules/release-flow.md"} {
		if !noteFileExists(t, root, name) {
			t.Errorf("%s was removed", name)
		}
	}
}

func TestKnowRm_UnparseableNote_RemovesItsFile(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"testing-rules/broken-note.md": "no frontmatter\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "rm", "broken-note")

	if exitCode != ExitOK || noteFileExists(t, root, "testing-rules/broken-note.md") {
		t.Errorf("know rm of an unparseable note = %d\n%s%s; want it removed", exitCode, stdout, stderr)
	}
	if !noteFileExists(t, root, "testing-rules/git-fixtures.md") {
		t.Errorf("a sibling was removed")
	}
}

func TestKnowRm_SupersededByTarget_DropsTheLinks(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\nsuperseded-by: [release-flow, cli-tests]\n---\n",
		"testing-rules.md":              "---\ntitle: Testing rules\nsuperseded-by: [release-flow]\n---\n",
	})

	mustRun(t, root, "know", "rm", "release-flow")

	want := map[string]string{
		"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\nsuperseded-by: [cli-tests]\n---\n",
		"testing-rules.md":              "---\ntitle: Testing rules\n---\n",
	}
	for name, content := range want {
		if got := readNoteFile(t, root, name); got != content {
			t.Errorf("%s = %q; want %q", name, got, content)
		}
	}
}

func TestKnowRm_SupersededByTarget_ReportsTheDroppedHolders(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\nsuperseded-by: [release-flow, cli-tests]\n---\n",
		"testing-rules.md":              "---\ntitle: Testing rules\nsuperseded-by: [release-flow]\n---\n",
	})
	rewritten := map[string]string{
		"testing-rules/git-fixtures.md": readNoteFile(t, root, "testing-rules/git-fixtures.md"),
		"testing-rules.md":              readNoteFile(t, root, "testing-rules.md"),
	}
	releaseFlow := readNoteFile(t, root, "release-flow.md")

	stdout := mustRun(t, root, "know", "rm", "release-flow")

	wantText := "id: release-flow\npath: .capcom/knowledge/release-flow.md\n" +
		"superseded_by_dropped: [git-fixtures, testing-rules]\n"
	if stdout != wantText {
		t.Errorf("know rm stdout = %q; want %q", stdout, wantText)
	}

	writeNoteFiles(t, root, rewritten)
	writeNoteFiles(t, root, map[string]string{"release-flow.md": releaseFlow})

	envelope := decodeEnvelope(t, mustRun(t, root, "know", "rm", "--json", "release-flow"))

	data, _ := envelope["data"].(map[string]any)
	dropped, _ := data["superseded_by_dropped"].([]any)
	if envelope["kind"] != kindNoteRemove || data["id"] != "release-flow" ||
		!slices.Equal(dropped, []any{"git-fixtures", "testing-rules"}) {
		t.Errorf("know rm --json = %v", envelope)
	}
}

func TestKnowRm_NoSuccessorLinks_ReportsAnEmptyList(t *testing.T) {
	root := newKnowStore(t)
	releaseFlow := readNoteFile(t, root, "release-flow.md")

	stdout := mustRun(t, root, "know", "rm", "release-flow")

	if strings.Contains(stdout, "superseded_by_dropped") {
		t.Errorf("know rm stdout = %q; want no superseded_by_dropped line", stdout)
	}

	writeNoteFiles(t, root, map[string]string{"release-flow.md": releaseFlow})

	envelope := decodeEnvelope(t, mustRun(t, root, "know", "rm", "--json", "release-flow"))

	data, _ := envelope["data"].(map[string]any)
	dropped, isList := data["superseded_by_dropped"].([]any)
	if !isList || len(dropped) != 0 {
		t.Errorf("know rm --json superseded_by_dropped = %#v; want []", data["superseded_by_dropped"])
	}
}

func TestKnowRm_UnparseableBystander_WarnsAndRemoves(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"testing-rules/broken-note.md": "no frontmatter\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "rm", "release-flow")

	warning := ".capcom/knowledge/testing-rules/broken-note.md: superseded-by not checked for release-flow: " +
		"the file does not parse"
	if exitCode != ExitOK || !strings.Contains(stdout+stderr, warning) {
		t.Errorf("know rm = %d\n%s%s; want success warning %q", exitCode, stdout, stderr, warning)
	}
	if noteFileExists(t, root, "release-flow.md") {
		t.Errorf("release-flow.md was not removed")
	}
}

func TestKnowRm_DependentWriteFails_RollsBackNamingTheDependentFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not stop file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only directory")
	}
	root := newKnowStore(t)
	dependent := "---\ntitle: Git fixtures\nsuperseded-by: [release-flow]\n---\n"
	writeNoteFiles(t, root, map[string]string{"testing-rules/git-fixtures.md": dependent})
	readOnlyDir := knowledgePath(root, "testing-rules")
	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	stdout, _, exitCode := runCLI(t, root, "--json", "know", "rm", "release-flow")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode == ExitOK || failure["path"] != ".capcom/knowledge/testing-rules/git-fixtures.md" {
		t.Errorf("exit = %d error = %v; want a failure at testing-rules/git-fixtures.md", exitCode, failure)
	}
	if !noteFileExists(t, root, "release-flow.md") {
		t.Errorf("release-flow.md was removed")
	}
	if got := readNoteFile(t, root, "testing-rules/git-fixtures.md"); got != dependent {
		t.Errorf("git-fixtures.md = %q; want %q", got, dependent)
	}
}
