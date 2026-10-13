package elsewhere

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	issuesPathspec = ".capcom/issues"
	scanDays       = 30
)

// issueFile is the content of an issue file with the given status.
func issueFile(status string) string {
	return "---\ntitle: Some work\nstatus: " + status + "\ntype: task\ncreated: 2026-10-11T09:00:00Z\n---\n"
}

// git runs a (possibly writing) git command for test setup.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

// newRepo creates a hermetic git repository (ADR 0007) and returns its symlink-free root. Tests
// using it set environment variables and so do not call t.Parallel.
func newRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	environment := map[string]string{
		"GIT_CONFIG_GLOBAL":       os.DevNull,
		"GIT_CONFIG_NOSYSTEM":     "1",
		"GIT_CEILING_DIRECTORIES": filepath.Dir(root),
		"GIT_AUTHOR_NAME":         "Test Author",
		"GIT_AUTHOR_EMAIL":        "author@example.com",
		"GIT_COMMITTER_NAME":      "Test Committer",
		"GIT_COMMITTER_EMAIL":     "committer@example.com",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	git(t, root, "init", "-q")
	return root
}

// commitIssues writes the issue files, named by slash paths below .capcom/issues/ (an empty
// content deletes the file), and commits them.
func commitIssues(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, ".capcom", "issues", filepath.FromSlash(name))
		if content == "" {
			if err := os.Remove(path); err != nil {
				t.Fatalf("Remove: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "issues")
}

// newIssueRepo creates a repository whose first commit holds two open issues.
func newIssueRepo(t *testing.T) string {
	t.Helper()
	root := newRepo(t)
	commitIssues(t, root, map[string]string{
		"login-flow.md":                 issueFile("open"),
		"login-flow/session-refresh.md": issueFile("open"),
	})
	return root
}

// onBranch creates branch from HEAD, commits files on it and checks the previous branch out again.
func onBranch(t *testing.T, root, branch string, files map[string]string) {
	t.Helper()
	git(t, root, "checkout", "-q", "-b", branch)
	commitIssues(t, root, files)
	git(t, root, "checkout", "-q", "-")
}

// scan runs Scan over the issue tree of root at the current time.
func scan(root string) Report {
	return Scan(Options{
		WorktreeRoot: root,
		Pathspecs:    []string{issuesPathspec},
		Days:         scanDays,
		Now:          time.Now(),
	})
}

func TestScan_BranchClosesAnIssue_ReportsItsStatus(t *testing.T) {
	root := newIssueRepo(t)
	onBranch(t, root, "fix-login", map[string]string{"login-flow/session-refresh.md": issueFile("done")})

	report := scan(root)

	want := []Entry{{
		ID:     "session-refresh",
		Branch: "fix-login",
		Change: Modified,
		Status: "done",
		Path:   ".capcom/issues/login-flow/session-refresh.md",
	}}
	if !reflect.DeepEqual(report.Entries, want) || len(report.Warnings) != 0 {
		t.Errorf("report = %+v\nwant entries %+v and no warnings", report, want)
	}
	if got := report.Entries[0].String(); got != "session-refresh: done on fix-login (unmerged)" {
		t.Errorf("String = %q", got)
	}
}

func TestScan_BranchRenamesAnIssue_ReportsRemovedPlusAdded(t *testing.T) {
	root := newIssueRepo(t)
	git(t, root, "checkout", "-q", "-b", "rename-work")
	git(t, root, "mv", ".capcom/issues/login-flow.md", ".capcom/issues/sign-in-flow.md")
	git(t, root, "mv", ".capcom/issues/login-flow", ".capcom/issues/sign-in-flow")
	git(t, root, "commit", "-q", "-m", "rename")
	git(t, root, "checkout", "-q", "-")

	report := scan(root)

	var lines []string
	for _, entry := range report.Entries {
		lines = append(lines, entry.String())
	}
	want := []string{
		"login-flow: removed on rename-work (unmerged)",
		"session-refresh: removed on rename-work (unmerged)",
		"sign-in-flow: added on rename-work (unmerged)",
		"session-refresh: added on rename-work (unmerged)",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("entries = %q\nwant %q", lines, want)
	}
}

func TestScan_MergedOrCurrentOrOldBranch_ReportsNothing(t *testing.T) {
	root := newIssueRepo(t)
	onBranch(t, root, "merged", map[string]string{"login-flow.md": issueFile("done")})
	git(t, root, "merge", "-q", "--ff-only", "merged")
	commitIssues(t, root, map[string]string{"login-flow.md": issueFile("parked")})
	onBranch(t, root, "old-work", map[string]string{"login-flow.md": issueFile("dropped")})

	report := Scan(Options{
		WorktreeRoot: root,
		Pathspecs:    []string{issuesPathspec},
		Days:         scanDays,
		Now:          time.Now().Add((scanDays + 1) * 24 * time.Hour),
	})
	current := scan(root)

	if len(report.Entries) != 0 || len(report.Warnings) != 0 {
		t.Errorf("beyond the window: report = %+v, want nothing", report)
	}
	for _, entry := range current.Entries {
		if entry.Branch != "old-work" {
			t.Errorf("entry %v: only old-work holds unmerged changes", entry)
		}
	}
}

func TestScan_BranchInAnotherWorktree_IsReportedButNotAgainstItself(t *testing.T) {
	root := newIssueRepo(t)
	worktree := filepath.Join(t.TempDir(), "side")
	git(t, root, "worktree", "add", "-q", "-b", "side", worktree)
	commitIssues(t, worktree, map[string]string{"login-flow.md": issueFile("parked")})

	fromMain := scan(root)
	fromWorktree := scan(worktree)

	want := []Entry{{ID: "login-flow", Branch: "side", Change: Modified, Status: "parked", Path: ".capcom/issues/login-flow.md"}}
	if !reflect.DeepEqual(fromMain.Entries, want) {
		t.Errorf("from main: entries = %+v, want %+v", fromMain.Entries, want)
	}
	if len(fromWorktree.Entries) != 0 || len(fromWorktree.Warnings) != 0 {
		t.Errorf("from the worktree on side: report = %+v, want nothing", fromWorktree)
	}
}

func TestScan_UnparsableCopy_ReportsModifiedWithAWarning(t *testing.T) {
	root := newIssueRepo(t)
	onBranch(t, root, "broken", map[string]string{"login-flow.md": "no frontmatter\n"})

	report := scan(root)

	if len(report.Entries) != 1 || report.Entries[0].String() != "login-flow: modified on broken (unmerged)" {
		t.Errorf("entries = %+v, want login-flow modified", report.Entries)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "broken:.capcom/issues/login-flow.md") {
		t.Errorf("warnings = %q, want one naming the branch copy", report.Warnings)
	}
}

func TestScan_OversizedBranchCopy_WarnsOnceAndReportsNoEntry(t *testing.T) {
	root := newIssueRepo(t)
	front := issueFile("done")
	oversized := front + strings.Repeat("x", int(maxBranchCopySize)+1-len(front))
	onBranch(t, root, "big", map[string]string{
		"login-flow.md":                 oversized,
		"login-flow/session-refresh.md": issueFile("done"),
	})

	report := scan(root)

	wantEntries := []string{"session-refresh: done on big (unmerged)"}
	if lines := entryLines(report); !reflect.DeepEqual(lines, wantEntries) {
		t.Errorf("entries = %q, want %q", lines, wantEntries)
	}
	wantWarnings := []string{"elsewhere: branch big: .capcom/issues/login-flow.md: file is larger than 1 MiB"}
	if !reflect.DeepEqual(report.Warnings, wantWarnings) {
		t.Errorf("warnings = %q, want %q", report.Warnings, wantWarnings)
	}
}

func TestScan_SingleFilePathspec_LimitsTheReport(t *testing.T) {
	root := newIssueRepo(t)
	onBranch(t, root, "both", map[string]string{
		"login-flow.md":                 issueFile("done"),
		"login-flow/session-refresh.md": issueFile("done"),
	})

	report := Scan(Options{
		WorktreeRoot: root,
		Pathspecs:    []string{".capcom/issues/login-flow.md"},
		Days:         scanDays,
		Now:          time.Now(),
	})

	if len(report.Entries) != 1 || report.Entries[0].ID != "login-flow" {
		t.Errorf("entries = %+v, want login-flow only", report.Entries)
	}
}

func TestScan_UnbornHead_WarnsOnce(t *testing.T) {
	root := newIssueRepo(t)
	git(t, root, "checkout", "-q", "--orphan", "fresh")

	report := scan(root)

	if len(report.Entries) != 0 || len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "HEAD") {
		t.Errorf("report = %+v, want one warning about HEAD", report)
	}
}

func TestScan_FreshRepository_ReportsNothing(t *testing.T) {
	root := newRepo(t)

	report := scan(root)

	if len(report.Entries) != 0 || len(report.Warnings) != 0 {
		t.Errorf("report = %+v, want nothing in a repository without branches", report)
	}
}

// scanZeroDays runs Scan over the issue tree of root at the current time with a window of 0 days.
func scanZeroDays(root string) Report {
	return Scan(Options{
		WorktreeRoot: root,
		Pathspecs:    []string{issuesPathspec},
		Days:         0,
		Now:          time.Now(),
	})
}

func TestScan_ZeroDays_FutureDatedBranch_ReportsNothing(t *testing.T) {
	root := newIssueRepo(t)
	nextYear := time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", nextYear)
	t.Setenv("GIT_COMMITTER_DATE", nextYear)
	onBranch(t, root, "fix-login", map[string]string{"login-flow/session-refresh.md": issueFile("done")})

	report := scanZeroDays(root)

	if len(report.Entries) != 0 || len(report.Warnings) != 0 {
		t.Errorf("report = %+v, want nothing with a window of 0 days", report)
	}
}

func TestScan_ZeroDays_NotARepository_RunsNoGit(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))

	report := scanZeroDays(root)

	if len(report.Entries) != 0 || len(report.Warnings) != 0 {
		t.Errorf("report = %+v, want no entries and no warnings: a window of 0 days runs no git", report)
	}
}

// newClone creates an origin repository whose main branch holds two open issues, clones it and
// returns both roots; the clone is on main, tracking origin/main.
func newClone(t *testing.T) (origin, clone string) {
	t.Helper()
	origin = newIssueRepo(t)
	git(t, origin, "branch", "-M", "main")
	clone, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	git(t, origin, "clone", "-q", origin, clone)
	return origin, clone
}

// entryLines renders every entry of report with Entry.String.
func entryLines(report Report) []string {
	var lines []string
	for _, entry := range report.Entries {
		lines = append(lines, entry.String())
	}
	return lines
}

func TestScan_RemoteOnlyBranch_IsReportedByItsShortName(t *testing.T) {
	origin, clone := newClone(t)
	onBranch(t, origin, "fix-login", map[string]string{"login-flow.md": issueFile("done")})
	git(t, clone, "fetch", "-q")

	report := scan(clone)

	want := []string{"login-flow: done on origin/fix-login (unmerged)"}
	if lines := entryLines(report); !reflect.DeepEqual(lines, want) || len(report.Warnings) != 0 {
		t.Errorf("entries = %q, warnings = %q; want %q and no warnings", lines, report.Warnings, want)
	}
}

func TestScan_PushedLocalBranch_IsReportedOnce(t *testing.T) {
	_, clone := newClone(t)
	onBranch(t, clone, "fix-login", map[string]string{"login-flow.md": issueFile("done")})
	git(t, clone, "push", "-q", "origin", "fix-login")

	report := scan(clone)

	want := []string{"login-flow: done on fix-login (unmerged)"}
	if lines := entryLines(report); !reflect.DeepEqual(lines, want) || len(report.Warnings) != 0 {
		t.Errorf("entries = %q, warnings = %q; want %q and no warnings", lines, report.Warnings, want)
	}
}

func TestScan_LocalBranchBehindItsRemote_ReportsTheRemoteOnly(t *testing.T) {
	origin, clone := newClone(t)
	onBranch(t, clone, "fix-login", map[string]string{"login-flow.md": issueFile("done")})
	git(t, clone, "push", "-q", "origin", "fix-login")
	git(t, origin, "checkout", "-q", "fix-login")
	commitIssues(t, origin, map[string]string{"login-flow/session-refresh.md": issueFile("done")})
	git(t, origin, "checkout", "-q", "main")
	git(t, clone, "fetch", "-q")

	report := scan(clone)

	want := []string{
		"login-flow: done on origin/fix-login (unmerged)",
		"session-refresh: done on origin/fix-login (unmerged)",
	}
	if lines := entryLines(report); !reflect.DeepEqual(lines, want) || len(report.Warnings) != 0 {
		t.Errorf("entries = %q, warnings = %q; want %q and no warnings", lines, report.Warnings, want)
	}
}

func TestScan_DivergedLocalAndRemoteBranch_ReportsBoth(t *testing.T) {
	origin, clone := newClone(t)
	onBranch(t, clone, "fix-login", map[string]string{"login-flow.md": issueFile("done")})
	onBranch(t, origin, "fix-login", map[string]string{"login-flow/session-refresh.md": issueFile("done")})
	git(t, clone, "fetch", "-q")

	report := scan(clone)

	want := []string{
		"login-flow: done on fix-login (unmerged)",
		"session-refresh: done on origin/fix-login (unmerged)",
	}
	if lines := entryLines(report); !reflect.DeepEqual(lines, want) || len(report.Warnings) != 0 {
		t.Errorf("entries = %q, warnings = %q; want %q and no warnings", lines, report.Warnings, want)
	}
}

func TestScan_UpstreamOfTheCurrentBranch_IsNotReported(t *testing.T) {
	origin, clone := newClone(t)
	commitIssues(t, origin, map[string]string{"login-flow.md": issueFile("done")})
	git(t, clone, "fetch", "-q")

	report := scan(clone)

	if len(report.Entries) != 0 || len(report.Warnings) != 0 {
		t.Errorf("report = %+v, want nothing for origin/main, the upstream of main", report)
	}
}

func TestScan_LocalUpstream_IsStillReported(t *testing.T) {
	_, clone := newClone(t)
	git(t, clone, "switch", "-q", "-c", "feat", "--track", "main")
	git(t, clone, "switch", "-q", "main")
	commitIssues(t, clone, map[string]string{"login-flow.md": issueFile("done")})
	git(t, clone, "switch", "-q", "feat")

	report := scan(clone)

	want := []string{"login-flow: done on main (unmerged)"}
	if lines := entryLines(report); !reflect.DeepEqual(lines, want) || len(report.Warnings) != 0 {
		t.Errorf("entries = %q, warnings = %q; want %q and no warnings", lines, report.Warnings, want)
	}
}

func TestScan_OriginHead_IsIgnored(t *testing.T) {
	origin, clone := newClone(t)
	onBranch(t, origin, "fix-login", map[string]string{"login-flow.md": issueFile("done")})
	git(t, clone, "fetch", "-q")
	git(t, clone, "remote", "set-head", "origin", "fix-login")

	report := scan(clone)

	want := []string{"login-flow: done on origin/fix-login (unmerged)"}
	if lines := entryLines(report); !reflect.DeepEqual(lines, want) {
		t.Errorf("entries = %q, want %q and nothing on origin/HEAD", lines, want)
	}
}

// gitOutput runs a read-only git command for test setup and returns its trimmed output.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output))
}

func TestScan_NotARepository_WarnsListBranchesOnce(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))

	report := scan(root)

	if len(report.Entries) != 0 || len(report.Warnings) != 1 ||
		!strings.HasPrefix(report.Warnings[0], "elsewhere: list branches: ") {
		t.Errorf("report = %+v, want no entries and one list-branches warning", report)
	}
}

func TestScan_BranchWithAMissingTree_WarnsForItAndReportsTheOthers(t *testing.T) {
	root := newIssueRepo(t)
	// a-broken sorts before b-healthy, so a scan that stopped at the first failing branch would
	// never reach the healthy one.
	onBranch(t, root, "a-broken", map[string]string{"login-flow.md": issueFile("done")})
	onBranch(t, root, "b-healthy", map[string]string{"login-flow/session-refresh.md": issueFile("done")})
	tree := gitOutput(t, root, "rev-parse", "a-broken^{tree}")
	object := filepath.Join(root, ".git", "objects", tree[:2], tree[2:])
	// git writes loose objects read-only, which Windows refuses to remove.
	if err := os.Chmod(object, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if err := os.Remove(object); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	report := scan(root)

	want := []string{"session-refresh: done on b-healthy (unmerged)"}
	if lines := entryLines(report); !reflect.DeepEqual(lines, want) {
		t.Errorf("entries = %q, want %q", lines, want)
	}
	if len(report.Warnings) != 1 || !strings.HasPrefix(report.Warnings[0], "elsewhere: branch a-broken: ") {
		t.Errorf("warnings = %q, want one for branch a-broken", report.Warnings)
	}
}

func TestScan_HiddenAndNonMarkdownFiles_AreNotReported(t *testing.T) {
	root := newIssueRepo(t)
	onBranch(t, root, "extras", map[string]string{
		".hidden.md":  issueFile("open"),
		"notes.txt":   "some notes\n",
		"new-work.md": issueFile("open"),
	})

	report := scan(root)

	want := []Entry{{ID: "new-work", Branch: "extras", Change: Added, Path: ".capcom/issues/new-work.md"}}
	if !reflect.DeepEqual(report.Entries, want) || len(report.Warnings) != 0 {
		t.Errorf("report = %+v\nwant entries %+v and no warnings", report, want)
	}
}
