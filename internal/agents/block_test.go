package agents

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// goldenNotes is the Knowledge top level of the golden block.
var goldenNotes = []Note{
	{ID: "release-flow", Title: "Release flow"},
	{ID: "testing-rules", Title: "Testing rules\nand fixtures"},
}

// goldenBlock is render(goldenNotes), byte for byte.
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

Tree lock: before a change that spans several files or commits, it is recommended to claim this
worktree with ` + "`capcom lock acquire --purpose <text>`" + ` and keep the token it prints.
` + "`capcom lock touch --token <token>`" + ` renews it during long work,
` + "`capcom lock release --token <token>`" + ` frees it when done, and acquiring again with ` + "`--token <token>`" + `
resumes it after a restart or compaction. When ` + "`capcom lock check --token <token>`" + ` exits 3, or
` + "`capcom lock check`" + ` does while you hold no token, another session holds this worktree: ask the user
before editing it.

Knowledge tree, top level:

- ` + "`release-flow`" + `: Release flow
- ` + "`testing-rules`" + `: Testing rules and fixtures
<!-- END CAPCOM -->`

// goldenLines is the number of lines goldenBlock spans.
var goldenLines = strings.Count(goldenBlock, "\n") + 1

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
	if err := os.WriteFile(guidePath(root), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return root
}

// readGuideText returns the AGENTS.md of root.
func readGuideText(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(guidePath(root))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

func TestRender_Notes_MatchesGolden(t *testing.T) {
	t.Parallel()

	block := render(goldenNotes)

	if block != goldenBlock {
		t.Errorf("render =\n%s\nwant\n%s", block, goldenBlock)
	}
}

func TestRender_NoNotes_RecommendsTheTreeLock(t *testing.T) {
	t.Parallel()

	block := render(nil)

	for _, want := range []string{"capcom lock acquire --purpose <text>", "--token <token>", "capcom lock check"} {
		if !strings.Contains(block, want) {
			t.Errorf("render(nil) =\n%s\nwant it to contain %q", block, want)
		}
	}
}

func TestRender_NoNotes_SaysSo(t *testing.T) {
	t.Parallel()

	block := render(nil)

	if !strings.HasSuffix(block, "Knowledge tree, top level: no Notes yet.\n"+EndMarker) {
		t.Errorf("render(nil) =\n%s\nwant the no-Notes line before the end marker", block)
	}
}

func TestWrite_ExistingGuideWithoutBlock_AppendsAfterABlankLine(t *testing.T) {
	t.Parallel()
	hand := "# Guide\n\nHand-written rules."
	root := writeGuide(t, hand)

	status, err := write(root, goldenNotes)

	if err != nil || status.State != StateAbsent {
		t.Fatalf("write = %+v, %v; want absent before writing", status, err)
	}
	if got, want := readGuideText(t, root), hand+"\n\n"+goldenBlock+"\n"; got != want {
		t.Errorf("AGENTS.md =\n%q\nwant\n%q", got, want)
	}
}

func TestWrite_StaleBlock_RefreshesInPlaceKeepingSurroundingBytes(t *testing.T) {
	t.Parallel()
	before := "# Guide\n\n  "
	after := "  \n\n## Hand-written tail\n\nKeep  this\ttext.\n"
	root := writeGuide(t, before+render(goldenNotes[:1])+after)

	status, err := write(root, goldenNotes)

	if err != nil || status.State != StateStale || status.Line != 3 {
		t.Fatalf("write = %+v, %v; want stale at line 3", status, err)
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

	status, err := write(root, goldenNotes)

	if err != nil || status.State != StateStale || status.Line != 3 {
		t.Fatalf("write = %+v, %v; want stale at line 3", status, err)
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

	status, err := write(root, goldenNotes)

	if err != nil || status.State != StateCurrent || status.Line != 1 {
		t.Fatalf("write = %+v, %v; want current at line 1", status, err)
	}
	if !os.SameFile(before, mustStat(t, root)) {
		t.Errorf("AGENTS.md was rewritten although its block is current")
	}
}

func TestWrite_MissingGuide_CreatesItWithTheBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	_, err := write(root, goldenNotes)

	if got := readGuideText(t, root); err != nil || got != goldenBlock+"\n" {
		t.Errorf("write err %v, AGENTS.md =\n%q\nwant the block alone", err, got)
	}
}

func TestWrite_CRLFGuide_StaysCRLFAndIsThenCurrent(t *testing.T) {
	t.Parallel()
	crlfOf := func(text string) string { return strings.ReplaceAll(text, "\n", "\r\n") }
	root := writeGuide(t, crlfOf("# Guide\n\n"+render(nil)+"\n\nTail.\n"))

	_, err := write(root, goldenNotes)
	status, inspectErr := inspect(root, goldenNotes)

	if err != nil || inspectErr != nil || status.State != StateCurrent {
		t.Fatalf("write %v, inspect = %+v, %v; want current", err, status, inspectErr)
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
	if err := os.Chmod(guidePath(root), 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	_, err := write(root, goldenNotes)

	if err != nil {
		t.Fatalf("write: %v", err)
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
	if err := os.Symlink("CLAUDE.md", guidePath(root)); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err := write(root, goldenNotes)

	if err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Lstat(guidePath(root))
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

	_, err := write(root, goldenNotes)

	if err == nil || !strings.HasPrefix(err.Error(), FileName+": ") {
		t.Errorf("write err = %v, want an error starting %q", err, FileName+": ")
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
		{"stale", render(nil) + "\n", StateStale},
		{"hand-edited inside", strings.Replace(goldenBlock, "most urgent", "least urgent", 1), StateStale},
		{"without the install line", "# Guide\n\n" + preInstallLineBlock + "\n", StateStale},
		{"before the store map", "# Guide\n\n" + preStoreMapBlock + "\n", StateStale},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, testCase.content)

			status, err := inspect(root, goldenNotes)

			if err != nil || status.State != testCase.wantState {
				t.Errorf("inspect = %+v, %v; want state %d", status, err, testCase.wantState)
			}
		})
	}
}

func TestInspect_MissingGuide_IsAbsent(t *testing.T) {
	t.Parallel()

	status, err := inspect(t.TempDir(), goldenNotes)

	if err != nil || status.State != StateAbsent {
		t.Errorf("inspect = %+v, %v; want absent", status, err)
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
		{"second begin", goldenBlock + "\n\n" + BeginMarker + "\n", goldenLines + 2},
		{"second end", goldenBlock + "\n" + EndMarker + "\n", goldenLines + 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, testCase.content)

			_, err := inspect(root, goldenNotes)

			var failure *Error
			if !errors.As(err, &failure) || failure.Line != testCase.wantLine || failure.Path != FileName {
				t.Errorf("inspect err = %v, want an *Error at %s:%d", err, FileName, testCase.wantLine)
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

	notes, err := topNotes(dir)

	want := []Note{{ID: "release-flow", Title: "Release flow"}, {ID: "testing-rules", Title: "Testing rules"}}
	if err != nil || len(notes) != len(want) || notes[0] != want[0] || notes[1] != want[1] {
		t.Errorf("topNotes = %v, %v; want %v", notes, err, want)
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

	notes, err := topNotes(dir)

	want := Note{ID: "release-flow", Title: "Release flow"}
	if err != nil || len(notes) != 1 || notes[0] != want {
		t.Errorf("topNotes = %v, %v; want [%v]", notes, err, want)
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

	notes, err := topNotes(dir)

	want := Note{ID: "release-flow", Title: "Release flow"}
	if err != nil || len(notes) != 1 || notes[0] != want {
		t.Errorf("topNotes = %v, %v; want [%v]", notes, err, want)
	}
}

func TestTopNotes_DuplicatedTopLevelID_IsLeftOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNoteFiles(t, dir, map[string]string{
		"zeta.md":     "---\ntitle: Zeta\n---\n",
		"zoo.md":      "---\ntitle: Zoo\n---\n",
		"zoo/zeta.md": "---\ntitle: Zeta too\n---\n",
	})

	notes, err := topNotes(dir)

	want := Note{ID: "zoo", Title: "Zoo"}
	if err != nil || len(notes) != 1 || notes[0] != want {
		t.Errorf("topNotes = %v, %v; want [%v]", notes, err, want)
	}
}

func TestTopNotes_SharedAlias_LeavesBothOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNoteFiles(t, dir, map[string]string{
		"alpha-flow.md":   "---\ntitle: Alpha flow\naliases: [old-flow]\n---\n",
		"beta-flow.md":    "---\ntitle: Beta flow\naliases: [old-flow]\n---\n",
		"release-flow.md": "---\ntitle: Release flow\n---\n",
	})

	notes, err := topNotes(dir)

	want := Note{ID: "release-flow", Title: "Release flow"}
	if err != nil || len(notes) != 1 || notes[0] != want {
		t.Errorf("topNotes = %v, %v; want [%v]", notes, err, want)
	}
}

// writeNoteFiles writes each of files, a slash-separated path below dir mapped to its content.
func writeNoteFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
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

	notes, err := topNotes(dir)

	if err != nil || len(notes) != 1 || notes[0] != want[0] {
		t.Errorf("topNotes = %v, %v; want %v", notes, err, want)
	}
}

func TestTopNotes_MissingDirectory_IsEmpty(t *testing.T) {
	t.Parallel()

	notes, err := topNotes(filepath.Join(t.TempDir(), "knowledge"))

	if err != nil || len(notes) != 0 {
		t.Errorf("topNotes = %v, %v; want none", notes, err)
	}
}

// writeKnowledge writes a Knowledge tree holding the one top-level Note release-flow and returns
// its directory, with the Notes topNotes reads from it.
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
	current := "# Guide\n\n" + render(notes) + "\n"
	stale := "# Guide\n\n" + render(nil) + "\n"
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
		{"write without block", ModeWrite, withoutBlock, StateAbsent, withoutBlock + "\n" + render(notes) + "\n"},
		{"write missing", ModeWrite, missing, StateAbsent, render(notes) + "\n"},
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
	content, err := os.ReadFile(guidePath(root))
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
	info, err := os.Stat(guidePath(root))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	return info
}

// linkGuideOutside makes a working tree whose AGENTS.md links to a file holding content in a
// sibling directory outside it, and returns the tree's root and the linked file's resolved path. It
// skips where a symlink test cannot run: on Windows, as root, and where os.Symlink fails.
func linkGuideOutside(t *testing.T, content string) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	base := t.TempDir()
	root := filepath.Join(base, "worktree")
	outsideDir := filepath.Join(base, "outside")
	for _, dir := range []string{root, outsideDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
	}
	outside := filepath.Join(outsideDir, "bashrc")
	if err := os.WriteFile(outside, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(outside, guidePath(root)); err != nil {
		t.Skipf("Symlink: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return root, resolved
}

func TestSync_GuideLinkedOutsideTheTree_FailsAndLeavesTheTargetAlone(t *testing.T) {
	t.Parallel()
	_, notes := writeKnowledge(t)
	current := "# Guide\n\n" + render(notes) + "\n"
	stale := "# Guide\n\n" + render(nil) + "\n"
	const withoutBlock = "# Guide\n"
	cases := []struct {
		name      string
		mode      Mode
		content   string
		isRefused bool
	}{
		{"write without markers", ModeWrite, withoutBlock, true},
		{"refresh stale", ModeRefresh, stale, false},
		{"inspect current", ModeInspect, current, false},
		{"inspect stale", ModeInspect, stale, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			knowledgeDir, _ := writeKnowledge(t)
			root, outside := linkGuideOutside(t, testCase.content)

			status, err := Sync(root, knowledgeDir, testCase.mode)

			if testCase.isRefused {
				want := FileName + ": links to " + outside +
					", outside the working tree; the capcom block is not read or written"
				var failure *Error
				if !errors.As(err, &failure) || failure.Path != FileName || failure.Line != 0 || err.Error() != want {
					t.Errorf("Sync err = %v, want an *Error %q", err, want)
				}
			} else if err != nil || status.State != StateAbsent || status.Line != 0 {
				t.Errorf("Sync = %+v, %v; want StateAbsent unread and no error", status, err)
			}
			content, readErr := os.ReadFile(outside)
			if readErr != nil || string(content) != testCase.content {
				t.Errorf("outside file = %q, %v; want it untouched", content, readErr)
			}
		})
	}
}

// oversizedGuideSize is one byte past the largest guide the guarded reader reads, 1 MiB.
const oversizedGuideSize = 1<<20 + 1

// skipSymlinkTest skips a test that makes a symlink where symlink tests do not run: on Windows and
// as root.
func skipSymlinkTest(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
}

// linkGuideTo makes the AGENTS.md of root a relative link to name, in root too, skipping where
// os.Symlink fails.
func linkGuideTo(t *testing.T, root string, name string) {
	t.Helper()
	if err := os.Symlink(name, guidePath(root)); err != nil {
		t.Skipf("Symlink: %v", err)
	}
}

// writeSizedFile writes size bytes of text to path.
func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// syncWithin runs Sync, failing the test when it has not returned after 5 s, as it does when it
// blocks opening a named pipe.
func syncWithin(t *testing.T, root, knowledgeDir string, mode Mode) (Status, error) {
	t.Helper()
	type outcome struct {
		status Status
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		status, err := Sync(root, knowledgeDir, mode)
		done <- outcome{status, err}
	}()
	select {
	case result := <-done:
		return result.status, result.err
	case <-time.After(5 * time.Second):
		t.Fatal("Sync did not return within 5 s")
		return Status{}, nil
	}
}

// assertDanglingGuideRead checks what Sync in mode did to a working tree whose AGENTS.md is a
// dangling link: it reads as no content, so every mode finds StateAbsent without an error,
// ModeWrite replaces the link with the block for notes, and the other modes leave the link alone.
func assertDanglingGuideRead(t *testing.T, root string, notes []Note, mode Mode, status Status, err error) {
	t.Helper()
	if err != nil || status.State != StateAbsent {
		t.Fatalf("Sync = %+v, %v; want StateAbsent and no error", status, err)
	}
	if mode == ModeWrite {
		if got, want := readGuideText(t, root), render(notes)+"\n"; got != want {
			t.Errorf("AGENTS.md =\n%q\nwant\n%q", got, want)
		}
		return
	}
	if info, statErr := os.Lstat(guidePath(root)); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Lstat AGENTS.md = %v, %v; want the dangling link left alone", info, statErr)
	}
}

func TestSync_IrregularGuide_FailsAsAnAgentsError(t *testing.T) {
	t.Parallel()
	const notRegular = "not a regular file"
	const tooLarge = "file is larger than 1 MiB"
	guides := []struct {
		name        string
		setUp       func(t *testing.T, root string)
		wantMessage string
	}{
		{"named pipe", func(t *testing.T, root string) {
			makeFIFO(t, guidePath(root))
		}, notRegular},
		{"link to an in-tree named pipe", func(t *testing.T, root string) {
			skipSymlinkTest(t)
			makeFIFO(t, filepath.Join(root, "pipe"))
			linkGuideTo(t, root, "pipe")
		}, notRegular},
		{"link to an in-tree file over 1 MiB", func(t *testing.T, root string) {
			skipSymlinkTest(t)
			writeSizedFile(t, filepath.Join(root, "CLAUDE.md"), oversizedGuideSize)
			linkGuideTo(t, root, "CLAUDE.md")
		}, tooLarge},
		{"directory", func(t *testing.T, root string) {
			if err := os.Mkdir(guidePath(root), 0o755); err != nil {
				t.Fatalf("Mkdir: %v", err)
			}
		}, notRegular},
		{"dangling link", func(t *testing.T, root string) {
			skipSymlinkTest(t)
			linkGuideTo(t, root, "missing.md")
		}, ""},
	}
	modes := []struct {
		name string
		mode Mode
	}{
		{"inspect", ModeInspect},
		{"refresh", ModeRefresh},
		{"write", ModeWrite},
	}
	for _, guide := range guides {
		for _, mode := range modes {
			t.Run(guide.name+"/"+mode.name, func(t *testing.T) {
				t.Parallel()
				knowledgeDir, notes := writeKnowledge(t)
				root := t.TempDir()
				guide.setUp(t, root)
				before, err := os.Lstat(guidePath(root))
				if err != nil {
					t.Fatalf("Lstat: %v", err)
				}

				status, err := syncWithin(t, root, knowledgeDir, mode.mode)

				if guide.wantMessage == "" {
					assertDanglingGuideRead(t, root, notes, mode.mode, status, err)
					return
				}
				want := FileName + ": " + guide.wantMessage
				var failure *Error
				if !errors.As(err, &failure) || failure.Path != FileName || failure.Line != 0 || err.Error() != want {
					t.Errorf("Sync err = %v, want an *Error %q", err, want)
				}
				after, err := os.Lstat(guidePath(root))
				if err != nil || after.Mode().Type() != before.Mode().Type() || !os.SameFile(before, after) {
					t.Errorf("AGENTS.md after Sync = %v, %v; want it untouched", after, err)
				}
			})
		}
	}
}
