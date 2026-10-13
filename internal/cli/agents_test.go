package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/agents"
)

// writeAgentsGuide writes content as the AGENTS.md of the working tree at root.
func writeAgentsGuide(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, agents.FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// readAgentsGuide returns the AGENTS.md of the working tree at root.
func readAgentsGuide(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, agents.FileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

// agentsBlockOf returns the Managed block of guide, markers included.
func agentsBlockOf(t *testing.T, guide string) string {
	t.Helper()
	start := strings.Index(guide, agents.BeginMarker)
	end := strings.Index(guide, agents.EndMarker)
	if start < 0 || end < start {
		t.Fatalf("no capcom block in:\n%s", guide)
	}
	return guide[start : end+len(agents.EndMarker)]
}

func TestAgents_AppendsThenRefreshesInPlace(t *testing.T) {
	root := newKnowStore(t)
	head := "# Guide\n\nHand-written rules.\n"
	writeAgentsGuide(t, root, head)

	first := mustRun(t, root, "agents")
	appended := readAgentsGuide(t, root)
	tail := "\n## Afterword\n\nKeep  me.\n"
	writeAgentsGuide(t, root, appended+tail)
	writeNoteFiles(t, root, map[string]string{"api-design.md": "---\ntitle: API design\n---\n"})
	second := mustRun(t, root, "agents")
	refreshed := readAgentsGuide(t, root)

	if first != "wrote the capcom block to AGENTS.md\n" || second != "refreshed the capcom block of AGENTS.md\n" {
		t.Errorf("agents output = %q then %q", first, second)
	}
	block := agentsBlockOf(t, refreshed)
	if refreshed != head+"\n"+block+"\n"+tail {
		t.Errorf("AGENTS.md =\n%s\nwant the hand-written text around the block byte for byte", refreshed)
	}
	for _, line := range []string{"- `api-design`: API design\n", "- `release-flow`: Release flow\n", "- `testing-rules`: Testing rules\n"} {
		if !strings.Contains(block, line) {
			t.Errorf("block lacks %q:\n%s", line, block)
		}
	}
	if strings.Contains(block, "cli-tests") {
		t.Errorf("block lists a nested Note:\n%s", block)
	}
}

func TestAgents_CRLFGuide_StaysCRLFAndIsNotStale(t *testing.T) {
	root := newKnowStore(t)
	writeAgentsGuide(t, root, "# Guide\r\n\r\nRules.\r\n")

	mustRun(t, root, "agents")
	stdout, stderr, exitCode := runCLI(t, root, "agents", "--check")

	guide := readAgentsGuide(t, root)
	if strings.Count(guide, "\n") != strings.Count(guide, "\r\n") {
		t.Errorf("AGENTS.md has bare LF line endings:\n%q", guide)
	}
	if exitCode != ExitOK || stderr != "" || stdout != "the capcom block of AGENTS.md is current\n" {
		t.Errorf("agents --check = %d, stdout %q, stderr %q; want current", exitCode, stdout, stderr)
	}
	if _, _, checkExit := runCLI(t, root, "check"); checkExit != ExitOK {
		t.Errorf("check exit = %d, want 0 for a current CRLF block", checkExit)
	}
}

func TestAgentsCheck_StaleBlock_FailsAtTheMarkerLine(t *testing.T) {
	root := newKnowStore(t)
	writeAgentsGuide(t, root, "# Guide\n")
	mustRun(t, root, "agents")
	writeAgentsGuide(t, root, strings.Replace(readAgentsGuide(t, root), "`: Release flow\n", "`: Release steps\n", 1))

	stdout, stderr, exitCode := runCLI(t, root, "agents", "--check")

	if exitCode != ExitError || stdout != "" || !strings.HasPrefix(stderr, "capcom: AGENTS.md:3: the capcom block is out of date") {
		t.Errorf("agents --check = %d, stdout %q, stderr %q; want 1 at AGENTS.md:3", exitCode, stdout, stderr)
	}
}

func TestCheck_AgentsBlockWithoutInstallLine_IsStaleUntilAgentsRefreshesIt(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")
	installParagraph := "Without a `capcom` binary, install it from https://github.com/airiclenz/capcom#install.\n\n"
	current := readAgentsGuide(t, root)
	writeAgentsGuide(t, root, strings.Replace(current, installParagraph, "", 1))

	stdout, _, exitCode := runCLI(t, root, "check", "--json")
	_, _, refreshExit := runCLI(t, root, "agents")
	_, _, afterExit := runCLI(t, root, "check")

	data, _ := decodeEnvelope(t, stdout)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("check --json on the pre-install-line block = %d, data %v; want one problem", exitCode, data)
	}
	if problem, _ := data[0].(map[string]any); exitCode != ExitError || problem["code"] != "agents-block" || problem["line"] != float64(1) {
		t.Errorf("check --json on the pre-install-line block = %d, data %v; want 1 and one agents-block at line 1", exitCode, data)
	}
	if refreshExit != ExitOK || afterExit != ExitOK || readAgentsGuide(t, root) != current {
		t.Errorf("agents = %d, then check = %d; want 0, 0 and the block with the install line back", refreshExit, afterExit)
	}
}

func TestCheck_AgentsBlockWithoutStoreMap_IsStaleUntilAgentsRefreshesIt(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")
	storeMapParagraph := "Where things are: an issue is `.capcom/issues/<id>.md`, and its children sit in the directory\n" +
		"`.capcom/issues/<id>/` beside it; Notes follow the same shape under `.capcom/knowledge/`.\n" +
		"Prefer capcom commands; after a hand edit run `capcom check`.\n\n"
	current := readAgentsGuide(t, root)
	if !strings.Contains(current, storeMapParagraph) {
		t.Fatalf("agents wrote a block without the store map:\n%s", current)
	}
	writeAgentsGuide(t, root, strings.Replace(current, storeMapParagraph, "", 1))

	_, _, checkExit := runCLI(t, root, "check")
	_, _, refreshExit := runCLI(t, root, "agents")
	_, _, afterExit := runCLI(t, root, "check")

	if checkExit != ExitError || refreshExit != ExitOK || afterExit != ExitOK || readAgentsGuide(t, root) != current {
		t.Errorf("check = %d, agents = %d, then check = %d; want 1, 0, 0 and the block with the store map back",
			checkExit, refreshExit, afterExit)
	}
}

func TestAgentsCheck_NoGuideOrNoBlock_ExitsZero(t *testing.T) {
	root := newKnowStore(t)

	missing, _, missingExit := runCLI(t, root, "agents", "--check")
	_, statErr := os.Stat(filepath.Join(root, agents.FileName))
	writeAgentsGuide(t, root, "# Guide\n")
	plain, _, plainExit := runCLI(t, root, "agents", "--check")

	want := "AGENTS.md has no capcom block\n"
	if missingExit != ExitOK || plainExit != ExitOK || missing != want || plain != want {
		t.Errorf("agents --check = %d %q, then %d %q; want 0 and %q", missingExit, missing, plainExit, plain, want)
	}
	if !os.IsNotExist(statErr) || readAgentsGuide(t, root) != "# Guide\n" {
		t.Errorf("agents --check wrote AGENTS.md (stat before: %v)", statErr)
	}
}

func TestAgents_TangledMarkers_FailAtTheirLine(t *testing.T) {
	root := newKnowStore(t)
	writeAgentsGuide(t, root, "# Guide\n"+agents.BeginMarker+"\n")

	_, stderr, exitCode := runCLI(t, root, "agents")

	if exitCode != ExitError || !strings.HasPrefix(stderr, "capcom: AGENTS.md:2: ") {
		t.Errorf("agents = %d, stderr %q; want 1 at AGENTS.md:2", exitCode, stderr)
	}
}

func TestAgents_GuideLinkedOutsideTheTree_FailsNamingTheLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	root := newKnowStore(t)
	const original = "# Shell profile\n"
	outside := filepath.Join(t.TempDir(), "bashrc")
	if err := os.WriteFile(outside, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, agents.FileName)); err != nil {
		t.Skipf("Symlink: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "agents")

	want := "capcom: AGENTS.md: links to " + resolved +
		", outside the working tree; the capcom block is not read or written\n"
	if exitCode != ExitError || stdout != "" || stderr != want {
		t.Errorf("agents = %d, stdout %q, stderr %q; want 1 and %q", exitCode, stdout, stderr, want)
	}
	if content, readErr := os.ReadFile(outside); readErr != nil || string(content) != original {
		t.Errorf("outside file = %q, %v; want it untouched", content, readErr)
	}
}

func TestCheck_OversizedAgentsGuide_ReportsAnAgentsBlockProblem(t *testing.T) {
	root := newKnowStore(t)
	writeAgentsGuide(t, root, strings.Repeat("x", 1<<20+1))

	stdout, _, exitCode := runCLI(t, root, "check")

	if exitCode != ExitError || !strings.Contains(stdout, "AGENTS.md: file is larger than 1 MiB\n") {
		t.Errorf("check = %d, stdout:\n%s\nwant 1 and the line %q", exitCode, stdout, "AGENTS.md: file is larger than 1 MiB")
	}
}
