package batch_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/batch"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/stage"
	"github.com/airiclenz/capcom/internal/store"
)

// runAuthor is the author the Log entries of these tests record.
const runAuthor = "Batch Tester"

// batchTime is the time every batch of these tests runs at: past midnight UTC in a zone ahead of
// it, with a fraction of a second a create and a close drop.
var batchTime = time.Date(2026, 10, 31, 20, 0, 0, 500_000_000, time.UTC)

// fixture is a store with its index and the staged ops a batch runs through.
type fixture struct {
	store   *store.Store
	ops     *ops.Ops
	overlay *stage.Overlay
}

// issueText returns the content of a valid issue file with title, status and deps.
func issueText(title string, status string, deps ...string) string {
	depLine := ""
	if len(deps) > 0 {
		depLine = fmt.Sprintf("deps: [%s]\n", strings.Join(deps, ", "))
	}
	return fmt.Sprintf(
		"---\ntitle: %s\nstatus: %s\ntype: task\npriority: 2\n%screated: 2026-10-11T09:00:00Z\n---\nBody.\n",
		title, status, depLine,
	)
}

// newFixture writes files, issue file contents keyed by id, into the issue tree of a fresh store
// with the default config, opens its index and returns the staged ops over a fresh Overlay. The
// index needs no git, so GitDir is a plain directory.
func newFixture(t *testing.T, files map[string]string) fixture {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", runAuthor)
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
		if err := os.WriteFile(filepath.Join(s.IssuesDir(), id+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
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
	overlay := stage.New(s)
	return fixture{
		store:   s,
		ops:     ops.Staged(s, overlay, overlay.RaiseFormat, overlay.Catalog(x)),
		overlay: overlay,
	}
}

// parse returns the operations of the apply file data.
func parse(t *testing.T, data string) []batch.Op {
	t.Helper()
	operations, err := batch.Parse(testFileName, []byte(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return operations
}

// run parses data and runs it against the fixture's staged ops at batchTime.
func (f fixture) run(t *testing.T, data string) (batch.Outcome, error) {
	t.Helper()
	return batch.Run(f.ops, f.overlay, parse(t, data), batchTime)
}

// staged returns the content of the file at path, relative to the working tree root, as the
// Overlay holds it.
func (f fixture) staged(t *testing.T, path string) string {
	t.Helper()
	data, err := f.overlay.ReadRegularFile(filepath.Join(f.store.WorktreeRoot, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("ReadRegularFile(%s): %v", path, err)
	}
	return string(data)
}

// diskETag returns the etag of issue id's file on disk.
func (f fixture) diskETag(t *testing.T, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.store.IssuesDir(), id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return nodefile.ETag(data)
}

// assertUnwritten fails unless the issue tree on disk holds exactly the files of ids.
func (f fixture) assertUnwritten(t *testing.T, ids ...string) {
	t.Helper()
	entries, err := os.ReadDir(f.store.IssuesDir())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	var want []string
	for _, id := range ids {
		want = append(want, id+".md")
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("issue tree on disk = %v, want %v: the batch wrote to disk", names, want)
	}
}

func TestRun_CreatesWithDeps_StagesEveryIssueAndCommitsNothing(t *testing.T) {
	f := newFixture(t, map[string]string{"base": issueText("Base", "open")})

	outcome, err := f.run(t, `
- op: create
  id: alpha
  title: Alpha
  body: First.
- op: create
  id: beta
  title: Beta
  dep: alpha
  body: Second.
- op: create
  id: gamma
  title: Gamma
  dep: [alpha, beta, base]
  body: Third.
`)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var ids []string
	for i, result := range outcome.Results {
		ids = append(ids, result.ID)
		if result.Position != i+1 || result.Kind != batch.KindCreate {
			t.Errorf("Results[%d] = position %d kind %q, want %d create", i, result.Position, result.Kind, i+1)
		}
	}
	if want := []string{"alpha", "beta", "gamma"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("result ids = %v, want %v", ids, want)
	}
	if gamma := f.staged(t, outcome.Results[2].Path); !strings.Contains(gamma, "deps: [alpha, beta, base]") {
		t.Errorf("gamma = %q, want deps on alpha, beta and base", gamma)
	}
	if !strings.Contains(f.staged(t, outcome.Results[0].Path), "created: 2026-10-31T20:00:00Z") {
		t.Errorf("alpha = %q, want created at the batch time in UTC, to the second", f.staged(t, outcome.Results[0].Path))
	}
	f.assertUnwritten(t, "base")
}

func TestRun_FailingOperation_ReturnsItsPositionAndCommitsNothing(t *testing.T) {
	f := newFixture(t, map[string]string{"base": issueText("Base", "open")})

	_, err := f.run(t, `
- op: create
  id: alpha
  title: Alpha
- op: log
  id: alpha
  text: started
- op: dep add
  id: alpha
  dep: missing
`)

	var opErr *batch.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("Run err = %v, want an *OpError", err)
	}
	if opErr.Index != 3 || opErr.Line != 8 || opErr.Op != batch.KindDepAdd {
		t.Errorf("OpError = index %d line %d op %q, want 3, 8, dep add", opErr.Index, opErr.Line, opErr.Op)
	}
	if !strings.HasPrefix(err.Error(), "operation 3 (dep add): ") {
		t.Errorf("Error() = %q, want it to name the operation", err.Error())
	}
	f.assertUnwritten(t, "base")
}

func TestRun_IfMatch_ComparesWithTheFileBeforeTheBatch(t *testing.T) {
	cases := []struct {
		name         string
		ifMatchOf    func(f fixture, t *testing.T, retitled string) string
		isConflicted bool
	}{
		{
			name: "the etag before the batch matches",
			ifMatchOf: func(f fixture, t *testing.T, _ string) string {
				return f.diskETag(t, "base")
			},
		},
		{
			name: "the etag after the first touch is stale",
			ifMatchOf: func(f fixture, t *testing.T, retitled string) string {
				return nodefile.ETag([]byte(retitled))
			},
			isConflicted: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, map[string]string{"base": issueText("Base", "open")})
			first, err := f.run(t, "- op: update\n  id: base\n  title: Retitled\n")
			if err != nil {
				t.Fatalf("first touch: %v", err)
			}
			retitled := f.staged(t, first.Results[0].Path)
			ifMatch := tc.ifMatchOf(f, t, retitled)

			_, err = f.run(t, fmt.Sprintf("- op: log\n  id: base\n  text: second touch\n  if-match: %s\n", ifMatch))

			var conflict *nodestore.ConflictError
			isConflict := errors.As(err, &conflict)
			if isConflict != tc.isConflicted {
				t.Fatalf("second touch err = %v, want a conflict: %v", err, tc.isConflicted)
			}
			if !tc.isConflicted {
				return
			}
			wantMessage := fmt.Sprintf(
				"etag %s does not match the file's etag %s; show the issue again", ifMatch, f.diskETag(t, "base"),
			)
			if conflict.Path != ".capcom/issues/base.md" || conflict.IfMatch != ifMatch || conflict.Message != wantMessage {
				t.Errorf("conflict = %+v, want the file, both etags and %q", conflict, wantMessage)
			}
		})
	}
}

func TestRun_IfMatchOnAnIssueTheBatchCreated_IsRefused(t *testing.T) {
	f := newFixture(t, nil)

	_, err := f.run(t, `
- op: create
  id: alpha
  title: Alpha
- op: log
  id: alpha
  text: started
  if-match: 0123456789abcdef
`)

	var opErr *batch.OpError
	if !errors.As(err, &opErr) || opErr.Index != 2 || !errors.Is(err, batch.ErrIfMatchCreated) {
		t.Errorf("Run err = %v, want operation 2 refused with ErrIfMatchCreated", err)
	}
}

func TestRun_Close(t *testing.T) {
	cases := []struct {
		name       string
		apply      string
		wantStatus string
		isRefused  func(err error) bool
	}{
		{
			name:       "without a status it takes the first done status",
			apply:      "- op: close\n  id: base\n  reason: shipped\n",
			wantStatus: "status: done",
		},
		{
			name:  "closing it twice in the batch is refused",
			apply: "- op: close\n  id: base\n  reason: shipped\n- op: close\n  id: base\n  reason: again\n",
			isRefused: func(err error) bool {
				return errors.Is(err, issue.ErrAlreadyClosed)
			},
		},
		{
			name:  "closing an issue Closed before the batch is refused",
			apply: "- op: close\n  id: shut\n  reason: again\n",
			isRefused: func(err error) bool {
				return errors.Is(err, issue.ErrAlreadyClosed)
			},
		},
		{
			name:  "an undeclared status is refused",
			apply: "- op: close\n  id: base\n  reason: shipped\n  status: nope\n",
			isRefused: func(err error) bool {
				var value *ops.ValueError
				return errors.As(err, &value)
			},
		},
		{
			name:  "a status outside the closed categories is refused as capcom close words it",
			apply: "- op: close\n  id: base\n  reason: shipped\n  status: open\n",
			isRefused: func(err error) bool {
				var category *batch.CategoryError
				return errors.As(err, &category) && strings.HasSuffix(err.Error(),
					`status "open" is in the todo category; close takes a status in the [done dropped] categories`)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, map[string]string{
				"base": issueText("Base", "open"),
				"shut": strings.Replace(issueText("Shut", "done"), "---\nBody",
					"closed: 2026-10-12T09:00:00Z\nreason: shipped\n---\nBody", 1),
			})

			outcome, err := f.run(t, tc.apply)

			if tc.isRefused != nil {
				if !tc.isRefused(err) {
					t.Errorf("Run err = %v, want it refused", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			closed := f.staged(t, outcome.Results[0].Path)
			if !strings.Contains(closed, tc.wantStatus) || !strings.Contains(closed, "closed: 2026-10-31T20:00:00Z") {
				t.Errorf("closed issue = %q, want %q closed at the batch time in UTC, to the second", closed, tc.wantStatus)
			}
		})
	}
}

func TestRun_Log_DatesTheEntryInLocalTimeAsCapcomLogDoes(t *testing.T) {
	local := time.Local
	time.Local = time.FixedZone("ahead", 10*60*60)
	t.Cleanup(func() { time.Local = local })
	f := newFixture(t, map[string]string{"base": issueText("Base", "open")})

	outcome, err := f.run(t, "- op: log\n  id: base\n  text: started\n")

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "- 2026-11-01 " + runAuthor + ": started\n"
	if logged := f.staged(t, outcome.Results[0].Path); !strings.Contains(logged, want) {
		t.Errorf("logged issue = %q, want the entry %q", logged, want)
	}
}

func TestRun_CreateWithoutBody_WarnsOnTheOutcome(t *testing.T) {
	f := newFixture(t, nil)

	outcome, err := f.run(t, "- op: create\n  id: alpha\n  title: Alpha\n- op: create\n  id: beta\n  title: Beta\n  body: Text.\n")

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Results[0].IsBodyEmpty || outcome.Results[1].IsBodyEmpty {
		t.Errorf("IsBodyEmpty = %v, %v, want true for the create without a body only",
			outcome.Results[0].IsBodyEmpty, outcome.Results[1].IsBodyEmpty)
	}
}

func TestRun_NowReady_NamesOnlyIssuesThatExistedBeforeTheBatch(t *testing.T) {
	f := newFixture(t, map[string]string{
		"blocker": issueText("Blocker", "open"),
		"waiting": issueText("Waiting", "open", "blocker"),
		"ready":   issueText("Ready", "open"),
	})

	outcome, err := f.run(t, `
- op: create
  id: fresh
  title: Fresh
  body: Ready from the start.
- op: create
  id: later
  title: Later
  dep: blocker
  body: Ready once blocker closes.
- op: close
  id: blocker
  reason: done
`)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if want := []string{"waiting"}; !reflect.DeepEqual(outcome.NowReady, want) {
		t.Errorf("NowReady = %v, want %v", outcome.NowReady, want)
	}
}
