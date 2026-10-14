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

func TestAgents_UnusableTopLevelNote_IsInNeitherTheBlockNorKnowTreeAndCheckReportsIt(t *testing.T) {
	notes := map[string]string{
		"empty-label.md":  "---\ntitle: Empty label\nlabels: [\"\"]\n---\n",
		"pathless-ref.md": "---\ntitle: Pathless ref\nrefs: [\"#symbol\"]\n---\n",
	}
	root := newKnowStore(t)
	writeNoteFiles(t, root, notes)

	mustRun(t, root, "agents")
	block := agentsBlockOf(t, readAgentsGuide(t, root))
	outline := mustRun(t, root, "know", "tree")
	report, _, checkExit := runCLI(t, root, "check")

	if !strings.Contains(block, "- `release-flow`: Release flow\n") {
		t.Errorf("block lacks the usable Note release-flow:\n%s", block)
	}
	if !strings.Contains(outline, "release-flow  Release flow\n") {
		t.Errorf("know tree lacks the usable Note release-flow:\n%s", outline)
	}
	if checkExit != ExitError {
		t.Errorf("check exit = %d; want %d\n%s", checkExit, ExitError, report)
	}
	for name := range notes {
		id := strings.TrimSuffix(name, ".md")
		if strings.Contains(block, id) {
			t.Errorf("block lists the unusable Note %s:\n%s", id, block)
		}
		if strings.Contains(outline, id+"  ") {
			t.Errorf("know tree lists the unusable Note %s:\n%s", id, outline)
		}
		if !strings.Contains(report, ".capcom/knowledge/"+name+":") {
			t.Errorf("check does not report %s:\n%s", name, report)
		}
	}
}

// writeGuideFile writes content as the agent guide name of the working tree at root.
func writeGuideFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// readGuideFile returns the agent guide name of the working tree at root.
func readGuideFile(t *testing.T, root string, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

func TestAgents_UnreachedGuide_GetsTheImportOnceAndTheBlockStaysInAgentsMD(t *testing.T) {
	root := newKnowStore(t)
	writeAgentsGuide(t, root, "# Guide\n")
	writeGuideFile(t, root, "CLAUDE.md", "# Claude\n")

	first := mustRun(t, root, "agents")
	imported := readGuideFile(t, root, "CLAUDE.md")
	second := mustRun(t, root, "agents")

	if first != "wrote the capcom block to AGENTS.md\nadded @AGENTS.md to CLAUDE.md\n" {
		t.Errorf("agents output = %q; want the block written and the import added", first)
	}
	if imported != "# Claude\n\n@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q; want the import line appended after a blank line", imported)
	}
	if second != "the capcom block of AGENTS.md is current\n" || readGuideFile(t, root, "CLAUDE.md") != imported {
		t.Errorf("agents re-run = %q, CLAUDE.md %q; want current and CLAUDE.md byte-identical",
			second, readGuideFile(t, root, "CLAUDE.md"))
	}
	agentsBlockOf(t, readAgentsGuide(t, root))
	if _, err := os.Stat(filepath.Join(root, "GEMINI.md")); !os.IsNotExist(err) {
		t.Errorf("agents created GEMINI.md: %v", err)
	}
}

func TestAgentsCheck_UnreachedGuides_FailNamingTheFirstGuide(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")
	writeGuideFile(t, root, "CLAUDE.md", "# Claude\n")
	writeGuideFile(t, root, "GEMINI.md", "# Gemini\n")

	stdout, stderr, exitCode := runCLI(t, root, "agents", "--check")

	want := "capcom: CLAUDE.md: does not reach the capcom block in AGENTS.md; run capcom agents to add @AGENTS.md\n"
	if exitCode != ExitError || stdout != "" || stderr != want {
		t.Errorf("agents --check = %d, stdout %q, stderr %q; want 1 and %q", exitCode, stdout, stderr, want)
	}
	if readGuideFile(t, root, "CLAUDE.md") != "# Claude\n" || readGuideFile(t, root, "GEMINI.md") != "# Gemini\n" {
		t.Errorf("agents --check wrote a guide")
	}
}

func TestAgentsCheck_GuideReachesOrBlockAbsent_ExitsZeroWithTheGuidesInJSON(t *testing.T) {
	root := newKnowStore(t)
	writeAgentsGuide(t, root, "# Guide\n")
	writeGuideFile(t, root, "CLAUDE.md", "# Claude\n")

	blockless, _, blocklessExit := runCLI(t, root, "agents", "--check")
	mustRun(t, root, "agents")
	current, _, currentExit := runCLI(t, root, "agents", "--check")
	jsonOut, _, jsonExit := runCLI(t, root, "agents", "--check", "--json")

	if blocklessExit != ExitOK || blockless != "AGENTS.md has no capcom block\n" {
		t.Errorf("agents --check without the block = %d %q; want 0 and no block", blocklessExit, blockless)
	}
	if currentExit != ExitOK || current != "the capcom block of AGENTS.md is current\n" {
		t.Errorf("agents --check with the guide reaching the block = %d %q; want 0 and current", currentExit, current)
	}
	data, _ := decodeEnvelope(t, jsonOut)["data"].(map[string]any)
	guides, _ := data["guides"].([]any)
	want := []map[string]any{{"path": "CLAUDE.md", "state": "reaches"}, {"path": "GEMINI.md", "state": "missing"}}
	if jsonExit != ExitOK || len(guides) != len(want) {
		t.Fatalf("agents --check --json = %d, data %v; want 0 and guides %v", jsonExit, data, want)
	}
	for index, guide := range guides {
		entry, _ := guide.(map[string]any)
		if entry["path"] != want[index]["path"] || entry["state"] != want[index]["state"] {
			t.Errorf("guides[%d] = %v; want %v", index, entry, want[index])
		}
	}
}

func TestAgents_GuideLinkedOutsideTheTree_IsSkippedAndLeftUntouched(t *testing.T) {
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
	if err := os.Symlink(outside, filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Skipf("Symlink: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "agents")
	_, _, checkExit := runCLI(t, root, "agents", "--check")

	want := "wrote the capcom block to AGENTS.md\nskipped CLAUDE.md: links outside the working tree\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("agents = %d, stdout %q, stderr %q; want 0 and %q", exitCode, stdout, stderr, want)
	}
	if checkExit != ExitOK {
		t.Errorf("agents --check = %d; want 0 for a guide outside the working tree", checkExit)
	}
	if content, readErr := os.ReadFile(outside); readErr != nil || string(content) != original {
		t.Errorf("outside file = %q, %v; want it untouched", content, readErr)
	}
}
