package cli

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
)

// mergePropertyIterationsVariable names the environment variable that overrides how many seeded
// cases a property test runs.
const mergePropertyIterationsVariable = "CAPCOM_PROPERTY_ITERATIONS"

// defaultMergePropertyIterations is how many seeded repositories the git-level property test
// builds without the override: fewer than a unit property test runs, since each is a git repo.
const defaultMergePropertyIterations = 20

// mergePropertyPackage is the import path of the capcom binary git runs as the merge driver. An
// import path builds from any working directory inside the module.
const mergePropertyPackage = "github.com/airiclenz/capcom/cmd/capcom"

// mergePropertyMain is the branch the base issues are committed on and both branches merge into.
const mergePropertyMain = "main"

// mergePropertyBranches are the two branches cut from mergePropertyMain; the name opens every
// title, reason and Log text the branch writes, so no two writes of the two branches agree.
var mergePropertyBranches = []string{"left", "right"}

// mergePropertyIDs are the ids of the issues a seeded repository can hold, in sorted order.
var mergePropertyIDs = []string{"issue-a", "issue-b", "issue-c", "issue-d"}

// mergePropertyOp is one kind of capcom write a branch runs on an issue.
type mergePropertyOp int

const (
	opLog mergePropertyOp = iota
	opPriority
	opTitle
	opDep
	opClose
)

// mergePropertyOps are the ops a branch draws from, opLog twice so most branches add Log entries.
var mergePropertyOps = []mergePropertyOp{opLog, opLog, opPriority, opTitle, opDep, opClose}

// mergePropertyRepo is one seeded repository: where it lives, the issues it holds and the Log text
// each issue was given on any branch.
type mergePropertyRepo struct {
	t      *testing.T
	seed   uint64
	random *rand.Rand
	root   string
	ids    []string
	// logs maps an issue id to the text of every Log entry written to it, on any branch.
	logs map[string][]string
}

func TestMergeDriver_RandomBranches_KeepEveryLogEntry(t *testing.T) {
	if testing.Short() {
		t.Skip("builds capcom and a git repository per seed")
	}
	iterations := mergePropertyIterations(t)
	putCapcomOnPath(t)
	conflicted := 0

	for seed := range uint64(iterations) {
		repo := newMergePropertyRepo(t, seed)
		repo.commitBase()
		for _, branch := range mergePropertyBranches {
			repo.editBranch(branch)
		}

		if repo.mergeBranches() {
			conflicted++
		}
		repo.assertMerge()
	}
	t.Logf("%d of %d seeded merges left a conflict", conflicted, iterations)
}

// mergePropertyIterations returns how many seeded repositories the property test builds:
// CAPCOM_PROPERTY_ITERATIONS when set, defaultMergePropertyIterations otherwise.
func mergePropertyIterations(t *testing.T) int {
	t.Helper()
	value := os.Getenv(mergePropertyIterationsVariable)
	if value == "" {
		return defaultMergePropertyIterations
	}
	iterations, err := strconv.Atoi(value)
	if err != nil || iterations < 1 {
		t.Fatalf("%s=%q: expected a whole number above 0", mergePropertyIterationsVariable, value)
	}
	return iterations
}

// putCapcomOnPath builds the capcom binary into a temp directory and puts that directory first on
// PATH, so git finds it when it runs the capcom-log merge driver. It runs before any runCLI, whose
// t.Chdir would leave the module.
func putCapcomOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	name := "capcom"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, name), mergePropertyPackage)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", mergePropertyPackage, err, output)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// newMergePropertyRepo returns a git repository with a capcom store whose config defines the
// capcom-log merge driver with the git config commands init --merge-driver prints, and the random
// source of seed.
func newMergePropertyRepo(t *testing.T, seed uint64) *mergePropertyRepo {
	t.Helper()
	repo := &mergePropertyRepo{
		t:      t,
		seed:   seed,
		random: rand.New(rand.NewPCG(seed, seed)),
		root:   newGitRepo(t),
		logs:   map[string][]string{},
	}
	initOut := repo.capcom("init", "--merge-driver", "--json")
	commands, _ := decodeEnvelope(t, initOut)["data"].(map[string]any)["git_config"].([]any)
	if len(commands) == 0 {
		repo.fail("init --merge-driver --json printed no git_config:\n%s", initOut)
	}
	for _, command := range commands {
		repo.git(gitConfigArgs(repo, fmt.Sprint(command))...)
	}
	return repo
}

// gitConfigArgs splits command, a line of the form `git config <key> "<value>"`, into the git
// arguments that run it.
func gitConfigArgs(repo *mergePropertyRepo, command string) []string {
	rest, isGitConfig := strings.CutPrefix(command, "git config ")
	key, value, hasValue := strings.Cut(rest, " ")
	if !isGitConfig || !hasValue {
		repo.fail("init printed %q, want git config <key> \"<value>\"", command)
	}
	return []string{"config", key, strings.Trim(value, `"`)}
}

// commitBase creates two to four issues, some with a Log entry, and commits them on
// mergePropertyMain.
func (r *mergePropertyRepo) commitBase() {
	r.ids = mergePropertyIDs[:2+r.random.IntN(len(mergePropertyIDs)-1)]
	for _, id := range r.ids {
		r.capcom("create", "--id", id, "--title", "base "+id)
		if r.random.IntN(2) == 0 {
			r.writeLog(id, "base log of "+id)
		}
	}
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "base")
	r.git("branch", "-M", mergePropertyMain)
}

// editBranch cuts branch from mergePropertyMain, runs two to five random capcom writes on it and
// commits them.
func (r *mergePropertyRepo) editBranch(branch string) {
	r.git("checkout", "-q", "-b", branch, mergePropertyMain)
	for step := range 2 + r.random.IntN(4) {
		r.runOp(branch, step)
	}
	r.git("add", "-A")
	r.git("commit", "-q", "--allow-empty", "-m", branch)
	r.git("checkout", "-q", mergePropertyMain)
}

// runOp runs one random capcom write on a random issue of branch. Every text it writes names the
// branch and step, so the merge outcome follows from the seed alone, not from when it ran. A dep
// runs from the issue that sorts later to the one that sorts earlier, so no merge makes a cycle.
func (r *mergePropertyRepo) runOp(branch string, step int) {
	id := r.ids[r.random.IntN(len(r.ids))]
	label := fmt.Sprintf("%s step %d", branch, step)
	switch mergePropertyOps[r.random.IntN(len(mergePropertyOps))] {
	case opLog:
		r.writeLog(id, label+" log of "+id)
	case opPriority:
		r.capcom("update", id, "--priority", strconv.Itoa(r.random.IntN(5)))
	case opTitle:
		r.capcom("update", id, "--title", label+" title")
	case opDep:
		from := 1 + r.random.IntN(len(r.ids)-1)
		r.capcom("dep", "add", r.ids[from], r.ids[r.random.IntN(from)])
	case opClose:
		r.capcom("close", id, "--reason", label+" reason")
	}
}

// writeLog adds a Log entry with text to the issue id and records it as one the merge must keep.
func (r *mergePropertyRepo) writeLog(id, text string) {
	r.capcom("log", id, text)
	r.logs[id] = append(r.logs[id], text)
}

// mergeBranches merges both branches into mergePropertyMain, stopping at the first merge that
// leaves a conflict, and reports whether one did. git merge exits 1 on a conflict, which runGit
// would fail the test on.
func (r *mergePropertyRepo) mergeBranches() (isConflicted bool) {
	for _, branch := range mergePropertyBranches {
		merge := exec.Command("git", "merge", "--no-edit", "-q", branch)
		merge.Dir = r.root
		output, err := merge.CombinedOutput()
		if err == nil {
			continue
		}
		if merge.ProcessState == nil || merge.ProcessState.ExitCode() != 1 {
			r.fail("git merge %s: %v\n%s", branch, err, output)
		}
		return true
	}
	return false
}

// assertMerge checks the merged repository: capcom check reports nothing, or only conflict problems
// and exactly in the files git left unmerged, and every Log entry written on any branch is in its
// issue's Log once, read with show --json for an issue that parses and from the raw file for one
// check reports as conflicted.
func (r *mergePropertyRepo) assertMerge() {
	unmerged := r.unmergedPaths()
	conflicted := r.checkConflicts()
	if !slices.Equal(unmerged, conflicted) {
		r.fail("git left %v unmerged, check reports conflicts in %v", unmerged, conflicted)
	}
	for _, id := range r.ids {
		path := filepath.ToSlash(filepath.Join(".capcom", "issues", id+".md"))
		var logged []byte
		if slices.Contains(conflicted, path) {
			raw, err := os.ReadFile(filepath.Join(r.root, path))
			if err != nil {
				r.fail("ReadFile %s: %v", path, err)
			}
			logged = raw
		} else {
			logged = []byte(r.shownBody(id))
		}
		r.assertLog(id, logged)
	}
}

// unmergedPaths returns the files git left unmerged, sorted.
func (r *mergePropertyRepo) unmergedPaths() []string {
	diff := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	diff.Dir = r.root
	output, err := diff.Output()
	if err != nil {
		r.fail("git diff --diff-filter=U: %v", err)
	}
	paths := strings.Fields(string(output))
	slices.Sort(paths)
	return paths
}

// checkConflicts runs capcom check and returns the files it reports conflict problems in, sorted
// and once each. Any other problem fails the test.
func (r *mergePropertyRepo) checkConflicts() []string {
	stdout, stderr, exitCode := runCLI(r.t, r.root, "check", "--json")
	if exitCode != ExitOK && exitCode != ExitError {
		r.fail("check exit = %d\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
	}
	problems, _ := decodeEnvelope(r.t, stdout)["data"].([]any)
	if exitCode == ExitError && len(problems) == 0 {
		r.fail("check exited %d with no problem:\n%s\n%s", exitCode, stdout, stderr)
	}
	var paths []string
	for _, entry := range problems {
		problem, _ := entry.(map[string]any)
		if problem["code"] != "conflict" {
			r.fail("check reports a problem other than conflict: %v", problem)
		}
		paths = append(paths, fmt.Sprint(problem["path"]))
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

// shownBody returns the body show --json prints for the issue id.
func (r *mergePropertyRepo) shownBody(id string) string {
	stdout := r.capcom("show", id, "--json")
	shown, _ := decodeEnvelope(r.t, stdout)["data"].([]any)
	if len(shown) != 1 {
		r.fail("show %s --json printed %d issues, want 1:\n%s", id, len(shown), stdout)
	}
	body, _ := shown[0].(map[string]any)["body"].(string)
	return body
}

// assertLog fails the test unless the Log in data, an issue file or body, holds only entries and
// every text written to the issue id exactly once.
func (r *mergePropertyRepo) assertLog(id string, data []byte) {
	entries, isEntryOnly := issue.LogEntries(data)
	if !isEntryOnly {
		r.fail("the Log of %s holds a line that is not an entry:\n%s", id, data)
	}
	for _, text := range r.logs[id] {
		count := 0
		for _, entry := range entries {
			if strings.HasSuffix(entry, ": "+text) {
				count++
			}
		}
		if count != 1 {
			r.fail("the Log of %s holds %q %d times, want once:\n%s", id, text, count, data)
		}
	}
}

// capcom runs args through runCLI in the repository and returns stdout, failing the test unless
// it exits 0.
func (r *mergePropertyRepo) capcom(args ...string) string {
	r.t.Helper()
	stdout, stderr, exitCode := runCLI(r.t, r.root, args...)
	if exitCode != ExitOK {
		r.fail("capcom %v exit = %d\nstdout = %s\nstderr = %s", args, exitCode, stdout, stderr)
	}
	return stdout
}

// git runs git with args in the repository, failing the test when it fails.
func (r *mergePropertyRepo) git(args ...string) {
	r.t.Helper()
	command := exec.Command("git", args...)
	command.Dir = r.root
	if output, err := command.CombinedOutput(); err != nil {
		r.fail("git %v: %v\n%s", args, err, output)
	}
}

// fail stops the test with the seed and the repository path ahead of the message.
func (r *mergePropertyRepo) fail(format string, args ...any) {
	r.t.Helper()
	r.t.Fatalf("seed %d, repo %s: %s", r.seed, r.root, fmt.Sprintf(format, args...))
}
