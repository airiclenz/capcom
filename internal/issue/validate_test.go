package issue_test

import (
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
)

// validIssue returns an open issue that passes Validate against the default config.
func validIssue() issue.Issue {
	return issue.Issue{
		Title:    "Login times out",
		Status:   "open",
		Type:     "bug",
		Priority: issue.DefaultPriority,
		Created:  time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC),
	}
}

func problemStrings(problems []*issue.Error) []string {
	rendered := make([]string, 0, len(problems))
	for _, problem := range problems {
		rendered = append(rendered, problem.Error())
	}
	return rendered
}

func TestValidate_ValidIssues_NoProblems(t *testing.T) {
	cases := map[string][]byte{
		"minimal golden":       readGolden(t, "minimal.md"),
		"full golden (closed)": readGolden(t, "full.md"),
		"priority zero golden": readGolden(t, "priority-zero.md"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			parsed := mustParse(t, data)

			problems := issue.Validate(testPath, parsed, config.Default())

			if problems != nil {
				t.Errorf("Validate = %q, want none", problemStrings(problems))
			}
		})
	}
}

func TestValidate_ParsedFile_ProblemCitesKeyLine(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "priority above range",
			content: "---\ntitle: T\nstatus: open\ntype: bug\npriority: 7\ncreated: 2026-10-11T09:30:00Z\n---\n",
			want:    testPath + ":5: priority 7 is out of range; use 0 to 4",
		},
		{
			name:    "priority below range",
			content: "---\ntitle: T\nstatus: open\ntype: bug\npriority: -1\ncreated: 2026-10-11T09:30:00Z\n---\n",
			want:    testPath + ":5: priority -1 is out of range; use 0 to 4",
		},
		{
			name:    "undeclared status",
			content: "---\ntitle: T\nstatus: review\ntype: bug\ncreated: 2026-10-11T09:30:00Z\n---\n",
			want:    testPath + `:3: status "review" is not declared in config; use one of [open active parked done dropped]`,
		},
		{
			name:    "undeclared type",
			content: "---\ntitle: T\nstatus: open\ntype: epic\ncreated: 2026-10-11T09:30:00Z\n---\n",
			want:    testPath + `:4: type "epic" is not declared in config; use one of [bug feature task chore]`,
		},
		{
			name:    "closed without reason",
			content: "---\ntitle: T\nstatus: done\ntype: bug\ncreated: 2026-10-11T09:30:00Z\nclosed: 2026-10-12T09:30:00Z\n---\n",
			want:    testPath + `:3: status "done" is in the done category, so reason is required`,
		},
		{
			name:    "dropped without closed time",
			content: "---\ntitle: T\nstatus: dropped\ntype: bug\ncreated: 2026-10-11T09:30:00Z\nreason: duplicate\n---\n",
			want:    testPath + `:3: status "dropped" is in the dropped category, so closed is required`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := mustParse(t, []byte(tc.content))

			problems := problemStrings(issue.Validate(testPath, parsed, config.Default()))

			if len(problems) != 1 || problems[0] != tc.want {
				t.Errorf("Validate = %q, want [%s]", problems, tc.want)
			}
		})
	}
}

func TestValidate_ReadsCategoryNotStatusName(t *testing.T) {
	cfg, err := config.Parse("config.yaml", []byte("statuses:\n  todo: todo\n  wontfix: dropped\n  shipped: done\n"))
	if err != nil {
		t.Fatalf("config.Parse: %v", err)
	}
	closedByCustomStatus := validIssue()
	closedByCustomStatus.Status = "wontfix"
	openByCustomStatus := validIssue()
	openByCustomStatus.Status = "todo"

	closedProblems := issue.Validate(testPath, closedByCustomStatus, cfg)
	openProblems := issue.Validate(testPath, openByCustomStatus, cfg)

	if len(closedProblems) != 2 {
		t.Errorf("Validate(wontfix) = %q, want closed and reason required", problemStrings(closedProblems))
	}
	if openProblems != nil {
		t.Errorf("Validate(todo) = %q, want none", problemStrings(openProblems))
	}
}

func TestValidate_ReturnsEveryProblem(t *testing.T) {
	broken := issue.Issue{Status: "done", Priority: 9}

	problems := problemStrings(issue.Validate(testPath, broken, config.Default()))

	want := []string{
		testPath + ": title is required",
		testPath + ": type is required; use one of [bug feature task chore]",
		testPath + ": priority 9 is out of range; use 0 to 4",
		testPath + ": created is required",
		testPath + `: status "done" is in the done category, so closed is required`,
		testPath + `: status "done" is in the done category, so reason is required`,
	}
	if strings.Join(problems, "\n") != strings.Join(want, "\n") {
		t.Errorf("Validate =\n%s\nwant\n%s", strings.Join(problems, "\n"), strings.Join(want, "\n"))
	}
}

func TestValidate_MissingStatus_IsRequired(t *testing.T) {
	missing := validIssue()
	missing.Status = ""

	problems := problemStrings(issue.Validate(testPath, missing, config.Default()))

	want := testPath + ": status is required; use one of [open active parked done dropped]"
	if len(problems) != 1 || problems[0] != want {
		t.Errorf("Validate = %q, want [%s]", problems, want)
	}
}
