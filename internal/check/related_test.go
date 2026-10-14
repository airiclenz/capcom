package check_test

import (
	"testing"

	"github.com/airiclenz/capcom/internal/diagnose"
)

func TestRun_RelatedNamingTheIssueItself_ReportsOneFieldProblemNamingIt(t *testing.T) {
	tests := []struct {
		name  string
		extra string
		want  string
	}{
		{
			name:  "by its id",
			extra: "related: [login-flow]\n",
			want:  `related "login-flow" names the issue itself`,
		},
		{
			name:  "by its alias",
			extra: "aliases: [old-flow]\nrelated: [old-flow]\n",
			want:  `related "old-flow" names the issue itself`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newRepo(t)
			writeFiles(t, root, map[string]string{
				".capcom/config.yaml":    "format: 2\n",
				issues + "login-flow.md": issueFile(test.extra),
			})

			problems := runCheck(t, root)

			if len(problems) != 1 {
				t.Fatalf("problems = %v, want exactly one", problems)
			}
			if problems[0].Code != diagnose.Field {
				t.Errorf("code = %q, want %q", problems[0].Code, diagnose.Field)
			}
			if problems[0].Message != test.want {
				t.Errorf("message = %q, want %q", problems[0].Message, test.want)
			}
		})
	}
}
