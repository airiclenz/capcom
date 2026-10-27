package briefing

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/elsewhere"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/store"
)

// briefingConfig declares a doing status, so the Briefing has issues in progress to list.
const briefingConfig = `statuses:
  open: todo
  active: doing
  parked: waiting
  done: done
  dropped: dropped
`

// newBriefingRepo creates a git repository in a temp dir, writes files into it (named by slash
// paths relative to its root) and returns the root. It isolates git from user and system
// configuration (ADR 0007), so its tests do not call t.Parallel.
func newBriefingRepo(t *testing.T, files map[string]string) string {
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
	runGit(t, root, "init", "-q", "-b", "main")
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return root
}

// runGit runs git with args in root and fails the test when it fails.
func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

// briefingStore is a store with planned (its plan file present), unplanned, in-progress and closed
// issues and a Note with one broken Ref, keyed by slash path relative to the worktree root.
func briefingStore() map[string]string {
	return map[string]string{
		".capcom/config.yaml":                       briefingConfig,
		".capcom/issues/plan-work.md":               readyIssue("open", 1, "plan: 2026-10-12 - 00 - work-plan\n"),
		".capcom/issues/quick-fix.md":               readyIssue("open", 0, ""),
		".capcom/issues/side-quest.md":              readyIssue("open", 3, ""),
		".capcom/issues/busy-work.md":               readyIssue("active", 2, ""),
		".capcom/issues/shipped-it.md":              readyIssue("done", 2, ""),
		".capcom/knowledge/build-notes.md":          "---\ntitle: Build notes\nrefs: [go.mod, gone.go]\n---\n",
		"docs/plans/2026-10-12 - 00 - work-plan.md": "# Plan\n",
		"go.mod": "module example\n",
	}
}

// entryIDs returns the id of every entry, in order.
func entryIDs(entries []index.Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

// failedSections returns the section of every failure, in order.
func failedSections(failures []Failure) []Section {
	sections := make([]Section, 0, len(failures))
	for _, failure := range failures {
		sections = append(sections, failure.Section)
	}
	return sections
}

func TestCompose_FixtureStore_ReturnsEverySection(t *testing.T) {
	root := newBriefingRepo(t, briefingStore())

	got := Compose(root, time.Now())

	if len(got.Failures) != 0 || got.Work == nil || got.LockOverview == nil || got.Health == nil {
		t.Fatalf("Compose = %+v; want every section and no failure", got)
	}
	if planned := entryIDs(got.Work.ReadyPlanned); !slices.Equal(planned, []string{"plan-work"}) {
		t.Errorf("ReadyPlanned = %v, want [plan-work]", planned)
	}
	if unplanned := entryIDs(got.Work.ReadyUnplanned); !slices.Equal(unplanned, []string{"quick-fix", "side-quest"}) {
		t.Errorf("ReadyUnplanned = %v, want [quick-fix side-quest]", unplanned)
	}
	if doing := entryIDs(got.Work.Doing); !slices.Equal(doing, []string{"busy-work"}) {
		t.Errorf("Doing = %v, want [busy-work]", doing)
	}
	if overview := got.LockOverview; len(overview.Locks) != 0 || got.Elsewhere != nil {
		t.Errorf("Locks = %v, Elsewhere = %v; want no lock and nothing elsewhere", overview.Locks, got.Elsewhere)
	}
	if overview := got.LockOverview; overview.Key != lock.MainWorktreeKey || overview.State != lock.StateFree {
		t.Errorf("lock %q is %q, want %q free", overview.Key, overview.State, lock.MainWorktreeKey)
	}
	wantRef := check.Problem{Path: ".capcom/knowledge/build-notes.md", Line: 3, Code: diagnose.BrokenRef}
	if got.Health.ProblemCount != 1 || len(got.Health.BrokenRefs) != 1 ||
		got.Health.BrokenRefs[0].Path != wantRef.Path || got.Health.BrokenRefs[0].Line != wantRef.Line ||
		got.Health.BrokenRefs[0].Code != wantRef.Code {
		t.Errorf("Health = %+v; want one problem, the broken ref at %s:%d", got.Health, wantRef.Path, wantRef.Line)
	}
	if got.Health.UnsharedRefs == nil || len(got.Health.UnsharedRefs) != 0 {
		t.Errorf("UnsharedRefs = %#v, want empty and not nil", got.Health.UnsharedRefs)
	}
}

func TestCompose_PlanThatResolvesToNoFile_IsReadyUnplanned(t *testing.T) {
	files := briefingStore()
	files[".capcom/issues/lost-plan.md"] = readyIssue("open", 0, "plan: no-such-plan\n")
	root := newBriefingRepo(t, files)

	got := Compose(root, time.Now())

	if got.Work == nil {
		t.Fatalf("Compose = %+v; want the work section", got)
	}
	if planned := entryIDs(got.Work.ReadyPlanned); !slices.Equal(planned, []string{"plan-work"}) {
		t.Errorf("ReadyPlanned = %v, want [plan-work]", planned)
	}
	if unplanned := entryIDs(got.Work.ReadyUnplanned); !slices.Contains(unplanned, "lost-plan") {
		t.Errorf("ReadyUnplanned = %v, want it to hold lost-plan", unplanned)
	}
}

func TestCompose_RefToIgnoredFile_IsAnUnsharedRefNotABrokenOne(t *testing.T) {
	files := briefingStore()
	files["logs/.gitignore"] = "*.log\n"
	files["logs/build.log"] = "ignored\n"
	files[".capcom/knowledge/log-notes.md"] = "---\ntitle: Log notes\nrefs: [logs/build.log]\n---\n"
	root := newBriefingRepo(t, files)

	got := Compose(root, time.Now())

	if got.Health == nil {
		t.Fatalf("Compose = %+v; want the health section", got)
	}
	wantRef := check.Problem{Path: ".capcom/knowledge/log-notes.md", Line: 3, Code: diagnose.UnsharedRef}
	if got.Health.ProblemCount != 2 || len(got.Health.BrokenRefs) != 1 || len(got.Health.UnsharedRefs) != 1 ||
		got.Health.UnsharedRefs[0].Path != wantRef.Path || got.Health.UnsharedRefs[0].Line != wantRef.Line ||
		got.Health.UnsharedRefs[0].Code != wantRef.Code {
		t.Errorf("Health = %+v; want two problems, one broken ref and the unshared ref at %s:%d",
			got.Health, wantRef.Path, wantRef.Line)
	}
}

func TestCompose_BrokenRefsInRetiredSubtree_AreNotCounted(t *testing.T) {
	files := briefingStore()
	files[".capcom/config.yaml"] = briefingConfig + "format: 3\n"
	files[".capcom/knowledge/old-notes.md"] = "---\ntitle: Old notes\nretired: true\nrefs: [gone.go]\n---\n"
	files[".capcom/knowledge/old-notes/old-step.md"] = "---\ntitle: Old step\nrefs: [gone.go]\n---\n"
	root := newBriefingRepo(t, files)

	got := Compose(root, time.Now())

	if got.Health == nil {
		t.Fatalf("Compose = %+v; want the health section", got)
	}
	if got.Health.ProblemCount != 1 || len(got.Health.BrokenRefs) != 1 ||
		got.Health.BrokenRefs[0].Path != ".capcom/knowledge/build-notes.md" {
		t.Errorf("Health = %+v; want only the broken ref of build-notes", got.Health)
	}
}

func TestCompose_WithoutStore_FailsTheStoreAndLeavesEverySectionOut(t *testing.T) {
	root := newBriefingRepo(t, nil)

	got := Compose(root, time.Now())

	if sections := failedSections(got.Failures); !slices.Equal(sections, []Section{SectionStore}) {
		t.Errorf("failed sections = %v, want [store]", sections)
	}
	if got.Work != nil || got.LockOverview != nil || got.Health != nil || got.Elsewhere != nil {
		t.Errorf("Compose = %+v; want every section left out", got)
	}
}

func TestCompose_BrokenIndexAndLocks_FailInOrderAndKeepTheRest(t *testing.T) {
	files := briefingStore()
	files[".git/capcom/index.db"] = strings.Repeat("not a database ", 512)
	files[".git/capcom/locks"] = "a file where the locks directory belongs\n"
	root := newBriefingRepo(t, files)

	got := Compose(root, time.Now())

	if sections := failedSections(got.Failures); !slices.Equal(sections, []Section{SectionIndex, SectionLocks}) {
		t.Errorf("failed sections = %v, want [index locks]", sections)
	}
	if got.Work != nil || got.LockOverview != nil || got.Health == nil {
		t.Errorf("Compose = %+v; want Work and LockOverview left out and Health kept", got)
	}
	if len(got.Failures) > 0 && !strings.HasSuffix(got.Failures[0].Err.Error(), "; run capcom reindex") {
		t.Errorf("index failure = %v; want it to end with the reindex hint", got.Failures[0].Err)
	}
}

func TestCompose_InvalidOwnLockRecord_IsOneLocksFailureAndKeepsTheOverview(t *testing.T) {
	files := briefingStore()
	files[".git/capcom/locks/_main.json"] = "not json"
	root := newBriefingRepo(t, files)

	got := Compose(root, time.Now())

	if sections := failedSections(got.Failures); !slices.Equal(sections, []Section{SectionLocks}) {
		t.Fatalf("failed sections = %v (failures %v), want [locks]", sections, got.Failures)
	}
	recordPath := filepath.Join(root, ".git", "capcom", "locks", "_main.json")
	if warning := got.Failures[0].Err.Error(); !strings.HasPrefix(warning, recordPath+": invalid lock record: ") {
		t.Errorf("locks failure = %q, want it to start with %q", warning, recordPath+": invalid lock record: ")
	}
	if got.LockOverview == nil || got.LockOverview.State != lock.StateInvalid {
		t.Errorf("LockOverview = %+v; want it kept, its worktree invalid", got.LockOverview)
	}
}

func TestCompose_ReadyOrDoingFails_FailsThatSectionAndKeepsTheRest(t *testing.T) {
	cases := []struct {
		name string
		fail func(error)
		want Section
	}{
		{
			name: "ready",
			fail: func(err error) {
				readyList = func(*index.Index, config.Config, ReadyOptions) ([]ReadyItem, error) { return nil, err }
			},
			want: SectionReady,
		},
		{
			name: "doing",
			fail: func(err error) {
				listIssues = func(*index.Index, index.Filter) ([]index.Entry, error) { return nil, err }
			},
			want: SectionDoing,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			originalReady, originalList := readyList, listIssues
			t.Cleanup(func() { readyList, listIssues = originalReady, originalList })
			sourceErr := errors.New(tc.name + " exploded")
			tc.fail(sourceErr)
			root := newBriefingRepo(t, briefingStore())

			got := Compose(root, time.Now())

			if sections := failedSections(got.Failures); !slices.Equal(sections, []Section{tc.want}) {
				t.Errorf("failed sections = %v, want [%s]", sections, tc.want)
			}
			if len(got.Failures) > 0 && !errors.Is(got.Failures[0].Err, sourceErr) {
				t.Errorf("failure = %v, want the %s error", got.Failures[0].Err, tc.name)
			}
			if got.Work != nil || got.LockOverview == nil || got.Health == nil {
				t.Errorf("Compose = %+v; want Work left out and LockOverview and Health kept", got)
			}
		})
	}
}

func TestCompose_CheckFailsAndElsewhereWarns_FailInOrderAndKeepTheRest(t *testing.T) {
	original := runCheck
	t.Cleanup(func() { runCheck = original })
	checkErr := errors.New("check exploded")
	runCheck = func(*store.Store) ([]check.Problem, error) { return nil, checkErr }
	files := briefingStore()
	files[".git/refs/heads/ghost"] = strings.Repeat("0123456789", 4) + "\n"
	root := newBriefingRepo(t, files)

	got := Compose(root, time.Now())

	if sections := failedSections(got.Failures); !slices.Equal(sections, []Section{SectionCheck, SectionElsewhere}) {
		t.Fatalf("failed sections = %v (failures %v), want [check elsewhere]", sections, got.Failures)
	}
	if !errors.Is(got.Failures[0].Err, checkErr) {
		t.Errorf("check failure = %v, want the check error", got.Failures[0].Err)
	}
	if warning := got.Failures[1].Err.Error(); !strings.HasPrefix(warning, "elsewhere: list branches: ") {
		t.Errorf("Elsewhere warning = %q, want it to start with \"elsewhere: list branches: \"", warning)
	}
	if got.Health != nil || got.Work == nil || got.LockOverview == nil {
		t.Errorf("Compose = %+v; want Health left out and Work and LockOverview kept", got)
	}
}

func TestCompose_PanickingSource_IsOneInternalFailureAndNoSection(t *testing.T) {
	original := runCheck
	t.Cleanup(func() { runCheck = original })
	runCheck = func(*store.Store) ([]check.Problem, error) { panic("check exploded") }
	root := newBriefingRepo(t, briefingStore())

	got := Compose(root, time.Now())

	if len(got.Failures) != 1 || got.Failures[0].Section != SectionInternal ||
		got.Failures[0].Err.Error() != "internal error: check exploded" {
		t.Errorf("Failures = %+v; want one internal failure reading \"internal error: check exploded\"", got.Failures)
	}
	if got.Work != nil || got.LockOverview != nil || got.Health != nil || got.Elsewhere != nil {
		t.Errorf("Compose = %+v; want every section left out", got)
	}
}

func TestCompose_IssueChangedOnAnotherBranch_ReportsItElsewhere(t *testing.T) {
	root := newBriefingRepo(t, briefingStore())
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "store")
	runGit(t, root, "switch", "-q", "-c", "feature")
	newIssue := filepath.Join(root, ".capcom", "issues", "new-idea.md")
	if err := os.WriteFile(newIssue, []byte(readyIssue("open", 2, "")), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "new idea")
	runGit(t, root, "switch", "-q", "main")

	got := Compose(root, time.Now())

	want := []elsewhere.Entry{{
		ID:     "new-idea",
		Branch: "feature",
		Change: elsewhere.Added,
		Path:   ".capcom/issues/new-idea.md",
	}}
	if len(got.Failures) != 0 || !slices.Equal(got.Elsewhere, want) {
		t.Errorf("Elsewhere = %+v, failures %v; want %+v", got.Elsewhere, got.Failures, want)
	}
}
