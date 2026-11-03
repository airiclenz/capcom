//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// harnessStartWait bounds how long a signal test waits for the harness to seed its workspace and
// start the agent: seeding runs the capcom build several times, which is slow on a small host.
const harnessStartWait = time.Minute

// harnessExitWait bounds how long a signal test waits for the harness to exit once signalled. It
// is past agentWaitDelay, so a harness that waits the delay out is told apart from one that hangs.
const harnessExitWait = 2 * agentWaitDelay

// secondSignalInterval is how often the second-signal test resends SIGINT. The harness restores
// the default action on a goroutine of its own once the first signal cancelled the run, so a
// single early resend may still be caught and swallowed.
const secondSignalInterval = 50 * time.Millisecond

// signalledHarness is agenteval started as a subprocess for one run of next-claim-close, whose
// agent writes its pid to agentPidFile.
type signalledHarness struct {
	command      *exec.Cmd
	report       string
	agentPidFile string
	stderrPath   string
	done         chan struct{}
	waitErr      error
}

// startSignalledHarness starts the test binary as agenteval, running next-claim-close against the
// real capcom build with `sh -c agentScript sh <pid file>` as the agent, so the script finds the
// pid file in $1. The harness's TMPDIR is a temp dir of the test, so a workspace it leaves behind
// is removed with the test; a harness still running when the test ends is killed.
func startSignalledHarness(t *testing.T, agentScript string) *signalledHarness {
	t.Helper()
	setHermeticGitEnv(t)
	dir := t.TempDir()
	harnessTemp := t.TempDir()
	harness := &signalledHarness{
		report:       filepath.Join(dir, "report.md"),
		agentPidFile: filepath.Join(dir, "agent.pid"),
		stderrPath:   filepath.Join(dir, "stderr.log"),
		done:         make(chan struct{}),
	}
	stderr, err := os.Create(harness.stderrPath)
	if err != nil {
		t.Fatalf("create the harness's stderr file: %v", err)
	}
	defer stderr.Close()

	harness.command = exec.Command(copyTestBinary(t, dir, agentevalName),
		"run", "--out", harness.report, "--capcom", copyTestBinary(t, dir, realCapcomName),
		"--only", "next-claim-close", "--", "sh", "-c", agentScript, "sh", harness.agentPidFile,
	)
	harness.command.Env = append(os.Environ(), "TMPDIR="+harnessTemp)
	harness.command.Stderr = stderr
	if err := harness.command.Start(); err != nil {
		t.Fatalf("start the harness: %v", err)
	}
	go func() {
		harness.waitErr = harness.command.Wait()
		close(harness.done)
	}()
	t.Cleanup(func() {
		select {
		case <-harness.done:
		default:
			if err := harness.command.Process.Kill(); err != nil {
				t.Logf("kill the harness: %v", err)
			}
			<-harness.done
		}
	})
	return harness
}

// waitForAgent returns the agent's pid once the agent has written it, failing the test when the
// harness exits first or the agent has not started within harnessStartWait. The agent leads a
// process group of its own, which a harness killed outright never sweeps, so the group is killed
// when the test ends, before the temp dirs are removed.
func (h *signalledHarness) waitForAgent(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(harnessStartWait)
	for {
		if text, err := os.ReadFile(h.agentPidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(text))); err == nil {
				t.Cleanup(func() {
					if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
						t.Logf("kill the agent's group %d: %v", pid, err)
					}
					waitUntilGone(t, h.agentPidFile)
				})
				return pid
			}
		}
		select {
		case <-h.done:
			t.Fatalf("the harness exited before its agent started: %v\n%s", h.waitErr, h.stderrText(t))
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the agent did not start within %s\n%s", harnessStartWait, h.stderrText(t))
		}
		time.Sleep(processPollInterval)
	}
}

// waitForExit waits for the harness to exit, failing the test when it is still running after
// harnessExitWait.
func (h *signalledHarness) waitForExit(t *testing.T) {
	t.Helper()
	select {
	case <-h.done:
	case <-time.After(harnessExitWait):
		t.Fatalf("the harness still runs %s after the signal\n%s", harnessExitWait, h.stderrText(t))
	}
}

// signal sends sig to the harness's own process.
func (h *signalledHarness) signal(t *testing.T, sig syscall.Signal) {
	t.Helper()
	if err := h.command.Process.Signal(sig); err != nil {
		t.Fatalf("send %v to the harness: %v", sig, err)
	}
}

// stderrText returns what the harness wrote to stderr so far, for a failure message.
func (h *signalledHarness) stderrText(t *testing.T) string {
	t.Helper()
	text, err := os.ReadFile(h.stderrPath)
	if err != nil {
		return "(stderr unreadable: " + err.Error() + ")"
	}
	return string(text)
}

func TestMainSignal_StopsTheAgentAndWritesNoReport(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			harness := startSignalledHarness(t, `echo $$ > "$1"; sleep 300`)
			harness.waitForAgent(t)

			harness.signal(t, sig)
			harness.waitForExit(t)

			var exited *exec.ExitError
			if !errors.As(harness.waitErr, &exited) || exited.ExitCode() != exitRunFailed {
				t.Errorf("harness exit = %v, want exit status %d\n%s", harness.waitErr, exitRunFailed, harness.stderrText(t))
			}
			waitUntilGone(t, harness.agentPidFile)
			if _, err := os.Stat(harness.report); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("stat the report = %v, want it never written", err)
			}
		})
	}
}

func TestMainSignal_ASecondSignalEndsTheHarnessAtOnce(t *testing.T) {
	harness := startSignalledHarness(t, `trap "" TERM; echo $$ > "$1"; sleep 300`)
	// The agent ignores SIGTERM, so it outlives the harness the second signal kills; waitForAgent's
	// cleanup kills its group.
	harness.waitForAgent(t)

	signalled := time.Now()
	harness.signal(t, syscall.SIGINT)
	resend := time.NewTicker(secondSignalInterval)
	defer resend.Stop()
	for exited := false; !exited; {
		select {
		case <-harness.done:
			exited = true
		case <-resend.C:
			if time.Since(signalled) > harnessExitWait {
				t.Fatalf("the harness still runs %s after the first signal\n%s", harnessExitWait, harness.stderrText(t))
			}
			if err := harness.command.Process.Signal(syscall.SIGINT); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Fatalf("resend SIGINT to the harness: %v", err)
			}
		}
	}
	elapsed := time.Since(signalled)

	status, isWaitStatus := harness.command.ProcessState.Sys().(syscall.WaitStatus)
	if !isWaitStatus || !status.Signaled() || status.Signal() != syscall.SIGINT {
		t.Errorf("harness exit = %v, want killed by SIGINT\n%s", harness.waitErr, harness.stderrText(t))
	}
	if limit := agentWaitDelay / 2; elapsed >= limit {
		t.Errorf("the harness took %s to exit after the first signal, want under %s", elapsed, limit)
	}
}
