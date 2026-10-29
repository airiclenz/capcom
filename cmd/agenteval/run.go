package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// defaultTaskTimeout is how long one task's agent may run before it is killed and the task failed.
const defaultTaskTimeout = 15 * time.Minute

// The files a run writes beside its report: the directory named after the report, holding each
// task's transcript and call log under the task's id.
const (
	transcriptsDirSuffix = ".transcripts"
	transcriptExtension  = ".log"
	callLogExtension     = ".calls.jsonl"
)

// The run's output files and directories are created with these permissions.
const (
	runFileMode = 0o644
	runDirMode  = 0o755
)

// capcomPackage is the package run builds the capcom under test from, when --capcom is not given.
const capcomPackage = "./cmd/capcom"

// runOptions is the run verb's command line.
type runOptions struct {
	out       string
	capcom    string
	only      []string
	timeout   time.Duration
	agentArgv []string
}

// runSettings is what every task of a run shares once the command line has been resolved.
type runSettings struct {
	capcomPath     string
	agentArgv      []string
	timeout        time.Duration
	transcriptsDir string
}

// newRunCommand builds the run verb. Its command line is checked before the run starts, so a
// missing agent command or an unknown --only id is a usage error, exit 2.
func newRunCommand() *cobra.Command {
	var options runOptions
	cmd := &cobra.Command{
		Use:   "run --out <file.md> [flags] -- <agent argv…>",
		Short: "Run the scenarios against an agent command and write the report",
		Long: "run seeds a fresh git repo for each scenario, runs the agent command in it with the\n" +
			"task's prompt on stdin, judges the task from the store files the agent left behind,\n" +
			"and writes the markdown report to --out. Each agent's output goes to\n" +
			"<out stem>.transcripts/<id>.log and its capcom calls to <id>.calls.jsonl there.\n" +
			"Without --capcom, run builds ./cmd/capcom from the working directory first.\n" +
			"It exits 0 once the report is written, whichever tasks failed.",
		Example: "  agenteval run --out eval.md -- claude -p\n" +
			"  agenteval run --out eval.md --capcom ./capcom --only next-claim-close -- ./my-agent",
		Args: agentArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			options.agentArgv = args[cmd.ArgsLenAtDash():]
			return checkRunOptions(options)
		},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			return runRun(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), options)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.out, "out", "", "the markdown report to write (required)")
	flags.StringVar(&options.capcom, "capcom", "", "the capcom build under test (default: build ./cmd/capcom)")
	flags.StringSliceVar(&options.only, "only", nil, "run only these scenario ids; repeat or separate with commas")
	flags.DurationVar(&options.timeout, "timeout", defaultTaskTimeout, "how long each task's agent may run")
	if err := cmd.MarkFlagRequired("out"); err != nil {
		panic(err) // the flag is declared just above; failing here is a defect of this program
	}
	return cmd
}

// agentArgs accepts a command line whose only arguments are the agent command, after `--`.
func agentArgs(cmd *cobra.Command, args []string) error {
	dash := cmd.ArgsLenAtDash()
	switch {
	case dash < 0 || dash == len(args):
		return errors.New("run needs the agent command after --")
	case dash > 0:
		return fmt.Errorf("unexpected argument %q before --", args[0])
	}
	return nil
}

// checkRunOptions refuses a timeout that is not positive and an --only id no scenario has.
func checkRunOptions(options runOptions) error {
	if options.timeout <= 0 {
		return fmt.Errorf("--timeout must be positive, got %s", options.timeout)
	}
	for _, id := range options.only {
		if _, isFound := findScenario(id); !isFound {
			return fmt.Errorf("--only: no scenario %q", id)
		}
	}
	return nil
}

// runRun runs the selected scenarios one after another and writes the report, then prints a line
// naming it and the tasks passed. A task the agent fails is a row of the report, not an error; an
// error is the run itself failing: the capcom build, a workspace, a call log or the report.
func runRun(ctx context.Context, stdout, stderr io.Writer, options runOptions) error {
	outPath, err := filepath.Abs(options.out)
	if err != nil {
		return err
	}
	settings := runSettings{
		timeout:        options.timeout,
		transcriptsDir: strings.TrimSuffix(outPath, filepath.Ext(outPath)) + transcriptsDirSuffix,
	}
	if err := os.MkdirAll(settings.transcriptsDir, runDirMode); err != nil {
		return err
	}
	if settings.agentArgv, err = resolveAgentArgv(options.agentArgv); err != nil {
		return err
	}
	capcomPath, cleanup, err := resolveCapcom(ctx, options.capcom)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(stderr, "agenteval: remove the capcom build: %v\n", err)
		}
	}()
	settings.capcomPath = capcomPath

	version, err := runIn("", capcomPath, "version")
	if err != nil {
		return err
	}
	results := make([]taskResult, 0, len(options.only))
	for _, task := range selectScenarios(options.only) {
		result, err := runTask(ctx, stderr, settings, task)
		if err != nil {
			return err
		}
		results = append(results, result)
	}

	info := runInfo{Date: time.Now(), CapcomVersion: version, AgentArgv: options.agentArgv}
	if err := writeReport(outPath, info, results); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s: %d/%d passed\n", outPath, countPassed(results), len(results))
	return nil
}

// selectScenarios returns the scenarios with the ids only names, in the order scenarios lists
// them, or every scenario when only is empty. Each id is known: the command line was checked.
func selectScenarios(only []string) []scenario {
	all := scenarios()
	if len(only) == 0 {
		return all
	}
	selected := make([]scenario, 0, len(only))
	for _, candidate := range all {
		for _, id := range only {
			if candidate.id == id {
				selected = append(selected, candidate)
				break
			}
		}
	}
	return selected
}

// runTask runs one scenario: it starts the task's transcript and call log afresh, seeds its
// workspace, runs the agent there, and judges the result. The workspace is removed afterwards; a
// removal that fails is reported on stderr and the run goes on.
func runTask(ctx context.Context, stderr io.Writer, settings runSettings, task scenario) (taskResult, error) {
	callLogPath := filepath.Join(settings.transcriptsDir, task.id+callLogExtension)
	if err := os.WriteFile(callLogPath, nil, runFileMode); err != nil {
		return taskResult{}, err
	}
	transcript, err := os.Create(filepath.Join(settings.transcriptsDir, task.id+transcriptExtension))
	if err != nil {
		return taskResult{}, err
	}
	defer transcript.Close()

	seeded, err := prepareWorkspace(settings.capcomPath, task)
	if err != nil {
		return taskResult{}, err
	}
	defer func() {
		if err := seeded.remove(); err != nil {
			fmt.Fprintf(stderr, "agenteval: remove the %s workspace: %v\n", task.id, err)
		}
	}()

	run, err := seeded.runAgent(
		ctx, settings.agentArgv, task.prompt, settings.capcomPath, callLogPath, transcript, settings.timeout,
	)
	if err != nil {
		return taskResult{}, err
	}
	calls, err := readCallLog(callLogPath)
	if err != nil {
		return taskResult{}, err
	}
	verdict := failed("timed out after %s", settings.timeout)
	if !run.IsTimedOut {
		verdict = task.check(seeded.repo, calls)
	}
	return taskResult{
		ID:       task.id,
		Passed:   verdict.Passed,
		Why:      verdict.Why,
		Score:    score(calls),
		Duration: run.Duration,
	}, nil
}

// resolveAgentArgv returns argv with its program resolved to an absolute path on the harness's own
// PATH, so the agent is the same program whichever directory it starts in and whatever its PATH
// puts first.
func resolveAgentArgv(argv []string) ([]string, error) {
	program, err := resolveExecutable(argv[0])
	if err != nil {
		return nil, fmt.Errorf("the agent command: %w", err)
	}
	return append([]string{program}, argv[1:]...), nil
}

// resolveCapcom returns the capcom build under test as an absolute path, and a cleanup for the
// run to call when it is done. A given path is resolved on the harness's own PATH; without one,
// ./cmd/capcom of the working directory is built into a temp dir, which the cleanup removes.
func resolveCapcom(ctx context.Context, given string) (string, func() error, error) {
	if given != "" {
		path, err := resolveExecutable(given)
		if err != nil {
			return "", nil, fmt.Errorf("--capcom: %w", err)
		}
		return path, func() error { return nil }, nil
	}
	dir, err := os.MkdirTemp("", "agenteval-capcom-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	path := filepath.Join(dir, executableName(shimName))
	build := exec.CommandContext(ctx, "go", "build", "-o", path, capcomPackage)
	if output, err := build.CombinedOutput(); err != nil {
		return "", nil, errors.Join(fmt.Errorf("build %s: %w\n%s", capcomPackage, err, output), cleanup())
	}
	return path, cleanup, nil
}

// resolveExecutable returns the absolute path of program as the harness's own PATH finds it: a
// bare name is looked up on PATH, a path is taken relative to the working directory.
func resolveExecutable(program string) (string, error) {
	found, err := exec.LookPath(program)
	if err != nil {
		return "", err
	}
	return filepath.Abs(found)
}

// writeReport renders the report of a run into the file at path, replacing any earlier one.
func writeReport(path string, info runInfo, results []taskResult) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := renderReport(file, info, results); err != nil {
		return errors.Join(fmt.Errorf("write %s: %w", path, err), file.Close())
	}
	return file.Close()
}

// countPassed reports how many of results passed.
func countPassed(results []taskResult) int {
	count := 0
	for _, result := range results {
		if result.Passed {
			count++
		}
	}
	return count
}
