//go:build stress

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// The stress tests run the real capcom binary as concurrent subprocesses, so they exercise the
// file locks, the SQLite index and the Tree lock across processes, which the in-process runCLI
// tests cannot. They sit behind the stress build tag:
//
//	CAPCOM_STRESS_WORKERS=4 go test -tags stress -run Stress ./internal/cli

// capcomPackage is the package the stress binary is built from. The test's working directory is
// internal/cli, so a relative ./cmd/capcom would not resolve.
const capcomPackage = "github.com/airiclenz/capcom/cmd/capcom"

// stressWorkersVariable overrides how many processes a stress test races.
const stressWorkersVariable = "CAPCOM_STRESS_WORKERS"

// defaultLockWorkers is how many processes TestStress_LockContention races by default.
const defaultLockWorkers = 8

// stressBuild builds the capcom binary once per test process; stressBinaryPath and
// stressBuildErr hold its outcome. The binary's directory outlives the tests, since no TestMain
// may remove it (bench_test.go shares this package).
var (
	stressBuild      sync.Once
	stressBinaryPath string
	stressBuildErr   error
)

// processResult is what one capcom subprocess wrote and how it exited.
type processResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// stressBinary returns the path of a capcom binary built from capcomPackage, building it on the
// first call. It must run before any t.Chdir, since go build resolves the package from the
// working directory's module.
func stressBinary(t *testing.T) string {
	t.Helper()
	stressBuild.Do(func() {
		dir, err := os.MkdirTemp("", "capcom-stress-")
		if err != nil {
			stressBuildErr = fmt.Errorf("make build dir: %w", err)
			return
		}
		path := filepath.Join(dir, "capcom")
		if runtime.GOOS == "windows" {
			path += ".exe"
		}
		build := exec.Command("go", "build", "-o", path, capcomPackage)
		if output, err := build.CombinedOutput(); err != nil {
			stressBuildErr = fmt.Errorf("go build %s: %w\n%s", capcomPackage, err, output)
			return
		}
		stressBinaryPath = path
	})
	if stressBuildErr != nil {
		t.Fatalf("build capcom: %v", stressBuildErr)
	}
	return stressBinaryPath
}

// stressWorkers returns how many processes a stress test races: CAPCOM_STRESS_WORKERS when set,
// otherwise fallback.
func stressWorkers(t *testing.T, fallback int) int {
	t.Helper()
	value, isSet := os.LookupEnv(stressWorkersVariable)
	if !isSet || value == "" {
		return fallback
	}
	workers, err := strconv.Atoi(value)
	if err != nil || workers < 1 {
		t.Fatalf("%s = %q, want a positive whole number", stressWorkersVariable, value)
	}
	return workers
}

// runBinary runs the capcom binary at binary with dir as its working directory and returns what
// it wrote and its exit code. The process inherits this process's environment, so the caller sets
// the hermetic git environment (setHermeticGitEnv) before starting any worker; runBinary itself
// never calls t.Setenv and is safe to call from several goroutines.
func runBinary(binary, dir string, args ...string) (processResult, error) {
	command := exec.Command(binary, args...)
	command.Dir = dir
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := processResult{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		result.exitCode = exitErr.ExitCode()
	case err != nil:
		return result, fmt.Errorf("run %s %v: %w", binary, args, err)
	}
	return result, nil
}

// assertCleanProcess fails the test when a process panicked or let SQLite's "database is locked"
// through: capcom must wait out or report index contention, never surface the driver's error.
func assertCleanProcess(t *testing.T, label string, result processResult) {
	t.Helper()
	output := result.stdout + result.stderr
	if strings.Contains(output, "panic:") {
		t.Errorf("%s panicked\nstdout = %s\nstderr = %s", label, result.stdout, result.stderr)
	}
	if strings.Contains(output, "database is locked") {
		t.Errorf("%s hit an unreported database lock\nstdout = %s\nstderr = %s", label, result.stdout, result.stderr)
	}
}

// raceProcesses starts workers processes of binary in dir at once, worker n with the arguments
// argsFor(n), and returns each one's result, indexed by n.
func raceProcesses(t *testing.T, binary, dir string, workers int, argsFor func(n int) []string) []processResult {
	t.Helper()
	results := make([]processResult, workers)
	runErrs := make([]error, workers)
	start := make(chan struct{})
	var group sync.WaitGroup
	for n := range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results[n], runErrs[n] = runBinary(binary, dir, argsFor(n)...)
		}()
	}
	close(start)
	group.Wait()
	for n, err := range runErrs {
		if err != nil {
			t.Fatalf("worker %d: %v", n, err)
		}
	}
	return results
}

func TestStress_LockContention(t *testing.T) {
	binary := stressBinary(t)
	workers := stressWorkers(t, defaultLockWorkers)
	root := newLockStore(t)
	setHermeticGitEnv(t)

	results := raceProcesses(t, binary, root, workers, func(n int) []string {
		return []string{"lock", "acquire", "--purpose", fmt.Sprintf("p%d", n), "--holder", fmt.Sprintf("h%d", n)}
	})

	winner := -1
	for n, result := range results {
		assertCleanProcess(t, fmt.Sprintf("worker %d", n), result)
		if result.exitCode != ExitOK {
			continue
		}
		if winner >= 0 {
			t.Fatalf("workers %d and %d both acquired the lock", winner, n)
		}
		winner = n
	}
	if winner < 0 {
		t.Fatalf("no worker of %d acquired the lock: %+v", workers, results)
	}
	holderWords := fmt.Sprintf("locked by h%d (p%d)", winner, winner)
	for n, result := range results {
		if n == winner {
			continue
		}
		if result.exitCode != ExitNotFound {
			t.Errorf("worker %d exit = %d, want %d\nstderr = %s", n, result.exitCode, ExitNotFound, result.stderr)
			continue
		}
		if !strings.Contains(result.stderr, holderWords) {
			t.Errorf("worker %d stderr = %q, want it to name the holder: %q", n, result.stderr, holderWords)
		}
	}
}
