package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runner is the one seam every external command this program runs goes through — `go`, `git`,
// `gh`, `komac` alike. The release verbs never call os/exec themselves, so a test hands them a
// recording fake and no test of a verb ever reaches the network, a real `git` or a real
// toolchain; only versionbump_test.go runs real `git`, under the workflow's script and never
// through this seam.
// What the verbs ask of the capcom checkout and its GitHub repository goes through the repo seam
// (repo.go) instead, whose execRepo is the one caller of the runner with that argv; the runner
// itself carries the toolchain, dist's read of its source tree's commit time, and the tap, bucket
// and winget-pkgs commands.
type runner interface {
	// run executes name with args in dir, with env appended to the inherited environment (a
	// later entry overrides an earlier one, so env wins), and returns what the command wrote to
	// stdout. A command that fails returns an error carrying its stderr.
	run(ctx context.Context, dir string, env []string, name string, args ...string) (string, error)
}

// execRunner is the production runner: a real process per call.
type execRunner struct{}

// run executes the command, capturing stdout as the result and stderr for the error.
func (execRunner) run(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	if len(env) > 0 {
		command.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return stdout.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return stdout.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, detail)
	}
	return stdout.String(), nil
}
