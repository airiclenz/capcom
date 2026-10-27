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

// alphaPlan is the plan indexIssueFiles writes a file for, so an issue naming it is Planned.
const alphaPlan = "2026-10-12 - 00 - alpha-plan"

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
func readyFixture(t *testing.T) (*index.Index, config.Config, string) {
	t.Helper()
	return indexIssueFiles(t, map[string]string{
		"alpha-task.md":  readyIssue("open", 2, "labels: [auth]\ndeps: [schema-task]\nplan: "+alphaPlan+"\n"),
		"beta-task.md":   readyIssue("open", 1, "labels: [ui]\n"),
		"delta-task.md":  readyIssue("open", 1, "deps: [beta-task]\n"),
		"gamma-task.md":  readyIssue("open", 2, "labels: [auth, ui]\n"),
		"later-task.md":  readyIssue("open", 0, "labels: ["+hiddenLabel+"]\n"),
		"schema-task.md": readyIssue("done", 1, ""),
	})
}

// unblockFixture indexes a tree in a temp dir whose config hides hiddenLabel. In tree order, each
// id ending in -task: alpha (Ready, P2; its one dependent, kilo, is hidden), bravo (Ready, P2,
// unblocks echo), charlie (Ready, P1), delta (Ready, P3, unblocks foxtrot and golf), echo, foxtrot
// and golf (todo, P2, each blocked by bravo or delta alone), hotel (todo, P2, blocked by bravo and
// delta), india (todo, P2) with its open child juliet (Ready, P2, unblocks india), kilo (todo, P2,
// hidden, blocked by alpha), lima (todo, P2, blocked by mike through mike's alias) and mike (Ready,
// P2, unblocks lima).
func unblockFixture(t *testing.T) (*index.Index, config.Config, string) {
	t.Helper()
	return indexIssueFiles(t, map[string]string{
		"alpha-task.md":             readyIssue("open", 2, ""),
		"bravo-task.md":             readyIssue("open", 2, ""),
		"charlie-task.md":           readyIssue("open", 1, ""),
		"delta-task.md":             readyIssue("open", 3, ""),
		"echo-task.md":              readyIssue("open", 2, "deps: [bravo-task]\n"),
		"foxtrot-task.md":           readyIssue("open", 2, "deps: [delta-task]\n"),
		"golf-task.md":              readyIssue("open", 2, "deps: [delta-task]\n"),
		"hotel-task.md":             readyIssue("open", 2, "deps: [bravo-task, delta-task]\n"),
		"india-task.md":             readyIssue("open", 2, ""),
		"india-task/juliet-task.md": readyIssue("open", 2, ""),
		"kilo-task.md":              readyIssue("open", 2, "labels: ["+hiddenLabel+"]\ndeps: [alpha-task]\n"),
		"lima-task.md":              readyIssue("open", 2, "deps: [mike-old]\n"),
		"mike-task.md":              readyIssue("open", 2, "aliases: [mike-old]\n"),
	})
}

// indexIssueFiles writes files, issue file paths relative to the issues directory, into a temp dir
// whose config hides hiddenLabel, writes the alphaPlan file into its first plans directory, and
// indexes the issues. It returns the index, the config and the temp dir, the worktree root.
func indexIssueFiles(t *testing.T, files map[string]string) (*index.Index, config.Config, string) {
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
	for name, content := range files {
		path := filepath.Join(s.IssuesDir(), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	planDir := filepath.Join(root, filepath.FromSlash(cfg.Plans[0]))
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, alphaPlan+".md"), []byte("# Plan\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
	return opened, cfg, root
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
	x, cfg, root := readyFixture(t)
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
		options := current.options
		options.Root = root

		got, err := ReadyList(x, cfg, options)

		if err != nil || !slices.Equal(itemIDs(got), current.want) {
			t.Errorf("%s: ReadyList = %v, %v; want %v", current.name, itemIDs(got), err, current.want)
		}
	}
}

func TestReadyList_PlanThatResolvesToNoFile_IsUnplanned(t *testing.T) {
	t.Parallel()
	x, cfg, root := indexIssueFiles(t, map[string]string{
		"alpha-task.md": readyIssue("open", 1, "plan: "+alphaPlan+"\n"),
		"lost-task.md":  readyIssue("open", 2, "plan: no-such-plan\n"),
		"plain-task.md": readyIssue("open", 3, ""),
	})
	cases := []struct {
		name string
		plan index.PlanState
		want []string
	}{
		{"planned keeps only the plan that resolves", index.PlanSet, []string{"alpha-task"}},
		{"unplanned keeps the plan that resolves to no file", index.PlanUnset, []string{"lost-task", "plain-task"}},
		{"any keeps every plan", index.PlanAny, []string{"alpha-task", "lost-task", "plain-task"}},
	}

	for _, current := range cases {
		got, err := ReadyList(x, cfg, ReadyOptions{Plan: current.plan, Root: root})

		if err != nil || !slices.Equal(itemIDs(got), current.want) {
			t.Errorf("%s: ReadyList = %v, %v; want %v", current.name, itemIDs(got), err, current.want)
		}
	}
}

func TestReadyList_NotReadyIncluded_CarriesTheVerdictBesideTheEntry(t *testing.T) {
	t.Parallel()
	x, cfg, _ := readyFixture(t)

	got, err := ReadyList(x, cfg, ReadyOptions{IsNotReadyIncluded: true, Limit: 2})

	want := []issue.Blocker{{Dep: "beta-task", Reason: string(config.CategoryTodo)}}
	if err != nil || len(got) != 2 || got[1].Entry.ID != "delta-task" || got[1].Readiness.IsReady ||
		!slices.Equal(got[1].Readiness.Blocking, want) || !got[0].Readiness.IsReady {
		t.Errorf("ReadyList = %+v, %v; want beta Ready, then delta blocked by %v", got, err, want)
	}
}

func TestReadyList_IndexClosed_Fails(t *testing.T) {
	t.Parallel()
	x, cfg, _ := readyFixture(t)
	if err := x.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := ReadyList(x, cfg, ReadyOptions{})

	if err == nil || got != nil {
		t.Errorf("ReadyList = %v, %v; want an error", itemIDs(got), err)
	}
}

func TestReadyList_PriorityTie_UnblockerFirst(t *testing.T) {
	t.Parallel()
	x, cfg, _ := unblockFixture(t)
	cases := []struct {
		name    string
		options ReadyOptions
		want    []string
	}{
		{"default: priority, then unblocks, then tree order", ReadyOptions{},
			[]string{"charlie-task", "bravo-task", "juliet-task", "mike-task", "alpha-task", "delta-task"}},
		{"not ready included: each counts 0", ReadyOptions{IsNotReadyIncluded: true}, []string{
			"charlie-task", "bravo-task", "juliet-task", "mike-task", "alpha-task", "echo-task",
			"foxtrot-task", "golf-task", "hotel-task", "india-task", "lima-task", "delta-task",
		}},
		{"limit after the sort", ReadyOptions{Limit: 2}, []string{"charlie-task", "bravo-task"}},
	}

	for _, current := range cases {
		got, err := ReadyList(x, cfg, current.options)

		if err != nil || !slices.Equal(itemIDs(got), current.want) {
			t.Errorf("%s: ReadyList = %v, %v; want %v", current.name, itemIDs(got), err, current.want)
		}
	}
}

func TestReadyList_ReadyItem_CarriesWhatItUnblocks(t *testing.T) {
	t.Parallel()
	x, cfg, _ := unblockFixture(t)

	got, err := ReadyList(x, cfg, ReadyOptions{IsNotReadyIncluded: true, IsHiddenIncluded: true})

	if err != nil {
		t.Fatalf("ReadyList: %v", err)
	}
	want := map[string][]string{
		"bravo-task":  {"echo-task"},
		"delta-task":  {"foxtrot-task", "golf-task"},
		"juliet-task": {"india-task"},
		"mike-task":   {"lima-task"},
	}
	for _, item := range got {
		if !slices.Equal(item.Unblocks, want[item.Entry.ID]) {
			t.Errorf("%s: Unblocks = %v, want %v", item.Entry.ID, item.Unblocks, want[item.Entry.ID])
		}
	}
}

func TestReadyList_Unblocks_MatchNewlyReady(t *testing.T) {
	t.Parallel()
	for name, fixture := range map[string]func(*testing.T) (*index.Index, config.Config, string){
		"ready fixture":   readyFixture,
		"unblock fixture": unblockFixture,
	} {
		x, cfg, _ := fixture(t)
		entries, err := x.List(index.Filter{IsClosedIncluded: true})
		if err != nil {
			t.Fatalf("%s: List: %v", name, err)
		}
		nodes := make([]issue.Node, 0, len(entries))
		for _, entry := range entries {
			nodes = append(nodes, entry.Node())
		}

		got, err := ReadyList(x, cfg, ReadyOptions{IsHiddenIncluded: true})

		if err != nil {
			t.Fatalf("%s: ReadyList: %v", name, err)
		}
		for _, item := range got {
			closed := map[string]config.Category{item.Entry.ID: config.CategoryDone}
			if want := issue.NewlyReady(nodes, cfg, closed); !slices.Equal(item.Unblocks, want) {
				t.Errorf("%s: %s Unblocks = %v, want %v", name, item.Entry.ID, item.Unblocks, want)
			}
		}
	}
}

func TestReadyList_Standing_NeverReadyNorUnblocked(t *testing.T) {
	t.Parallel()
	x, cfg, _ := indexIssueFiles(t, map[string]string{
		"bare-task.md":            readyIssue("open", 2, "standing: true\n"),
		"epic-task.md":            readyIssue("open", 2, "standing: true\n"),
		"epic-task/child-task.md": readyIssue("open", 2, ""),
	})
	cases := []struct {
		name    string
		options ReadyOptions
		want    []string
	}{
		{name: "default", options: ReadyOptions{}, want: []string{"child-task"}},
		{
			name:    "not ready included",
			options: ReadyOptions{IsNotReadyIncluded: true},
			want:    []string{"bare-task", "epic-task", "child-task"},
		},
		{
			name:    "not ready and hidden included",
			options: ReadyOptions{IsNotReadyIncluded: true, IsHiddenIncluded: true},
			want:    []string{"bare-task", "epic-task", "child-task"},
		},
	}

	for _, current := range cases {
		got, err := ReadyList(x, cfg, current.options)

		if err != nil || !slices.Equal(itemIDs(got), current.want) {
			t.Errorf("%s: ReadyList = %v, %v; want %v", current.name, itemIDs(got), err, current.want)
		}
		for _, item := range got {
			isStanding := item.Entry.ID != "child-task"
			if item.Readiness.IsStanding != isStanding || item.Readiness.IsReady == isStanding {
				t.Errorf("%s: %s readiness = %+v, want standing %v", current.name, item.Entry.ID,
					item.Readiness, isStanding)
			}
			if len(item.Unblocks) != 0 {
				t.Errorf("%s: %s Unblocks = %v, want none", current.name, item.Entry.ID, item.Unblocks)
			}
		}
	}
}
