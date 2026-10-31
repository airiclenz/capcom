package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// parserFixPath is the file of the parser-fix issue the update tests write.
const parserFixPath = ".capcom/issues/parser-fix.md"

func TestUpdateIssue_NoChange_IsRefusedAndLeavesTheFile(t *testing.T) {
	before := issueText("Fix the parser", "open")
	o, s := newOps(t, config.Default(), map[string]string{"parser-fix": before})

	_, err := o.UpdateIssue("parser-fix", "", IssueUpdate{})

	if !errors.Is(err, ErrNoChange) {
		t.Errorf("err = %v, want ErrNoChange", err)
	}
	if after := readIssue(t, s, "parser-fix"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

func TestUpdateIssue_ValueNotAccepted_IsAnUnwrappedValueErrorBeforeTheRefResolves(t *testing.T) {
	o, _ := newOps(t, config.Default(), nil)
	status := "blocked"
	isTextRead := false
	change := IssueUpdate{
		Status:     &status,
		AppendText: func() (string, error) { isTextRead = true; return "Seen on Safari.", nil },
	}

	_, err := o.UpdateIssue("no-such-issue", "", change)

	var refusal *ValueError
	var changeErr *ChangeError
	if !errors.As(err, &refusal) || refusal.Field != FieldStatus || errors.As(err, &changeErr) ||
		errors.Is(err, tree.ErrNotFound) {
		t.Errorf("err = %v, want an unwrapped *ValueError of status", err)
	}
	if isTextRead {
		t.Error("AppendText ran before the value check")
	}
}

func TestUpdateIssue_ClosedStatus_IsAChangeErrorAtTheFile(t *testing.T) {
	before := issueText("Fix the parser", "open")
	o, s := newOps(t, config.Default(), map[string]string{"parser-fix": before})
	status := "done"

	_, err := o.UpdateIssue("parser-fix", "", IssueUpdate{Status: &status})

	var refusal *ChangeError
	if !errors.As(err, &refusal) || refusal.Path != parserFixPath || !errors.Is(err, issue.ErrEntersClosed) {
		t.Errorf("err = %v, want a *ChangeError at the file wrapping ErrEntersClosed", err)
	}
	if after := readIssue(t, s, "parser-fix"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

func TestUpdateIssue_ReplacementNotFound_IsAReplacementErrorAndLeavesTheFile(t *testing.T) {
	before := issueText("Fix the parser", "open")
	o, s := newOps(t, config.Default(), map[string]string{"parser-fix": before})
	change := IssueUpdate{Replacements: []Replacement{{Old: "Body.", New: "Text."}, {Old: "missing", New: "found"}}}

	_, err := o.UpdateIssue("parser-fix", "", change)

	var refusal *ReplacementError
	if !errors.As(err, &refusal) || refusal.Index != 1 || refusal.Old != "missing" ||
		!errors.Is(err, edit.ErrReplaceMatch) {
		t.Errorf("err = %v, want the *ReplacementError of the second pair", err)
	}
	if after := readIssue(t, s, "parser-fix"); after != before {
		t.Errorf("file changed:\n%s", after)
	}
}

func TestUpdateIssue_Standing_SetsAndClearsTheMark(t *testing.T) {
	o, s := newOps(t, config.Default(), map[string]string{"parser-fix": issueText("Fix the parser", "open")})
	isStanding, isNotStanding := true, false

	_, setErr := o.UpdateIssue("parser-fix", "", IssueUpdate{IsStanding: &isStanding})
	marked := readIssue(t, s, "parser-fix")
	_, clearErr := o.UpdateIssue("parser-fix", "", IssueUpdate{IsStanding: &isNotStanding})
	cleared := readIssue(t, s, "parser-fix")

	if setErr != nil || clearErr != nil {
		t.Fatalf("set err = %v, clear err = %v, want none", setErr, clearErr)
	}
	if !strings.Contains(marked, "standing: true") || strings.Contains(cleared, "standing:") {
		t.Errorf("marked =\n%s\ncleared =\n%s\nwant the mark set, then cleared", marked, cleared)
	}
}

func TestUpdateIssue_EveryField_WritesThemInOrder(t *testing.T) {
	o, s := newOps(t, config.Default(), map[string]string{"parser-fix": issueText("Fix the parser", "open")})
	title, status, issueType, priority, plan := "Fix the lexer", "active", "bug", 0, "lexer-plan"
	change := IssueUpdate{
		Title:        &title,
		Status:       &status,
		Type:         &issueType,
		Priority:     &priority,
		Plan:         &plan,
		AddLabels:    []string{"parser", "lexer"},
		RemoveLabels: []string{"parser"},
		Replacements: []Replacement{{Old: "Body.", New: "Text."}},
		Appends:      []string{"First."},
		AppendText:   func() (string, error) { return "Last.", nil },
	}

	written, err := o.UpdateIssue("parser-fix", "", change)

	if err != nil || written.ID != "parser-fix" || written.ETag == "" {
		t.Fatalf("written = %+v, err = %v, want the issue written", written, err)
	}
	after := readIssue(t, s, "parser-fix")
	for _, want := range []string{
		"title: Fix the lexer\n", "status: active\n", "type: bug\n", "priority: 0\n", "plan: lexer-plan\n",
		"labels: [lexer]\n", "Text.\n\nFirst.\n\nLast.\n",
	} {
		if !strings.Contains(after, want) {
			t.Errorf("file lacks %q:\n%s", want, after)
		}
	}
}
