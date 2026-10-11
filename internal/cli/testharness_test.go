package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// runCLI runs capcom in process with dir as the working directory (skipped when dir is empty)
// and returns its stdout, stderr and exit code. It sets the hermetic git environment of ADR 0007,
// so tests using it must not call t.Parallel.
func runCLI(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
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
func setHermeticGitEnv(t *testing.T) {
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
