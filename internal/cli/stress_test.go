//go:build stress

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The stress tests run the real capcom binary as concurrent subprocesses, so they exercise the
// file locks, the SQLite index and the Tree lock across processes, which the in-process runCLI
// tests cannot. They sit behind the stress build tag:
//
//	CAPCOM_STRESS_WORKERS=4 go test -tags stress -run Stress ./internal/cli
//
// A test that makes random choices logs its seed; CAPCOM_STRESS_SEED=<seed> replays those choices.

// capcomPackage is the package the stress binary is built from. The test's working directory is
// internal/cli, so a relative ./cmd/capcom would not resolve.
const capcomPackage = "github.com/airiclenz/capcom/cmd/capcom"

// stressWorkersVariable overrides how many processes a stress test races.
const stressWorkersVariable = "CAPCOM_STRESS_WORKERS"

// defaultLockWorkers is how many processes TestStress_LockContention races by default.
const defaultLockWorkers = 8

// stressSeedVariable fixes the seed of the random command sequences, to replay a failed run.
const stressSeedVariable = "CAPCOM_STRESS_SEED"

// defaultMixedWorkers is how many goroutines TestStress_OneWorktree runs by default, and
// mixedCommandsPerWorker how many commands each one runs.
const (
	defaultMixedWorkers    = 16
	mixedCommandsPerWorker = 50
)

// busyWaitFloor is how long a command must have run before its "the index is busy" failure counts
// as documented: the index waits 5 seconds (index.defaultBusyTimeout) for another command's write
// before it fails busy, and SQLite's busy handler can give up a little early. A busy failure that
// comes sooner skipped that wait, which is a bug.
const busyWaitFloor = 4 * time.Second

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

// stressSeed returns the seed of a stress test's random choices: CAPCOM_STRESS_SEED when set,
// otherwise a fresh one. It logs the seed, so a failed run can be replayed.
func stressSeed(t *testing.T) uint64 {
	t.Helper()
	seed := rand.Uint64()
	if value, isSet := os.LookupEnv(stressSeedVariable); isSet && value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			t.Fatalf("%s = %q, want a whole number", stressSeedVariable, value)
		}
		seed = parsed
	}
	t.Logf("seed %d; replay with %s=%d", seed, stressSeedVariable, seed)
	return seed
}

// mustRunBinary runs the capcom binary in dir from the test goroutine and fails the test unless it
// exits ok and cleanly; it returns stdout.
func mustRunBinary(t *testing.T, binary, dir string, args ...string) string {
	t.Helper()
	result, err := runBinary(binary, dir, args...)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assertCleanProcess(t, fmt.Sprintf("%v", args), result)
	if result.exitCode != ExitOK {
		t.Fatalf("%v exit = %d\nstdout = %s\nstderr = %s", args, result.exitCode, result.stdout, result.stderr)
	}
	return result.stdout
}

// mixedCommand is one command a mixedWorker ran and how it ended.
type mixedCommand struct {
	args    []string
	result  processResult
	elapsed time.Duration
	// documented reports whether the command ended as its documentation allows: exit 0, or one of
	// the failures the worker expects of it.
	documented bool
}

// mixedWorker runs one goroutine's random commands against a shared store, choosing only among
// its own Issues and its own lock purpose, and remembers what it wrote. Its random choices come
// from its own seeded source; the outcome of a dep add or a lock acquire, which other workers
// decide, feeds back into later choices, so a replay repeats the choices, not the interleaving.
type mixedWorker struct {
	n       int
	binary  string
	dir     string
	random  *rand.Rand
	issues  []string
	open    []string
	logs    map[string][]string
	token   string
	ran     []mixedCommand
	runErr  error
	counter int
}

// The commands a mixedWorker chooses among.
const (
	mixedCreate = iota
	mixedLog
	mixedUpdate
	mixedDepAdd
	mixedClose
	mixedReady
	mixedPrime
	mixedReindex
	mixedLock
	mixedCommandCount
)

// run runs count commands, then frees the lock when the worker still holds it.
func (w *mixedWorker) run(count int) {
	for range count {
		if w.runErr != nil {
			return
		}
		w.step(w.choose())
	}
	if w.token != "" && w.runErr == nil {
		w.releaseLock()
	}
}

// choose picks the next command, falling back to create while the worker has too few Issues for
// the one picked.
func (w *mixedWorker) choose() int {
	command := w.random.IntN(mixedCommandCount)
	switch {
	case command == mixedDepAdd && len(w.issues) < 2:
		return mixedCreate
	case command == mixedClose && len(w.open) == 0:
		return mixedCreate
	case (command == mixedLog || command == mixedUpdate) && len(w.issues) == 0:
		return mixedCreate
	}
	return command
}

// step runs one command of the kind command.
func (w *mixedWorker) step(command int) {
	w.counter++
	switch command {
	case mixedCreate:
		id := fmt.Sprintf("w%d-i%d", w.n, w.counter)
		result := w.exec("create", "--id", id, "--title", "Stress "+id, "--body", "Stress body.")
		if result.exitCode == ExitOK {
			w.issues = append(w.issues, id)
			w.open = append(w.open, id)
		}
	case mixedLog:
		id := w.pick(w.issues)
		text := fmt.Sprintf("stress entry w%d-c%d", w.n, w.counter)
		if w.exec("log", id, text).exitCode == ExitOK {
			w.logs[id] = append(w.logs[id], text)
		}
	case mixedUpdate:
		w.exec("update", w.pick(w.issues), "--priority", strconv.Itoa(w.random.IntN(5)))
	case mixedDepAdd:
		w.depAdd()
	case mixedClose:
		index := w.random.IntN(len(w.open))
		id := w.open[index]
		if w.exec("close", id, "--reason", "stress").exitCode == ExitOK {
			w.open = append(w.open[:index], w.open[index+1:]...)
		}
	case mixedReady:
		w.exec("ready")
	case mixedPrime:
		w.exec("prime")
	case mixedReindex:
		w.exec("reindex")
	case mixedLock:
		if w.token == "" {
			w.acquireLock()
		} else {
			w.releaseLock()
		}
	}
}

// depAdd makes one of the worker's Issues depend on another of them; a refusal that names the
// cycle the edge would close is documented.
func (w *mixedWorker) depAdd() {
	first := w.random.IntN(len(w.issues))
	second := (first + 1 + w.random.IntN(len(w.issues)-1)) % len(w.issues)
	w.execAllowing(func(result processResult) bool {
		return result.exitCode == ExitError && strings.Contains(result.stderr, "would create the cycle")
	}, "dep", "add", w.issues[first], w.issues[second])
}

// acquireLock claims the Tree lock for the worker's own purpose and keeps the token it prints; a
// refusal because another worker holds the lock is documented.
func (w *mixedWorker) acquireLock() {
	result := w.execAllowing(func(result processResult) bool {
		return result.exitCode == ExitNotFound && strings.Contains(result.stdout, "STATUS=locked")
	}, "lock", "acquire", "--purpose", fmt.Sprintf("p%d", w.n), "--holder", fmt.Sprintf("h%d", w.n), "--kv")
	if result.exitCode != ExitOK {
		return
	}
	for _, line := range strings.Split(result.stdout, "\n") {
		if token, found := strings.CutPrefix(line, "TOKEN="); found {
			w.token = token
		}
	}
}

// releaseLock frees the lock the worker holds, by the token its own acquire printed.
func (w *mixedWorker) releaseLock() {
	if w.exec("lock", "release", "--token", w.token).exitCode == ExitOK {
		w.token = ""
	}
}

// pick returns a random element of ids.
func (w *mixedWorker) pick(ids []string) string {
	return ids[w.random.IntN(len(ids))]
}

// exec runs one command that is documented only when it exits 0.
func (w *mixedWorker) exec(args ...string) processResult {
	return w.execAllowing(func(processResult) bool { return false }, args...)
}

// execAllowing runs one command and records it; it is documented when it exits 0, when
// isExpectedFailure accepts how it failed, or when it failed busy after waiting out the index's
// busy timeout, as many commands writing the one index at once can. A process that cannot be
// started stops the worker.
func (w *mixedWorker) execAllowing(isExpectedFailure func(processResult) bool, args ...string) processResult {
	started := time.Now()
	result, err := runBinary(w.binary, w.dir, args...)
	elapsed := time.Since(started)
	if err != nil {
		w.runErr = err
		return result
	}
	isBusyAfterWait := result.exitCode == ExitError && strings.Contains(result.stderr, "the index is busy") &&
		elapsed >= busyWaitFloor
	documented := result.exitCode == ExitOK || isExpectedFailure(result) || isBusyAfterWait
	w.ran = append(w.ran, mixedCommand{args: args, result: result, elapsed: elapsed, documented: documented})
	return result
}

func TestStress_OneWorktree(t *testing.T) {
	binary := stressBinary(t)
	workers := stressWorkers(t, defaultMixedWorkers)
	seed := stressSeed(t)
	root := newLockStore(t)
	setHermeticGitEnv(t)

	mixed := make([]*mixedWorker, workers)
	for n := range workers {
		mixed[n] = &mixedWorker{
			n:      n,
			binary: binary,
			dir:    root,
			random: rand.New(rand.NewPCG(seed, uint64(n))),
			logs:   map[string][]string{},
		}
	}
	start := make(chan struct{})
	var group sync.WaitGroup
	for _, worker := range mixed {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			worker.run(mixedCommandsPerWorker)
		}()
	}
	close(start)
	group.Wait()

	for _, worker := range mixed {
		if worker.runErr != nil {
			t.Fatalf("worker %d: %v", worker.n, worker.runErr)
		}
		for _, command := range worker.ran {
			label := fmt.Sprintf("worker %d: %v", worker.n, command.args)
			assertCleanProcess(t, label, command.result)
			if !command.documented {
				t.Errorf("%s exit = %d after %v\nstdout = %s\nstderr = %s",
					label, command.result.exitCode, command.elapsed, command.result.stdout, command.result.stderr)
			}
		}
	}

	mustRunBinary(t, binary, root, "check")
	for _, worker := range mixed {
		for _, id := range worker.issues {
			data, err := os.ReadFile(filepath.Join(root, ".capcom", "issues", id+".md"))
			if err != nil {
				t.Errorf("worker %d created %s, but its file cannot be read: %v", worker.n, id, err)
				continue
			}
			content := string(data)
			if !strings.Contains(content, "title: Stress "+id+"\n") {
				t.Errorf("%s file lacks its title:\n%s", id, content)
			}
			for _, text := range worker.logs[id] {
				if !strings.Contains(content, ": "+text+"\n") {
					t.Errorf("%s file lacks the Log entry %q:\n%s", id, text, content)
				}
			}
		}
	}
	before := mustRunBinary(t, binary, root, "list", "--all", "--json")
	mustRunBinary(t, binary, root, "reindex")
	after := mustRunBinary(t, binary, root, "list", "--all", "--json")
	if before != after {
		t.Errorf("list --all --json changed across reindex\nbefore = %s\nafter  = %s", before, after)
	}
}
