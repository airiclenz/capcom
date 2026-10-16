package ui

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
)

// refreshWidth is wide enough for the status bar to hold every notice these tests look for.
const refreshWidth = 240

// reviewConfig is the config.yaml capcom init writes with the status review declared in the doing
// category.
var reviewConfig = strings.Replace(config.DefaultFile, "  active: doing\n", "  active: doing\n  review: doing\n", 1)

// tick polls the index once, as the next tickMsg asks, and drives the reload the poll starts.
func tick(t *testing.T, m model) model {
	t.Helper()
	return press(t, m, tickMsg{})
}

// pollOnce runs the poll a tickMsg starts and returns its message, unapplied.
func pollOnce(t *testing.T, m model) refreshedMsg {
	t.Helper()
	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("tickMsg returned no command, want the poll")
	}
	msg, isRefreshed := cmd().(refreshedMsg)
	if !isRefreshed {
		t.Fatalf("the poll returned %T, want refreshedMsg", msg)
	}
	return msg
}

// writeStoreFile writes content to the store file name, relative to .capcom with forward slashes.
func writeStoreFile(t *testing.T, ts *testStore, name, content string) {
	t.Helper()
	writeTestFile(t, filepath.Join(ts.store.Root, filepath.FromSlash(name)), content)
}

// removeStoreFile removes the store file name, relative to .capcom with forward slashes.
func removeStoreFile(t *testing.T, ts *testStore, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(ts.store.Root, filepath.FromSlash(name))); err != nil {
		t.Fatalf("Remove %s: %v", name, err)
	}
}

// closeSessionAtEnd closes, when the test ends, an index the model's session re-opened.
func closeSessionAtEnd(t *testing.T, m model) {
	t.Helper()
	t.Cleanup(func() {
		if err := m.session.close(); err != nil {
			t.Errorf("close the session: %v", err)
		}
	})
}

func TestRefresh_IssueEditedOnDisk_ShowsAfterOneTick(t *testing.T) {
	ts := newTestStore(t, map[string]string{
		"issues/alpha.md": issueFile("title: Alpha\nstatus: open\npriority: 2\n", "The first body.\n"),
	})
	m := newIssuesModel(t, ts, "alpha", 100, 20)
	writeStoreFile(t, ts, "issues/alpha.md", issueFile("title: Alpha renamed\nstatus: open\npriority: 2\n", "The second body.\n"))

	m = tick(t, m)

	if rows := strings.Join(sidebarText(m), "\n"); !strings.Contains(rows, "Alpha renamed") {
		t.Errorf("sidebar = %q, want the new title", rows)
	}
	if detail := detailText(m); !strings.Contains(detail, "The second body.") {
		t.Errorf("detail = %q, want the new body", detail)
	}
}

func TestRefresh_SelectedNodeRemoved_MovesTheSelection(t *testing.T) {
	cases := []struct {
		name     string
		files    []string
		selected string
		removed  string
		want     string
	}{
		{name: "to its parent", files: []string{"epic", "epic/child", "epic/other"}, selected: "child", removed: "epic/child", want: "epic"},
		{name: "else to the next sibling", files: []string{"alpha", "beta", "gamma"}, selected: "beta", removed: "beta", want: "gamma"},
		{name: "else to the previous one", files: []string{"alpha", "beta"}, selected: "beta", removed: "beta", want: "alpha"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			files := map[string]string{}
			for _, name := range testCase.files {
				files["issues/"+name+".md"] = issueFile("title: "+name+"\nstatus: open\npriority: 2\n", "")
			}
			ts := newTestStore(t, files)
			m := newIssuesModel(t, ts, testCase.selected, 100, 20)
			removeStoreFile(t, ts, "issues/"+testCase.removed+".md")

			m = tick(t, m)

			if got := selectedIssueID(m); got != testCase.want {
				t.Errorf("selected = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestRefresh_NoChangeTick_ReloadsNothing(t *testing.T) {
	ts := newTestStore(t, map[string]string{"issues/valid-issue.md": validIssue})
	m := newIssuesModel(t, ts, "", 100, 20)

	msg := pollOnce(t, m)
	_, cmd := m.Update(msg)

	if msg.isChanged || msg.reload != reloadNone || msg.err != nil {
		t.Errorf("poll = %+v, want no change, no reload and no error", msg)
	}
	if cmd != nil {
		t.Error("a no-change poll returned a command, want none: no tab reloads")
	}
}

func TestRefresh_LockAcquiredOnDisk_ShowsAfterOneTick(t *testing.T) {
	ts := newTestStore(t, nil)
	m := newTestModel(t, ts, 100, 20)
	acquireTestLock(t, ts)

	m = tick(t, m)

	want := regexp.MustCompile(`^ lock  held by bob: refactor the parser \(heartbeat 0m ago\) · `)
	if row := statusRow(m); !want.MatchString(row) {
		t.Errorf("status bar = %q, want it to match %s", row, want)
	}
}

func TestRefresh_ConfigDeclaresStatus_ShowsWithoutRebuildLoop(t *testing.T) {
	ts := newTestStore(t, map[string]string{
		"issues/plain.md":     validIssue,
		"issues/in-review.md": issueFile("title: In review\nstatus: review\npriority: 2\n", ""),
	})
	m := newIssuesModel(t, ts, "", 100, 20)
	closeSessionAtEnd(t, m)
	if slices.Contains(visibleIDs(m), "in-review") {
		t.Fatalf("visible = %v before the config change, want no in-review", visibleIDs(m))
	}
	writeStoreFile(t, ts, "config.yaml", reviewConfig)

	m = tick(t, m)
	next := pollOnce(t, m)

	if !slices.Contains(visibleIDs(m), "in-review") {
		t.Errorf("visible = %v, want in-review under the newly declared status", visibleIDs(m))
	}
	if next.isChanged || next.reload != reloadNone {
		t.Errorf("next poll = %+v, want no change and no reload", next)
	}
}

func TestRefresh_SchemaVersionMismatch_StopsPollingWithTheMessage(t *testing.T) {
	ts := newTestStore(t, nil)
	m := newTestModel(t, ts, refreshWidth, 20)
	db, err := sql.Open("sqlite", index.Path(ts.store))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close the database: %v", err)
		}
	})
	const foreignVersion = "9999"
	if _, err := db.Exec(`UPDATE meta SET value = ? WHERE key = 'schema_version'`, foreignVersion); err != nil {
		t.Fatalf("record another schema version: %v", err)
	}

	m = tick(t, m)
	_, cmd := m.Update(tickMsg{})

	if row := statusRow(m); !strings.Contains(row, foreignSchemaText) {
		t.Errorf("status bar = %q, want %q", row, foreignSchemaText)
	}
	if cmd != nil {
		t.Error("a tick after the stop returned a command, want polling stopped")
	}
	var recorded string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&recorded); err != nil {
		t.Fatalf("read the schema version: %v", err)
	}
	if recorded != foreignVersion {
		t.Errorf("schema version = %s, want %s: the UI must never rebuild the index", recorded, foreignVersion)
	}
}

func TestRefresh_FilterSet_IssueEditedOnDisk_RowsStillFiltered(t *testing.T) {
	ts := newTestStore(t, map[string]string{
		"issues/alpha.md": issueFile("title: Alpha\nstatus: open\npriority: 2\nlabels: [ui]\n", ""),
		"issues/beta.md":  issueFile("title: Beta\nstatus: open\npriority: 2\n", ""),
	})
	m := applyFilterText(t, newIssuesModel(t, ts, "", 100, 20), "label:ui")
	writeStoreFile(t, ts, "issues/beta.md", issueFile("title: Beta renamed\nstatus: open\npriority: 2\n", ""))
	writeStoreFile(t, ts, "issues/gamma.md", issueFile("title: Gamma\nstatus: open\npriority: 2\nlabels: [ui]\n", ""))

	m = tick(t, m)

	if got, want := visibleIDs(m), []string{"alpha", "gamma"}; !slices.Equal(got, want) {
		t.Errorf("visible = %v, want %v: the filter matched again over the reload", got, want)
	}
	if text := m.filters[TabIssues].text; text != "label:ui" {
		t.Errorf("filter = %q, want label:ui still in effect", text)
	}
}

func TestRefresh_ConfigYAMLError_KeepsTheSessionThenReopensOnTheFix(t *testing.T) {
	ts := newTestStore(t, map[string]string{"issues/valid-issue.md": validIssue})
	m := newIssuesModel(t, ts, "", refreshWidth, 20)
	closeSessionAtEnd(t, m)
	writeStoreFile(t, ts, "config.yaml", "statuses: [\n")

	m = tick(t, m)
	retry := pollOnce(t, m)

	if row := statusRow(m); !strings.Contains(row, "reload config: ") {
		t.Errorf("status bar = %q, want the config error", row)
	}
	if m.session.store != ts.store || m.session.index != ts.index {
		t.Error("the session swapped its store or index, want the old ones kept")
	}
	if retry.reload != reloadNone {
		t.Errorf("the next poll's reload = %v, want none until config.yaml changes again", retry.reload)
	}

	writeStoreFile(t, ts, "config.yaml", reviewConfig)
	m = tick(t, m)

	if row := statusRow(m); strings.Contains(row, "reload config: ") {
		t.Errorf("status bar = %q, want the config error gone", row)
	}
	if m.session.store == ts.store || m.session.index == ts.index {
		t.Error("the session kept its old store or index, want them re-opened")
	}
	if _, isDeclared := m.session.store.Config.CategoryOf("review"); !isDeclared {
		t.Error("the re-opened store does not declare review, want the fixed config.yaml")
	}
}
