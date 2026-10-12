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
			name:  "related resolving to nothing",
			files: map[string]string{issues + "login-flow.md": issueFile("labels: [auth]\nrelated: [no-such-issue]\n")},
			want:  issues + "login-flow.md:8:" + check.CodeMissingRelated,
		},
		{
			name:  "related naming the issue itself",
			files: map[string]string{issues + "login-flow.md": issueFile("related: [login-flow]\n")},
			want:  issues + "login-flow.md:7:" + check.CodeField,
		},
		{
			name:  "related naming the issue itself by its alias",
			files: map[string]string{issues + "login-flow.md": issueFile("aliases: [old-flow]\nrelated: [old-flow]\n")},
			want:  issues + "login-flow.md:8:" + check.CodeField,
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
		{
			name: "superseded-by resolving to nothing",
			files: map[string]string{
				".capcom/config.yaml":     "format: 3\n",
				knowledge + "old-flow.md": "---\ntitle: Old flow\nlabels: [flow]\nsuperseded-by: [no-such-note]\n---\n",
			},
			want: knowledge + "old-flow.md:4:" + check.CodeMissingSupersededBy,
		},
		{
			name: "retired false in a format-2 store",
			files: map[string]string{
				".capcom/config.yaml":     "format: 2\n",
				knowledge + "old-flow.md": "---\ntitle: Old flow\nretired: false\n---\n",
			},
			want: knowledge + "old-flow.md:3:" + check.CodeFormatTooOld,
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

func TestRun_RetiredSubtree_SkipsRefChecksButStillValidatesFields(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		".capcom/config.yaml": "format: 3\n",
		knowledge + "old-flow.md": "---\ntitle: Old flow\nretired: true\n" +
			"superseded-by: [old-flow]\nrefs: [no/such/file.go]\n---\n",
		knowledge + "old-flow/step-one.md": "---\ntitle: Step one\nrefs: [no/such/file.go]\n---\n",
		knowledge + "live-flow.md":         "---\ntitle: Live flow\nrefs: [no/such/file.go]\n---\n",
	})

	problems := runCheck(t, root)

	got := locations(t, problems)
	want := []string{
		knowledge + "live-flow.md:3:" + check.CodeBrokenRef,
		knowledge + "old-flow.md:4:" + check.CodeField,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}

func TestRun_RelatedNamingAnAlias_ReportsNothing(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "login-flow.md":      issueFile("related: [apogee-kk0.4]\n"),
		issues + "session-timeout.md": issueFile("aliases: [apogee-kk0.4]\n"),
	})

	problems := runCheck(t, root)

	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestRun_SupersessionKeysWithoutConfig_ReportFormatTooOldAtEachKey(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		knowledge + "old-flow.md":  "---\ntitle: Old flow\nretired: true\nsuperseded-by: [new-flow]\n---\n",
		knowledge + "new-flow.md":  "---\ntitle: New flow\n---\n",
		knowledge + "live-flow.md": "---\ntitle: Live flow\nsuperseded-by: []\n---\n",
	})

	problems := runCheck(t, root)

	var got []string
	for _, problem := range problems {
		got = append(got, problem.Code+" "+problem.String())
	}
	const advice = ": set format: 3 in .capcom/config.yaml, or run a capcom write verb that raises it"
	want := []string{
		check.CodeFormatTooOld + " " + knowledge +
			"live-flow.md:3: superseded-by needs store format 3, but the store declares format 1" + advice,
		check.CodeFormatTooOld + " " + knowledge +
			"old-flow.md:3: retired needs store format 3, but the store declares format 1" + advice,
		check.CodeFormatTooOld + " " + knowledge +
			"old-flow.md:4: superseded-by needs store format 3, but the store declares format 1" + advice,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestRun_SupersededByNamingAnAlias_ReportsNothing(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		".capcom/config.yaml":     "format: 3\n",
		knowledge + "old-flow.md": "---\ntitle: Old flow\nretired: true\nsuperseded-by: [adr-0010]\n---\n",
		knowledge + "new-flow.md": "---\ntitle: New flow\naliases: [adr-0010]\n---\n",
	})

	problems := runCheck(t, root)

	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestRun_SupersededByNamingOnlyAnIDPrefix_ReportsMissingSupersededBy(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		".capcom/config.yaml":     "format: 3\n",
		knowledge + "old-flow.md": "---\ntitle: Old flow\nsuperseded-by:\n  - new\n---\n",
		knowledge + "new-flow.md": "---\ntitle: New flow\n---\n",
	})

	problems := runCheck(t, root)

	got := locations(t, problems)
	want := []string{knowledge + "old-flow.md:4:" + check.CodeMissingSupersededBy}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %v, want %v", problems, want)
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

func TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "Bad_Name.md":              issueFile(""),
		issues + "Bad_Name/child-issue.md":  issueFile(""),
		issues + "orphan-dir/lost-issue.md": issueFile(""),
	})

	problems := runCheck(t, root)

	var got []string
	for _, problem := range problems {
		got = append(got, problem.Code+" "+problem.String())
	}
	want := []string{
		check.CodeOrphanDirectory + " " + issues +
			"Bad_Name: directory left out: its node file Bad_Name.md has an invalid id",
		check.CodeInvalidID + " " + issues + `Bad_Name.md: invalid id "Bad_Name": ` +
			"must be 1 to 4 words of a-z and 0-9 joined by single hyphens",
		check.CodeOrphanDirectory + " " + issues +
			"orphan-dir: directory without its node file: no orphan-dir.md beside it; left out",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestRun_RepeatedAliases_WordedByTheSharedAliasCheck(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "login-flow.md": issueFile("aliases: [login-flow, old-name, old-name]\n"),
	})

	problems := runCheck(t, root)

	var got []string
	for _, problem := range problems {
		got = append(got, problem.Code+" "+problem.String())
	}
	want := []string{
		check.CodeDuplicate + " " + issues + `login-flow.md:7: alias "login-flow" repeats the node's own id`,
		check.CodeDuplicate + " " + issues + `login-flow.md:7: alias "old-name" is listed twice`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
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

// logConflict is a Log body as a merge of two concurrent `capcom log` calls leaves it, from line 8
// on below an issueFile(""): its markers are at lines 13, 15 and 17.
const logConflict = "\nThe body.\n\n## Log\n\n" +
	"<<<<<<< HEAD\n- 2026-10-11 alice: reproduced it.\n=======\n" +
	"- 2026-10-11 bob: found the cause.\n>>>>>>> other-branch\n"

func TestRun_ConflictMarkers_ReportedAtEachMarkerLine(t *testing.T) {
	conflictAt := func(path string, lines ...int) []string {
		result := make([]string, 0, len(lines))
		for _, line := range lines {
			result = append(result, path+":"+strconv.Itoa(line)+":"+check.CodeConflict)
		}
		return result
	}
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name:  "markers in the Log",
			files: map[string]string{issues + "login-flow.md": issueFile("") + logConflict},
			want:  conflictAt(issues+"login-flow.md", 13, 15, 17),
		},
		{
			name: "markers in the Log with CRLF line endings",
			files: map[string]string{issues + "login-flow.md": strings.ReplaceAll(
				issueFile("")+logConflict, "\n", "\r\n",
			)},
			want: conflictAt(issues+"login-flow.md", 13, 15, 17),
		},
		{
			name: "diff3 markers with the base section",
			files: map[string]string{issues + "login-flow.md": issueFile("") +
				"\n<<<<<<< HEAD\n- mine\n||||||| base\n=======\n- theirs\n>>>>>>> other-branch\n"},
			want: conflictAt(issues+"login-flow.md", 9, 11, 12, 14),
		},
		{
			name:  "markers in a Note body",
			files: map[string]string{knowledge + "testing-rules.md": "---\ntitle: Testing rules\n---\n" + logConflict},
			want:  conflictAt(knowledge+"testing-rules.md", 9, 11, 13),
		},
		{
			name:  "a setext heading underline alone",
			files: map[string]string{issues + "login-flow.md": issueFile("") + "\nA heading\n=======\n\nText.\n"},
			want:  []string{},
		},
		{
			name: "markers inside a backtick code fence",
			files: map[string]string{issues + "login-flow.md": issueFile("") +
				"\n```text\n<<<<<<< HEAD\n=======\n>>>>>>> other-branch\n```\n"},
			want: []string{},
		},
		{
			name: "markers inside a tilde code fence",
			files: map[string]string{issues + "login-flow.md": issueFile("") +
				"\n~~~~\n<<<<<<< HEAD\n```\n=======\n>>>>>>> other-branch\n~~~~\n"},
			want: []string{},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			writeFiles(t, root, testCase.files)

			problems := runCheck(t, root)

			if got := locations(t, problems); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("problems = %v\nwant %v", problems, testCase.want)
			}
		})
	}
}

func TestRun_ConflictMarker_MessageQuotesTheMarkerAndSaysHowToResolve(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{issues + "login-flow.md": issueFile("") + logConflict})

	problems := runCheck(t, root)

	want := check.Problem{
		Path:    issues + "login-flow.md",
		Line:    13,
		Code:    check.CodeConflict,
		Message: `git conflict marker "<<<<<<< HEAD"; resolve the conflict and delete the markers`,
	}
	if len(problems) == 0 || !reflect.DeepEqual(problems[0], want) {
		t.Errorf("problems = %+v; want the first to be %+v", problems, want)
	}
}

func TestRun_ConflictMarkersInFrontmatter_ReportedOnceWithoutAFrontmatterProblem(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "login-flow.md": "---\ntitle: Some issue\n<<<<<<< HEAD\nstatus: done\n=======\n" +
			"status: active\n>>>>>>> other-branch\ntype: task\ncreated: 2026-10-11T09:00:00Z\n---\n",
	})

	problems := runCheck(t, root)

	conflict := func(line int, marker string) check.Problem {
		return check.Problem{
			Path:    issues + "login-flow.md",
			Line:    line,
			Code:    check.CodeConflict,
			Message: "git conflict marker " + strconv.Quote(marker) + "; resolve the conflict and delete the markers",
		}
	}
	want := []check.Problem{
		conflict(3, "<<<<<<< HEAD"),
		conflict(5, "======="),
		conflict(7, ">>>>>>> other-branch"),
	}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("problems = %+v\nwant %+v", problems, want)
	}
}
