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
		{"--retire", "--unretire"},
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

// knowUpdateFormatRaisedText is the warning line of the know update that raises a store to format 3.
const knowUpdateFormatRaisedText = "warning: .capcom/config.yaml: store format raised to 3; " +
	"capcom releases that read only format 2 can no longer read this store\n"

func TestKnowUpdate_RetireAndSupersededBy_WriteThenClearTheKeys(t *testing.T) {
	root := newKnowStore(t)
	before := readNoteFile(t, root, "release-flow.md")

	mustRun(t, root, "know", "update", "release-flow", "--retire",
		"--add-superseded-by", "cli-tests", "--add-superseded-by", "git-fixtures")
	retired := readNoteFile(t, root, "release-flow.md")
	mustRun(t, root, "know", "update", "release-flow", "--unretire",
		"--remove-superseded-by", "cli-tests", "--remove-superseded-by", "git-fixtures")

	want := "---\ntitle: Release flow\nlabels: [process]\nretired: true\n" +
		"superseded-by: [cli-tests, git-fixtures]\n---\n\nTag, then publish.\n"
	if retired != want {
		t.Errorf("retired note file = %q; want %q", retired, want)
	}
	if after := readNoteFile(t, root, "release-flow.md"); after != before {
		t.Errorf("cleared note file = %q; want %q", after, before)
	}
}

func TestKnowUpdate_SupersededByAliasOrCitation_StoresTheID(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\naliases: [fixtures]\n---\n",
	})

	mustRun(t, root, "know", "update", "release-flow", "--add-superseded-by", "fixtures",
		"--add-superseded-by", "CLI Tests")

	if got := readNoteFile(t, root, "release-flow.md"); !strings.Contains(got, "superseded-by: [git-fixtures, cli-tests]\n") {
		t.Errorf("note file = %q; want the canonical ids", got)
	}
}

func TestKnowUpdate_RemoveSupersededBy_MatchesTheNamedNoteThenTheText(t *testing.T) {
	cases := []struct {
		name   string
		notes  map[string]string
		stored string
		arg    string
		want   string
	}{
		{
			name:   "alias",
			notes:  map[string]string{"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\naliases: [fixtures]\n---\n"},
			stored: "[git-fixtures, cli-tests]",
			arg:    "fixtures",
			want:   "[cli-tests]",
		},
		{
			name:   "every spelling of the named note",
			notes:  map[string]string{"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\naliases: [fixtures]\n---\n"},
			stored: "[fixtures, cli-tests, git-fixtures]",
			arg:    "git-fixtures",
			want:   "[cli-tests]",
		},
		{
			name: "alias of a retired note",
			notes: map[string]string{
				"old-flow.md": "---\ntitle: Old flow\nretired: true\naliases: [legacy-flow]\n---\n",
			},
			stored: "[legacy-flow, cli-tests]",
			arg:    "old-flow",
			want:   "[cli-tests]",
		},
		{
			name:   "unique id prefix",
			stored: "[git-fixtures, cli-tests]",
			arg:    "git-fix",
			want:   "[cli-tests]",
		},
		{
			name:   "stale entry whose Note is gone",
			stored: "[gone-note, cli-tests]",
			arg:    "gone-note",
			want:   "[cli-tests]",
		},
		{
			name:   "gone entry whose text is a unique prefix of another",
			stored: "[cli, cli-tests]",
			arg:    "cli",
			want:   "[cli-tests]",
		},
		{
			name: "ambiguous prefix that is an entry's text",
			notes: map[string]string{
				"release-a.md": "---\ntitle: Release A\n---\n",
				"release-b.md": "---\ntitle: Release B\n---\n",
			},
			stored: "[release, release-a]",
			arg:    "release",
			want:   "[release-a]",
		},
		{
			name:   "name of no entry",
			stored: "[cli-tests]",
			arg:    "git-fixtures",
			want:   "[cli-tests]",
		},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			root := newKnowStore(t)
			writeNoteFiles(t, root, current.notes)
			writeNoteFiles(t, root, map[string]string{
				"release-flow.md": "---\ntitle: Release flow\nsuperseded-by: " + current.stored + "\n---\n",
			})

			mustRun(t, root, "know", "update", "release-flow", "--remove-superseded-by", current.arg)

			got := readNoteFile(t, root, "release-flow.md")
			if !strings.Contains(got, "superseded-by: "+current.want+"\n") {
				t.Errorf("note file = %q; want superseded-by: %s", got, current.want)
			}
		})
	}
}

func TestKnowUpdate_RemoveSupersededByAmbiguousPrefix_IsRefusedNamingTheCandidates(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"release-a.md":    "---\ntitle: Release A\n---\n",
		"release-b.md":    "---\ntitle: Release B\n---\n",
		"release-flow.md": "---\ntitle: Release flow\nsuperseded-by: [release-a, release-b]\n---\n",
	})
	before := readNoteFile(t, root, "release-flow.md")

	_, stderr, exitCode := runCLI(t, root, "know", "update", "release-flow", "--remove-superseded-by", "release")

	if exitCode != ExitError || !strings.Contains(stderr, "release-a") || !strings.Contains(stderr, "release-b") {
		t.Errorf("exit = %d stderr = %q; want 1 naming the candidates", exitCode, stderr)
	}
	if after := readNoteFile(t, root, "release-flow.md"); after != before {
		t.Errorf("note file changed to %q", after)
	}
}

func TestKnowUpdate_AddThenRemoveTheSameSuccessor_RemovesIt(t *testing.T) {
	cases := []struct {
		name   string
		stored string
	}{
		{name: "not stored", stored: "[cli-tests]"},
		{name: "already stored", stored: "[git-fixtures, cli-tests]"},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			root := newKnowStore(t)
			writeNoteFiles(t, root, map[string]string{
				"release-flow.md": "---\ntitle: Release flow\nsuperseded-by: " + current.stored + "\n---\n",
			})

			mustRun(t, root, "know", "update", "release-flow",
				"--add-superseded-by", "git-fixtures", "--remove-superseded-by", "git-fixtures")

			if got := readNoteFile(t, root, "release-flow.md"); !strings.Contains(got, "superseded-by: [cli-tests]\n") {
				t.Errorf("note file = %q; want superseded-by: [cli-tests]", got)
			}
		})
	}
}

func TestKnowUpdate_RemoveSupersededByDuplicatedName_FailsNamingBothFilesAndWritesNothing(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"foo.md":               "---\ntitle: Foo\n---\n",
		"testing-rules/foo.md": "---\ntitle: Other foo\n---\n",
		"release-flow.md":      "---\ntitle: Release flow\nsuperseded-by: [foo, cli-tests]\n---\n",
	})
	before := readNoteFile(t, root, "release-flow.md")

	_, stderr, exitCode := runCLI(t, root, "know", "update", "release-flow", "--remove-superseded-by", "foo")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/knowledge/foo.md", ".capcom/knowledge/testing-rules/foo.md")
	if after := readNoteFile(t, root, "release-flow.md"); after != before {
		t.Errorf("note file changed to %q", after)
	}
}

func TestKnowUpdate_AddSupersededByBesideItsAliasSpelling_AddsNothing(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"testing-rules/git-fixtures.md": "---\ntitle: Git fixtures\naliases: [fixtures]\n---\n",
		"release-flow.md":               "---\ntitle: Release flow\nsuperseded-by: [fixtures]\n---\n",
	})
	before := readNoteFile(t, root, "release-flow.md")

	mustRun(t, root, "know", "update", "release-flow", "--add-superseded-by", "git-fixtures")

	if after := readNoteFile(t, root, "release-flow.md"); after != before {
		t.Errorf("note file = %q; want it unchanged, %q", after, before)
	}
}

func TestKnowUpdate_UnknownOrSelfSuccessor_IsRefusedAndWritesNothing(t *testing.T) {
	root := newKnowStore(t)
	writeStoreConfig(t, root, "format: 2\n")
	before := readNoteFile(t, root, "release-flow.md")
	cases := []struct {
		args     []string
		wantCode int
	}{
		{[]string{"--add-superseded-by", "absent"}, ExitNotFound},
		{[]string{"--add-superseded-by", "release-flow"}, ExitError},
		{[]string{"--retire", "--add-superseded-by", "release"}, ExitError},
	}

	for _, current := range cases {
		_, stderr, exitCode := runCLI(t, root, append([]string{"know", "update", "release-flow"}, current.args...)...)

		if exitCode != current.wantCode || !strings.HasPrefix(stderr, "capcom: .capcom/knowledge/release-flow.md: ") {
			t.Errorf("know update %v = %d, stderr %q; want %d at the note file", current.args, exitCode, stderr, current.wantCode)
		}
	}
	if after := readNoteFile(t, root, "release-flow.md"); after != before {
		t.Errorf("note file changed to %q", after)
	}
	if config := readStoreConfig(t, root); config != "format: 2\n" {
		t.Errorf("config.yaml = %q; want it left at format 2", config)
	}
}

func TestKnowUpdate_FormatTwoStore_RaisedToThreeOnceWithAWarning(t *testing.T) {
	root := newKnowStore(t)
	writeStoreConfig(t, root, "format: 2\n")

	first := mustRun(t, root, "know", "update", "release-flow", "--retire")
	second := mustRun(t, root, "know", "update", "cli-tests", "--add-superseded-by", "git-fixtures")

	if !strings.Contains(first, knowUpdateFormatRaisedText) || strings.Contains(second, "warning:") {
		t.Errorf("first = %q\nsecond = %q; want the warning on the first only", first, second)
	}
	if config := readStoreConfig(t, root); config != "format: 3\n" {
		t.Errorf("config.yaml = %q; want format 3", config)
	}
}

func TestKnowUpdate_FormatTwoStore_SupersededByAdd_RaisesToThree(t *testing.T) {
	root := newKnowStore(t)
	writeStoreConfig(t, root, "format: 2\n")

	stdout := mustRun(t, root, "know", "update", "release-flow", "--add-superseded-by", "cli-tests")

	if config := readStoreConfig(t, root); config != "format: 3\n" || !strings.Contains(stdout, knowUpdateFormatRaisedText) {
		t.Errorf("config.yaml = %q stdout = %q", config, stdout)
	}
}

func TestKnowUpdate_WritesCarryingAKeyLine_RaiseTheConfigOnceWhateverTheyGain(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"release-flow.md": "---\ntitle: Release flow\nretired: true\nsuperseded-by: [cli-tests]\n---\n",
	})
	writeStoreConfig(t, root, "format: 2\n")
	staleETag := "0000000000000000"
	cases := []struct {
		args       []string
		wantCode   int
		isWarned   bool
		wantConfig string
	}{
		// A refused edit writes nothing, so it raises nothing, whatever lines the file carries.
		{[]string{"--retire", "--if-match", staleETag}, ExitError, false, "format: 2\n"},
		{[]string{"--retire", "--title", ""}, ExitError, false, "format: 2\n"},
		// The first write gains no key value, but the file it writes carries both format-3 lines.
		{[]string{"--retire", "--add-superseded-by", "cli-tests"}, ExitOK, true, "format: 3\n"},
		{[]string{"--remove-superseded-by", "cli-tests"}, ExitOK, false, "format: 3\n"},
		{[]string{"--unretire"}, ExitOK, false, "format: 3\n"},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"know", "update", "release-flow"}, current.args...)...)

		warnings := strings.Count(stdout, knowUpdateFormatRaisedText)
		if exitCode != current.wantCode || strings.Contains(stdout, "warning:") != current.isWarned ||
			(current.isWarned && warnings != 1) {
			t.Errorf("know update %v = %d\n%s%s; want %d, warned %t once",
				current.args, exitCode, stdout, stderr, current.wantCode, current.isWarned)
		}
		if config := readStoreConfig(t, root); config != current.wantConfig {
			t.Fatalf("know update %v: config.yaml = %q; want %q", current.args, config, current.wantConfig)
		}
	}
}
