package tree_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/tree"
)

// writeNodes creates every relative path under dir as a file, making parent directories.
func writeNodes(t *testing.T, dir string, paths ...string) {
	t.Helper()
	for _, relative := range paths {
		path := filepath.Join(dir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
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

// writeSizedFile writes size bytes to path.
func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// aliasesByFile returns an AliasFunc that looks a node's aliases up by its file name.
func aliasesByFile(aliases map[string][]string) tree.AliasFunc {
	return func(path string) ([]string, error) {
		return aliases[filepath.Base(path)], nil
	}
}

// aliasesByPath returns an AliasFunc that looks a node's aliases up by its path relative to dir,
// slash-separated, so two files of one id in different directories can list different aliases.
func aliasesByPath(dir string, aliases map[string][]string) tree.AliasFunc {
	return func(path string) ([]string, error) {
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return nil, err
		}
		return aliases[filepath.ToSlash(relative)], nil
	}
}

// ids returns the ids of nodes, in order.
func ids(nodes []tree.Node) []string {
	result := make([]string, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node.ID)
	}
	return result
}

// mustLoad loads dir and fails the test on error.
func mustLoad(t *testing.T, dir string, aliases tree.AliasFunc) *tree.Tree {
	t.Helper()
	loaded, err := tree.Load(dir, "issue", aliases)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded
}

func TestValidateID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id       string
		wantRule string
	}{
		{"login-flow", ""},
		{"login-timeout-race", ""},
		{"fix-login-timeout-race", ""},
		{"v2-api", ""},
		{"2026-plan", ""},
		{strings.Repeat("a", 19) + "-" + strings.Repeat("b", 20), ""},
		{"login", ""},
		{strings.Repeat("a", 20) + "-" + strings.Repeat("b", 20), "longer than 40 characters"},
		{"Fix-the-login-timeout-race-" + strings.Repeat("x", 20), "longer than 40 characters"},
		{"fix-the-login-timeout-race", "5 words, at most 4"},
		{"a-b-c-d-e-f", "6 words, at most 4"},
		{"Login-flow", `upper-case letter "L"; ids are lower-case`},
		{"Bad_Name", `upper-case letter "B"; ids are lower-case`},
		{"login-a-b-c-D_x", `upper-case letter "D"; ids are lower-case`},
		{"login_flow", `character "_" not allowed; use a-z, 0-9 and hyphens`},
		{"apogee-kk0.4", `character "." not allowed; use a-z, 0-9 and hyphens`},
		{"café-flow", `character "é" not allowed; use a-z, 0-9 and hyphens`},
		{"a-b-c-d-e_f", `character "_" not allowed; use a-z, 0-9 and hyphens`},
		{"login--flow", "empty word; join words with single hyphens"},
		{"-login-flow", "empty word; join words with single hyphens"},
		{"login-flow-", "empty word; join words with single hyphens"},
		{"-Login", "empty word; join words with single hyphens"},
		{"", "empty word; join words with single hyphens"},
	}
	for _, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			err := tree.ValidateID(testCase.id)

			if testCase.wantRule == "" {
				if err != nil {
					t.Fatalf("ValidateID(%q) = %v, want nil", testCase.id, err)
				}
				return
			}
			want := fmt.Sprintf("invalid id %q: %s", testCase.id, testCase.wantRule)
			if !errors.Is(err, tree.ErrInvalidID) || err.Error() != want {
				t.Fatalf("ValidateID(%q) = %v, want %q wrapping ErrInvalidID", testCase.id, err, want)
			}
		})
	}
}

func TestValidateAlias(t *testing.T) {
	t.Parallel()
	cases := []struct {
		alias   string
		isValid bool
	}{
		{"apogee-kk0.4", true},
		{"apogee-qi3", true},
		{"login", true},
		{"old-login-flow", true},
		{"", false},
		{"Apogee-qi3", false},
		{"apogee qi3", false},
		{"apogee/qi3", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.alias, func(t *testing.T) {
			err := tree.ValidateAlias(testCase.alias)

			if testCase.isValid && err != nil {
				t.Fatalf("ValidateAlias(%q) = %v, want nil", testCase.alias, err)
			}
			if !testCase.isValid && !errors.Is(err, tree.ErrInvalidAlias) {
				t.Fatalf("ValidateAlias(%q) = %v, want ErrInvalidAlias", testCase.alias, err)
			}
		})
	}
}

func TestLoad_NestedDirectories_BuildsParents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir,
		"login-flow.md",
		"login-flow/login-timeout-race.md",
		"login-flow/login-timeout-race/retry-backoff-bug.md",
		"login-flow/session-cookie-scope.md",
		"export-csv.md",
	)

	loaded := mustLoad(t, dir, nil)

	want := map[string]string{
		"login-flow":           "",
		"login-timeout-race":   "login-flow",
		"retry-backoff-bug":    "login-timeout-race",
		"session-cookie-scope": "login-flow",
		"export-csv":           "",
	}
	got := map[string]string{}
	for _, node := range loaded.Nodes() {
		got[node.ID] = node.ParentID
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parents = %v, want %v", got, want)
	}
	deep, _ := loaded.Lookup("retry-backoff-bug")
	wantPath := filepath.Join(dir, "login-flow", "login-timeout-race", "retry-backoff-bug.md")
	if deep.Path != wantPath {
		t.Fatalf("Path = %s, want %s", deep.Path, wantPath)
	}
}

func TestChildrenOf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir,
		"login-flow.md",
		"login-flow/session-cookie-scope.md",
		"login-flow/login-timeout-race.md",
		"export-csv.md",
	)
	loaded := mustLoad(t, dir, nil)
	cases := []struct {
		id   string
		want []string
	}{
		{"", []string{"export-csv", "login-flow"}},
		{"login-flow", []string{"login-timeout-race", "session-cookie-scope"}},
		{"export-csv", []string{}},
		{"no-such-node", []string{}},
	}
	for _, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			got := ids(loaded.ChildrenOf(testCase.id))

			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("ChildrenOf(%q) = %v, want %v", testCase.id, got, testCase.want)
			}
		})
	}
}

func TestLoad_DirectoryWithoutNodeFile_Fails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow/login-timeout-race.md")

	_, err := tree.Load(dir, "issue", nil)

	if !errors.Is(err, tree.ErrOrphanDirectory) {
		t.Fatalf("Load = %v, want ErrOrphanDirectory", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "login-flow")) {
		t.Fatalf("error %q does not name the directory", err)
	}
}

func TestWalk_DirectoryProblems_WordedAndLeftOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir,
		"Bad_Name.md",
		"Bad_Name/child-issue.md",
		"good-issue.md",
		"good-issue/nested-issue.md",
		"orphan-dir/lost-issue.md",
	)

	nodes, problems, err := tree.Walk(dir)

	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := ids(nodes); !reflect.DeepEqual(got, []string{"good-issue", "nested-issue"}) {
		t.Errorf("nodes = %v, want the valid nodes only", got)
	}
	want := map[string]string{
		filepath.Join(dir, "Bad_Name"): "directory left out: its node file Bad_Name.md has an invalid id",
		filepath.Join(dir, "Bad_Name.md"): `invalid id "Bad_Name": ` +
			`upper-case letter "B"; ids are lower-case`,
		filepath.Join(dir, "orphan-dir"): "directory without its node file: no orphan-dir.md beside it; left out",
	}
	got := map[string]string{}
	for _, problem := range problems {
		got[problem.Path] = problem.Message()
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q\nwant %q", got, want)
	}
}

func TestWalk_InvalidIDDirectory_IsAnOrphanDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "Bad_Name.md", "Bad_Name/child-issue.md")

	_, problems, err := tree.Walk(dir)

	if err != nil || len(problems) != 2 {
		t.Fatalf("Walk = %v, %v; want the invalid id and its directory", problems, err)
	}
	directory := problems[1]
	if directory.Path != filepath.Join(dir, "Bad_Name") || !errors.Is(directory, tree.ErrOrphanDirectory) {
		t.Errorf("directory problem = %v, want it at Bad_Name wrapping ErrOrphanDirectory", directory)
	}
}

func TestWalk_MissingDirectory_WalksEmpty(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "issues")

	nodes, problems, err := tree.Walk(dir)

	if nodes != nil || problems != nil || err != nil {
		t.Errorf("Walk = %v, %v, %v; want nothing", nodes, problems, err)
	}
}

func TestWalk_ChildDirectoryGoneBeforeItsRead_WalksTheRest(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "a.md", "a/b.md", "c.md")
	childDir := filepath.Join(dir, "a")
	tree.SetReadDir(t, func(path string) ([]fs.DirEntry, error) {
		if path == childDir {
			if err := os.RemoveAll(childDir); err != nil {
				t.Fatalf("RemoveAll: %v", err)
			}
		}
		return os.ReadDir(path)
	})

	nodes, problems, err := tree.Walk(dir)

	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got, want := ids(nodes), []string{"a", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Walk ids = %v, want %v", got, want)
	}
	if problems != nil {
		t.Errorf("Walk problems = %v, want none", problems)
	}
}

func TestLoad_InvalidID_FailsNamingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md", "login-flow/Bad_Name.md")

	_, err := tree.Load(dir, "issue", nil)

	var treeErr *tree.Error
	if !errors.As(err, &treeErr) || !errors.Is(err, tree.ErrInvalidID) {
		t.Fatalf("Load = %v, want *tree.Error wrapping ErrInvalidID", err)
	}
	if treeErr.Path != filepath.Join(dir, "login-flow", "Bad_Name.md") {
		t.Fatalf("Path = %s", treeErr.Path)
	}
}

func TestLoad_DuplicateIDInTwoSubtrees_FailsNamingBoth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir,
		"login-flow.md",
		"login-flow/timeout-race.md",
		"export-csv.md",
		"export-csv/timeout-race.md",
	)

	_, err := tree.Load(dir, "issue", nil)

	if !errors.Is(err, tree.ErrDuplicate) {
		t.Fatalf("Load = %v, want ErrDuplicate", err)
	}
	for _, path := range []string{
		filepath.Join(dir, "login-flow", "timeout-race.md"),
		filepath.Join(dir, "export-csv", "timeout-race.md"),
	} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("error %q does not name %s", err, path)
		}
	}
}

func TestLoad_DuplicateIDWithChildren_LeavesBothFilesAndTheSubtreeOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "a.md", "a/x.md", "b.md", "b/x.md", "b/x/c2.md")

	loaded, err := tree.Load(dir, "issue", nil)

	var problems tree.Errors
	if !errors.As(err, &problems) || loaded == nil {
		t.Fatalf("Load = %v, %v; want a tree and tree.Errors", loaded, err)
	}
	if got := ids(loaded.Nodes()); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("Nodes = %v, want [a b]: no file of x and nothing below one", got)
	}
	if children := loaded.ChildrenOf("x"); len(children) != 0 {
		t.Errorf("ChildrenOf(x) = %v, want none", children)
	}
	if got := ids(loaded.LeftOut()); !reflect.DeepEqual(got, []string{"c2", "x", "x"}) {
		t.Errorf("LeftOut = %v, want [c2 x x]", got)
	}
	var directoryProblems []*tree.Error
	for _, problem := range problems {
		if errors.Is(problem, tree.ErrOrphanDirectory) {
			directoryProblems = append(directoryProblems, problem)
		}
	}
	want := filepath.Join(dir, "b", "x") + ": directory left out: its node file x.md has a duplicate id"
	if len(directoryProblems) != 1 || directoryProblems[0].Error() != want {
		t.Errorf("directory problems = %v, want one: %q", directoryProblems, want)
	}
}

func TestLoad_AliasConflicts_Fail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		aliases map[string][]string
		want    error
	}{
		{
			name:    "alias repeats another id",
			aliases: map[string][]string{"login-flow.md": {"export-csv"}},
			want:    tree.ErrDuplicate,
		},
		{
			name:    "alias repeats its own id",
			aliases: map[string][]string{"login-flow.md": {"login-flow"}},
			want:    tree.ErrRepeatedAlias,
		},
		{
			name:    "alias listed twice",
			aliases: map[string][]string{"login-flow.md": {"apogee-qi3", "apogee-qi3"}},
			want:    tree.ErrRepeatedAlias,
		},
		{
			name: "alias on two nodes",
			aliases: map[string][]string{
				"login-flow.md": {"apogee-qi3"},
				"export-csv.md": {"apogee-qi3"},
			},
			want: tree.ErrDuplicate,
		},
		{
			name:    "invalid alias",
			aliases: map[string][]string{"export-csv.md": {"Apogee QI3"}},
			want:    tree.ErrInvalidAlias,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeNodes(t, dir, "login-flow.md", "export-csv.md")

			_, err := tree.Load(dir, "issue", aliasesByFile(testCase.aliases))

			if !errors.Is(err, testCase.want) {
				t.Fatalf("Load = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestLoad_AliasFuncError_ReturnedUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md")
	failure := errors.New("login-flow.md:3: bad front matter")

	_, err := tree.Load(dir, "issue", func(string) ([]string, error) { return nil, failure })

	if err != failure {
		t.Fatalf("Load = %v, want %v", err, failure)
	}
}

func TestLoad_DuplicateAlias_NamesTheOtherFileApart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "export-csv.md", "login-flow.md")

	_, err := tree.Load(dir, "issue", aliasesByFile(map[string][]string{
		"export-csv.md": {"apogee-qi3"},
		"login-flow.md": {"apogee-qi3"},
	}))

	var problem *tree.Error
	if !errors.As(err, &problem) || !errors.Is(err, tree.ErrDuplicate) {
		t.Fatalf("Load = %v, want *tree.Error wrapping ErrDuplicate", err)
	}
	want := tree.Error{
		Path:  filepath.Join(dir, "login-flow.md"),
		Other: filepath.Join(dir, "export-csv.md"),
		Alias: "apogee-qi3",
	}
	if problem.Path != want.Path || problem.Other != want.Other || problem.Alias != want.Alias {
		t.Fatalf("problem = %+v, want Path %s, Other %s, Alias %s", problem, want.Path, want.Other, want.Alias)
	}
	if message := `duplicate alias "apogee-qi3": also an alias of ` + want.Other; problem.Message() != message {
		t.Fatalf("Message = %q, want %q", problem.Message(), message)
	}
}

func TestLoad_DroppedFileAliasAnotherFileClaims_IsADuplicateNamingBoth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		files   []string
		aliases map[string][]string
		want    [2]string
		message string
	}{
		{
			name:    "id of a kept file",
			files:   []string{"foo.md", "bar.md", "sub.md", "sub/foo.md"},
			aliases: map[string][]string{"sub/foo.md": {"bar"}},
			want:    [2]string{"sub/foo.md", "bar.md"},
			message: `duplicate alias "bar": the id of `,
		},
		{
			name:    "alias of a kept file",
			files:   []string{"foo.md", "bar.md", "sub.md", "sub/foo.md"},
			aliases: map[string][]string{"bar.md": {"legacy-1"}, "sub/foo.md": {"legacy-1"}},
			want:    [2]string{"sub/foo.md", "bar.md"},
			message: `duplicate alias "legacy-1": also an alias of `,
		},
		{
			name:    "alias of an earlier dropped file",
			files:   []string{"foo.md", "bar.md", "sub.md", "sub/foo.md", "sub/bar.md"},
			aliases: map[string][]string{"sub/bar.md": {"legacy-1"}, "sub/foo.md": {"legacy-1"}},
			want:    [2]string{"sub/foo.md", "sub/bar.md"},
			message: `duplicate alias "legacy-1": also an alias of `,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeNodes(t, dir, testCase.files...)

			_, err := tree.Load(dir, "issue", aliasesByPath(dir, testCase.aliases))

			var problems tree.Errors
			if !errors.As(err, &problems) {
				t.Fatalf("Load = %v, want Errors", err)
			}
			var aliasProblems []*tree.Error
			for _, problem := range problems {
				if problem.Alias != "" {
					aliasProblems = append(aliasProblems, problem)
				}
			}
			path := filepath.Join(dir, filepath.FromSlash(testCase.want[0]))
			other := filepath.Join(dir, filepath.FromSlash(testCase.want[1]))
			if len(aliasProblems) != 1 || !errors.Is(aliasProblems[0], tree.ErrDuplicate) {
				t.Fatalf("alias problems = %v, want one wrapping ErrDuplicate", aliasProblems)
			}
			problem := aliasProblems[0]
			if problem.Path != path || problem.Other != other || problem.Message() != testCase.message+other {
				t.Fatalf("problem = %+v, want Path %s and Message %q", problem, path, testCase.message+other)
			}
		})
	}
}

func TestLoad_AliasOnlyADroppedFileLists_IsNoProblem(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "foo.md", "sub.md", "sub/foo.md")

	_, err := tree.Load(dir, "issue", aliasesByPath(dir, map[string][]string{"sub/foo.md": {"legacy-1"}}))

	var problems tree.Errors
	if !errors.As(err, &problems) || len(problems) != 1 || problems[0].Alias != "" {
		t.Fatalf("Load = %v, want the duplicate id alone", err)
	}
}

func TestCheckAliases_EachProblem_ReportedWithItsMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		aliases  []string
		wantKept []string
		want     []string
		sentinel error
	}{
		{
			name:     "invalid alias",
			aliases:  []string{"Apogee QI3"},
			want:     []string{`invalid alias "Apogee QI3": must use only a-z, 0-9, dots and hyphens`},
			sentinel: tree.ErrInvalidAlias,
		},
		{
			name:     "alias repeats its own id",
			aliases:  []string{"login-flow"},
			want:     []string{`alias "login-flow" repeats the issue's own id`},
			sentinel: tree.ErrRepeatedAlias,
		},
		{
			name:     "alias listed twice",
			aliases:  []string{"apogee-qi3", "old-name", "apogee-qi3"},
			wantKept: []string{"apogee-qi3", "old-name"},
			want:     []string{`alias "apogee-qi3" is listed twice`},
			sentinel: tree.ErrRepeatedAlias,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			kept, problems := tree.CheckAliases("login-flow.md", "issue", "login-flow", testCase.aliases)

			if !reflect.DeepEqual(kept, testCase.wantKept) {
				t.Errorf("kept = %q, want %q", kept, testCase.wantKept)
			}
			var got []string
			for _, problem := range problems {
				got = append(got, problem.Message())
				if problem.Path != "login-flow.md" || problem.Other != "" || problem.Alias == "" {
					t.Errorf("problem = %+v, want Path login-flow.md, no Other and the Alias", problem)
				}
				if !errors.Is(problem, testCase.sentinel) {
					t.Errorf("problem %v does not wrap %v", problem, testCase.sentinel)
				}
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("messages = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestCheckAliases_RepeatedAlias_IsADuplicate(t *testing.T) {
	t.Parallel()

	_, problems := tree.CheckAliases("login-flow.md", "Note", "login-flow", []string{"login-flow"})

	if len(problems) != 1 || !errors.Is(problems[0], tree.ErrDuplicate) {
		t.Fatalf("problems = %v, want one wrapping ErrDuplicate", problems)
	}
	if want := `alias "login-flow" repeats the Note's own id`; problems[0].Message() != want {
		t.Errorf("Message = %q, want %q", problems[0].Message(), want)
	}
}

func TestLoad_RepeatedAliases_DropOnlyTheOffendingAlias(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md")

	loaded, err := tree.Load(dir, "Note", aliasesByFile(map[string][]string{
		"login-flow.md": {"login-flow", "apogee-qi3", "apogee-qi3"},
	}))

	var problems tree.Errors
	if !errors.As(err, &problems) {
		t.Fatalf("Load = %v, want tree.Errors", err)
	}
	var got []string
	for _, problem := range problems {
		got = append(got, problem.Message())
	}
	want := []string{
		`alias "login-flow" repeats the Note's own id`,
		`alias "apogee-qi3" is listed twice`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("messages = %q, want %q", got, want)
	}
	if node, found := loaded.Lookup("apogee-qi3"); !found || node.ID != "login-flow" {
		t.Errorf("Lookup(apogee-qi3) = %v, %v; want login-flow", node, found)
	}
}

func TestLoad_ManyProblems_AllReportedWithTheRestOfTheTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir,
		"login-flow.md",
		"login-flow/Bad_Name.md",
		"orphan-dir/child-node.md",
		"export-csv.md",
		"export-csv/login-flow.md",
	)

	loaded, err := tree.Load(dir, "issue", aliasesByFile(map[string][]string{"export-csv.md": {"Bad Alias"}}))

	var problems tree.Errors
	if !errors.As(err, &problems) {
		t.Fatalf("Load = %v, want tree.Errors", err)
	}
	var paths []string
	for _, problem := range problems {
		paths = append(paths, problem.Path)
	}
	want := []string{
		filepath.Join(dir, "export-csv.md"),
		filepath.Join(dir, "login-flow"),
		filepath.Join(dir, "login-flow.md"),
		filepath.Join(dir, "login-flow", "Bad_Name.md"),
		filepath.Join(dir, "orphan-dir"),
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("problem paths = %v, want %v", paths, want)
	}
	for _, sentinel := range []error{tree.ErrDuplicate, tree.ErrInvalidAlias, tree.ErrInvalidID, tree.ErrOrphanDirectory} {
		if !errors.Is(err, sentinel) {
			t.Errorf("Load error does not wrap %v", sentinel)
		}
	}
	if got := ids(loaded.Nodes()); !reflect.DeepEqual(got, []string{"export-csv"}) {
		t.Errorf("Nodes = %v, want the placeable nodes", got)
	}
}

func TestLoad_MissingDirectory_LoadsEmpty(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "issues")

	loaded := mustLoad(t, dir, nil)

	if len(loaded.Nodes()) != 0 {
		t.Fatalf("Nodes = %v, want none", loaded.Nodes())
	}
}

func TestLoad_SkipsDotfilesTempFilesAndOtherFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir,
		"login-flow.md",
		".login-flow.md.tmp-123456",
		".hidden-node.md",
		".git-like/x.md",
		"notes.txt",
		"README",
		"login-flow/.export-csv.md.tmp-99",
	)

	loaded := mustLoad(t, dir, nil)

	if got := ids(loaded.Nodes()); !reflect.DeepEqual(got, []string{"login-flow"}) {
		t.Fatalf("Nodes = %v, want [login-flow]", got)
	}
}

func TestLoad_NonRegularAndOversizeFiles_LeftOutAndWarnedOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "good-node.md", "x/child-node.md")
	makeSymlink(t, os.DevNull, filepath.Join(dir, "x.md"))
	makeSymlink(t, filepath.Join(dir, "good-node.md"), filepath.Join(dir, "notes-link"))
	makeFIFO(t, filepath.Join(dir, "y.md"))
	writeSizedFile(t, filepath.Join(dir, "z.md"), 1<<20+1)

	loaded, err := tree.Load(dir, "issue", nil)

	if got := ids(loaded.Nodes()); !reflect.DeepEqual(got, []string{"good-node"}) {
		t.Errorf("Nodes = %v, want [good-node]", got)
	}
	var problems tree.Errors
	if !errors.As(err, &problems) {
		t.Fatalf("Load = %v, want its problems", err)
	}
	want := map[string]string{
		filepath.Join(dir, "notes-link"): "not a regular file",
		filepath.Join(dir, "x"):          "directory left out: its node file x.md is not a regular file",
		filepath.Join(dir, "x.md"):       "not a regular file",
		filepath.Join(dir, "y.md"):       "not a regular file",
		filepath.Join(dir, "z.md"):       "file is larger than 1 MiB",
	}
	got := map[string]string{}
	for _, problem := range problems {
		got[problem.Path] += problem.Message()
	}
	if !reflect.DeepEqual(got, want) || len(problems) != len(want) {
		t.Errorf("problems = %q\nwant each once: %q", got, want)
	}
}

func TestWalk_NonRegularFile_WrapsErrNotRegularAndItsDirectoryErrOrphanDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "x/child-node.md")
	makeSymlink(t, os.DevNull, filepath.Join(dir, "x.md"))

	_, problems, err := tree.Walk(dir)

	if err != nil || len(problems) != 2 {
		t.Fatalf("Walk = %v, %v; want the symlink and its directory", problems, err)
	}
	if !errors.Is(problems[0], tree.ErrNotRegular) || !errors.Is(problems[1], tree.ErrOrphanDirectory) {
		t.Errorf("problems = %v, want ErrNotRegular then ErrOrphanDirectory", problems)
	}
}

func TestPathFor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md", "login-flow/login-timeout-race.md")
	loaded := mustLoad(t, dir, nil)
	cases := []struct {
		id       string
		parentID string
		want     string
	}{
		{"export-csv", "", filepath.Join(dir, "export-csv.md")},
		{"session-cookie", "login-flow", filepath.Join(dir, "login-flow", "session-cookie.md")},
		{
			"retry-backoff",
			"login-timeout-race",
			filepath.Join(dir, "login-flow", "login-timeout-race", "retry-backoff.md"),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			got, err := loaded.PathFor(testCase.id, testCase.parentID)

			if err != nil || got != testCase.want {
				t.Fatalf("PathFor = %s, %v; want %s", got, err, testCase.want)
			}
		})
	}
}

// notRealDirCases are the children paths PathFor refuses. Each make puts one at childrenDir; outside
// is a directory beyond the tree that a symlink points at.
var notRealDirCases = []struct {
	name string
	make func(t *testing.T, childrenDir string, outside string)
}{
	{"symlink", func(t *testing.T, childrenDir string, outside string) {
		makeSymlink(t, outside, childrenDir)
	}},
	{"regular file", func(t *testing.T, childrenDir string, _ string) {
		writeSizedFile(t, childrenDir, 1)
	}},
}

// loadKeepingProblems loads dir as an issue tree, accepting the tree.Errors Load returns beside the
// tree, such as the one a symlink in it draws.
func loadKeepingProblems(t *testing.T, dir string) *tree.Tree {
	t.Helper()
	loaded, err := tree.Load(dir, "issue", nil)
	var problems tree.Errors
	if loaded == nil || (err != nil && !errors.As(err, &problems)) {
		t.Fatalf("Load = %v, %v; want a tree", loaded, err)
	}
	return loaded
}

func TestPathFor_ChildrenPathNotARealDirectory_RefusedNamingIt(t *testing.T) {
	t.Parallel()
	for _, testCase := range notRealDirCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir, outside := t.TempDir(), t.TempDir()
			writeNodes(t, dir, "foo.md")
			childrenDir := filepath.Join(dir, "foo")
			testCase.make(t, childrenDir, outside)
			loaded := loadKeepingProblems(t, dir)

			_, err := loaded.PathFor("bar", "foo")

			var problem *tree.Error
			if !errors.As(err, &problem) || problem.Path != childrenDir || !errors.Is(err, tree.ErrNotDirectory) {
				t.Fatalf("PathFor = %v; want a *tree.Error at %s wrapping ErrNotDirectory", err, childrenDir)
			}
		})
	}
}

func TestPathFor_Failures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md")
	loaded := mustLoad(t, dir, nil)
	cases := []struct {
		id       string
		parentID string
		want     error
	}{
		{"../escape", "", tree.ErrInvalidID},
		{"export-csv", "no-such-parent", tree.ErrNotFound},
	}
	for _, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			_, err := loaded.PathFor(testCase.id, testCase.parentID)

			if !errors.Is(err, testCase.want) {
				t.Fatalf("PathFor = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestLayout_IDOfAndChildrenDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name         string
		path         string
		wantID       string
		wantChildren string
	}{
		{
			"absolute path",
			filepath.Join(dir, "login-flow.md"),
			"login-flow",
			filepath.Join(dir, "login-flow"),
		},
		{
			"relative forward-slash path",
			".capcom/issues/login-flow.md",
			"login-flow",
			".capcom/issues/login-flow",
		},
		{
			"nested path",
			filepath.Join(dir, "login-flow", "login-timeout-race", "retry-backoff.md"),
			"retry-backoff",
			filepath.Join(dir, "login-flow", "login-timeout-race", "retry-backoff"),
		},
		{
			"nested forward-slash path",
			".capcom/issues/login-flow/login-timeout-race.md",
			"login-timeout-race",
			".capcom/issues/login-flow/login-timeout-race",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			gotID := tree.IDOf(testCase.path)
			gotChildren := tree.ChildrenDir(testCase.path)

			if gotID != testCase.wantID {
				t.Errorf("IDOf(%s) = %q, want %q", testCase.path, gotID, testCase.wantID)
			}
			if gotChildren != testCase.wantChildren {
				t.Errorf("ChildrenDir(%s) = %q, want %q", testCase.path, gotChildren, testCase.wantChildren)
			}
		})
	}
}

// parallelTreeFiles is how many node files writeParallelTree writes.
const parallelTreeFiles = 200

// overlapTimeout is how long a call of an overlapProbe waits for a second call to be in flight.
const overlapTimeout = 10 * time.Second

// writeParallelTree writes a tree of parallelTreeFiles node files, 20 parents with 9 children each,
// every file listing its aliases one per line, and returns its directory. Three files are broken:
// one lists an alias CheckAliases rejects, one its own id, and one an alias another file lists.
func writeParallelTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	broken := map[string]string{
		"group-03.md":           "Bad Alias\n",
		"group-07/item-07-4.md": "alias-item-07-4\nshared-alias\n",
		"group-15/item-15-2.md": "shared-alias\n",
		"group-11/item-11-0.md": "item-11-0\n",
	}
	for group := range 20 {
		parent := fmt.Sprintf("group-%02d.md", group)
		relatives := []string{parent}
		for item := range 9 {
			relatives = append(relatives, fmt.Sprintf("group-%02d/item-%02d-%d.md", group, group, item))
		}
		for _, relative := range relatives {
			text, isBroken := broken[relative]
			if !isBroken {
				text = "alias-" + strings.TrimSuffix(filepath.Base(relative), ".md") + "\n"
			}
			path := filepath.Join(dir, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}
	}
	return dir
}

// aliasesFromFile is an AliasFunc reading a node's aliases from its file, one per line. It is safe
// to call concurrently.
func aliasesFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"), nil
}

// loadSummary is what a load gave, in a form two loads compare by: the nodes and the left-out
// files in order, and every problem's text, which names its file, in order.
type loadSummary struct {
	nodes    []tree.Node
	leftOut  []tree.Node
	problems []string
}

// summarize returns the loadSummary of loaded and err, a load's results, failing the test when err
// is anything but the load's problems.
func summarize(t *testing.T, loaded *tree.Tree, err error) loadSummary {
	t.Helper()
	var problems tree.Errors
	if !errors.As(err, &problems) {
		t.Fatalf("load = %v, want tree.Errors", err)
	}
	summary := loadSummary{nodes: loaded.Nodes(), leftOut: loaded.LeftOut()}
	for _, problem := range problems {
		summary.problems = append(summary.problems, problem.Error())
	}
	return summary
}

// overlapProbe wraps an AliasFunc to count the calls in flight at once. Each call waits, up to
// overlapTimeout, until a second call is in flight, so a load that reads on two or more goroutines
// overlaps two calls whatever the scheduling; once the wait times out, no call waits again.
type overlapProbe struct {
	inFlight atomic.Int32
	peak     atomic.Int32
	second   chan struct{}
	release  sync.Once
	// isWaiting is false when the load reads on one goroutine, which no second call can join.
	isWaiting bool
}

// newOverlapProbe returns a probe whose calls wait for a second only when bound, the most
// goroutines the load reads on, is at least 2.
func newOverlapProbe(bound int) *overlapProbe {
	return &overlapProbe{second: make(chan struct{}), isWaiting: bound >= 2}
}

// wrap returns aliases counting its calls on the probe.
func (p *overlapProbe) wrap(aliases tree.AliasFunc) tree.AliasFunc {
	return func(path string) ([]string, error) {
		current := p.inFlight.Add(1)
		defer p.inFlight.Add(-1)
		for peak := p.peak.Load(); current > peak && !p.peak.CompareAndSwap(peak, current); {
			peak = p.peak.Load()
		}
		if current >= 2 {
			p.release.Do(func() { close(p.second) })
		}
		if p.isWaiting {
			select {
			case <-p.second:
			case <-time.After(overlapTimeout):
				// No second call came: release the rest, so a load that reads one file at a time
				// fails on its peak instead of waiting out every call.
				p.release.Do(func() { close(p.second) })
			}
		}
		return aliases(path)
	}
}

func TestLoadParallel_BrokenFiles_MatchesLoadEveryRun(t *testing.T) {
	t.Parallel()
	dir := writeParallelTree(t)
	sequential, err := tree.Load(dir, "issue", aliasesFromFile)
	want := summarize(t, sequential, err)
	if len(want.problems) != 3 {
		t.Fatalf("Load problems = %q, want the 3 of the broken files", want.problems)
	}

	for run := range 20 {
		loaded, err := tree.LoadParallel(dir, "issue", aliasesFromFile)
		if got := summarize(t, loaded, err); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: LoadParallel = %+v, want Load's %+v", run, got, want)
		}
	}
}

func TestLoadParallel_ReadsOnAtMostNumCPUGoroutines(t *testing.T) {
	t.Parallel()
	dir := writeParallelTree(t)
	bound := min(tree.ReadWorkers(), parallelTreeFiles)
	probe := newOverlapProbe(bound)

	loaded, err := tree.LoadParallel(dir, "issue", probe.wrap(aliasesFromFile))

	summarize(t, loaded, err)
	peak := int(probe.peak.Load())
	if peak > runtime.NumCPU() {
		t.Errorf("peak concurrent calls = %d, want at most NumCPU %d", peak, runtime.NumCPU())
	}
	if bound >= 2 && peak < 2 {
		t.Errorf("peak concurrent calls = %d, want at least 2 with a bound of %d", peak, bound)
	}
}
