package briefing

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/elsewhere"
	"github.com/airiclenz/capcom/internal/index"
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

	if len(got.Failures) != 0 || got.Work == nil || got.Locks == nil || got.Health == nil {
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
	if len(got.Locks) != 0 || got.Elsewhere != nil {
		t.Errorf("Locks = %v, Elsewhere = %v; want no lock and nothing elsewhere", got.Locks, got.Elsewhere)
	}
	wantRef := check.Problem{Path: ".capcom/knowledge/build-notes.md", Line: 3, Code: check.CodeBrokenRef}
	if got.Health.ProblemCount != 1 || len(got.Health.BrokenRefs) != 1 ||
		got.Health.BrokenRefs[0].Path != wantRef.Path || got.Health.BrokenRefs[0].Line != wantRef.Line ||
		got.Health.BrokenRefs[0].Code != wantRef.Code {
		t.Errorf("Health = %+v; want one problem, the broken ref at %s:%d", got.Health, wantRef.Path, wantRef.Line)
	}
}

func TestCompose_WithoutStore_FailsTheStoreAndLeavesEverySectionOut(t *testing.T) {
	root := newBriefingRepo(t, nil)

	got := Compose(root, time.Now())

	if sections := failedSections(got.Failures); !slices.Equal(sections, []Section{SectionStore}) {
		t.Errorf("failed sections = %v, want [store]", sections)
	}
	if got.Work != nil || got.Locks != nil || got.Health != nil || got.Elsewhere != nil {
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
	if got.Work != nil || got.Locks != nil || got.Health == nil {
		t.Errorf("Compose = %+v; want Work and Locks left out and Health kept", got)
	}
	if len(got.Failures) > 0 && !strings.HasSuffix(got.Failures[0].Err.Error(), "; run capcom reindex") {
		t.Errorf("index failure = %v; want it to end with the reindex hint", got.Failures[0].Err)
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
