package agents

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

Without a ` + "`capcom`" + ` binary, install it from https://github.com/airiclenz/capcom#install.

- ` + "`capcom ready`" + `: the issues ready to work on, most urgent first
- ` + "`capcom show <id>`" + `: one issue in full, with what blocks it
- ` + "`capcom create --id <slug> --title <title> [--dep <id>]`" + `: file a new issue
- ` + "`capcom dep add <id> <dep-id>`" + `: make an issue wait for another; ` + "`capcom dep list <id>`" + ` shows both sides
- ` + "`capcom close <id> --reason <text>`" + `: close an issue; it names the issues this made ready
- ` + "`capcom list --tree --depth 2`" + `: the outline of the issues
- ` + "`capcom know tree`" + `: the outline of the Knowledge tree; ` + "`capcom know show <id>`" + ` reads a Note

An issue's body says what is wrong or wanted, why, and how to tell it is done (` + "`## Acceptance`" + `),
readable on its own; a plan says how and in what order.

Where things are: an issue is ` + "`.capcom/issues/<id>.md`" + `, and its children sit in the directory
` + "`.capcom/issues/<id>/`" + ` beside it; Notes follow the same shape under ` + "`.capcom/knowledge/`" + `.
Prefer capcom commands; after a hand edit run ` + "`capcom check`" + `.

Knowledge tree, top level:

- ` + "`release-flow`" + `: Release flow
- ` + "`testing-rules`" + `: Testing rules and fixtures
<!-- END CAPCOM -->`

// installLine is the paragraph that points an agent without a capcom binary at its source.
const installLine = "Without a `capcom` binary, install it from https://github.com/airiclenz/capcom#install."

// preInstallLineBlock is goldenBlock as capcom rendered it before the block gained installLine.
const preInstallLineBlock = `<!-- BEGIN CAPCOM -->
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

// preStoreMapBlock is goldenBlock as capcom rendered it before the block gained the dependency
// commands, the issue-body rule and the store map.
const preStoreMapBlock = `<!-- BEGIN CAPCOM -->
<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under ` + "`.capcom/`" + `, versioned with the code; track work there and nowhere else.

Without a ` + "`capcom`" + ` binary, install it from https://github.com/airiclenz/capcom#install.

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

func TestWrite_BlockWithoutInstallLine_PutsItInsideTheMarkersKeepingSurroundingBytes(t *testing.T) {
	t.Parallel()
	before := "# Guide\n\n  "
	after := "  \n\n## Hand-written tail\n\nKeep  this\ttext.\n"
	root := writeGuide(t, before+preInstallLineBlock+after)

	status, err := Write(root, goldenNotes)

	if err != nil || status.State != StateStale || status.Line != 3 {
		t.Fatalf("Write = %+v, %v; want stale at line 3", status, err)
	}
	got := readGuideText(t, root)
	if want := before + goldenBlock + after; got != want {
		t.Errorf("AGENTS.md =\n%q\nwant\n%q", got, want)
	}
	begin, line, end := strings.Index(got, BeginMarker), strings.Index(got, installLine), strings.Index(got, EndMarker)
	if begin < 0 || line < begin || end < line {
		t.Errorf("install line at %d, want between the markers at %d and %d", line, begin, end)
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

func TestWrite_PrivateGuide_KeepsItsMode(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not kept on Windows")
	}
	root := writeGuide(t, "# Guide\n")
	if err := os.Chmod(Path(root), 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	_, err := Write(root, goldenNotes)

	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := mustStat(t, root).Mode().Perm(); got != 0o600 {
		t.Errorf("AGENTS.md mode = %o, want 600", got)
	}
}

func TestWrite_SymlinkedGuide_StaysALinkAndItsTargetGetsTheBlock(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	root := t.TempDir()
	target := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(target, []byte("# Guide\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink("CLAUDE.md", Path(root)); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err := Write(root, goldenNotes)

	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Lstat(Path(root))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Lstat AGENTS.md = %v, %v; want a symlink", info, err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got, want := string(content), "# Guide\n\n"+goldenBlock+"\n"; got != want {
		t.Errorf("CLAUDE.md =\n%q\nwant\n%q", got, want)
	}
}

func TestWrite_ReadOnlyDirectory_FailsNamingTheGuideAndLeavesItIntact(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission tests do not run as root")
	}
	const original = "# Guide\n"
	root := writeGuide(t, original)
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	_, err := Write(root, goldenNotes)

	if err == nil || !strings.HasPrefix(err.Error(), FileName+": ") {
		t.Errorf("Write err = %v, want an error starting %q", err, FileName+": ")
	}
	if got := readGuideText(t, root); got != original {
		t.Errorf("AGENTS.md = %q, want it intact as %q", got, original)
	}
	temps, globErr := filepath.Glob(filepath.Join(root, "."+FileName+".tmp-*"))
	if globErr != nil || len(temps) != 0 {
		t.Errorf("temp files = %v, %v; want none", temps, globErr)
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
		{"without the install line", "# Guide\n\n" + preInstallLineBlock + "\n", StateStale},
		{"before the store map", "# Guide\n\n" + preStoreMapBlock + "\n", StateStale},
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
		{"second begin", goldenBlock + "\n\n" + BeginMarker + "\n", 31},
		{"second end", goldenBlock + "\n" + EndMarker + "\n", 30},
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

func TestTopNotes_RetiredTopLevelNote_IsLeftOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := map[string]string{
		"old-flow.md":             "---\ntitle: Old flow\nretired: true\n---\n",
		"old-flow/step-one.md":    "---\ntitle: Step one\n---\n",
		"release-flow.md":         "---\ntitle: Release flow\nretired: false\n---\n",
		"release-flow/retired.md": "---\ntitle: Retired child\nretired: true\n---\n",
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

	want := Note{ID: "release-flow", Title: "Release flow"}
	if err != nil || len(notes) != 1 || notes[0] != want {
		t.Errorf("TopNotes = %v, %v; want [%v]", notes, err, want)
	}
}

func TestTopNotes_BackgroundTopLevelNote_IsLeftOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := map[string]string{
		"rejected-options.md": "---\ntitle: Rejected options\nlabels: [adr, background]\n---\n",
		"release-flow.md":     "---\ntitle: Release flow\nlabels: [process]\n---\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	notes, err := TopNotes(dir)

	want := Note{ID: "release-flow", Title: "Release flow"}
	if err != nil || len(notes) != 1 || notes[0] != want {
		t.Errorf("TopNotes = %v, %v; want [%v]", notes, err, want)
	}
}

func TestTopNotes_SymlinkedTopLevelNote_IsLeftOutWithoutError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	dir, want := writeKnowledge(t)
	target := filepath.Join(t.TempDir(), "linked-flow.md")
	if err := os.WriteFile(target, []byte("---\ntitle: Linked flow\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	link := filepath.Join(dir, "linked-flow.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%s): %v", link, err)
	}

	notes, err := TopNotes(dir)

	if err != nil || len(notes) != 1 || notes[0] != want[0] {
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

// writeKnowledge writes a Knowledge tree holding the one top-level Note release-flow and returns
// its directory, with the Notes TopNotes reads from it.
func writeKnowledge(t *testing.T) (string, []Note) {
	t.Helper()
	dir := t.TempDir()
	note := []byte("---\ntitle: Release flow\n---\n")
	if err := os.WriteFile(filepath.Join(dir, "release-flow.md"), note, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return dir, []Note{{ID: "release-flow", Title: "Release flow"}}
}

func TestSync_EachModeAndState_ReportsTheStateFoundAndWritesOnlyWhatTheModeAllows(t *testing.T) {
	t.Parallel()
	_, notes := writeKnowledge(t)
	current := "# Guide\n\n" + Render(notes) + "\n"
	stale := "# Guide\n\n" + Render(nil) + "\n"
	const withoutBlock = "# Guide\n"
	const missing = ""
	cases := []struct {
		name        string
		mode        Mode
		content     string
		wantState   State
		wantContent string
	}{
		{"inspect current", ModeInspect, current, StateCurrent, current},
		{"inspect stale", ModeInspect, stale, StateStale, stale},
		{"inspect without block", ModeInspect, withoutBlock, StateAbsent, withoutBlock},
		{"inspect missing", ModeInspect, missing, StateAbsent, missing},
		{"refresh current", ModeRefresh, current, StateCurrent, current},
		{"refresh stale", ModeRefresh, stale, StateStale, current},
		{"refresh without block", ModeRefresh, withoutBlock, StateAbsent, withoutBlock},
		{"refresh missing", ModeRefresh, missing, StateAbsent, missing},
		{"write current", ModeWrite, current, StateCurrent, current},
		{"write stale", ModeWrite, stale, StateStale, current},
		{"write without block", ModeWrite, withoutBlock, StateAbsent, withoutBlock + "\n" + Render(notes) + "\n"},
		{"write missing", ModeWrite, missing, StateAbsent, Render(notes) + "\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			knowledgeDir, _ := writeKnowledge(t)
			root := t.TempDir()
			if testCase.content != missing {
				root = writeGuide(t, testCase.content)
			}

			status, err := Sync(root, knowledgeDir, testCase.mode)

			if err != nil || status.State != testCase.wantState {
				t.Fatalf("Sync = %+v, %v; want state %d", status, err, testCase.wantState)
			}
			if got := readGuideOrEmpty(t, root); got != testCase.wantContent {
				t.Errorf("AGENTS.md =\n%q\nwant\n%q", got, testCase.wantContent)
			}
		})
	}
}

func TestSync_TangledMarkers_FailsAtTheMarkerLineWithoutWriting(t *testing.T) {
	t.Parallel()
	knowledgeDir, _ := writeKnowledge(t)
	content := "# Guide\n" + BeginMarker + "\ntext\n"
	root := writeGuide(t, content)

	_, err := Sync(root, knowledgeDir, ModeWrite)

	var failure *Error
	if !errors.As(err, &failure) || failure.Line != 2 {
		t.Errorf("Sync err = %v, want an *Error at %s:2", err, FileName)
	}
	if got := readGuideText(t, root); got != content {
		t.Errorf("AGENTS.md =\n%q\nwant it untouched", got)
	}
}

// readGuideOrEmpty returns the AGENTS.md of root, or "" when there is none.
func readGuideOrEmpty(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(Path(root))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
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
