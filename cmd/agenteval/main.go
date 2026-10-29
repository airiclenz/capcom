package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// The exit codes. A bad command line is told apart from a run that started and failed, so a
// script or skill driving this program can react to each.
const (
	exitRunFailed = 1
	exitBadUsage  = 2
)

// main runs the shim when the executable was started under the shim's name, and the command
// tree otherwise.
func main() {
	if programName(os.Args[0]) == shimName {
		os.Exit(runShim(os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr))
	}
	if err := newRootCommand().ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "agenteval:", err)
		os.Exit(exitCodeFor(err))
	}
}

// newRootCommand builds the command tree. Errors are printed by main, not by Cobra, so a failure
// reads the same whichever verb produced it; usage still prints for a bad command line, because
// that is the one failure the usage text actually answers.
func newRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "agenteval",
		Short: "Evaluate how well an agent uses capcom",
		Long: "agenteval runs seeded capcom tasks against an agent command, judges each from the\n" +
			"store files the agent left behind, counts the capcom calls it made through a PATH\n" +
			"shim, and writes a markdown report.",
		SilenceErrors: true,
	}
}

// runError marks a failure that happened after the command line was accepted, so exitCodeFor
// can tell a mistyped invocation from a run that started and failed.
type runError struct{ err error }

func (e runError) Error() string { return e.err.Error() }
func (e runError) Unwrap() error { return e.err }

// runE adapts a verb's body: usage stops being printed the moment the command line has been
// accepted, and every failure from here on is marked as a run failure rather than a usage one.
// Cobra's own flag and argument errors never pass through here, which is what makes them
// distinguishable.
func runE(run func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if err := run(cmd, args); err != nil {
			return runError{err: err}
		}
		return nil
	}
}

// exitCodeFor reports the status an error asks for: a run that started and failed exits 1, a
// command line that never started one exits 2 with its usage printed.
func exitCodeFor(err error) int {
	var failed runError
	if errors.As(err, &failed) {
		return exitRunFailed
	}
	return exitBadUsage
}
