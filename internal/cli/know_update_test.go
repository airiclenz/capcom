package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// readNoteFile returns the content of the Note file at a slash path below .capcom/knowledge/.
func readNoteFile(t *testing.T, root string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".capcom", "knowledge", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

// writeInputFile writes content to a new file outside the working tree and returns its path.
func writeInputFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestKnowUpdate_TitleLabelsAndRefs_RewriteTheFrontmatter(t *testing.T) {
	root := newKnowStore(t)

	stdout := mustRun(t, root, "know", "update", "release-flow", "--title", "Release steps",
		"--add-label", "ops", "--remove-label", "process", "--add-ref", "go.mod", "--add-ref", "go.mod#module")
	mustRun(t, root, "know", "update", "release-flow", "--remove-ref", "go.mod")

	want := "---\ntitle: Release steps\nlabels: [ops]\nrefs: [go.mod#module]\n---\n\nTag, then publish.\n"
	if got := readNoteFile(t, root, "release-flow.md"); got != want {
		t.Errorf("note file = %q; want %q", got, want)
	}
	if !strings.HasPrefix(stdout, "id: release-flow\npath: .capcom/knowledge/release-flow.md\netag: ") {
		t.Errorf("know update stdout = %q", stdout)
	}
}

func TestKnowUpdate_AddLabelWithEmptyEntry_DropsIt(t *testing.T) {
	root := newKnowStore(t)

	mustRun(t, root, "know", "update", "git-fixtures", "--add-label", "x,,y")

	want := "---\ntitle: Git fixtures\nlabels: [x, y]\n---\n"
	if got := readNoteFile(t, root, "testing-rules/git-fixtures.md"); got != want {
		t.Errorf("note file = %q; want %q", got, want)
	}
	mustRun(t, root, "know", "show", "git-fixtures")
	mustRun(t, root, "check")
}

func TestKnowUpdate_PrintsTheEtagOfTheWrittenFile(t *testing.T) {
	root := newKnowStore(t)

	textOutput := mustRun(t, root, "know", "update", "release-flow", "--title", "Release steps")
	textWant := noteFileETag(t, root, "release-flow.md")
	jsonOutput := mustRun(t, root, "know", "update", "release-flow", "--title", "Release flow", "--json")

	jsonData, _ := decodeEnvelope(t, jsonOutput)["data"].(map[string]any)
	if textEtag := textETag(textOutput); textEtag != textWant {
		t.Errorf("text etag = %q; want %q", textEtag, textWant)
	}
	if jsonEtag, _ := jsonData["etag"].(string); jsonEtag != noteFileETag(t, root, "release-flow.md") {
		t.Errorf("--json etag = %q; want the file's\n%s", jsonEtag, jsonOutput)
	}
}

func TestKnowUpdate_BodyEdits_ReplaceThenAppend(t *testing.T) {
	root := newKnowStore(t)
	appendFile := writeInputFile(t, "Hermetic.")

	mustRun(t, root, "know", "update", "cli-tests", "--body-replace-old", "runCLI.",
		"--body-replace-new", "runCLIWith.", "--body-append", "Never parallel.")
	mustRun(t, root, "know", "update", "cli-tests", "--body-append-file", appendFile)

	want := "---\ntitle: CLI tests\n---\n\nThrough runCLIWith.\n\nNever parallel.\n\nHermetic.\n"
	if got := readNoteFile(t, root, "testing-rules/cli-tests.md"); got != want {
		t.Errorf("note file = %q; want %q", got, want)
	}
}

func TestKnowUpdate_BodyFileFromStdin_ReplacesTheWholeBody(t *testing.T) {
	root := newKnowStore(t)

	_, stderr, exitCode := runCLIWith(t, root, "New body\r\nSecond line", nil,
		"know", "update", "cli-tests", "--body-file", "-")

	want := "---\ntitle: CLI tests\n---\n\nNew body\nSecond line\n"
	if got := readNoteFile(t, root, "testing-rules/cli-tests.md"); exitCode != ExitOK || got != want {
		t.Errorf("know update = %d %q, note file = %q; want %q", exitCode, stderr, got, want)
	}
}

func TestKnowUpdate_BlankBodyFile_LeavesNoBody(t *testing.T) {
	root := newKnowStore(t)

	mustRun(t, root, "know", "update", "cli-tests", "--body-file", writeInputFile(t, "  \n"))

	if got := readNoteFile(t, root, "testing-rules/cli-tests.md"); got != "---\ntitle: CLI tests\n---\n" {
		t.Errorf("note file = %q; want no body", got)
	}
}

func TestKnowUpdate_BodyFile_GivesTheBodyKnowAddWrites(t *testing.T) {
	root := newKnowStore(t)
	bodyFile := writeInputFile(t, "Line one\r\nLine two")

	mustRun(t, root, "know", "add", "--id", "api-design", "--title", "API design", "--body-file", bodyFile)
	mustRun(t, root, "know", "update", "release-flow", "--body-file", bodyFile)

	added := noteBodyOf(t, "api-design.md", readNoteFile(t, root, "api-design.md"))
	updated := noteBodyOf(t, "release-flow.md", readNoteFile(t, root, "release-flow.md"))
	if added != updated || added != "\nLine one\nLine two\n" {
		t.Errorf("know add body = %q, know update body = %q; want identical", added, updated)
	}
}

// noteBodyOf returns the body of the Note file content.
func noteBodyOf(t *testing.T, name string, content string) string {
	t.Helper()
	note, err := knowledge.Parse(name, []byte(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return note.Body
}

func TestKnowUpdate_IfMatchFromShow_Writes(t *testing.T) {
	root := newKnowStore(t)
	etag := textETag(mustRun(t, root, "know", "show", "release-flow"))

	stdout, stderr, exitCode := runCLI(t, root, "know", "update", "release-flow", "--if-match", etag, "--title", "Release steps")

	if exitCode != ExitOK || !strings.Contains(readNoteFile(t, root, "release-flow.md"), "title: Release steps\n") {
		t.Errorf("know update --if-match %s = %d\n%s%s", etag, exitCode, stdout, stderr)
	}
}

func TestKnowUpdate_StaleIfMatch_ConflictsAndLeavesTheFile(t *testing.T) {
	root := newKnowStore(t)
	etag := textETag(mustRun(t, root, "know", "show", "release-flow"))
	mustRun(t, root, "know", "update", "release-flow", "--if-match", etag, "--title", "First")
	before := readNoteFile(t, root, "release-flow.md")

	stdout, _, exitCode := runCLI(t, root, "know", "update", "release-flow", "--json", "--if-match", etag, "--title", "Second")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeConflict) || failure["path"] != ".capcom/knowledge/release-flow.md" {
		t.Errorf("stale --if-match = %d, error %v; want conflict at the note file", exitCode, failure)
	}
	if after := readNoteFile(t, root, "release-flow.md"); after != before {
		t.Errorf("note file changed to %q", after)
	}
}

func TestKnowUpdate_RefusedEdits_NameTheFileAndWriteNothing(t *testing.T) {
	root := newKnowStore(t)
	before := readNoteFile(t, root, "release-flow.md")
	cases := []struct {
		args       []string
		wantPrefix string
	}{
		{[]string{"--add-ref", "../outside.go"}, "capcom: .capcom/knowledge/release-flow.md: "},
		{[]string{"--title", ""}, "capcom: .capcom/knowledge/release-flow.md:2: "},
		{[]string{"--body-replace-old", "absent", "--body-replace-new", "x"}, "capcom: .capcom/knowledge/release-flow.md: "},
	}

	for _, current := range cases {
		_, stderr, exitCode := runCLI(t, root, append([]string{"know", "update", "release-flow"}, current.args...)...)

		if exitCode != ExitError || !strings.HasPrefix(stderr, current.wantPrefix) {
			t.Errorf("know update %v = %d, stderr %q; want 1 at %q", current.args, exitCode, stderr, current.wantPrefix)
		}
	}
	if after := readNoteFile(t, root, "release-flow.md"); after != before {
		t.Errorf("note file changed to %q", after)
	}
}

func TestKnowUpdate_NoChangeOrClashingFlags_AreUsageErrors(t *testing.T) {
	root := newKnowStore(t)
	cases := [][]string{
		{},
		{"--if-match", "0000000000000000"},
		{"--body-file", "-", "--body-append", "more"},
		{"--body-file", "-", "--body-replace-old", "a", "--body-replace-new", "b"},
	}

	for _, args := range cases {
		_, stderr, exitCode := runCLI(t, root, append([]string{"know", "update", "release-flow"}, args...)...)

		if exitCode != ExitUsage {
			t.Errorf("know update %v = %d, stderr %q; want usage", args, exitCode, stderr)
		}
	}
}

func TestKnowUpdate_TopLevelTitle_RefreshesTheAgentsBlock(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")

	text := mustRun(t, root, "know", "update", "release-flow", "--title", "Release steps")
	jsonOutput := mustRun(t, root, "know", "update", "release-flow", "--title", "Release flow", "--json")

	jsonData, _ := decodeEnvelope(t, jsonOutput)["data"].(map[string]any)
	if !strings.HasSuffix(text, "refreshed: AGENTS.md\n") || jsonData["agents"] != blockRefreshed {
		t.Errorf("know update = %q then %s; want the block refreshed both times", text, jsonOutput)
	}
	if !strings.Contains(readAgentsGuide(t, root), "- `release-flow`: Release flow\n") {
		t.Errorf("AGENTS.md =\n%s\nwant the new title listed", readAgentsGuide(t, root))
	}
	if stdout, _, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after know update = %d\n%s", exitCode, stdout)
	}
}

func TestKnowUpdate_NestedTitle_LeavesAgentsByteIdentical(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")
	before := readAgentsGuide(t, root)

	stdout := mustRun(t, root, "know", "update", "cli-tests", "--title", "CLI test rules")

	if after := readAgentsGuide(t, root); after != before || strings.Contains(stdout, "refreshed") {
		t.Errorf("know update = %q, AGENTS.md changed to\n%s", stdout, after)
	}
}

func TestKnowUpdate_TangledAgentsMarkers_WarnsAndWritesTheNote(t *testing.T) {
	root := newKnowStore(t)
	tangled := "# Guide\n" + agents.BeginMarker + "\n"
	writeAgentsGuide(t, root, tangled)

	stdout, stderr, exitCode := runCLI(t, root, "know", "update", "release-flow", "--title", "Release steps")

	if exitCode != ExitOK || !strings.Contains(stdout, "warning: AGENTS.md:2: ") {
		t.Errorf("know update = %d\n%s%s; want ok with a warning at AGENTS.md:2", exitCode, stdout, stderr)
	}
	if !strings.Contains(readNoteFile(t, root, "release-flow.md"), "title: Release steps\n") {
		t.Errorf("note file was not written")
	}
	if readAgentsGuide(t, root) != tangled {
		t.Errorf("AGENTS.md changed to %q", readAgentsGuide(t, root))
	}
}
