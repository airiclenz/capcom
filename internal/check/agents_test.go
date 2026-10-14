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
