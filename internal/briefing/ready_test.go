package briefing

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/store"
)

// hiddenLabel is the ready.hide label of the fixture's config.
const hiddenLabel = "someday"

// readyIssue is the content of a valid issue file with the given status, priority and extra
// frontmatter lines.
func readyIssue(status string, priority int, extra string) string {
	closing := ""
	if status == "done" {
		closing = "closed: 2026-10-12T09:00:00Z\nreason: finished\n"
	}
	return fmt.Sprintf(
		"---\ntitle: An issue\nstatus: %s\ntype: task\npriority: %d\n%screated: 2026-10-11T09:00:00Z\n%s---\n",
		status, priority, extra, closing,
	)
}

// readyFixture indexes a tree in a temp dir whose config hides hiddenLabel. In tree order, each id
// ending in -task: alpha (Ready, priority 2, auth, planned, its dep on schema done), beta (Ready,
// priority 1, ui), delta (todo, priority 1, blocked by beta), gamma (Ready, priority 2, auth and
// ui), later (Ready, priority 0, hidden) and schema (done, priority 1).
func readyFixture(t *testing.T) (*index.Index, config.Config) {
	t.Helper()
	root := t.TempDir()
	cfg := config.Default()
	cfg.Ready.Hide = []string{hiddenLabel}
	s := &store.Store{
		Root:         filepath.Join(root, ".capcom"),
		WorktreeRoot: root,
		GitDir:       filepath.Join(root, ".git"),
		GitCommonDir: filepath.Join(root, ".git"),
		Config:       cfg,
	}
	files := map[string]string{
		"alpha-task.md":  readyIssue("open", 2, "labels: [auth]\ndeps: [schema-task]\nplan: 2026-10-12 - 00 - alpha-plan\n"),
		"beta-task.md":   readyIssue("open", 1, "labels: [ui]\n"),
		"delta-task.md":  readyIssue("open", 1, "deps: [beta-task]\n"),
		"gamma-task.md":  readyIssue("open", 2, "labels: [auth, ui]\n"),
		"later-task.md":  readyIssue("open", 0, "labels: ["+hiddenLabel+"]\n"),
		"schema-task.md": readyIssue("done", 1, ""),
	}
	if err := os.MkdirAll(s.IssuesDir(), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(s.IssuesDir(), name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	opened, err := index.Open(s, index.ModeRefresh)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := opened.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return opened, cfg
}

// itemIDs returns the id of every item, in order.
func itemIDs(items []ReadyItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Entry.ID)
	}
	return ids
}

func TestReadyList_Options_SelectAndSortStablyByPriority(t *testing.T) {
	t.Parallel()
	x, cfg := readyFixture(t)
	cases := []struct {
		name    string
		options ReadyOptions
		want    []string
	}{
		{"default: Ready and not hidden, by priority then tree order", ReadyOptions{}, []string{"beta-task", "alpha-task", "gamma-task"}},
		{"label, its dep left out by the filter", ReadyOptions{Labels: []string{"auth"}}, []string{"alpha-task", "gamma-task"}},
		{"every label", ReadyOptions{Labels: []string{"auth", "ui"}}, []string{"gamma-task"}},
		{"no label", ReadyOptions{NoLabels: []string{"ui"}}, []string{"alpha-task"}},
		{"planned", ReadyOptions{Plan: index.PlanSet}, []string{"alpha-task"}},
		{"unplanned", ReadyOptions{Plan: index.PlanUnset}, []string{"beta-task", "gamma-task"}},
		{"not ready included", ReadyOptions{IsNotReadyIncluded: true}, []string{"beta-task", "delta-task", "alpha-task", "gamma-task"}},
		{"hidden included", ReadyOptions{IsHiddenIncluded: true}, []string{"later-task", "beta-task", "alpha-task", "gamma-task"}},
		{"limit after the sort", ReadyOptions{Limit: 2}, []string{"beta-task", "alpha-task"}},
	}

	for _, current := range cases {
		got, err := ReadyList(x, cfg, current.options)

		if err != nil || !slices.Equal(itemIDs(got), current.want) {
			t.Errorf("%s: ReadyList = %v, %v; want %v", current.name, itemIDs(got), err, current.want)
		}
	}
}

func TestReadyList_NotReadyIncluded_CarriesTheVerdictBesideTheEntry(t *testing.T) {
	t.Parallel()
	x, cfg := readyFixture(t)

	got, err := ReadyList(x, cfg, ReadyOptions{IsNotReadyIncluded: true, Limit: 2})

	want := []issue.Blocker{{Dep: "beta-task", Reason: string(config.CategoryTodo)}}
	if err != nil || len(got) != 2 || got[1].Entry.ID != "delta-task" || got[1].Readiness.IsReady ||
		!slices.Equal(got[1].Readiness.Blocking, want) || !got[0].Readiness.IsReady {
		t.Errorf("ReadyList = %+v, %v; want beta Ready, then delta blocked by %v", got, err, want)
	}
}

func TestReadyList_IndexClosed_Fails(t *testing.T) {
	t.Parallel()
	x, cfg := readyFixture(t)
	if err := x.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := ReadyList(x, cfg, ReadyOptions{})

	if err == nil || got != nil {
		t.Errorf("ReadyList = %v, %v; want an error", itemIDs(got), err)
	}
}
