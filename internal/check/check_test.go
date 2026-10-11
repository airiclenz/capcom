package check_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/check"
)

// newRepo creates a git repository holding an empty store in a temp dir and returns its root. It
// isolates git from user and system configuration (ADR 0007), so tests do not call t.Parallel.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(root, ".capcom", "issues"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return root
}

// writeFiles writes each file, named by a slash path relative to root, creating its directories.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
}

// issueFile is a valid open issue whose frontmatter ends with extra, from line 7 on.
func issueFile(extra string) string {
	return "---\ntitle: Some issue\nstatus: open\ntype: task\npriority: 2\n" +
		"created: 2026-10-11T09:00:00Z\n" + extra + "---\n"
}

// Store paths of the fixtures, relative to the working tree root.
const (
	issues    = ".capcom/issues/"
	knowledge = ".capcom/knowledge/"
)

// runCheck runs check in root and fails the test on error.
func runCheck(t *testing.T, root string) []check.Problem {
	t.Helper()
	problems, err := check.Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return problems
}

// locations renders each problem as "path:line:code", and fails the test on an empty message.
func locations(t *testing.T, problems []check.Problem) []string {
	t.Helper()
	result := make([]string, 0, len(problems))
	for _, problem := range problems {
		if problem.Message == "" {
			t.Errorf("problem %+v has no message", problem)
		}
		result = append(result, problem.Path+":"+strconv.Itoa(problem.Line)+":"+problem.Code)
	}
	return result
}

func TestRun_CleanStore_ReportsNothing(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "login-flow.md": issueFile(
			"deps: [apogee-kk0.4]\nplan: 2026-10-01 - 00 - login-plan\n",
		),
		issues + "login-flow/session-timeout.md": issueFile("aliases: [apogee-kk0.4]\n"),
		issues + "old-idea.md": "---\ntitle: Old idea\nstatus: dropped\ntype: task\npriority: 2\n" +
			"created: 2026-10-11T09:00:00Z\nclosed: 2026-10-12T09:00:00Z\nreason: shelved\n---\n",
		knowledge + "testing-rules.md":                        "---\ntitle: Testing rules\nrefs: [go.mod, go.mod#module]\n---\n",
		"docs/plans/archived/2026-10-01 - 00 - login-plan.md": "# Login plan\n",
		"go.mod": "module example\n",
	})

	problems := runCheck(t, root)

	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "config error",
			files: map[string]string{".capcom/config.yaml": "types: [task]\nbogus: 1\n"},
			want:  ".capcom/config.yaml:2:" + check.CodeConfig,
		},
		{
			name:  "unknown issue key",
			files: map[string]string{issues + "login-flow.md": issueFile("estimate: 3\n")},
			want:  issues + "login-flow.md:7:" + check.CodeFrontmatter,
		},
		{
			name: "undeclared status",
			files: map[string]string{issues + "login-flow.md": strings.Replace(
				issueFile(""), "status: open", "status: wip", 1,
			)},
			want: issues + "login-flow.md:3:" + check.CodeField,
		},
		{
			name: "undeclared type",
			files: map[string]string{issues + "login-flow.md": strings.Replace(
				issueFile(""), "type: task", "type: epic", 1,
			)},
			want: issues + "login-flow.md:4:" + check.CodeField,
		},
		{
			name:  "malformed id",
			files: map[string]string{issues + "Login_Flow.md": issueFile("")},
			want:  issues + "Login_Flow.md:0:" + check.CodeInvalidID,
		},
		{
			name:  "orphan directory",
			files: map[string]string{issues + "login-flow/session-timeout.md": issueFile("")},
			want:  issues + "login-flow:0:" + check.CodeOrphanDirectory,
		},
		{
			name: "duplicate id",
			files: map[string]string{
				issues + "login-flow.md":                 issueFile(""),
				issues + "login-flow/session-timeout.md": issueFile(""),
				issues + "export-csv.md":                 issueFile(""),
				issues + "export-csv/session-timeout.md": issueFile(""),
			},
			want: issues + "login-flow/session-timeout.md:0:" + check.CodeDuplicate,
		},
		{
			name: "duplicate alias",
			files: map[string]string{
				issues + "login-flow.md": issueFile("aliases: [apogee-qi3]\n"),
				issues + "export-csv.md": issueFile("aliases: [apogee-qi3]\n"),
			},
			want: issues + "login-flow.md:7:" + check.CodeDuplicate,
		},
		{
			name:  "dep resolving to nothing",
			files: map[string]string{issues + "login-flow.md": issueFile("labels: [auth]\ndeps: [no-such-issue]\n")},
			want:  issues + "login-flow.md:8:" + check.CodeMissingDep,
		},
		{
			name: "closed without closed",
			files: map[string]string{issues + "login-flow.md": strings.Replace(
				issueFile("reason: finished\n"), "status: open", "status: done", 1,
			)},
			want: issues + "login-flow.md:3:" + check.CodeField,
		},
		{
			name:  "plan in no plans dir",
			files: map[string]string{issues + "login-flow.md": issueFile("plan: 2026-10-01 - 00 - nowhere\n")},
			want:  issues + "login-flow.md:7:" + check.CodeMissingPlan,
		},
		{
			name:  "note with an unknown key",
			files: map[string]string{knowledge + "testing-rules.md": "---\ntitle: Testing rules\nowner: me\n---\n"},
			want:  knowledge + "testing-rules.md:3:" + check.CodeFrontmatter,
		},
		{
			name:  "note without a title",
			files: map[string]string{knowledge + "testing-rules.md": "---\nlabels: [invariant]\n---\n"},
			want:  knowledge + "testing-rules.md:0:" + check.CodeField,
		},
		{
			name:  "broken note ref",
			files: map[string]string{knowledge + "testing-rules.md": "---\ntitle: Testing rules\nrefs:\n  - no/such/file.go\n---\n"},
			want:  knowledge + "testing-rules.md:4:" + check.CodeBrokenRef,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			writeFiles(t, root, testCase.files)

			problems := runCheck(t, root)

			got := locations(t, problems)
			if want := []string{testCase.want}; !reflect.DeepEqual(got, want) {
				t.Errorf("problems = %v, want %v", problems, want)
			}
		})
	}
}

func TestRun_DuplicateIDAndAlias_NameBothFilesRelativeToTheWorktree(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "export-csv.md":                 issueFile("aliases: [apogee-qi3]\n"),
		issues + "login-flow.md":                 issueFile("aliases: [apogee-qi3]\n"),
		issues + "export-csv/session-timeout.md": issueFile(""),
		issues + "login-flow/session-timeout.md": issueFile(""),
	})

	problems := runCheck(t, root)

	var got []string
	for _, problem := range problems {
		got = append(got, problem.String())
	}
	want := []string{
		issues + `login-flow.md:7: duplicate alias "apogee-qi3": also an alias of ` + issues + "export-csv.md",
		issues + `login-flow/session-timeout.md: duplicate id "session-timeout", also at ` +
			issues + "export-csv/session-timeout.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
	for _, line := range got {
		if strings.Contains(line, root) {
			t.Errorf("problem %q names an absolute path", line)
		}
	}
}

func TestRun_ThreeCycle_ReportedExactlyOnce(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "alpha-task.md": issueFile("deps: [beta-task]\n"),
		issues + "beta-task.md":  issueFile("deps: [gamma-task]\n"),
		issues + "gamma-task.md": issueFile("deps: [alpha-task]\n"),
	})

	problems := runCheck(t, root)

	want := []check.Problem{{
		Path:    issues + "alpha-task.md",
		Line:    7,
		Code:    check.CodeCycle,
		Message: "dependency cycle: alpha-task -> beta-task -> gamma-task -> alpha-task",
	}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}

func TestRun_ManyProblems_AllReportedInOneRunInPathOrder(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "Bad_Name.md":                   issueFile(""),
		issues + "orphan-dir/child-node.md":      issueFile(""),
		issues + "login-flow.md":                 issueFile("estimate: 3\n"),
		issues + "export-csv.md":                 strings.Replace(issueFile("deps: [nothing-here]\n"), "type: task", "type: epic", 1),
		knowledge + "testing-rules.md":           "---\nrefs: [missing.go]\n---\n",
		knowledge + "testing-rules/cli-tests.md": "---\ntitle: CLI tests\nbogus: x\n---\n",
	})

	problems := runCheck(t, root)

	want := []string{
		issues + "Bad_Name.md:0:" + check.CodeInvalidID,
		issues + "export-csv.md:4:" + check.CodeField,
		issues + "export-csv.md:7:" + check.CodeMissingDep,
		issues + "login-flow.md:7:" + check.CodeFrontmatter,
		issues + "orphan-dir:0:" + check.CodeOrphanDirectory,
		knowledge + "testing-rules.md:0:" + check.CodeField,
		knowledge + "testing-rules.md:2:" + check.CodeBrokenRef,
		knowledge + "testing-rules/cli-tests.md:3:" + check.CodeFrontmatter,
	}
	if got := locations(t, problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %v\nwant %v", got, want)
	}
}
