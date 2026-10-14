package check_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/store"
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

// runCheck opens the store in root, runs check on it and fails the test on error.
func runCheck(t *testing.T, root string) []check.Problem {
	t.Helper()
	opened, err := store.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	problems, err := check.Run(opened)
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
			name: "related resolving to nothing",
			files: map[string]string{
				".capcom/config.yaml":    "format: 2\n",
				issues + "login-flow.md": issueFile("labels: [auth]\nrelated: [no-such-issue]\n"),
			},
			want: issues + "login-flow.md:8:" + check.CodeMissingRelated,
		},
		{
			name: "related naming the issue itself",
			files: map[string]string{
				".capcom/config.yaml":    "format: 2\n",
				issues + "login-flow.md": issueFile("related: [login-flow]\n"),
			},
			want: issues + "login-flow.md:7:" + check.CodeField,
		},
		{
			name: "related naming the issue itself by its alias",
			files: map[string]string{
				".capcom/config.yaml":    "format: 2\n",
				issues + "login-flow.md": issueFile("aliases: [old-flow]\nrelated: [old-flow]\n"),
			},
			want: issues + "login-flow.md:8:" + check.CodeField,
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

func TestConfigProblems_UnusableConfig_ReportsTheOneConfigProblem(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		".capcom/config.yaml":    "types: [task]\nbogus: 1\n",
		issues + "login-flow.md": issueFile("estimate: 3\n"),
	})
	_, err := store.Open(root)

	problems := check.ConfigProblems(err)

	want := []string{".capcom/config.yaml:2:" + check.CodeConfig}
	if got := locations(t, problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}

func TestConfigProblems_OtherOpenFailure_ReportsNothing(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	_, err := store.Open(root)
	if err == nil {
		t.Fatal("Open succeeded without a store")
	}

	if problems := check.ConfigProblems(err); problems != nil {
		t.Errorf("problems = %v, want nil", problems)
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
		".capcom/config.yaml":         "format: 2\n",
		issues + "login-flow.md":      issueFile("related: [apogee-kk0.4]\n"),
		issues + "session-timeout.md": issueFile("aliases: [apogee-kk0.4]\n"),
	})

	problems := runCheck(t, root)

	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestRun_RelatedBelowFormatTwo_ReportsFormatTooOldAtTheRelatedLine(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{name: "no config", files: map[string]string{}},
		{
			name:  "keyless config",
			files: map[string]string{".capcom/config.yaml": "types: [bug, feature, task, chore]\n"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRepo(t)
			writeFiles(t, root, testCase.files)
			writeFiles(t, root, map[string]string{
				issues + "login-flow.md":      issueFile("labels: [auth]\nrelated: [session-timeout]\n"),
				issues + "session-timeout.md": issueFile(""),
			})

			problems := runCheck(t, root)

			var got []string
			for _, problem := range problems {
				got = append(got, problem.Code+" "+problem.String())
			}
			want := []string{
				check.CodeFormatTooOld + " " + issues + "login-flow.md:8: related needs store format 2, " +
					"but the store declares format 1: set format: 2 in .capcom/config.yaml, " +
					"or run a capcom write verb that raises it",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("problems = %q\nwant %q", got, want)
			}
		})
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

func TestRun_SupersededByNamingALeftOutNote_ReportsNoMissingSupersededBy(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		".capcom/config.yaml": "format: 3\n",
		knowledge + "old-flow.md": "---\ntitle: Old flow\nretired: true\n" +
			"superseded-by: [new-flow, step-one, legacy-step]\n---\n",
		knowledge + "new-flow.md":          "---\ntitle: New flow\n---\n",
		knowledge + "new-flow/step-one.md": "---\ntitle: Step one\naliases: [legacy-step]\n---\n",
		knowledge + "zz-other.md":          "---\ntitle: Other\n---\n",
		knowledge + "zz-other/new-flow.md": "---\ntitle: New flow too\n---\n",
	})

	problems := runCheck(t, root)

	got := locations(t, problems)
	want := []string{
		knowledge + "new-flow:0:" + check.CodeOrphanDirectory,
		knowledge + "zz-other/new-flow.md:0:" + check.CodeDuplicate,
	}
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

func TestRun_DroppedDuplicateIDFiles_AliasClashAndParseFailureReported(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "export-csv.md":                 issueFile("aliases: [apogee-qi3]\n"),
		issues + "login-flow.md":                 issueFile(""),
		issues + "export-csv/session-timeout.md": issueFile(""),
		issues + "login-flow/session-timeout.md": issueFile("aliases: [apogee-qi3]\n"),
		issues + "session-timeout.md":            issueFile("estimate: 3\n"),
	})

	problems := runCheck(t, root)

	var got []string
	for _, problem := range problems {
		got = append(got, problem.Code+" "+problem.String())
	}
	keptTwin := issues + "export-csv/session-timeout.md"
	want := []string{
		check.CodeDuplicate + " " + issues + `login-flow/session-timeout.md: duplicate id "session-timeout", also at ` +
			keptTwin,
		check.CodeDuplicate + " " + issues + `login-flow/session-timeout.md:7: duplicate alias "apogee-qi3": ` +
			"also an alias of " + issues + "export-csv.md",
		check.CodeDuplicate + " " + issues + `session-timeout.md: duplicate id "session-timeout", also at ` + keptTwin,
		check.CodeFrontmatter + " " + issues + `session-timeout.md:7: unknown key "estimate"; an issue has ` +
			"title, status, type, priority, labels, deps, related, plan, aliases, created, closed and reason",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
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

// makeSymlink creates link pointing at target, skipping the test on Windows and as root.
func makeSymlink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%s): %v", link, err)
	}
}

// The index pins the same fixture's warnings in TestRefresh_FileProblems_PinKindsAndWording.
func TestRun_FileProblems_PinCodesAndWording(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		issues + "unknown-key.md":         issueFile("estimate: 3\n"),
		issues + "bad-status.md":          strings.Replace(issueFile(""), "status: open", "status: wip", 1),
		issues + "bad-alias.md":           issueFile("aliases: [Bad_Alias]\n"),
		issues + "own-alias.md":           issueFile("aliases: [own-alias]\n"),
		issues + "top-issue.md":           issueFile(""),
		issues + "top-issue/dup-issue.md": issueFile(""),
		issues + "dup-issue.md":           issueFile(""),
		"outside.md":                      issueFile(""),
	})
	link := filepath.Join(root, filepath.FromSlash(issues+"link-issue.md"))
	makeSymlink(t, filepath.Join(root, "outside.md"), link)

	problems := runCheck(t, root)

	var got []string
	for _, problem := range problems {
		got = append(got, problem.Code+" "+problem.String())
	}
	want := []string{
		check.CodeInvalidAlias + " " + issues + `bad-alias.md:7: invalid alias "Bad_Alias": ` +
			"must use only a-z, 0-9, dots and hyphens",
		check.CodeField + " " + issues + `bad-status.md:3: status "wip" is not declared in config; ` +
			"use one of [open active parked done dropped]",
		check.CodeNotRegular + " " + issues + "link-issue.md: not a regular file",
		check.CodeDuplicate + " " + issues + `own-alias.md:7: alias "own-alias" repeats the node's own id`,
		check.CodeDuplicate + " " + issues + `top-issue/dup-issue.md: duplicate id "dup-issue", also at ` +
			issues + "dup-issue.md",
		check.CodeFrontmatter + " " + issues + `unknown-key.md:7: unknown key "estimate"; an issue has ` +
			"title, status, type, priority, labels, deps, related, plan, aliases, created, closed and reason",
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

// fifoCheckTimeout bounds a check run over a named pipe: a run that opens the pipe blocks for good.
const fifoCheckTimeout = 30 * time.Second

func TestRun_NamedPipeIssueFile_ReportedNotRegularWithoutBlocking(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{issues + "login-flow.md": issueFile("")})
	makeFIFO(t, filepath.Join(root, filepath.FromSlash(issues+"stuck-issue.md")))
	done := make(chan []check.Problem, 1)

	go func() {
		opened, err := store.Open(root)
		if err != nil {
			t.Errorf("Open: %v", err)
			done <- nil
			return
		}
		problems, err := check.Run(opened)
		if err != nil {
			t.Errorf("Run: %v", err)
		}
		done <- problems
	}()

	select {
	case problems := <-done:
		want := []string{issues + "stuck-issue.md:0:" + check.CodeNotRegular}
		if got := locations(t, problems); !reflect.DeepEqual(got, want) {
			t.Errorf("problems = %v, want %v", problems, want)
		}
	case <-time.After(fifoCheckTimeout):
		t.Fatal("Run blocked on the named pipe")
	}
}

// noteFile is a valid Note whose frontmatter ends with extra, from line 3 on.
func noteFile(extra string) string {
	return "---\ntitle: Some note\n" + extra + "---\n"
}

// The Note-tree twin of TestRun_FileProblems_PinCodesAndWording; the index pins the same
// fixture's warnings in TestNotes_FileProblems_PinKindsAndWording.
func TestRun_NoteFileProblems_PinCodesAndWording(t *testing.T) {
	root := newRepo(t)
	writeFiles(t, root, map[string]string{
		knowledge + "unknown-key.md":       noteFile("owner: me\n"),
		knowledge + "bad-ref.md":           noteFile("refs: [\"#Symbol\"]\n"),
		knowledge + "bad-alias.md":         noteFile("aliases: [Bad_Alias]\n"),
		knowledge + "own-alias.md":         noteFile("aliases: [own-alias]\n"),
		knowledge + "top-note.md":          noteFile(""),
		knowledge + "top-note/dup-note.md": noteFile(""),
		knowledge + "dup-note.md":          noteFile(""),
		"outside.md":                       noteFile(""),
	})
	link := filepath.Join(root, filepath.FromSlash(knowledge+"link-note.md"))
	makeSymlink(t, filepath.Join(root, "outside.md"), link)

	problems := runCheck(t, root)

	var got []string
	for _, problem := range problems {
		got = append(got, problem.Code+" "+problem.String())
	}
	want := []string{
		check.CodeInvalidAlias + " " + knowledge + `bad-alias.md:3: invalid alias "Bad_Alias": ` +
			"must use only a-z, 0-9, dots and hyphens",
		check.CodeField + " " + knowledge + `bad-ref.md:3: ref "#Symbol" names no path; ` +
			"write path or path#symbol",
		check.CodeNotRegular + " " + knowledge + "link-note.md: not a regular file",
		check.CodeDuplicate + " " + knowledge + `own-alias.md:3: alias "own-alias" repeats the node's own id`,
		check.CodeDuplicate + " " + knowledge + `top-note/dup-note.md: duplicate id "dup-note", also at ` +
			knowledge + "dup-note.md",
		check.CodeFrontmatter + " " + knowledge + `unknown-key.md:3: unknown key "owner"; a note has ` +
			"title, labels, refs, retired, superseded-by and aliases",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

// codeConstants returns the value of every exported Code constant declared in the check package's
// non-test source files, keyed by the constant's name.
func codeConstants(t *testing.T) map[string]string {
	t.Helper()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	constants := map[string]string{}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		for _, declaration := range file.Decls {
			general, isGeneral := declaration.(*ast.GenDecl)
			if !isGeneral || general.Tok != token.CONST {
				continue
			}
			for _, spec := range general.Specs {
				for i, name := range spec.(*ast.ValueSpec).Names {
					if !strings.HasPrefix(name.Name, "Code") {
						continue
					}
					literal, isLiteral := spec.(*ast.ValueSpec).Values[i].(*ast.BasicLit)
					if !isLiteral {
						t.Fatalf("%s is not a string literal", name.Name)
					}
					value, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatalf("%s: %v", name.Name, err)
					}
					constants[name.Name] = value
				}
			}
		}
	}
	return constants
}

// The codes are part of check --json and prime --json: a code added, renamed or removed changes
// what a script reading them sees, so the whole set is pinned here.
func TestCodes_ClosedSet_PinsEveryCodeString(t *testing.T) {
	want := map[string]string{
		"CodeConfig":              "config",
		"CodeInvalidID":           "invalid-id",
		"CodeInvalidAlias":        "invalid-alias",
		"CodeDuplicate":           "duplicate",
		"CodeOrphanDirectory":     "orphan-directory",
		"CodeNotRegular":          "not-regular",
		"CodeFrontmatter":         "frontmatter",
		"CodeConflict":            "conflict",
		"CodeField":               "field",
		"CodeMissingDep":          "missing-dep",
		"CodeMissingRelated":      "missing-related",
		"CodeMissingSupersededBy": "missing-superseded-by",
		"CodeFormatTooOld":        "format-too-old",
		"CodeCycle":               "cycle",
		"CodeMissingPlan":         "missing-plan",
		"CodeBrokenRef":           "broken-ref",
		"CodeAgentsBlock":         "agents-block",
	}
	exported := map[string]string{
		"CodeConfig":              check.CodeConfig,
		"CodeInvalidID":           check.CodeInvalidID,
		"CodeInvalidAlias":        check.CodeInvalidAlias,
		"CodeDuplicate":           check.CodeDuplicate,
		"CodeOrphanDirectory":     check.CodeOrphanDirectory,
		"CodeNotRegular":          check.CodeNotRegular,
		"CodeFrontmatter":         check.CodeFrontmatter,
		"CodeConflict":            check.CodeConflict,
		"CodeField":               check.CodeField,
		"CodeMissingDep":          check.CodeMissingDep,
		"CodeMissingRelated":      check.CodeMissingRelated,
		"CodeMissingSupersededBy": check.CodeMissingSupersededBy,
		"CodeFormatTooOld":        check.CodeFormatTooOld,
		"CodeCycle":               check.CodeCycle,
		"CodeMissingPlan":         check.CodeMissingPlan,
		"CodeBrokenRef":           check.CodeBrokenRef,
		"CodeAgentsBlock":         check.CodeAgentsBlock,
	}

	declared := codeConstants(t)

	if !reflect.DeepEqual(exported, want) {
		t.Errorf("code values = %v\nwant %v", exported, want)
	}
	if !reflect.DeepEqual(declared, want) {
		t.Errorf("declared codes = %v\nwant exactly %v", declared, want)
	}
}
