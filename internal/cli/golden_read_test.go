package cli

import (
	"testing"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// TestGolden_ReadKinds pins the --json envelope of each read command's kind against the golden
// store, one golden file per case.
func TestGolden_ReadKinds(t *testing.T) {
	tests := []struct {
		golden string
		args   []string
	}{
		{golden: "issue", args: []string{"show", "session-timeout"}},
		{golden: "issue-list", args: []string{"list"}},
		{golden: "ready", args: []string{"ready"}},
		{golden: "next", args: []string{"next"}},
		{golden: "context", args: []string{"context", "session-timeout"}},
		{golden: "deps", args: []string{"dep", "list", "refresh-retry"}},
		{golden: "dep-graph", args: []string{"dep", "graph"}},
		{golden: "search", args: []string{"search", "refresh"}},
		{golden: "search-any", args: []string{"search", "refresh", "zebra"}},
		{golden: "prime", args: []string{"prime"}},
		{golden: "recap", args: []string{"recap"}},
		{golden: "check", args: []string{"check"}},
		{golden: "version", args: []string{"version"}},
		{golden: "reindex", args: []string{"reindex"}},
		{golden: "agents", args: []string{"agents"}},
	}
	for _, test := range tests {
		t.Run(test.golden, func(t *testing.T) {
			root := newGoldenStore(t)

			stdout := mustRun(t, root, append(test.args, "--json")...)

			assertGoldenJSON(t, test.golden, root, stdout)
		})
	}
}

func TestGolden_NextEmpty_PinsNullDataWithExitNothingReady(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "init")

	stdout, stderr, exitCode := runCLI(t, root, "next", "--json")

	if exitCode != output.ExitNothingReady || stderr != "" {
		t.Fatalf("exit = %d stderr = %q, want %d and no stderr", exitCode, stderr, output.ExitNothingReady)
	}
	assertGoldenJSON(t, "next-empty", root, stdout)
}

func TestGolden_CheckProblems_PinsTheProblemList(t *testing.T) {
	root := newGoldenStore(t)
	mustRun(t, root, "know", "add", "--id", "gone-ref", "--title", "A Note on a removed file",
		"--ref", "internal/removed.go")

	stdout, stderr, exitCode := runCLI(t, root, "check", "--json")

	if exitCode != ExitError || stderr != "" {
		t.Fatalf("exit = %d stderr = %q, want %d and no stderr", exitCode, stderr, ExitError)
	}
	assertGoldenJSON(t, "check-problems", root, stdout)
}

func TestGolden_Init_PinsTheCreatedStore(t *testing.T) {
	root := newGitRepo(t)

	stdout := mustRun(t, root, "init", "--json")

	assertGoldenJSON(t, "init", root, stdout)
}
