package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/reflist"
	"github.com/airiclenz/capcom/internal/store"
)

// staleETag is an if-match etag no issue file of these tests carries.
const staleETag = "0000000000000000"

// closedAt is the time the tests close, reopen and log at.
var closedAt = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

// issueText returns the content of a valid issue file with title and status, depending on deps.
func issueText(title string, status string, deps ...string) string {
	depsLine := ""
	if len(deps) > 0 {
		depsLine = fmt.Sprintf("deps: [%s]\n", strings.Join(deps, ", "))
	}
	return fmt.Sprintf(
		"---\ntitle: %s\nstatus: %s\ntype: task\npriority: 2\n%screated: 2026-10-11T09:00:00Z\n---\nBody.\n",
		title, status, depsLine,
	)
}

// newOps writes files, issue file contents keyed by id, into the issue tree of a fresh store with
// cfg, and returns the Ops over it with its index open. The index needs no git, so GitDir is a
// plain directory.
func newOps(t *testing.T, cfg config.Config, files map[string]string) (*Ops, *store.Store) {
	t.Helper()
	root := t.TempDir()
	opened := &store.Store{
		Root:         filepath.Join(root, ".capcom"),
		WorktreeRoot: root,
		GitDir:       filepath.Join(root, ".git"),
		GitCommonDir: filepath.Join(root, ".git"),
		Config:       cfg,
	}
	if err := os.MkdirAll(opened.IssuesDir(), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for id, content := range files {
		if err := os.WriteFile(filepath.Join(opened.IssuesDir(), id+".md"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	x, err := index.Open(opened, index.ModeRefresh)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := x.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return New(opened, x), opened
}

// readIssue returns the content of the issue file of id in s.
func readIssue(t *testing.T, s *store.Store, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.IssuesDir(), id+".md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

func TestWrites_StaleIfMatch_FailWithAConflictAndLeaveTheFile(t *testing.T) {
	tests := []struct {
		name  string
		write func(o *Ops) error
	}{
		{name: "close", write: func(o *Ops) error {
			_, err := o.Close("parser-fix", staleETag, "done", "shipped", closedAt)
			return err
		}},
		{name: "reopen", write: func(o *Ops) error {
			_, err := o.Reopen("parser-fix", staleETag, "open", "came back", closedAt)
			return err
		}},
		{name: "append log", write: func(o *Ops) error {
			_, err := o.AppendLog("parser-fix", staleETag, "reproduced", closedAt)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := issueText("Fix the parser", "open")
			o, s := newOps(t, config.Default(), map[string]string{"parser-fix": before})

			err := test.write(o)

			var conflict *nodestore.ConflictError
			if !errors.As(err, &conflict) {
				t.Errorf("err = %v, want a *nodestore.ConflictError", err)
			}
			if after := readIssue(t, s, "parser-fix"); after != before {
				t.Errorf("file changed:\n%s", after)
			}
		})
	}
}

func TestWrites_BlankText_IsRefusedAndLeavesTheFile(t *testing.T) {
	tests := []struct {
		name  string
		want  error
		write func(o *Ops) error
	}{
		{name: "close", want: ErrBlankReason, write: func(o *Ops) error {
			_, err := o.Close("parser-fix", "", "done", " \t", closedAt)
			return err
		}},
		{name: "reopen", want: ErrBlankReason, write: func(o *Ops) error {
			_, err := o.Reopen("parser-fix", "", "open", "", closedAt)
			return err
		}},
		{name: "append log", want: ErrBlankLog, write: func(o *Ops) error {
			_, err := o.AppendLog("parser-fix", "", "\n ", closedAt)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := issueText("Fix the parser", "open")
			o, s := newOps(t, config.Default(), map[string]string{"parser-fix": before})

			err := test.write(o)

			if !errors.Is(err, test.want) {
				t.Errorf("err = %v, want %v", err, test.want)
			}
			if after := readIssue(t, s, "parser-fix"); after != before {
				t.Errorf("file changed:\n%s", after)
			}
		})
	}
}

func TestClaim_NoDoingStatus_FailsAndLeavesTheFile(t *testing.T) {
	cfg := config.Default()
	cfg.Statuses = slices.DeleteFunc(slices.Clone(cfg.Statuses), func(status config.Status) bool {
		return status.Category == config.CategoryDoing
	})
	before := issueText("Fix the parser", "open")
	o, s := newOps(t, cfg, map[string]string{"parser-fix": before})

	_, err := o.Claim("parser-fix", "")

	if !errors.Is(err, ErrNoDoingStatus) {
		t.Errorf("err = %v, want ErrNoDoingStatus", err)
	}
	if after := readIssue(t, s, "parser-fix"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

func TestClaim_DoingStatusDeclared_MovesTheIssueToIt(t *testing.T) {
	o, s := newOps(t, config.Default(), map[string]string{"parser-fix": issueText("Fix the parser", "open")})

	written, err := o.Claim("parser-fix", "")

	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	after := readIssue(t, s, "parser-fix")
	if !strings.Contains(after, "\nstatus: active\n") || written.ID != "parser-fix" {
		t.Errorf("written = %+v, file:\n%s", written, after)
	}
}

func TestSetStatus_ChangeRefused_IsAChangeErrorAtTheFile(t *testing.T) {
	before := issueText("Fix the parser", "open")
	o, s := newOps(t, config.Default(), map[string]string{"parser-fix": before})

	written, err := o.SetStatus("parser-fix", "", "done")

	var refusal *ChangeError
	if !errors.As(err, &refusal) || refusal.Path != ".capcom/issues/parser-fix.md" ||
		!errors.Is(err, issue.ErrEntersClosed) {
		t.Errorf("err = %v, want a *ChangeError at the file wrapping ErrEntersClosed", err)
	}
	if written.Path != ".capcom/issues/parser-fix.md" || written.ETag != "" {
		t.Errorf("written = %+v, want the located file with no etag", written)
	}
	if after := readIssue(t, s, "parser-fix"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

// standingIssueText returns the content of a valid issue file with title and status, as
// issueText writes it, marked standing.
func standingIssueText(title string, status string) string {
	return strings.Replace(issueText(title, status), "\ncreated: ", "\nstanding: true\ncreated: ", 1)
}

func TestStatusChanges_StandingIssue_AreAChangeErrorAtTheFile(t *testing.T) {
	cases := []struct {
		name   string
		change func(o *Ops) (nodestore.Written, error)
	}{
		{name: "claim", change: func(o *Ops) (nodestore.Written, error) {
			return o.Claim("epic", "")
		}},
		{name: "set status", change: func(o *Ops) (nodestore.Written, error) {
			return o.SetStatus("epic", "", "active")
		}},
		{name: "close", change: func(o *Ops) (nodestore.Written, error) {
			closed, err := o.Close("epic", "", "done", "shipped", closedAt)
			return closed.Written, err
		}},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			before := standingIssueText("Group the auth work", "open")
			o, s := newOps(t, config.Default(), map[string]string{"epic": before})

			_, err := current.change(o)

			var refusal *ChangeError
			if !errors.As(err, &refusal) || refusal.Path != ".capcom/issues/epic.md" ||
				!errors.Is(err, issue.ErrStanding) {
				t.Errorf("err = %v, want a *ChangeError at the file wrapping ErrStanding", err)
			}
			if after := readIssue(t, s, "epic"); after != before {
				t.Errorf("file changed:\n%s", after)
			}
		})
	}
}

func TestCloser_SeveralCloses_NewlyReadyCountsTheEarlierCloses(t *testing.T) {
	o, _ := newOps(t, config.Default(), map[string]string{
		"db-schema":  issueText("Design the schema", "open"),
		"login-flow": issueText("Build the login", "open"),
		"ship-it":    issueText("Ship it", "open", "db-schema", "login-flow"),
	})
	closer, err := o.NewCloser()
	if err != nil {
		t.Fatalf("NewCloser: %v", err)
	}

	first, firstErr := closer.Close("db-schema", "", "done", "shipped", closedAt)
	second, secondErr := closer.Close("login-flow", "", "done", "shipped", closedAt)

	if firstErr != nil || secondErr != nil {
		t.Fatalf("Close: %v, %v", firstErr, secondErr)
	}
	if len(first.NewlyReady) != 0 || !slices.Equal(second.NewlyReady, []string{"ship-it"}) {
		t.Errorf("NewlyReady = %v then %v, want [] then [ship-it]", first.NewlyReady, second.NewlyReady)
	}
}

func TestAddDep_StaleIfMatch_FailsWithAConflictAndLeavesTheFile(t *testing.T) {
	before := issueText("Ship it", "open")
	o, s := newOps(t, config.Default(), map[string]string{
		"db-schema": issueText("Design the schema", "open"),
		"ship-it":   before,
	})

	_, err := o.AddDep("ship-it", staleETag, "db-schema")

	var conflict *nodestore.ConflictError
	if !errors.As(err, &conflict) {
		t.Errorf("err = %v, want a *nodestore.ConflictError", err)
	}
	if after := readIssue(t, s, "ship-it"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

func TestAddDep_NewDep_WritesItsCanonicalID(t *testing.T) {
	o, s := newOps(t, config.Default(), map[string]string{
		"db-schema": issueText("Design the schema", "open"),
		"ship-it":   issueText("Ship it", "open"),
	})

	written, err := o.AddDep("ship-it", "", "db-sch")

	if err != nil {
		t.Fatalf("AddDep: %v", err)
	}
	after := readIssue(t, s, "ship-it")
	if !strings.Contains(after, "\ndeps: [db-schema]\n") || written.ID != "ship-it" || written.ETag == "" {
		t.Errorf("written = %+v, file:\n%s", written, after)
	}
}

func TestAddDep_Cycle_IsARefusalBesideTheLocatedDependent(t *testing.T) {
	before := issueText("Design the schema", "open")
	o, s := newOps(t, config.Default(), map[string]string{
		"db-schema": before,
		"ship-it":   issueText("Ship it", "open", "db-schema"),
	})

	written, err := o.AddDep("db-schema", "", "ship-it")

	var refusal *reflist.Refusal
	if !errors.As(err, &refusal) || refusal.Reason != reflist.ReasonCycle {
		t.Errorf("err = %v, want a *reflist.Refusal of reason Cycle", err)
	}
	if written.Path != ".capcom/issues/db-schema.md" || written.ETag != "" {
		t.Errorf("written = %+v, want the located dependent with no etag", written)
	}
	if after := readIssue(t, s, "db-schema"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

func TestRemoveDep_NotADep_IsAChangeErrorAtTheDependentsFile(t *testing.T) {
	before := issueText("Ship it", "open")
	o, s := newOps(t, config.Default(), map[string]string{
		"db-schema": issueText("Design the schema", "open"),
		"ship-it":   before,
	})

	written, err := o.RemoveDep("ship-it", "", "db-schema")

	var change *ChangeError
	var refusal *reflist.Refusal
	if !errors.As(err, &change) || change.Path != ".capcom/issues/ship-it.md" ||
		!errors.As(err, &refusal) || refusal.Reason != reflist.ReasonNotFound {
		t.Errorf("err = %v, want a *ChangeError at the file wrapping a NotFound *reflist.Refusal", err)
	}
	if written.Path != ".capcom/issues/ship-it.md" {
		t.Errorf("written = %+v, want the located dependent", written)
	}
	if after := readIssue(t, s, "ship-it"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

func TestRemoveDep_LastDep_LeavesTheDepsKeyOut(t *testing.T) {
	o, s := newOps(t, config.Default(), map[string]string{
		"db-schema": issueText("Design the schema", "open"),
		"ship-it":   issueText("Ship it", "open", "db-schema"),
	})

	_, err := o.RemoveDep("ship-it", "", "db-schema")

	if err != nil {
		t.Fatalf("RemoveDep: %v", err)
	}
	if after := readIssue(t, s, "ship-it"); strings.Contains(after, "deps:") {
		t.Errorf("deps key kept:\n%s", after)
	}
}
