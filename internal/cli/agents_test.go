package cli

import (
	"os"
	"path/filepath"
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
	writeAgentsGuide(t, root, "# Guide\n\n"+agents.Render(nil)+"\n")

	stdout, stderr, exitCode := runCLI(t, root, "agents", "--check")

	if exitCode != ExitError || stdout != "" || !strings.HasPrefix(stderr, "capcom: AGENTS.md:3: the capcom block is out of date") {
		t.Errorf("agents --check = %d, stdout %q, stderr %q; want 1 at AGENTS.md:3", exitCode, stdout, stderr)
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
