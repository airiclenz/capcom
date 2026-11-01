package check_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/store"
)

// oversizedGuideSize is one byte past the largest AGENTS.md the guarded reader reads, 1 MiB.
const oversizedGuideSize = 1<<20 + 1

// writeGuide writes content as the AGENTS.md of root.
func writeGuide(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, agents.FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// codedProblems renders each problem as its code, a space and its String form.
func codedProblems(problems []check.Problem) []string {
	result := make([]string, 0, len(problems))
	for _, problem := range problems {
		result = append(result, string(problem.Code)+" "+problem.String())
	}
	return result
}

// runCheckWithin runs check on the store in root like runCheck, but off the test goroutine, failing
// the test when the run has not returned within fifoCheckTimeout, as it does when it blocks
// opening a named pipe.
func runCheckWithin(t *testing.T, root string) []check.Problem {
	t.Helper()
	done := make(chan []check.Problem, 1)
	go func() {
		opened, err := store.Open(root)
		if err != nil {
			t.Errorf("Open: %v", err)
			done <- nil
			return
		}
		problems, err := check.Run(opened)
		if err != nil {
			t.Errorf("Run: %v", err)
		}
		done <- problems
	}()
	select {
	case problems := <-done:
		return problems
	case <-time.After(fifoCheckTimeout):
		t.Fatal("Run blocked on AGENTS.md")
		return nil
	}
}

func TestRun_StaleAgentsBlock_ReportsOneProblemAtTheBeginMarkerLine(t *testing.T) {
	root := newRepo(t)
	writeGuide(t, root, "# Guide\n\n"+agents.BeginMarker+"\nan older block\n"+agents.EndMarker+"\n")

	problems := runCheck(t, root)

	want := []string{
		string(diagnose.AgentsBlock) + " " + agents.FileName +
			":3: the capcom block is out of date; run capcom agents to refresh it",
	}
	if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestRun_RenamedDoingStatus_KeepsTheBlockCurrent(t *testing.T) {
	root := newRepo(t)
	writeCurrentBlock(t, root)
	writeFiles(t, root, map[string]string{".capcom/config.yaml": "statuses:\n  open: todo\n  wip: doing\n  closed: done\n"})

	problems := runCheck(t, root)

	if got := codedProblems(problems); len(got) != 0 {
		t.Errorf("problems = %q\nwant none: the claim line names capcom start, not the doing status", got)
	}
}

func TestRun_NoDoingStatus_ReportsTheBlockOutOfDate(t *testing.T) {
	root := newRepo(t)
	writeCurrentBlock(t, root)
	writeFiles(t, root, map[string]string{".capcom/config.yaml": "statuses:\n  open: todo\n  closed: done\n"})

	problems := runCheck(t, root)

	if got := codedProblems(problems); len(got) != 1 || !strings.Contains(got[0], "the capcom block is out of date") {
		t.Errorf("problems = %q\nwant one stale-block problem", got)
	}
}

func TestRun_TangledAgentsMarkers_ReportsTheMarkerErrorWithoutFailing(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		wantLine string
	}{
		{"never closed", "# Guide\n" + agents.BeginMarker + "\ntext\n",
			":2: " + agents.BeginMarker + " is never closed by " + agents.EndMarker},
		{"end first", agents.EndMarker + "\n" + agents.BeginMarker + "\n",
			":1: " + agents.EndMarker + " without a " + agents.BeginMarker + " before it"},
		{"second begin", agents.BeginMarker + "\n" + agents.EndMarker + "\n" + agents.BeginMarker + "\n",
			":3: a second " + agents.BeginMarker + "; keep one capcom block"},
		{"second end", agents.BeginMarker + "\n" + agents.EndMarker + "\n" + agents.EndMarker + "\n",
			":3: a second " + agents.EndMarker + "; keep one capcom block"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			writeGuide(t, root, testCase.content)

			problems := runCheck(t, root)

			want := []string{string(diagnose.AgentsBlock) + " " + agents.FileName + testCase.wantLine}
			if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
				t.Errorf("problems = %q\nwant %q", got, want)
			}
		})
	}
}

func TestRun_NoAgentsBlock_ReportsNothing(t *testing.T) {
	cases := []struct {
		name  string
		setUp func(t *testing.T, root string)
	}{
		{"no AGENTS.md", func(*testing.T, string) {}},
		{"AGENTS.md without a block", func(t *testing.T, root string) {
			writeGuide(t, root, "# Guide\n\nHand-written guidance only.\n")
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			testCase.setUp(t, root)

			problems := runCheck(t, root)

			if len(problems) != 0 {
				t.Errorf("problems = %v, want none", problems)
			}
		})
	}
}

func TestRun_UnreadableAgentsFile_ReportsAnAgentsBlockProblem(t *testing.T) {
	const notRegular = "not a regular file"
	const tooLarge = "file is larger than 1 MiB"
	cases := []struct {
		name        string
		setUp       func(t *testing.T, path string)
		wantMessage string
	}{
		{"named pipe", func(t *testing.T, path string) {
			makeFIFO(t, path)
		}, notRegular},
		{"directory", func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatalf("Mkdir: %v", err)
			}
		}, notRegular},
		{"over 1 MiB", func(t *testing.T, path string) {
			content := strings.Repeat("x", oversizedGuideSize)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}, tooLarge},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			testCase.setUp(t, filepath.Join(root, agents.FileName))

			problems := runCheckWithin(t, root)

			want := []string{string(diagnose.AgentsBlock) + " " + agents.FileName + ": " + testCase.wantMessage}
			if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
				t.Errorf("problems = %q\nwant %q", got, want)
			}
		})
	}
}

// unreachedGuideMessage is how check reports an agent guide at the working tree root that does not
// reach the block.
const unreachedGuideMessage = "does not reach the capcom block in AGENTS.md; run capcom agents to add @AGENTS.md"

// writeCurrentBlock writes the AGENTS.md of root carrying a block that is up to date.
func writeCurrentBlock(t *testing.T, root string) {
	t.Helper()
	opened, err := store.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doingStatus, _ := opened.Config.DefaultStatus(config.CategoryDoing)
	if _, err := agents.Sync(opened.WorktreeRoot, agents.FileName, opened.KnowledgeDir(), doingStatus, agents.ModeWrite); err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

func TestRun_UnreachedAgentGuide_ReportsAnAgentsGuideProblem(t *testing.T) {
	root := newRepo(t)
	writeCurrentBlock(t, root)
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# Claude\n"})

	problems := runCheck(t, root)

	want := []string{string(diagnose.AgentsGuide) + " CLAUDE.md: " + unreachedGuideMessage}
	if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestRun_AgentGuideWithoutABlockToReach_ReportsNothing(t *testing.T) {
	cases := []struct {
		name  string
		setUp func(t *testing.T, root string)
	}{
		{"no block", func(t *testing.T, root string) {
			writeGuide(t, root, "# Guide\n\nHand-written guidance only.\n")
		}},
		{"no guides configured", func(t *testing.T, root string) {
			writeFiles(t, root, map[string]string{".capcom/config.yaml": "format: 4\nagents:\n  guides: []\n"})
			writeCurrentBlock(t, root)
		}},
		{"AGENTS.md linked outside the tree", func(t *testing.T, root string) {
			outside := filepath.Join(t.TempDir(), "AGENTS.md")
			content := agents.BeginMarker + "\nan older block\n" + agents.EndMarker + "\n"
			if err := os.WriteFile(outside, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			makeSymlink(t, outside, filepath.Join(root, agents.FileName))
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			testCase.setUp(t, root)
			writeFiles(t, root, map[string]string{"CLAUDE.md": "# Claude\n"})

			problems := runCheck(t, root)

			if len(problems) != 0 {
				t.Errorf("problems = %v, want none", problems)
			}
		})
	}
}

func TestRun_UnreadableAgentGuide_ReportsAnAgentsGuideProblem(t *testing.T) {
	cases := []struct {
		name  string
		setUp func(t *testing.T, path string)
	}{
		{"named pipe", func(t *testing.T, path string) {
			makeFIFO(t, path)
		}},
		{"directory", func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatalf("Mkdir: %v", err)
			}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			writeCurrentBlock(t, root)
			testCase.setUp(t, filepath.Join(root, "CLAUDE.md"))

			problems := runCheckWithin(t, root)

			want := []string{string(diagnose.AgentsGuide) + " CLAUDE.md: not a regular file"}
			if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
				t.Errorf("problems = %q\nwant %q", got, want)
			}
		})
	}
}

func TestRun_UnreachedGuideInASubdirectory_NamesTheImportFromItsDirectory(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		".capcom/config.yaml": "format: 4\nagents:\n  guides: [docs/AI.md]\n",
		"docs/AI.md":          "# AI\n",
	})
	writeCurrentBlock(t, root)

	problems := runCheck(t, root)

	want := []string{string(diagnose.AgentsGuide) +
		" docs/AI.md: does not reach the capcom block in AGENTS.md; run capcom agents to add @../AGENTS.md"}
	if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

// blockFileConfig is a config.yaml keeping the Managed block in .capcom/AGENTS-capcom.md.
const blockFileConfig = "format: 6\nagents:\n  file: .capcom/AGENTS-capcom.md\n"

// customBlockFile is the block file blockFileConfig names.
const customBlockFile = ".capcom/AGENTS-capcom.md"

// importingAgentsGuide is an AGENTS.md that reaches customBlockFile by an import line.
const importingAgentsGuide = "# Guide\n\n@" + customBlockFile + "\n"

// newBlockFileRepo returns a repo whose config.yaml names customBlockFile, its block current there
// and its AGENTS.md holding agentsContent.
func newBlockFileRepo(t *testing.T, agentsContent string) string {
	t.Helper()
	root := newRepo(t)
	writeFiles(t, root, map[string]string{".capcom/config.yaml": blockFileConfig, agents.FileName: agentsContent})
	opened, err := store.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doingStatus, _ := opened.Config.DefaultStatus(config.CategoryDoing)
	if _, err := agents.Sync(opened.WorktreeRoot, opened.Config.Agents.File, opened.KnowledgeDir(), doingStatus, agents.ModeWrite); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	return root
}

func TestRun_BlockFileImportedByAgentsMD_ReportsNothing(t *testing.T) {
	root := newBlockFileRepo(t, importingAgentsGuide)

	problems := runCheck(t, root)

	if len(problems) != 0 {
		t.Errorf("problems = %v, want none", problems)
	}
}

func TestRun_AgentsMDNotImportingTheBlockFile_ReportsAnAgentsGuideProblem(t *testing.T) {
	root := newBlockFileRepo(t, "# Guide\n")

	problems := runCheck(t, root)

	want := []string{string(diagnose.AgentsGuide) + " AGENTS.md: does not reach the capcom block in " +
		customBlockFile + "; run capcom agents to add @" + customBlockFile}
	if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestRun_AgentsMDMissingBesideTheBlockFile_ReportsItWithTheCreateHint(t *testing.T) {
	root := newBlockFileRepo(t, importingAgentsGuide)
	if err := os.Remove(filepath.Join(root, agents.FileName)); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	problems := runCheck(t, root)

	want := []string{string(diagnose.AgentsGuide) + " AGENTS.md: " +
		check.MissingAGENTSMdMessage(customBlockFile) + "; " + check.MissingAGENTSMdHint("@"+customBlockFile)}
	if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestRun_StaleBlockInTheBlockFile_ReportsItAtTheBlockFile(t *testing.T) {
	root := newBlockFileRepo(t, importingAgentsGuide)
	writeFiles(t, root, map[string]string{".capcom/knowledge/api-design.md": "---\ntitle: API design\n---\n"})

	problems := runCheck(t, root)

	got := codedProblems(problems)
	if len(got) != 1 || !strings.HasPrefix(got[0], string(diagnose.AgentsBlock)+" "+customBlockFile+":") {
		t.Errorf("problems = %q; want one agents-block problem at %s", got, customBlockFile)
	}
}

func TestRun_CapcomMarkersLeftInAgentsMD_ReportsThemWithTheRemoveHint(t *testing.T) {
	root := newBlockFileRepo(t, importingAgentsGuide+"\n"+agents.BeginMarker+"\nold block\n"+agents.EndMarker+"\n")

	problems := runCheck(t, root)

	want := []string{string(diagnose.AgentsGuide) + " AGENTS.md: " +
		check.StrayMarkersMessage(customBlockFile) + "; " + check.StrayMarkersHint}
	if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestRun_GuideReachingAgentsMDWithABlockFile_ReportsOnlyTheUnreachedGuide(t *testing.T) {
	root := newBlockFileRepo(t, importingAgentsGuide)
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# Claude\n", "GEMINI.md": "# Gemini\n\n@AGENTS.md\n"})

	problems := runCheck(t, root)

	want := []string{string(diagnose.AgentsGuide) + " CLAUDE.md: " + unreachedGuideMessage}
	if got := codedProblems(problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}
