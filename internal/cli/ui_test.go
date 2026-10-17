package cli

import (
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/ui"
)

func TestUI_WithoutTerminal_ExitsOneNamingTheTerminal(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "no argument", args: []string{"ui"}},
		{name: "tab", args: []string{"ui", "issues"}},
		{name: "tab and id", args: []string{"ui", "issues", "valid-issue"}},
	}
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, root, testCase.args...)

			if exitCode != ExitError {
				t.Fatalf("%v: exit = %d, want %d\nstdout: %s\nstderr: %s", testCase.args, exitCode, ExitError, stdout, stderr)
			}
			if !strings.Contains(stderr, "stdin is not a terminal") {
				t.Errorf("%v: stderr = %q, want it to name the terminal", testCase.args, stderr)
			}
		})
	}
}

func TestUI_MalformedInvocation_ExitsUsage(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "json", args: []string{"ui", "--json"}, message: "--json"},
		{name: "unknown tab", args: []string{"ui", "bogus"}, message: `unknown tab "bogus"`},
		{name: "id without tab", args: []string{"ui", "valid-issue"}, message: `unknown tab "valid-issue"`},
		{name: "too many", args: []string{"ui", "issues", "valid-issue", "extra"}, message: "at most 2"},
	}
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, root, testCase.args...)

			if exitCode != ExitUsage {
				t.Fatalf("%v: exit = %d, want %d\nstdout: %s\nstderr: %s", testCase.args, exitCode, ExitUsage, stdout, stderr)
			}
			if !strings.Contains(stdout+stderr, testCase.message) {
				t.Errorf("%v: output = %q, want it to contain %q", testCase.args, stdout+stderr, testCase.message)
			}
		})
	}
}

func TestUITarget_ValidArguments_ReportsTabIDAndWhetherATabWasGiven(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		wantTab        ui.Tab
		wantID         string
		wantIsTabGiven bool
	}{
		{name: "no argument", args: nil, wantTab: ui.TabReady, wantIsTabGiven: false},
		{name: "ready", args: []string{"ready"}, wantTab: ui.TabReady, wantIsTabGiven: true},
		{name: "issues", args: []string{"issues"}, wantTab: ui.TabIssues, wantIsTabGiven: true},
		{name: "knowledge with id", args: []string{"knowledge", "store-layout"}, wantTab: ui.TabKnowledge, wantID: "store-layout", wantIsTabGiven: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tab, id, isTabGiven, err := uiTarget(testCase.args)

			if err != nil {
				t.Fatalf("uiTarget(%v): %v", testCase.args, err)
			}
			if tab != testCase.wantTab || id != testCase.wantID || isTabGiven != testCase.wantIsTabGiven {
				t.Errorf("uiTarget(%v) = (%v, %q, %t), want (%v, %q, %t)",
					testCase.args, tab, id, isTabGiven, testCase.wantTab, testCase.wantID, testCase.wantIsTabGiven)
			}
		})
	}
}
