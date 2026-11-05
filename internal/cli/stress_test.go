//go:build stress

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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

// stressWorktrees are the linked worktrees TestStress_Worktrees adds, each name also the tag of
// its workers' holders, purposes and Issue ids.
var stressWorktrees = []string{"tree-a", "tree-b", "tree-c"}

// oneWorktreeTag tags the holders, purposes and Issue ids of TestStress_OneWorktree's workers.
const oneWorktreeTag = "main"

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
// its own Issues and its own lock purpose, and remembers what it wrote. Its holder, purpose and
// Issue ids start with its tag and a dash, so a lock or an Issue can be traced to the workers of
// one store. Its random choices come from its own seeded source; the outcome of a dep add or a
// lock acquire, which other workers decide, feeds back into later choices, so a replay repeats the
// choices, not the interleaving.
type mixedWorker struct {
	n       int
	tag     string
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
		id := fmt.Sprintf("%s-w%d-i%d", w.tag, w.n, w.counter)
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
	}, "lock", "acquire", "--purpose", fmt.Sprintf("%s-p%d", w.tag, w.n), "--holder", fmt.Sprintf("%s-%d", w.tag, w.n), "--kv")
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

// newMixedWorkers returns count workers tagged tag on the store at dir, worker n drawing its
// choices from the stream firstStream+n of seed, so workers on different stores choose apart.
func newMixedWorkers(binary, dir, tag string, count int, seed, firstStream uint64) []*mixedWorker {
	workers := make([]*mixedWorker, count)
	for n := range count {
		workers[n] = &mixedWorker{
			n:      n,
			tag:    tag,
			binary: binary,
			dir:    dir,
			random: rand.New(rand.NewPCG(seed, firstStream+uint64(n))),
			logs:   map[string][]string{},
		}
	}
	return workers
}

// runTogether starts every job at once, each in its own goroutine, and waits for all of them.
func runTogether(jobs ...func()) {
	start := make(chan struct{})
	var group sync.WaitGroup
	for _, job := range jobs {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			job()
		}()
	}
	close(start)
	group.Wait()
}

// workloadJobs returns one job per worker, running mixedCommandsPerWorker commands.
func workloadJobs(workers []*mixedWorker) []func() {
	jobs := make([]func(), len(workers))
	for n, worker := range workers {
		jobs[n] = func() { worker.run(mixedCommandsPerWorker) }
	}
	return jobs
}

// assertDocumentedCommands fails the test when a worker could not start a process, or when one of
// its commands panicked, let a database lock through or failed in a way it does not document.
func assertDocumentedCommands(t *testing.T, workers []*mixedWorker) {
	t.Helper()
	for _, worker := range workers {
		if worker.runErr != nil {
			t.Fatalf("%s worker %d: %v", worker.tag, worker.n, worker.runErr)
		}
		for _, command := range worker.ran {
			label := fmt.Sprintf("%s worker %d: %v", worker.tag, worker.n, command.args)
			assertCleanProcess(t, label, command.result)
			if !command.documented {
				t.Errorf("%s exit = %d after %v\nstdout = %s\nstderr = %s",
					label, command.result.exitCode, command.elapsed, command.result.stdout, command.result.stderr)
			}
		}
	}
}

// assertStoreHoldsWrites fails the test unless the store at dir passes check, holds every Issue
// its workers created and every Log entry they wrote, and lists the same Issues before and after
// a reindex. It returns that listing.
func assertStoreHoldsWrites(t *testing.T, binary, dir string, workers []*mixedWorker) string {
	t.Helper()
	mustRunBinary(t, binary, dir, "check")
	for _, worker := range workers {
		for _, id := range worker.issues {
			data, err := os.ReadFile(filepath.Join(dir, ".capcom", "issues", id+".md"))
			if err != nil {
				t.Errorf("%s worker %d created %s, but its file cannot be read: %v", worker.tag, worker.n, id, err)
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
	before := mustRunBinary(t, binary, dir, "list", "--all", "--json")
	mustRunBinary(t, binary, dir, "reindex")
	after := mustRunBinary(t, binary, dir, "list", "--all", "--json")
	if before != after {
		t.Errorf("%s: list --all --json changed across reindex\nbefore = %s\nafter  = %s", dir, before, after)
	}
	return after
}

func TestStress_OneWorktree(t *testing.T) {
	binary := stressBinary(t)
	workers := stressWorkers(t, defaultMixedWorkers)
	seed := stressSeed(t)
	root := newLockStore(t)
	setHermeticGitEnv(t)
	mixed := newMixedWorkers(binary, root, oneWorktreeTag, workers, seed, 0)

	runTogether(workloadJobs(mixed)...)

	assertDocumentedCommands(t, mixed)
	assertStoreHoldsWrites(t, binary, root, mixed)
}

// lockPoller runs capcom lock status in one worktree, over and over, until it is told to stop,
// and keeps every result.
type lockPoller struct {
	binary  string
	dir     string
	results []processResult
	runErr  error
}

// run polls the worktree's lock until done is closed. A process that cannot be started stops it.
func (p *lockPoller) run(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		default:
		}
		result, err := runBinary(p.binary, p.dir, "lock", "status", "--kv")
		if err != nil {
			p.runErr = err
			return
		}
		p.results = append(p.results, result)
	}
}

// foreignHolders returns every holder result names, on a HOLDER= line or in a "locked by"
// message, whose name does not start with tag and a dash.
func foreignHolders(result processResult, tag string) []string {
	var foreign []string
	isForeign := func(holder string) bool { return holder != "" && !strings.HasPrefix(holder, tag+"-") }
	for _, line := range strings.Split(result.stdout, "\n") {
		if holder, found := strings.CutPrefix(line, "HOLDER="); found && isForeign(holder) {
			foreign = append(foreign, holder)
		}
	}
	rest := result.stdout + result.stderr
	for {
		_, after, found := strings.Cut(rest, "locked by ")
		if !found {
			return foreign
		}
		holder, _, _ := strings.Cut(after, " ")
		if isForeign(holder) {
			foreign = append(foreign, holder)
		}
		rest = after
	}
}

// assertOwnHolders fails the test when a command of one worktree's workers, or a lock status its
// poller ran, names a holder of another worktree.
func assertOwnHolders(t *testing.T, tag string, workers []*mixedWorker, poller *lockPoller) {
	t.Helper()
	for _, worker := range workers {
		for _, command := range worker.ran {
			if foreign := foreignHolders(command.result, tag); len(foreign) > 0 {
				t.Errorf("%s worker %d: %v names the holders %v of another worktree\nstdout = %s\nstderr = %s",
					tag, worker.n, command.args, foreign, command.result.stdout, command.result.stderr)
			}
		}
	}
	if poller.runErr != nil {
		t.Fatalf("%s lock status: %v", tag, poller.runErr)
	}
	for _, result := range poller.results {
		assertCleanProcess(t, tag+" lock status", result)
		if result.exitCode != ExitOK {
			t.Errorf("%s lock status exit = %d\nstdout = %s\nstderr = %s", tag, result.exitCode, result.stdout, result.stderr)
		}
		if foreign := foreignHolders(result, tag); len(foreign) > 0 {
			t.Errorf("%s lock status names the holders %v of another worktree\nstdout = %s", tag, foreign, result.stdout)
		}
	}
}

// assertIndexMatchesFiles fails the test unless listing, a list --all --json of the worktree at
// dir, names exactly the Issue files under its .capcom/issues, each one tagged tag.
func assertIndexMatchesFiles(t *testing.T, dir, tag, listing string) {
	t.Helper()
	var envelope struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(listing), &envelope); err != nil {
		t.Fatalf("%s: decode list --all --json: %v\n%s", tag, err, listing)
	}
	listed := make([]string, 0, len(envelope.Data))
	for _, issue := range envelope.Data {
		listed = append(listed, issue.ID)
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".capcom", "issues"))
	if err != nil {
		t.Fatalf("%s: read the Issue files: %v", tag, err)
	}
	var files []string
	for _, entry := range entries {
		if id, found := strings.CutSuffix(entry.Name(), ".md"); found && !entry.IsDir() {
			files = append(files, id)
		}
	}
	slices.Sort(listed)
	slices.Sort(files)
	if !slices.Equal(listed, files) {
		t.Errorf("%s: the index lists %v, the files are %v", tag, listed, files)
	}
	for _, id := range files {
		if !strings.HasPrefix(id, tag+"-") {
			t.Errorf("%s holds %s, an Issue of another worktree", tag, id)
		}
	}
}

func TestStress_Worktrees(t *testing.T) {
	binary := stressBinary(t)
	workers := stressWorkers(t, defaultMixedWorkers)
	seed := stressSeed(t)
	root := newLockStore(t)
	setHermeticGitEnv(t)
	dirs := make([]string, len(stressWorktrees))
	mixed := make([][]*mixedWorker, len(stressWorktrees))
	pollers := make([]*lockPoller, len(stressWorktrees))
	for index, tag := range stressWorktrees {
		dirs[index] = addWorktree(t, root, tag)
		mixed[index] = newMixedWorkers(binary, dirs[index], tag, workers, seed, uint64(index*workers))
		pollers[index] = &lockPoller{binary: binary, dir: dirs[index]}
	}
	done := make(chan struct{})
	var workloads sync.WaitGroup
	var jobs []func()
	for index := range stressWorktrees {
		for _, job := range workloadJobs(mixed[index]) {
			workloads.Add(1)
			jobs = append(jobs, func() {
				defer workloads.Done()
				job()
			})
		}
		poller := pollers[index]
		jobs = append(jobs, func() { poller.run(done) })
	}
	go func() {
		workloads.Wait()
		close(done)
	}()

	runTogether(jobs...)

	for index, tag := range stressWorktrees {
		assertDocumentedCommands(t, mixed[index])
		assertOwnHolders(t, tag, mixed[index], pollers[index])
		listing := assertStoreHoldsWrites(t, binary, dirs[index], mixed[index])
		assertIndexMatchesFiles(t, dirs[index], tag, listing)
	}
	for index, tag := range stressWorktrees {
		mustRunBinary(t, binary, dirs[index], "lock", "acquire", "--purpose", tag+"-final", "--holder", tag+"-final")
	}
	for index, tag := range stressWorktrees {
		status := mustRunBinary(t, binary, dirs[index], "lock", "status", "--kv")
		if holder := kvValue(t, status, "HOLDER"); holder != tag+"-final" {
			t.Errorf("%s lock status HOLDER = %q, want %q", tag, holder, tag+"-final")
		}
	}
}
