package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// benchSizes are the issue counts every command benchmark runs at; benchLargeSize joins them when
// benchLargeEnv is 1, since a 50000-issue store takes minutes to build and measure.
var benchSizes = []int{1000, 10000}

const (
	benchLargeSize = 50000
	benchLargeEnv  = "CAPCOM_BENCH_LARGE"
)

// The shape of a generated benchmark store, besides its issue count.
const (
	benchNoteCount     = 200
	benchCommitCount   = 500
	benchBranchCount   = 20
	benchMaxDepth      = 4
	benchMaxLogs       = 5
	benchMaxDeps       = 3
	benchBranchEdits   = 3
	benchNotesPerGroup = 10
)

// benchParentChance is the chance that a generated issue is the child of an earlier one, and
// benchSearchChance the chance that its body holds benchSearchWord, so search finds a few hits
// rather than every issue.
const (
	benchParentChance = 0.6
	benchSearchChance = 0.05
	benchSearchWord   = "throughput"
)

// benchSeed seeds the store content, so every run generates the same issues and Notes. Commit
// dates are not seeded: they are relative to the time the store is built.
const benchSeed = 20261105

// benchLogIssue is the issue BenchmarkLog writes to: the first generated issue, which is top level
// and exists from the first commit at every store size.
const benchLogIssue = "bench-00000"

// benchHistorySpan is how far back the first generated commit lies, benchMainTipAge how old the
// last commit on the checked-out branch is, and benchRecapDays the window recap is asked about:
// most of the history is older than that window, and every branch tip is inside the 30-day
// elsewhere.days default.
const (
	benchHistorySpan = 150 * 24 * time.Hour
	benchMainTipAge  = 2 * 24 * time.Hour
	benchRecapDays   = 30
)

// benchContentEpoch is the first created stamp of the generated issues; content dates are fixed
// so the files are the same on every run.
var benchContentEpoch = time.Date(2025, time.January, 6, 9, 0, 0, 0, time.UTC)

// benchStatuses are the statuses generated issues take, repeated to weight them: mostly open,
// with active, parked and closed issues mixed in.
var benchStatuses = []string{"open", "open", "open", "open", "open", "active", "parked", "done", "done", "dropped"}

// benchWords is the vocabulary of generated titles and bodies. It never holds benchSearchWord.
var benchWords = strings.Fields(`parser index branch config render table merge store lock tree
	note issue graph cache flag output error stream token commit history window search label
	priority status reason worker queue schema format upgrade report export import refresh`)

var benchTypes = []string{"bug", "feature", "task", "chore"}

var benchLabels = []string{"parser", "index", "ui", "git", "docs", "perf", "release"}

// benchShape is what newBenchStore generates.
type benchShape struct {
	issues   int
	notes    int
	commits  int
	branches int
}

// defaultBenchShape is the shape every command benchmark measures, at issues issues.
func defaultBenchShape(issues int) benchShape {
	return benchShape{issues: issues, notes: benchNoteCount, commits: benchCommitCount, branches: benchBranchCount}
}

// benchIssue is one generated issue: its frontmatter, its body, the commit on the checked-out
// branch that adds it and the commits that add each of its Log entries, in order.
type benchIssue struct {
	id         string
	path       string
	depth      int
	title      string
	status     string
	kind       string
	priority   int
	labels     []string
	deps       []string
	created    time.Time
	body       string
	commit     int
	logCommits []int
}

// benchBranch is one generated branch: the commit it forks from and the issues its one commit
// changes.
type benchBranch struct {
	name   string
	fork   int
	issues []int
}

// newBenchStore builds a git repository holding a capcom store of the given shape in a temp dir
// of tb and returns its root. The checked-out branch has shape.commits commits that add the
// issues and Notes and then their Log entries; shape.branches more branches each fork from a
// recent commit and change a few issues. The whole history is streamed to one git fast-import,
// since one git process per commit would take minutes at 10000 issues.
//
// Each issue is the child of an earlier one or top level, at most benchMaxDepth deep, and depends
// only on earlier issues, so the deps are acyclic.
func newBenchStore(tb testing.TB, shape benchShape) string {
	tb.Helper()
	root := newGitRepo(tb)
	if _, stderr, exitCode := runCLI(tb, root, "init"); exitCode != ExitOK {
		tb.Fatalf("init exit = %d stderr = %s", exitCode, stderr)
	}
	configPath := filepath.Join(root, ".capcom", "config.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		tb.Fatalf("ReadFile: %v", err)
	}
	// The history adds the config again; reset --hard then writes it back tracked.
	if err := os.Remove(configPath); err != nil {
		tb.Fatalf("Remove: %v", err)
	}

	random := rand.New(rand.NewPCG(benchSeed, uint64(shape.issues)))
	issues := benchIssues(random, shape)
	branches := benchBranches(random, shape, issues)
	headRef := gitOutput(tb, root, "symbolic-ref", "HEAD")

	command := exec.Command("git", "fast-import", "--quiet")
	command.Dir = root
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	stdin, err := command.StdinPipe()
	if err != nil {
		tb.Fatalf("StdinPipe: %v", err)
	}
	if err := command.Start(); err != nil {
		tb.Fatalf("git fast-import: %v", err)
	}
	stream := bufio.NewWriter(stdin)
	writeBenchHistory(stream, headRef, shape, issues, branches, string(config), time.Now())
	if err := stream.Flush(); err != nil {
		tb.Fatalf("write fast-import stream: %v", err)
	}
	if err := stdin.Close(); err != nil {
		tb.Fatalf("close fast-import stream: %v", err)
	}
	if err := command.Wait(); err != nil {
		tb.Fatalf("git fast-import: %v\n%s", err, output.Bytes())
	}
	runGit(tb, root, "reset", "--hard", "-q")
	// The index re-reads a file whose mtime or change time falls in the second of the refresh that
	// recorded it (racy-clean), so the first command must start after the second the checkout wrote
	// in, or the commands after it re-read those files and the index is never warm.
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	return root
}

// benchIssues generates the issues of shape. The first half are added by the first commit, the
// rest spread over the later ones in order, so a parent and every dep exist before the issue.
func benchIssues(random *rand.Rand, shape benchShape) []benchIssue {
	issues := make([]benchIssue, shape.issues)
	initialCount := shape.issues / 2
	for number := range issues {
		item := &issues[number]
		item.id = fmt.Sprintf("bench-%05d", number)
		item.path = ".capcom/issues/" + item.id + ".md"
		item.depth = 1
		if number > 0 && random.Float64() < benchParentChance {
			if parent := issues[random.IntN(number)]; parent.depth < benchMaxDepth {
				item.path = strings.TrimSuffix(parent.path, ".md") + "/" + item.id + ".md"
				item.depth = parent.depth + 1
			}
		}
		item.title = benchSentence(random, 4)
		item.status = benchStatuses[random.IntN(len(benchStatuses))]
		item.kind = benchTypes[random.IntN(len(benchTypes))]
		item.priority = random.IntN(5)
		item.labels = []string{benchLabels[random.IntN(len(benchLabels))]}
		if number > 0 {
			for range random.IntN(benchMaxDeps + 1) {
				dep := issues[random.IntN(number)].id
				if !slices.Contains(item.deps, dep) {
					item.deps = append(item.deps, dep)
				}
			}
		}
		item.created = benchContentEpoch.Add(time.Duration(number) * time.Hour)
		item.body = benchSentence(random, 24)
		if random.Float64() < benchSearchChance {
			item.body += " The " + benchSearchWord + " of this path matters."
		}

		if number >= initialCount && shape.commits > 1 {
			item.commit = 1 + (number-initialCount)*(shape.commits-1)/(shape.issues-initialCount)
		}
		for range random.IntN(benchMaxLogs + 1) {
			item.logCommits = append(item.logCommits, item.commit+random.IntN(shape.commits-item.commit))
		}
		slices.Sort(item.logCommits)
	}
	return issues
}

// benchBranches generates the branches of shape: each forks from one of the newest commits and
// changes benchBranchEdits issues that exist there.
func benchBranches(random *rand.Rand, shape benchShape, issues []benchIssue) []benchBranch {
	branches := make([]benchBranch, shape.branches)
	for number := range branches {
		branch := &branches[number]
		branch.name = fmt.Sprintf("bench-branch-%02d", number)
		branch.fork = shape.commits - 1 - number%shape.commits
		existing := 0
		for existing < len(issues) && issues[existing].commit <= branch.fork {
			existing++
		}
		for range benchBranchEdits {
			branch.issues = append(branch.issues, random.IntN(existing))
		}
		slices.Sort(branch.issues)
		branch.issues = slices.Compact(branch.issues)
	}
	return branches
}

// benchSentence returns count words of benchWords, the first capitalised, ending in a full stop.
func benchSentence(random *rand.Rand, count int) string {
	words := make([]string, count)
	for index := range words {
		words[index] = benchWords[random.IntN(len(benchWords))]
	}
	return strings.ToUpper(words[0][:1]) + strings.Join(words, " ")[1:] + "."
}

// writeBenchHistory writes the git fast-import stream of the generated history to stream: the
// commits of headRef, then one commit per branch. Commit c of headRef lies benchHistorySpan
// before now when c is 0 and benchMainTipAge before now when it is the last; branch tips are
// newer still, an hour apart.
func writeBenchHistory(
	stream io.Writer,
	headRef string,
	shape benchShape,
	issues []benchIssue,
	branches []benchBranch,
	config string,
	now time.Time,
) {
	changedByCommit := make([][]int, shape.commits)
	for number, item := range issues {
		changedByCommit[item.commit] = append(changedByCommit[item.commit], number)
		for _, commit := range item.logCommits {
			changedByCommit[commit] = append(changedByCommit[commit], number)
		}
	}

	commitStep := time.Duration(0)
	if shape.commits > 1 {
		commitStep = (benchHistorySpan - benchMainTipAge) / time.Duration(shape.commits-1)
	}
	for commit, changed := range changedByCommit {
		at := now.Add(-benchHistorySpan + time.Duration(commit)*commitStep)
		writeBenchCommit(stream, headRef, commit+1, 0, at, fmt.Sprintf("bench: commit %d", commit))
		if commit == 0 {
			writeBenchFile(stream, ".capcom/config.yaml", config)
			for number := range shape.notes {
				writeBenchFile(stream, benchNotePath(number), benchNote(number))
			}
		}
		slices.Sort(changed)
		for _, number := range slices.Compact(changed) {
			writeBenchFile(stream, issues[number].path, renderBenchIssue(issues[number], commit, ""))
		}
	}

	for number, branch := range branches {
		at := now.Add(-time.Duration(number+1) * time.Hour)
		mark := shape.commits + 1 + number
		writeBenchCommit(stream, "refs/heads/"+branch.name, mark, branch.fork+1, at, "bench: "+branch.name)
		for _, issueNumber := range branch.issues {
			item := issues[issueNumber]
			writeBenchFile(stream, item.path, renderBenchIssue(item, branch.fork, branch.name))
		}
	}
}

// writeBenchCommit starts commit mark on ref at time at; from, when not 0, is the mark of its
// parent, which a new branch needs and a branch continuing in the stream does not.
func writeBenchCommit(stream io.Writer, ref string, mark int, from int, at time.Time, message string) {
	stamp := fmt.Sprintf("%d +0000", at.Unix())
	fmt.Fprintf(stream, "commit %s\nmark :%d\n", ref, mark)
	fmt.Fprintf(stream, "author Test Author <author@example.com> %s\n", stamp)
	fmt.Fprintf(stream, "committer Test Committer <committer@example.com> %s\n", stamp)
	fmt.Fprintf(stream, "data %d\n%s\n", len(message), message)
	if from != 0 {
		fmt.Fprintf(stream, "from :%d\n", from)
	}
}

// writeBenchFile adds path with content to the commit being written.
func writeBenchFile(stream io.Writer, path, content string) {
	fmt.Fprintf(stream, "M 100644 inline %s\ndata %d\n%s\n", path, len(content), content)
}

// renderBenchIssue returns item's file as of commit: the Log entries added up to it, and, when
// branch is not empty, the change that branch makes: an open issue becomes active and gains an
// entry.
func renderBenchIssue(item benchIssue, commit int, branch string) string {
	status := item.status
	if branch != "" && status == "open" {
		status = "active"
	}
	var text strings.Builder
	fmt.Fprintf(&text, "---\ntitle: %s\nstatus: %s\ntype: %s\npriority: %d\n", item.title, status, item.kind, item.priority)
	fmt.Fprintf(&text, "labels: [%s]\n", strings.Join(item.labels, ", "))
	if len(item.deps) > 0 {
		fmt.Fprintf(&text, "deps: [%s]\n", strings.Join(item.deps, ", "))
	}
	fmt.Fprintf(&text, "created: %s\n", item.created.Format(time.RFC3339))
	if status == "done" || status == "dropped" {
		fmt.Fprintf(&text, "closed: %s\nreason: settled\n", item.created.Add(24*time.Hour).Format(time.RFC3339))
	}
	fmt.Fprintf(&text, "---\n\n%s\n\n## Acceptance\n\n- The %s works.\n", item.body, item.id)

	var entries []string
	for number, logCommit := range item.logCommits {
		if logCommit <= commit {
			date := item.created.AddDate(0, 0, number+1).Format(time.DateOnly)
			entries = append(entries, fmt.Sprintf("- %s Test Author: progress step %d.", date, number+1))
		}
	}
	if branch != "" {
		entries = append(entries, fmt.Sprintf("- %s Test Author: picked up on %s.", item.created.AddDate(0, 0, benchMaxLogs+1).Format(time.DateOnly), branch))
	}
	if len(entries) > 0 {
		text.WriteString("\n## Log\n\n" + strings.Join(entries, "\n") + "\n")
	}
	return text.String()
}

// benchNotePath returns the path of Note number: every benchNotesPerGroup-th Note is top level and
// the ones after it are its children.
func benchNotePath(number int) string {
	name := fmt.Sprintf("note-%03d.md", number)
	if group := number / benchNotesPerGroup * benchNotesPerGroup; group != number {
		return fmt.Sprintf(".capcom/knowledge/note-%03d/%s", group, name)
	}
	return ".capcom/knowledge/" + name
}

// benchNote returns the file of Note number.
func benchNote(number int) string {
	return fmt.Sprintf("---\ntitle: Benchmark note %d\nlabels: [%s]\n---\n\nNote %d records how the %s works.\n",
		number, benchLabels[number%len(benchLabels)], number, benchWords[number%len(benchWords)])
}

// gitOutput runs git with args in dir and returns its trimmed stdout.
func gitOutput(tb testing.TB, dir string, args ...string) string {
	tb.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		tb.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}

// benchStoreSizes returns the issue counts to benchmark: benchSizes, and benchLargeSize when
// benchLargeEnv is 1.
func benchStoreSizes() []int {
	if os.Getenv(benchLargeEnv) == "1" {
		return append(slices.Clone(benchSizes), benchLargeSize)
	}
	return benchSizes
}

// benchCommand benchmarks capcom args at every store size, as a sub-benchmark N=<size>, with
// nothing done between the timed runs; see benchCommandEach.
func benchCommand(b *testing.B, isPerIssue bool, args ...string) {
	benchCommandEach(b, isPerIssue, nil, args...)
}

// benchCommandEach benchmarks capcom args at every store size, as a sub-benchmark N=<size>. Each
// size's store is built once, in b's temp dir, which lasts until every round of the
// sub-benchmark has run. Each round runs args once untimed, which changes into the store, sets
// the hermetic git environment and warms the index, then times b.N runs. A non-nil beforeEach
// runs on the store's root before every timed run, with the timer stopped. With isPerIssue it
// also reports the time per run divided by the issue count, for a command whose cost scales with
// the store.
func benchCommandEach(
	b *testing.B,
	isPerIssue bool,
	beforeEach func(b *testing.B, root string),
	args ...string,
) {
	for _, size := range benchStoreSizes() {
		root := newBenchStore(b, defaultBenchShape(size))
		b.Run(fmt.Sprintf("N=%d", size), func(b *testing.B) {
			if _, stderr, exitCode := runCLI(b, root, args...); exitCode != ExitOK {
				b.Fatalf("%v exit = %d stderr = %s", args, exitCode, stderr)
			}
			var stdout, stderr bytes.Buffer
			b.ResetTimer()

			for range b.N {
				if beforeEach != nil {
					b.StopTimer()
					beforeEach(b, root)
					b.StartTimer()
				}
				stdout.Reset()
				stderr.Reset()
				if exitCode := Run(args, strings.NewReader(""), &stdout, &stderr); exitCode != ExitOK {
					b.Fatalf("%v exit = %d stderr = %s", args, exitCode, stderr.String())
				}
			}
			if isPerIssue {
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(size), "ns/issue")
			}
		})
	}
}

// removeBenchIndex deletes the index of the store at root together with the WAL sidecars SQLite
// keeps beside it (sidecarSuffixes in internal/index), so the next command builds it from nothing.
func removeBenchIndex(b *testing.B, root string) {
	path := filepath.Join(root, ".git", "capcom", "index.db")
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			b.Fatalf("Remove: %v", err)
		}
	}
}

// BenchmarkPrime measures prime with a warm index, including its scan of the other branches.
func BenchmarkPrime(b *testing.B) {
	benchCommand(b, true, "prime")
}

// BenchmarkReady measures ready, the list of issues ready to work on.
func BenchmarkReady(b *testing.B) {
	benchCommand(b, true, "ready")
}

// BenchmarkNext measures next, the most urgent ready issue in full.
func BenchmarkNext(b *testing.B) {
	benchCommand(b, true, "next")
}

// BenchmarkListAllJSON measures list --all --json, every issue of the store.
func BenchmarkListAllJSON(b *testing.B) {
	benchCommand(b, true, "list", "--all", "--json")
}

// BenchmarkSearch measures a search for a word a few issue bodies hold.
func BenchmarkSearch(b *testing.B) {
	benchCommand(b, true, "search", benchSearchWord)
}

// BenchmarkCheck measures check, which parses and validates every file of the store.
func BenchmarkCheck(b *testing.B) {
	benchCommand(b, true, "check")
}

// BenchmarkRecap measures recap over the last benchRecapDays days of history. Its cost follows
// the commits in that window rather than the issue count, so it reports no per-issue time.
func BenchmarkRecap(b *testing.B) {
	since := time.Now().AddDate(0, 0, -benchRecapDays).Format(time.DateOnly)
	benchCommand(b, false, "recap", "--since", since)
}

// BenchmarkReindex measures reindex, which rebuilds the index from every file of the store.
func BenchmarkReindex(b *testing.B) {
	benchCommand(b, true, "reindex")
}

// BenchmarkColdFirstCommand measures the first command after the index is deleted: list, which
// has to build the index from every file of the store before it can answer.
func BenchmarkColdFirstCommand(b *testing.B) {
	benchCommandEach(b, true, removeBenchIndex, "list")
}

// BenchmarkLog measures one log write and the index update that follows it. Every run appends
// one more entry to benchLogIssue, in a store this benchmark builds for itself. The update walks
// the whole store, so it reports a per-issue time.
func BenchmarkLog(b *testing.B) {
	benchCommand(b, true, "log", benchLogIssue, "a benchmark Log entry")
}

func TestBenchStore_Shape(t *testing.T) {
	const historyCommits = 20
	shape := benchShape{issues: 200, notes: benchNoteCount, commits: historyCommits, branches: benchBranchCount}

	root := newBenchStore(t, shape)

	depsByID := map[string][]string{}
	maxDepth := 0
	issuesDir := filepath.Join(root, ".capcom", "issues")
	err := filepath.WalkDir(issuesDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(issuesDir, path)
		if err != nil {
			return err
		}
		maxDepth = max(maxDepth, len(strings.Split(relative, string(filepath.Separator))))
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		depsByID[strings.TrimSuffix(entry.Name(), ".md")] = benchFileDeps(string(data))
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}

	if len(depsByID) != shape.issues {
		t.Errorf("store holds %d issues, want %d", len(depsByID), shape.issues)
	}
	if maxDepth != benchMaxDepth {
		t.Errorf("deepest issue is at depth %d, want %d", maxDepth, benchMaxDepth)
	}
	if cycle := benchDepCycle(depsByID); cycle != "" {
		t.Errorf("deps hold a cycle through %s, want none", cycle)
	}
	branches := strings.Fields(gitOutput(t, root, "for-each-ref", "--format=%(refname:short)", "refs/heads"))
	if len(branches) != shape.branches+1 {
		t.Errorf("repository has %d branches %v, want %d besides the checked-out one", len(branches), branches, shape.branches)
	}
	if count := gitOutput(t, root, "rev-list", "--count", "HEAD"); count != fmt.Sprint(historyCommits) {
		t.Errorf("HEAD has %s commits, want %d", count, historyCommits)
	}
	if stdout, stderr, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check exit = %d\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
	}
}

// benchFileDeps returns the deps listed in the frontmatter of the issue file data.
func benchFileDeps(data string) []string {
	for line := range strings.Lines(data) {
		if list, isDeps := strings.CutPrefix(strings.TrimSpace(line), "deps: ["); isDeps {
			return strings.Split(strings.TrimSuffix(list, "]"), ", ")
		}
	}
	return nil
}

// benchDepCycle returns the id of an issue on a dep cycle of depsByID, or "" when there is none.
func benchDepCycle(depsByID map[string][]string) string {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := map[string]int{}
	var visit func(id string) string
	visit = func(id string) string {
		switch state[id] {
		case visiting:
			return id
		case visited:
			return ""
		}
		state[id] = visiting
		for _, dep := range depsByID[id] {
			if cycle := visit(dep); cycle != "" {
				return cycle
			}
		}
		state[id] = visited
		return ""
	}
	for id := range depsByID {
		if cycle := visit(id); cycle != "" {
			return cycle
		}
	}
	return ""
}
