package stage_test

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/reflist"
	"github.com/airiclenz/capcom/internal/stage"
	"github.com/airiclenz/capcom/internal/store"
)

// The fixed times the batches create, log and close at, so a staged run and its twin write the
// same bytes.
var (
	createdAt = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	loggedAt  = time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	closedAt  = time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
)

// authorName is the author the Log entries of these tests record.
const authorName = "Stage Tester"

// issueText returns the content of a valid issue file with title and status.
func issueText(title string, status string) string {
	return fmt.Sprintf(
		"---\ntitle: %s\nstatus: %s\ntype: task\npriority: 2\ncreated: 2026-10-11T09:00:00Z\n---\nBody.\n",
		title, status,
	)
}

// newIssueStore writes files, issue file contents keyed by id, into the issue tree of a fresh
// store with the default config and returns the store with its index open. The index needs no
// git, so GitDir is a plain directory.
func newIssueStore(t *testing.T, files map[string]string) (*store.Store, *index.Index) {
	t.Helper()
	root := t.TempDir()
	s := &store.Store{
		Root:         filepath.Join(root, ".capcom"),
		WorktreeRoot: root,
		GitDir:       filepath.Join(root, ".git"),
		GitCommonDir: filepath.Join(root, ".git"),
		Config:       config.Default(),
	}
	if err := os.MkdirAll(s.IssuesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for id, content := range files {
		writeFile(t, filepath.Join(s.IssuesDir(), id+".md"), content)
	}
	x, err := index.Open(s, index.ModeRefresh)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := x.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s, x
}

// newStaged returns the writes on the issues of s staged in a fresh Overlay, resolving through the
// Overlay's Catalog over x, and the Overlay.
func newStaged(s *store.Store, x *index.Index) (*ops.Ops, *stage.Overlay) {
	overlay := stage.New(s)
	return ops.Staged(s, overlay, overlay.RaiseFormat, overlay.Catalog(x)), overlay
}

// readStore returns the content of every file below the store root of s, keyed by its path
// relative to that root with forward slashes.
func readStore(t *testing.T, s *store.Store) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(s.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(s.Root, path)
		files[filepath.ToSlash(relative)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// runBatch runs the batch the end-to-end test stages through o, calling afterWrite after each
// write, and returns the issues the close made Ready. It creates alpha, then beta below alpha
// depending on it, makes base depend on alpha, logs on beta and closes alpha.
func runBatch(t *testing.T, o *ops.Ops, afterWrite func()) []string {
	t.Helper()
	mustWrite := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		afterWrite()
	}
	_, err := o.CreateIssue(ops.NewIssue{ID: "alpha", Title: "Alpha", Created: createdAt})
	mustWrite("create alpha", err)
	_, err = o.CreateIssue(ops.NewIssue{
		ID: "beta", Parent: "alpha", Title: "Beta", Deps: []string{"alpha"}, Created: createdAt,
	})
	mustWrite("create beta", err)
	_, err = o.AddDep("base", "", "alp")
	mustWrite("dep add", err)
	_, err = o.AppendLog("bet", "", "started", loggedAt)
	mustWrite("log", err)
	closer, err := o.NewCloser()
	if err != nil {
		t.Fatalf("NewCloser: %v", err)
	}
	result, err := closer.Close("alpha", "", "done", "shipped", closedAt)
	mustWrite("close", err)
	return result.NewlyReady
}

func TestStaged_Batch_CommitsTheFilesTheDiskWritesWrite(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", authorName)
	disk := map[string]string{"base": issueText("Base", "open")}
	stagedStore, stagedIndex := newIssueStore(t, disk)
	twinStore, twinIndex := newIssueStore(t, disk)
	staged, overlay := newStaged(stagedStore, stagedIndex)
	twin := ops.New(twinStore, twinIndex)

	stagedReady := runBatch(t, staged, func() {})
	twinReady := runBatch(t, twin, func() {
		if _, err := twinIndex.Refresh(); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
	})
	if _, err := overlay.Plan().Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got, want := readStore(t, stagedStore), readStore(t, twinStore); !maps.Equal(got, want) {
		t.Errorf("staged store =\n%v\nwant the twin's\n%v", got, want)
	}
	if !slices.Equal(stagedReady, twinReady) || !slices.Contains(stagedReady, "base") {
		t.Errorf("NewlyReady = %v, want the twin's %v, naming base", stagedReady, twinReady)
	}
}

func TestStaged_CycleAcrossTwoStagedDepAdds_IsRefused(t *testing.T) {
	s, x := newIssueStore(t, map[string]string{
		"alpha": issueText("Alpha", "open"),
		"base":  issueText("Base", "open"),
	})
	staged, _ := newStaged(s, x)
	if _, err := staged.AddDep("alpha", "", "base"); err != nil {
		t.Fatalf("AddDep alpha base: %v", err)
	}

	_, err := staged.AddDep("base", "", "alpha")

	var refusal *reflist.Refusal
	if !errors.As(err, &refusal) || refusal.Reason != reflist.ReasonCycle {
		t.Errorf("err = %v, want a *reflist.Refusal of ReasonCycle", err)
	}
}

func TestStaged_StageDroppedUncommitted_LeavesTheStoreUntouched(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", authorName)
	s, x := newIssueStore(t, map[string]string{"base": issueText("Base", "open")})
	before := readStore(t, s)
	staged, _ := newStaged(s, x)

	runBatch(t, staged, func() {})

	if after := readStore(t, s); !maps.Equal(after, before) {
		t.Errorf("store =\n%v\nwant it as before\n%v", after, before)
	}
}
