package agents

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// defaultGuideNames are the agent guides capcom checks by default.
var defaultGuideNames = []string{"CLAUDE.md", "GEMINI.md"}

// blockGuide is an AGENTS.md carrying the block.
const blockGuide = "# Guide\n\n" + goldenBlock + "\n"

// writeTreeFile writes content to the file name, relative to root in slash form, making its
// directory.
func writeTreeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := namedGuidePath(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// readTreeFile returns the file name, relative to root in slash form.
func readTreeFile(t *testing.T, root, name string) string {
	t.Helper()
	content, err := os.ReadFile(namedGuidePath(root, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

// linkTreeFile makes the file name of root a link to target, skipping where symlink tests do not
// run.
func linkTreeFile(t *testing.T, root, name, target string) {
	t.Helper()
	skipSymlinkTest(t)
	path := namedGuidePath(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("Symlink: %v", err)
	}
}

// guidesWithin runs Guides, failing the test when it has not returned after 5 s, as it does when
// it blocks opening a named pipe.
func guidesWithin(t *testing.T, root string, names []string, mode Mode) ([]Guide, error) {
	t.Helper()
	type outcome struct {
		guides []Guide
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		guides, err := Guides(root, names, mode)
		done <- outcome{guides, err}
	}()
	select {
	case result := <-done:
		return result.guides, result.err
	case <-time.After(5 * time.Second):
		t.Fatal("Guides did not return within 5 s")
		return nil, nil
	}
}

// onlyGuide runs Guides for the one guide name and returns what it found.
func onlyGuide(t *testing.T, root, name string, mode Mode) Guide {
	t.Helper()
	guides, err := guidesWithin(t, root, []string{name}, mode)
	if err != nil || len(guides) != 1 {
		t.Fatalf("Guides = %v, %v; want one guide", guides, err)
	}
	return guides[0]
}

func TestGuides_ImportLine_Reaches(t *testing.T) {
	t.Parallel()
	contents := []struct {
		name    string
		content string
	}{
		{"plain", "# Claude\n\n@AGENTS.md\n"},
		{"dotted", "@./AGENTS.md\n"},
		{"indented", "# Claude\n   @AGENTS.md  \nMore.\n"},
		{"CRLF", "# Claude\r\n@AGENTS.md\r\n"},
		{"no final newline", "# Claude\n@AGENTS.md"},
	}
	for _, testCase := range contents {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, blockGuide)
			writeTreeFile(t, root, "CLAUDE.md", testCase.content)

			guide := onlyGuide(t, root, "CLAUDE.md", ModeWrite)

			want := Guide{Path: "CLAUDE.md", State: GuideReaches, ImportLine: "@AGENTS.md"}
			if guide != want {
				t.Errorf("Guides = %+v, want %+v", guide, want)
			}
			if got := readTreeFile(t, root, "CLAUDE.md"); got != testCase.content {
				t.Errorf("CLAUDE.md = %q, want it untouched", got)
			}
		})
	}
}

func TestGuides_SameFileAsAgents_ReachesAndStaysUntouched(t *testing.T) {
	t.Parallel()
	setUps := []struct {
		name  string
		setUp func(t *testing.T, root string)
	}{
		{"CLAUDE.md links to AGENTS.md", func(t *testing.T, root string) {
			writeTreeFile(t, root, FileName, blockGuide)
			linkTreeFile(t, root, "CLAUDE.md", FileName)
		}},
		{"AGENTS.md links to CLAUDE.md", func(t *testing.T, root string) {
			writeTreeFile(t, root, "CLAUDE.md", blockGuide)
			linkTreeFile(t, root, FileName, "CLAUDE.md")
		}},
	}
	for _, testCase := range setUps {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			testCase.setUp(t, root)

			guide := onlyGuide(t, root, "CLAUDE.md", ModeWrite)

			if want := (Guide{Path: "CLAUDE.md", State: GuideReaches}); guide != want {
				t.Errorf("Guides = %+v, want %+v", guide, want)
			}
			if got := readTreeFile(t, root, "CLAUDE.md"); got != blockGuide {
				t.Errorf("CLAUDE.md = %q, want it untouched", got)
			}
		})
	}
}

func TestGuides_UnreachedGuideOnWrite_AppendsTheImportLine(t *testing.T) {
	t.Parallel()
	contents := []struct {
		name    string
		content string
		want    string
	}{
		{"LF", "# Claude\n", "# Claude\n\n@AGENTS.md\n"},
		{"CRLF", "# Claude\r\nRules.\r\n", "# Claude\r\nRules.\r\n\r\n@AGENTS.md\r\n"},
		{"no final newline", "# Claude", "# Claude\n\n@AGENTS.md\n"},
		{"empty", "", "@AGENTS.md\n"},
	}
	for _, testCase := range contents {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, blockGuide)
			writeTreeFile(t, root, "CLAUDE.md", testCase.content)

			first := onlyGuide(t, root, "CLAUDE.md", ModeWrite)
			afterFirst := readTreeFile(t, root, "CLAUDE.md")
			second := onlyGuide(t, root, "CLAUDE.md", ModeWrite)

			if want := (Guide{Path: "CLAUDE.md", State: GuideImported, ImportLine: "@AGENTS.md"}); first != want {
				t.Errorf("first Guides = %+v, want %+v", first, want)
			}
			if afterFirst != testCase.want {
				t.Errorf("CLAUDE.md = %q, want %q", afterFirst, testCase.want)
			}
			if second.State != GuideReaches {
				t.Errorf("second Guides state = %v, want GuideReaches", second.State)
			}
			if got := readTreeFile(t, root, "CLAUDE.md"); got != afterFirst {
				t.Errorf("CLAUDE.md after the second run = %q, want %q", got, afterFirst)
			}
		})
	}
}

func TestGuides_PrivateGuideOnWrite_KeepsItsMode(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not kept on Windows")
	}
	root := writeGuide(t, blockGuide)
	writeTreeFile(t, root, "CLAUDE.md", "# Claude\n")
	if err := os.Chmod(namedGuidePath(root, "CLAUDE.md"), 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	onlyGuide(t, root, "CLAUDE.md", ModeWrite)

	info, err := os.Stat(namedGuidePath(root, "CLAUDE.md"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("CLAUDE.md mode = %v, %v; want 600", info, err)
	}
}

func TestGuides_UnreachedGuideWithoutWrite_IsReportedAndUntouched(t *testing.T) {
	t.Parallel()
	for _, mode := range []Mode{ModeInspect, ModeRefresh} {
		root := writeGuide(t, blockGuide)
		writeTreeFile(t, root, "CLAUDE.md", "# Claude\n")

		guide := onlyGuide(t, root, "CLAUDE.md", mode)

		want := Guide{Path: "CLAUDE.md", State: GuideUnreached, ImportLine: "@AGENTS.md"}
		if guide != want {
			t.Errorf("mode %d: Guides = %+v, want %+v", mode, guide, want)
		}
		if got := readTreeFile(t, root, "CLAUDE.md"); got != "# Claude\n" {
			t.Errorf("mode %d: CLAUDE.md = %q, want it untouched", mode, got)
		}
	}
}

func TestGuides_GuideInSubdirectory_ImportsFromItsDirectory(t *testing.T) {
	t.Parallel()
	root := writeGuide(t, blockGuide)
	writeTreeFile(t, root, "docs/AI.md", "# AI\n")

	guide := onlyGuide(t, root, "docs/AI.md", ModeWrite)

	want := Guide{Path: "docs/AI.md", State: GuideImported, ImportLine: "@../AGENTS.md"}
	if guide != want {
		t.Errorf("Guides = %+v, want %+v", guide, want)
	}
	if got := readTreeFile(t, root, "docs/AI.md"); got != "# AI\n\n@../AGENTS.md\n" {
		t.Errorf("docs/AI.md = %q", got)
	}
}

func TestGuides_TwoGuidesSharingAFile_ImportOnce(t *testing.T) {
	t.Parallel()
	root := writeGuide(t, blockGuide)
	writeTreeFile(t, root, "CLAUDE.md", "# Claude\n")
	linkTreeFile(t, root, "GEMINI.md", "CLAUDE.md")

	guides, err := guidesWithin(t, root, defaultGuideNames, ModeWrite)

	want := []Guide{
		{Path: "CLAUDE.md", State: GuideImported, ImportLine: "@AGENTS.md"},
		{Path: "GEMINI.md", State: GuideImported, ImportLine: "@AGENTS.md"},
	}
	if err != nil || len(guides) != len(want) || guides[0] != want[0] || guides[1] != want[1] {
		t.Errorf("Guides = %+v, %v; want %+v", guides, err, want)
	}
	if got := readTreeFile(t, root, "CLAUDE.md"); got != "# Claude\n\n@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q, want one import line", got)
	}
}

func TestGuides_SubdirectoryLinkToRootGuide_ImportsFromTheTargetsDirectory(t *testing.T) {
	t.Parallel()
	root := writeGuide(t, blockGuide)
	writeTreeFile(t, root, "CLAUDE.md", "# Claude\n")
	linkTreeFile(t, root, "docs/AI.md", "../CLAUDE.md")

	guide := onlyGuide(t, root, "docs/AI.md", ModeWrite)

	want := Guide{Path: "docs/AI.md", State: GuideImported, ImportLine: "@AGENTS.md"}
	if guide != want {
		t.Errorf("Guides = %+v, want %+v", guide, want)
	}
	if got := readTreeFile(t, root, "CLAUDE.md"); got != "# Claude\n\n@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q, want @AGENTS.md once and no @../AGENTS.md", got)
	}
}

func TestGuides_AgentsWithoutBlock_ReturnsNil(t *testing.T) {
	t.Parallel()
	root := writeGuide(t, "# Guide\n")
	writeTreeFile(t, root, "CLAUDE.md", "# Claude\n")

	guides, err := guidesWithin(t, root, defaultGuideNames, ModeWrite)

	if guides != nil || err != nil {
		t.Errorf("Guides = %+v, %v; want nil, nil", guides, err)
	}
	if got := readTreeFile(t, root, "CLAUDE.md"); got != "# Claude\n" {
		t.Errorf("CLAUDE.md = %q, want it untouched", got)
	}
}

func TestGuides_AgentsThatCannotBeInspected_ReturnsNil(t *testing.T) {
	t.Parallel()
	setUps := []struct {
		name  string
		setUp func(t *testing.T, root string)
	}{
		{"missing", func(t *testing.T, root string) {}},
		{"tangled markers", func(t *testing.T, root string) {
			writeTreeFile(t, root, FileName, BeginMarker+"\n"+BeginMarker+"\n"+EndMarker+"\n")
		}},
		{"directory", func(t *testing.T, root string) {
			if err := os.Mkdir(guidePath(root), 0o755); err != nil {
				t.Fatalf("Mkdir: %v", err)
			}
		}},
		{"named pipe", func(t *testing.T, root string) {
			makeFIFO(t, guidePath(root))
		}},
		{"over 1 MiB", func(t *testing.T, root string) {
			writeSizedFile(t, guidePath(root), oversizedGuideSize)
		}},
		{"linked outside the tree", func(t *testing.T, root string) {
			outside := filepath.Join(t.TempDir(), "AGENTS.md")
			writeTreeFile(t, filepath.Dir(outside), FileName, blockGuide)
			linkTreeFile(t, root, FileName, outside)
		}},
	}
	for _, testCase := range setUps {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			testCase.setUp(t, root)
			writeTreeFile(t, root, "CLAUDE.md", "# Claude\n")

			guides, err := guidesWithin(t, root, defaultGuideNames, ModeWrite)

			if guides != nil || err != nil {
				t.Errorf("Guides = %+v, %v; want nil, nil", guides, err)
			}
			if got := readTreeFile(t, root, "CLAUDE.md"); got != "# Claude\n" {
				t.Errorf("CLAUDE.md = %q, want it untouched", got)
			}
		})
	}
}

func TestGuides_MissingGuide_IsNotCreated(t *testing.T) {
	t.Parallel()
	setUps := []struct {
		name  string
		setUp func(t *testing.T, root string)
	}{
		{"absent", func(t *testing.T, root string) {}},
		{"dangling link", func(t *testing.T, root string) {
			linkTreeFile(t, root, "CLAUDE.md", "missing.md")
		}},
	}
	for _, testCase := range setUps {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, blockGuide)
			testCase.setUp(t, root)

			guide := onlyGuide(t, root, "CLAUDE.md", ModeWrite)

			if want := (Guide{Path: "CLAUDE.md", State: GuideMissing}); guide != want {
				t.Errorf("Guides = %+v, want %+v", guide, want)
			}
			for _, name := range []string{"CLAUDE.md", "missing.md"} {
				if _, err := os.Stat(namedGuidePath(root, name)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s exists after Guides (%v), want it never created", name, err)
				}
			}
		})
	}
}

func TestGuides_GuideLinkedOutsideTheTree_IsLeftAlone(t *testing.T) {
	t.Parallel()
	root := writeGuide(t, blockGuide)
	outsideDir := t.TempDir()
	writeTreeFile(t, outsideDir, "CLAUDE.md", "# Elsewhere\n")
	linkTreeFile(t, root, "CLAUDE.md", filepath.Join(outsideDir, "CLAUDE.md"))

	guide := onlyGuide(t, root, "CLAUDE.md", ModeWrite)

	if want := (Guide{Path: "CLAUDE.md", State: GuideOutside}); guide != want {
		t.Errorf("Guides = %+v, want %+v", guide, want)
	}
	if got := readTreeFile(t, outsideDir, "CLAUDE.md"); got != "# Elsewhere\n" {
		t.Errorf("outside CLAUDE.md = %q, want it untouched", got)
	}
}

func TestGuides_UnreadableGuide_FailsNamingTheGuide(t *testing.T) {
	t.Parallel()
	setUps := []struct {
		name        string
		setUp       func(t *testing.T, root string)
		wantMessage string
	}{
		{"named pipe", func(t *testing.T, root string) {
			makeFIFO(t, namedGuidePath(root, "CLAUDE.md"))
		}, "not a regular file"},
		{"over 1 MiB", func(t *testing.T, root string) {
			writeSizedFile(t, namedGuidePath(root, "CLAUDE.md"), oversizedGuideSize)
		}, "file is larger than 1 MiB"},
	}
	for _, testCase := range setUps {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := writeGuide(t, blockGuide)
			testCase.setUp(t, root)

			_, err := guidesWithin(t, root, []string{"CLAUDE.md"}, ModeWrite)

			want := "CLAUDE.md: " + testCase.wantMessage
			var failure *Error
			if !errors.As(err, &failure) || failure.Path != "CLAUDE.md" || err.Error() != want {
				t.Errorf("Guides err = %v, want an *Error %q", err, want)
			}
		})
	}
}

func TestGuideProblem_Unreached_NamesTheImportLine(t *testing.T) {
	t.Parallel()
	const message = "does not reach the capcom block in AGENTS.md; run capcom agents to add "
	guides := []struct {
		guide Guide
		want  string
	}{
		{Guide{Path: "CLAUDE.md", State: GuideUnreached, ImportLine: "@AGENTS.md"}, message + "@AGENTS.md"},
		{Guide{Path: "docs/AI.md", State: GuideUnreached, ImportLine: "@../AGENTS.md"}, message + "@../AGENTS.md"},
		{Guide{Path: "CLAUDE.md", State: GuideReaches, ImportLine: "@AGENTS.md"}, ""},
		{Guide{Path: "CLAUDE.md", State: GuideImported, ImportLine: "@AGENTS.md"}, ""},
	}
	for _, testCase := range guides {
		if got := testCase.guide.Problem(); got != testCase.want {
			t.Errorf("%+v.Problem() = %q, want %q", testCase.guide, got, testCase.want)
		}
	}
}
