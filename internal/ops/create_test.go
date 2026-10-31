package ops

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/reflist"
)

// createdAt is the created time the create tests give.
var createdAt = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

// newIssueFileExists reports whether the issue file of id exists at the top of o's issue tree.
func newIssueFileExists(t *testing.T, o *Ops, id string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(o.store.IssuesDir(), id+".md"))
	return err == nil
}

func TestCreateIssue_OnlyTheTitle_TakesTheDefaultsAndTheGivenCreatedTime(t *testing.T) {
	o, s := newOps(t, config.Default(), nil)

	created, err := o.CreateIssue(NewIssue{ID: "cache-warmup", Title: "Warm the cache", Created: createdAt})

	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	content := readIssue(t, s, "cache-warmup")
	for _, line := range []string{
		"\nstatus: open\n", "\ntype: task\n", "\npriority: 2\n", "\ncreated: 2001-02-03T04:05:06Z\n",
	} {
		if !strings.Contains(content, line) {
			t.Errorf("file lacks %q:\n%s", line, content)
		}
	}
	if created.ID != "cache-warmup" || created.Path != ".capcom/issues/cache-warmup.md" || created.ETag == "" {
		t.Errorf("written = %+v, want the new file with its etag", created.Written)
	}
	if !created.IsBodyEmpty || created.MissingPlan != "" {
		t.Errorf("IsBodyEmpty = %v, MissingPlan = %q, want true and none", created.IsBodyEmpty, created.MissingPlan)
	}
}

func TestCreateIssue_NoTaskType_DefaultsToTheFirstDeclaredType(t *testing.T) {
	cfg := config.Default()
	cfg.Types = []string{"story", "bug"}
	o, s := newOps(t, cfg, nil)

	_, err := o.CreateIssue(NewIssue{ID: "cache-warmup", Title: "Warm the cache", Created: createdAt})

	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if content := readIssue(t, s, "cache-warmup"); !strings.Contains(content, "\ntype: story\n") {
		t.Errorf("file lacks type story:\n%s", content)
	}
}

func TestCreateIssue_FullSpec_WritesEveryFieldAndBuildsTheBody(t *testing.T) {
	o, s := newOps(t, config.Default(), map[string]string{"db-schema": issueText("Design the schema", "open")})
	priority := issue.MinPriority

	created, err := o.CreateIssue(NewIssue{
		ID:         "cache-warmup",
		Title:      "Warm the cache",
		Status:     "active",
		Type:       "bug",
		Priority:   &priority,
		Labels:     []string{"perf", "perf"},
		Plan:       "warmup-plan",
		Deps:       []string{"db-sch", "", "db-schema"},
		Body:       func() (string, error) { return "Deploys start cold.", nil },
		Why:        "The first request times out.",
		Acceptance: []string{"Warm after deploy", ""},
		Created:    createdAt,
	})

	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	content := readIssue(t, s, "cache-warmup")
	for _, part := range []string{
		"\nstatus: active\n", "\ntype: bug\n", "\npriority: 0\n", "\nlabels: [perf]\n",
		"\nplan: warmup-plan\n", "\ndeps: [db-schema]\n",
		"\nDeploys start cold.\n\n## Why\n\nThe first request times out.\n\n## Acceptance\n\n- Warm after deploy\n",
	} {
		if !strings.Contains(content, part) {
			t.Errorf("file lacks %q:\n%s", part, content)
		}
	}
	if created.IsBodyEmpty || !strings.Contains(created.MissingPlan, `"warmup-plan"`) {
		t.Errorf("IsBodyEmpty = %v, MissingPlan = %q, want false and the plan's message",
			created.IsBodyEmpty, created.MissingPlan)
	}
}

func TestCreateIssue_ValueNotAccepted_IsAValueErrorAndWritesNothing(t *testing.T) {
	cfg := config.Default()
	outOfRange := issue.MaxPriority + 1
	tests := []struct {
		name      string
		spec      NewIssue
		wantField string
		wantErr   error
		wantValid []string
	}{
		{
			name:      "undeclared type",
			spec:      NewIssue{Type: "epic"},
			wantField: FieldType,
			wantErr:   ErrUndeclaredType,
			wantValid: cfg.Types,
		},
		{
			name:      "undeclared status",
			spec:      NewIssue{Status: "blocked"},
			wantField: FieldStatus,
			wantErr:   ErrUndeclaredStatus,
			wantValid: []string{"open (todo)", "active (doing)", "parked (waiting)", "done (done)", "dropped (dropped)"},
		},
		{
			name:      "priority out of range",
			spec:      NewIssue{Priority: &outOfRange},
			wantField: FieldPriority,
			wantErr:   ErrPriorityRange,
			wantValid: []string{"0", "1", "2", "3", "4"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o, _ := newOps(t, cfg, nil)
			test.spec.ID, test.spec.Title = "cache-warmup", "Warm the cache"

			_, err := o.CreateIssue(test.spec)

			var refusal *ValueError
			if !errors.As(err, &refusal) || refusal.Field != test.wantField || !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want a *ValueError of %s", err, test.wantField)
			}
			if !slices.Equal(refusal.Valid, test.wantValid) {
				t.Errorf("Valid = %q, want %q", refusal.Valid, test.wantValid)
			}
			if newIssueFileExists(t, o, "cache-warmup") {
				t.Error("the issue file was written")
			}
		})
	}
}

func TestCreateIssue_ClosedStatus_IsAnUnwrappedStatusErrorBeforeTheBodyIsRead(t *testing.T) {
	o, _ := newOps(t, config.Default(), nil)
	isBodyRead := false

	_, err := o.CreateIssue(NewIssue{
		ID:     "cache-warmup",
		Title:  "Warm the cache",
		Status: "done",
		Body:   func() (string, error) { isBodyRead = true; return "", nil },
	})

	var refusal *issue.StatusError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrCreatedClosed) || refusal.Category != config.CategoryDone {
		t.Fatalf("err = %v, want an *issue.StatusError wrapping ErrCreatedClosed", err)
	}
	var change *ChangeError
	if errors.As(err, &change) {
		t.Errorf("err = %v, want no *ChangeError", err)
	}
	if isBodyRead || newIssueFileExists(t, o, "cache-warmup") {
		t.Errorf("body read = %v or the issue file was written", isBodyRead)
	}
}

func TestCreateIssue_StandingInTheDoingCategory_IsAnUnwrappedStatusError(t *testing.T) {
	o, _ := newOps(t, config.Default(), nil)

	_, err := o.CreateIssue(NewIssue{ID: "auth-work", Title: "Group the auth work", Status: "active", IsStanding: true})

	var change *ChangeError
	if !errors.Is(err, issue.ErrStandingNotOpen) || errors.As(err, &change) {
		t.Errorf("err = %v, want an unwrapped refusal wrapping ErrStandingNotOpen", err)
	}
}

func TestCreateIssue_DepNamingNoIssue_IsANotFoundRefusalAndWritesNothing(t *testing.T) {
	tests := []struct {
		name string
		dep  string
	}{
		{name: "unknown dep", dep: "no-such-issue"},
		{name: "self dep", dep: "cache-warmup"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o, _ := newOps(t, config.Default(), nil)

			_, err := o.CreateIssue(NewIssue{ID: "cache-warmup", Title: "Warm the cache", Deps: []string{test.dep}})

			var refusal *reflist.Refusal
			if !errors.As(err, &refusal) || refusal.Reason != reflist.ReasonNotFound || refusal.Holder != "" {
				t.Errorf("err = %v, want a *reflist.Refusal of reason NotFound with no holder", err)
			}
			if newIssueFileExists(t, o, "cache-warmup") {
				t.Error("the issue file was written")
			}
		})
	}
}
