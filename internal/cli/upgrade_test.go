package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
)

// formatLine is the format key line of a config.yaml that declares format.
func formatLine(format int) string {
	return fmt.Sprintf("\nformat: %d\n", format)
}

func TestUpgrade_FormatFourToFive_RaisesAndListsFormatFive(t *testing.T) {
	root := copyFixtureStore(t, config.FormatAgents)

	stdout := mustRun(t, root, "upgrade", "--format", "5")

	want := "store format 4 → 5\n" +
		"  5: " + config.FormatAdditions(config.FormatAgents, config.FormatStanding)[0].Adds + "\n" +
		"capcom releases that read at most format 4 can no longer read this store\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if got := readStoreConfig(t, root); !strings.Contains(got, formatLine(config.FormatStanding)) {
		t.Errorf("config.yaml = %q, want it to declare format %d", got, config.FormatStanding)
	}
}

func TestUpgrade_NoFormat_RaisesToTheNewestListingEachAddedFormat(t *testing.T) {
	root := copyFixtureStore(t, config.FormatSupersession)

	stdout := mustRun(t, root, "upgrade")

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	wantHead := fmt.Sprintf("store format %d → %d", config.FormatSupersession, config.FormatVersion)
	wantTail := fmt.Sprintf("capcom releases that read at most format %d can no longer read this store",
		config.FormatVersion-1)
	if lines[0] != wantHead || lines[len(lines)-1] != wantTail {
		t.Fatalf("stdout = %q, want it to open with %q and end with %q", stdout, wantHead, wantTail)
	}
	for format := config.FormatSupersession + 1; format <= config.FormatVersion; format++ {
		if !strings.Contains(stdout, fmt.Sprintf("\n  %d: ", format)) {
			t.Errorf("stdout = %q, want a line for format %d", stdout, format)
		}
	}
	if got := readStoreConfig(t, root); !strings.Contains(got, formatLine(config.FormatVersion)) {
		t.Errorf("config.yaml = %q, want it to declare format %d", got, config.FormatVersion)
	}
}

func TestUpgrade_StoreAtOrAboveTheTarget_ExitsZeroAndWritesNothing(t *testing.T) {
	cases := []struct {
		name   string
		store  int
		target int
	}{
		{"at the target", config.FormatAgents, config.FormatAgents},
		{"above the target", config.FormatStanding, config.FormatSupersession},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := copyFixtureStore(t, testCase.store)
			before := readStoreConfig(t, root)

			stdout := mustRun(t, root, "upgrade", "--format", fmt.Sprint(testCase.target))

			if want := fmt.Sprintf("store is already at format %d\n", testCase.store); stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
			if after := readStoreConfig(t, root); after != before {
				t.Errorf("config.yaml changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestUpgrade_FormatOutsideTheKnownFormats_IsAUsageError(t *testing.T) {
	for _, target := range []int{0, config.FormatVersion + 1} {
		t.Run(fmt.Sprintf("format %d", target), func(t *testing.T) {
			root := copyFixtureStore(t, config.FormatAgents)
			before := readStoreConfig(t, root)

			stdout, stderr, exitCode := runCLI(t, root, "upgrade", "--format", fmt.Sprint(target))

			if exitCode != ExitUsage || stdout != "" || !strings.Contains(stderr, "--format") {
				t.Errorf("exit = %d stdout = %q stderr = %q, want %d, empty and a --format failure",
					exitCode, stdout, stderr, ExitUsage)
			}
			if after := readStoreConfig(t, root); after != before {
				t.Errorf("config.yaml changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestUpgrade_NoConfigFile_CreatesOneAtTheTarget(t *testing.T) {
	root := copyFixtureStore(t, config.MissingFormat)
	if err := os.Remove(filepath.Join(root, ".capcom", "config.yaml")); err != nil {
		t.Fatalf("Remove config.yaml: %v", err)
	}
	commitFixtureStore(t, root, "drop config.yaml")

	stdout := mustRun(t, root, "upgrade", "--format", fmt.Sprint(config.FormatSupersession))

	if !strings.HasPrefix(stdout, fmt.Sprintf("store format %d → %d\n", config.MissingFormat, config.FormatSupersession)) {
		t.Errorf("stdout = %q, want it to raise format %d to %d", stdout, config.MissingFormat, config.FormatSupersession)
	}
	if got := readStoreConfig(t, root); got != fmt.Sprintf("format: %d\n", config.FormatSupersession) {
		t.Errorf("config.yaml = %q, want it to hold only the format key at %d", got, config.FormatSupersession)
	}
}
