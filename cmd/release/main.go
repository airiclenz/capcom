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

func main() {
	commands := execRunner{}
	if err := newRootCommand(commands, execRepos(commands)).ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(exitCodeFor(err))
	}
}

// newRootCommand builds the command tree over commands, the runner every verb executes its
// external commands through, and openRepo, which every verb that reads the capcom checkout
// opens its repo with. Errors are printed by main, not by Cobra, so a failure reads the
// same whichever verb produced it; usage still prints for a bad command line, because that is the
// one failure the usage text actually answers. The root declares no Args, so Cobra's own check
// refuses an unknown verb instead of printing help and exiting 0.
func newRootCommand(commands runner, openRepo repoOpener) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Build and publish a capcom release",
		Long: "release does the deterministic work of a capcom release: it cross-builds the\n" +
			"release archives and their checksums, reports the release gate and where each\n" +
			"publish step stands, makes the CHANGELOG rollup and VERSION bump commits, publishes\n" +
			"the GitHub release, the Homebrew formula, the Scoop manifest and the winget\n" +
			"submission, and smoke-tests a published release, the same way on every host.",
		SilenceErrors: true,
	}
	cmd.AddCommand(newDistCommand(commands))
	cmd.AddCommand(newStatusCommand(commands, openRepo))
	cmd.AddCommand(newPrepareCommand(commands, openRepo))
	cmd.AddCommand(newPublishCommand(commands, openRepo))
	cmd.AddCommand(newSmokeCommand(commands, openRepo))
	return cmd
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
