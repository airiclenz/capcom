package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/proc"
)

// init stands an empty process ancestry in for the real one, so no lock a test acquires records a
// holder process and every test's writes meet a held lock as a caller outside it; withAncestry
// gives one test a fake ancestry. It covers runCLI, runCLIWith and the benchmarks' direct Run
// calls alike.
func init() {
	processAncestry = func() []proc.Process { return nil }
}

// withAncestry makes chain the calling process's ancestry, parent first, for the rest of the test,
// and restores the previous one when it ends.
func withAncestry(t testing.TB, chain ...proc.Process) {
	t.Helper()
	previous := processAncestry
	processAncestry = func() []proc.Process { return chain }
	t.Cleanup(func() { processAncestry = previous })
}

// runCLI runs capcom in process with dir as the working directory (skipped when dir is empty)
// and returns its stdout, stderr and exit code. It sets the hermetic git environment of ADR 0007,
// so tests using it must not call t.Parallel.
func runCLI(t testing.TB, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	if dir != "" {
		t.Chdir(dir)
	}
	setHermeticGitEnv(t)

	var stdoutBuffer, stderrBuffer bytes.Buffer
	exitCode = Run(args, strings.NewReader(""), &stdoutBuffer, &stderrBuffer)
	return stdoutBuffer.String(), stderrBuffer.String(), exitCode
}

// setHermeticGitEnv isolates git from user and system configuration and fixes the author and
// committer identity, so git-backed tests behave the same on every machine.
func setHermeticGitEnv(t testing.TB) {
	t.Helper()
	environment := map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Test Author",
		"GIT_AUTHOR_EMAIL":    "author@example.com",
		"GIT_COMMITTER_NAME":  "Test Committer",
		"GIT_COMMITTER_EMAIL": "committer@example.com",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
}
