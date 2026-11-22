package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The seeded repo's git identity, set in the repo's own config so the agent's commits and capcom's
// Log entries never depend on the machine running the harness.
const (
	seedUserName  = "agenteval"
	seedUserEmail = "agenteval@example.invalid"
)

// The layout of a task's workspace: the seeded repo the agent works in, the directory put first
// on the agent's PATH, holding the shim, and the process group ledger, outside the repo.
const (
	workspaceRepoDir           = "repo"
	workspaceBinDir            = "bin"
	workspaceProcessGroupsFile = "process-groups"
)

// The permissions the workspace's directories and its shim copy are created with.
const (
	workspaceDirMode = 0o755
	shimFileMode     = 0o755
)

// processGroupsFileMode is the permission the process group ledger is created with.
const processGroupsFileMode = 0o644

// seedHelperArg, as the first argument, starts this executable as the seed helper
// (runSeedHelper) instead of its command tree: `agenteval __seed-capcom <capcom> <args…>`.
const seedHelperArg = "__seed-capcom"

// agentWaitDelay bounds how long the harness waits for the agent's process to be gone after it
// was killed at the timeout, so an agent whose child ignores the kill cannot hang the run.
const agentWaitDelay = 5 * time.Second

// workspace is one task's throwaway directory: a git repo seeded with the task's starting state,
// and a bin directory holding the shim. waitDelay, when set, replaces agentWaitDelay, so a test
// need not wait the full delay out.
type workspace struct {
	root      string
	repo      string
	binDir    string
	waitDelay time.Duration
}

// agentRun is how one agent run ended: how long the agent ran, and whether it was killed at the
// timeout.
type agentRun struct {
	Duration   time.Duration
	IsTimedOut bool
}

// prepareWorkspace makes task's workspace in a new temp dir: a git repo with its own git identity,
// a capcom store and its AGENTS.md block written by the build at capcomPath, the task's seed, all
// in one commit; and a bin directory holding a copy of this executable under the shim's name. The
// caller removes it with remove. On an error nothing is left behind.
func prepareWorkspace(capcomPath string, task scenario) (workspace, error) {
	root, err := os.MkdirTemp("", "agenteval-"+task.id+"-")
	if err != nil {
		return workspace{}, err
	}
	created := workspace{
		root:   root,
		repo:   filepath.Join(root, workspaceRepoDir),
		binDir: filepath.Join(root, workspaceBinDir),
	}
	if err := created.populate(capcomPath, task); err != nil {
		return workspace{}, errors.Join(fmt.Errorf("prepare the %s workspace: %w", task.id, err), created.remove())
	}
	return created, nil
}

// populate creates the workspace's repo and bin directory and seeds the repo for task.
func (w workspace) populate(capcomPath string, task scenario) error {
	for _, dir := range []string{w.repo, w.binDir} {
		if err := os.Mkdir(dir, workspaceDirMode); err != nil {
			return err
		}
	}
	if err := installShim(filepath.Join(w.binDir, executableName(shimName))); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.name", seedUserName},
		{"config", "user.email", seedUserEmail},
	} {
		if _, err := runIn(w.repo, "git", args...); err != nil {
			return err
		}
	}
	env := seedEnv{
		repo: w.repo,
		capcom: func(args ...string) (string, error) {
			return runSeedCapcom(w.repo, capcomPath, args...)
		},
	}
	for _, args := range [][]string{{"init"}, {"agents"}} {
		if _, err := env.capcom(args...); err != nil {
			return err
		}
	}
	if err := task.seed(env); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	for _, args := range [][]string{
		{"add", "--all"},
		{"-c", "commit.gpgsign=false", "commit", "--quiet", "--message", "seed " + task.id},
	} {
		if _, err := runIn(w.repo, "git", args...); err != nil {
			return err
		}
	}
	return nil
}

// remove deletes the workspace and everything the agent left in it.
func (w workspace) remove() error {
	return os.RemoveAll(w.root)
}

// runAgent runs agentArgv in the workspace's repo with prompt on stdin and both output streams
// written to transcript, the shim first on its PATH, and the shim pointed at the build at
// capcomPath and the call log at callLogPath. The agent is stopped when it runs past timeout or
// ctx is cancelled, and on Unix every process it started is killed before runAgent returns, so
// none outlives the workspace: those in its process group, and those in the groups an agenteval
// agent loop listed in the process group ledger, which AGENTEVAL_PROCESS_GROUPS names to every
// agent. Its exit status is noted at the end of the transcript and never judges the task; only an
// agent that cannot be started, or a ledger that cannot be created, is an error.
func (w workspace) runAgent(
	ctx context.Context,
	agentArgv []string,
	prompt, capcomPath, callLogPath string,
	transcript *os.File,
	timeout time.Duration,
) (agentRun, error) {
	timed, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(timed, agentArgv[0], agentArgv[1:]...)
	command.Dir = w.repo
	command.Stdin = strings.NewReader(prompt)
	command.Stdout, command.Stderr = transcript, transcript
	command.WaitDelay = agentWaitDelay
	if w.waitDelay > 0 {
		command.WaitDelay = w.waitDelay
	}
	ledgerPath := filepath.Join(w.root, workspaceProcessGroupsFile)
	if err := os.WriteFile(ledgerPath, nil, processGroupsFileMode); err != nil {
		return agentRun{}, fmt.Errorf("create the process group ledger: %w", err)
	}
	sweep := isolateAgentProcessGroup(command, processGroupLedger(ledgerPath))
	command.Env = append(os.Environ(),
		"PATH="+w.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		capcomVariable+"="+capcomPath,
		callLogVariable+"="+callLogPath,
		processGroupsVariable+"="+ledgerPath,
	)

	started := time.Now()
	err := command.Run()
	run := agentRun{Duration: time.Since(started), IsTimedOut: errors.Is(timed.Err(), context.DeadlineExceeded)}
	if sweepErr := sweep(); sweepErr != nil {
		fmt.Fprintf(transcript, "\nagenteval: kill what the agent left running: %v\n", sweepErr)
	}

	// Only an agent without a process never started, and one the timeout beat to its start is
	// timed out. One stopped by the timeout or by ctx may exit 0 on SIGTERM, and Run then
	// returns the context's error rather than an exit error.
	if err != nil && command.Process == nil && !run.IsTimedOut {
		return agentRun{}, fmt.Errorf("start the agent %s: %w", agentArgv[0], err)
	}
	if err != nil {
		fmt.Fprintf(transcript, "\nagenteval: the agent ended with %v\n", err)
	}
	return run, nil
}

// installShim copies this executable to path, where its name makes it the shim.
func installShim(path string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find this executable to copy as the shim: %w", err)
	}
	source, err := os.Open(executable)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, shimFileMode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		return errors.Join(fmt.Errorf("copy the shim to %s: %w", path, err), target.Close())
	}
	return target.Close()
}

// executableName is the file name of the executable called name on this system: name with the
// .exe suffix on Windows, name itself elsewhere.
func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + windowsExecutableSuffix
	}
	return name
}

// runSeedCapcom runs the build at capcomPath with args in dir, as runIn does, under this executable
// started as the seed helper. capcom records the nearest ancestor of a lock acquire or start that
// is not a shell as the lock's holder process, whose descendants write without a lock warning.
// Run straight from the harness, that would be the harness itself, an ancestor of every agent
// call; the helper is that ancestor instead, and it exits with the call, so a lock the seed takes
// stays another session's lock to the agent.
func runSeedCapcom(dir, capcomPath string, args ...string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find this executable: %w", err)
	}
	return runIn(dir, executable, append([]string{seedHelperArg, capcomPath}, args...)...)
}

// isSeedHelper reports whether args, a program's arguments without its name, start it as the
// seed helper.
func isSeedHelper(args []string) bool {
	return len(args) > 0 && args[0] == seedHelperArg
}

// runSeedHelper runs the capcom build that args[1] names with the arguments after it and the given
// streams, waits for it, and returns its exit code. args[0] is seedHelperArg. It returns 1 when no
// build is named, or the build cannot be started or is killed by a signal, with the failure on
// stderr.
func runSeedHelper(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "agenteval seed helper: no capcom build named")
		return exitRunFailed
	}
	command := exec.Command(args[1], args[2:]...)
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	err := command.Run()
	if err == nil {
		return 0
	}
	var exited *exec.ExitError
	if errors.As(err, &exited) && exited.ExitCode() >= 0 {
		return exited.ExitCode()
	}
	fmt.Fprintf(stderr, "agenteval seed helper: %s: %v\n", args[1], err)
	return exitRunFailed
}

// runIn runs program with args in dir and returns its stdout. A failure is an error naming the
// command, with its stderr.
func runIn(dir, program string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command(program, args...)
	command.Dir = dir
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s",
			filepath.Base(program), strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
