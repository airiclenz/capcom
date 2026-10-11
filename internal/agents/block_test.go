package agents

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenNotes is the Knowledge top level of the golden block.
var goldenNotes = []Note{
	{ID: "release-flow", Title: "Release flow"},
	{ID: "testing-rules", Title: "Testing rules\nand fixtures"},
}

// goldenBlock is Render(goldenNotes), byte for byte.
const goldenBlock = `<!-- BEGIN CAPCOM -->
<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under ` + "`.capcom/`" + `, versioned with the code; track work there and nowhere else.

- ` + "`capcom ready`" + `: the issues ready to work on, most urgent first
- ` + "`capcom show <id>`" + `: one issue in full
- ` + "`capcom create --id <slug> --title <title>`" + `: file a new issue
- ` + "`capcom close <id> --reason <text>`" + `: close an issue
- ` + "`capcom know tree`" + `: the outline of the Knowledge tree; ` + "`capcom know show <id>`" + ` reads a Note

Knowledge tree, top level:

- ` + "`release-flow`" + `: Release flow
- ` + "`testing-rules`" + `: Testing rules and fixtures
<!-- END CAPCOM -->`

// writeGuide writes content as the AGENTS.md of a new temp working tree and returns the tree.
func writeGuide(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(Path(root), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return root
}

// readGuideText returns the AGENTS.md of root.
func readGuideText(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

func TestRender_Notes_MatchesGolden(t *testing.T) {
	t.Parallel()

	block := Render(goldenNotes)

	if block != goldenBlock {
		t.Errorf("Render =\n%s\nwant\n%s", block, goldenBlock)
	}
}

func TestRender_NoNotes_SaysSo(t *testing.T) {
	t.Parallel()

	block := Render(nil)

	if !strings.HasSuffix(block, "Knowledge tree, top level: no Notes yet.\n"+EndMarker) {
		t.Errorf("Render(nil) =\n%s\nwant the no-Notes line before the end marker", block)
	}
}

func TestWrite_ExistingGuideWithoutBlock_AppendsAfterABlankLine(t *testing.T) {
	t.Parallel()
	hand := "# Guide\n\nHand-written rules."
	root := writeGuide(t, hand)

	status, err := Write(root, goldenNotes)

	if err != nil || status.State != StateAbsent {
		t.Fatalf("Write = %+v, %v; want absent before writing", status, err)
	}
	if got, want := readGuideText(t, root), hand+"\n\n"+goldenBlock+"\n"; got != want {
		t.Errorf("AGENTS.md =\n%q\nwant\n%q", got, want)
	}
}

func TestWrite_StaleBlock_RefreshesInPlaceKeepingSurroundingBytes(t *testing.T) {
	t.Parallel()
	before := "# Guide\n\n  "
	after := "  \n\n## Hand-written tail\n\nKeep  this\ttext.\n"
	root := writeGuide(t, before+Render(goldenNotes[:1])+after)

	status, err := Write(root, goldenNotes)

	if err != nil || status.State != StateStale || status.Line != 3 {
		t.Fatalf("Write = %+v, %v; want stale at line 3", status, err)
	}
	if got, want := readGuideText(t, root), before+goldenBlock+after; got != want {
		t.Errorf("AGENTS.md =\n%q\nwant\n%q", got, want)
	}
}

func TestWrite_CurrentBlock_LeavesTheFileAlone(t *testing.T) {
	t.Parallel()
	root := writeGuide(t, goldenBlock+"\n")
	before := mustStat(t, root)

	status, err := Write(root, goldenNotes)

	if err != nil || status.State != StateCurrent || status.Line != 1 {
		t.Fatalf("Write = %+v, %v; want current at line 1", status, err)
	}
	if !os.SameFile(before, mustStat(t, root)) {
		t.Errorf("AGENTS.md was rewritten although its block is current")
	}
}

func TestWrite_MissingGuide_CreatesItWithTheBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	_, err := Write(root, goldenNotes)

	if got := readGuideText(t, root); err != nil || got != goldenBlock+"\n" {
		t.Errorf("Write err %v, AGENTS.md =\n%q\nwant the block alone", err, got)
	}
}

func TestWrite_CRLFGuide_StaysCRLFAndIsThenCurrent(t *testing.T) {
	t.Parallel()
	crlfOf := func(text string) string { return strings.ReplaceAll(text, "\n", "\r\n") }
	root := writeGuide(t, crlfOf("# Guide\n\n"+Render(nil)+"\n\nTail.\n"))

	_, err := Write(root, goldenNotes)
	status, inspectErr := Inspect(root, goldenNotes)

	if err != nil || inspectErr != nil || status.State != StateCurrent {
		t.Fatalf("Write %v, Inspect = %+v, %v; want current", err, status, inspectErr)
	}
	if got, want := readGuideText(t, root), crlfOf("# Guide\n\n"+goldenBlock+"\n\nTail.\n"); got != want {
		t.Errorf("AGENTS.md =\n%q\nwant\n%q", got, want)
	}
}

func TestInspect_States(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		content   string
		wantState State
	}{
		{"no markers", "# Guide\n", StateAbsent},
		{"current", "# Guide\n\n" + goldenBlock + "\n", StateCurrent},
		{"current in CRLF", strings.ReplaceAll(goldenBlock, "\n", "\r\n") + "\r\n", StateCurrent},
		{"stale", Render(nil) + "\n", StateStale},
		{"hand-edited inside", strings.Replace(goldenBlock, "most urgent", "least urgent", 1), StateStale},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, testCase.content)

			status, err := Inspect(root, goldenNotes)

			if err != nil || status.State != testCase.wantState {
				t.Errorf("Inspect = %+v, %v; want state %d", status, err, testCase.wantState)
			}
		})
	}
}

func TestInspect_MissingGuide_IsAbsent(t *testing.T) {
	t.Parallel()

	status, err := Inspect(t.TempDir(), goldenNotes)

	if err != nil || status.State != StateAbsent {
		t.Errorf("Inspect = %+v, %v; want absent", status, err)
	}
}

func TestInspect_TangledMarkers_FailAtTheMarkerLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		content  string
		wantLine int
	}{
		{"never closed", "# Guide\n" + BeginMarker + "\ntext\n", 2},
		{"end first", EndMarker + "\n" + BeginMarker + "\n", 1},
		{"second begin", goldenBlock + "\n\n" + BeginMarker + "\n", 20},
		{"second end", goldenBlock + "\n" + EndMarker + "\n", 19},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, testCase.content)

			_, err := Inspect(root, goldenNotes)

			var failure *Error
			if !errors.As(err, &failure) || failure.Line != testCase.wantLine || failure.Path != FileName {
				t.Errorf("Inspect err = %v, want an *Error at %s:%d", err, FileName, testCase.wantLine)
			}
		})
	}
}

func TestTopNotes_KnowledgeTree_ListsTopLevelInIDOrderSkippingUnparsable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := map[string]string{
		"testing-rules.md":           "---\ntitle: Testing rules\n---\n",
		"testing-rules/cli-tests.md": "---\ntitle: CLI tests\n---\n",
		"release-flow.md":            "---\ntitle: Release flow\n---\n",
		"broken-note.md":             "no frontmatter\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	notes, err := TopNotes(dir)

	want := []Note{{ID: "release-flow", Title: "Release flow"}, {ID: "testing-rules", Title: "Testing rules"}}
	if err != nil || len(notes) != len(want) || notes[0] != want[0] || notes[1] != want[1] {
		t.Errorf("TopNotes = %v, %v; want %v", notes, err, want)
	}
}

func TestTopNotes_MissingDirectory_IsEmpty(t *testing.T) {
	t.Parallel()

	notes, err := TopNotes(filepath.Join(t.TempDir(), "knowledge"))

	if err != nil || len(notes) != 0 {
		t.Errorf("TopNotes = %v, %v; want none", notes, err)
	}
}

// mustStat returns the file info of root's AGENTS.md; a rewrite replaces the file, so os.SameFile
// tells a rewritten AGENTS.md from an untouched one.
func mustStat(t *testing.T, root string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(Path(root))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	return info
}
