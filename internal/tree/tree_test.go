package tree_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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

// aliasesByFile returns an AliasFunc that looks a node's aliases up by its file name.
func aliasesByFile(aliases map[string][]string) tree.AliasFunc {
	return func(path string) ([]string, error) {
		return aliases[filepath.Base(path)], nil
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
	loaded, err := tree.Load(dir, aliases)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded
}

func TestValidateID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id      string
		isValid bool
	}{
		{"login-flow", true},
		{"login-timeout-race", true},
		{"fix-login-timeout-race", true},
		{"v2-api", true},
		{"2026-plan", true},
		{strings.Repeat("a", 19) + "-" + strings.Repeat("b", 20), true},
		{strings.Repeat("a", 20) + "-" + strings.Repeat("b", 20), false},
		{"login", false},
		{"fix-the-login-timeout-race", false},
		{"Login-flow", false},
		{"login--flow", false},
		{"-login-flow", false},
		{"login-flow-", false},
		{"login_flow", false},
		{"apogee-kk0.4", false},
		{"", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			err := tree.ValidateID(testCase.id)

			if testCase.isValid && err != nil {
				t.Fatalf("ValidateID(%q) = %v, want nil", testCase.id, err)
			}
			if !testCase.isValid && !errors.Is(err, tree.ErrInvalidID) {
				t.Fatalf("ValidateID(%q) = %v, want ErrInvalidID", testCase.id, err)
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

	_, err := tree.Load(dir, nil)

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
			"must be 2 to 4 words of a-z and 0-9 joined by single hyphens",
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

func TestLoad_InvalidID_FailsNamingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md", "login-flow/Bad_Name.md")

	_, err := tree.Load(dir, nil)

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

	_, err := tree.Load(dir, nil)

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

			_, err := tree.Load(dir, aliasesByFile(testCase.aliases))

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

	_, err := tree.Load(dir, func(string) ([]string, error) { return nil, failure })

	if err != failure {
		t.Fatalf("Load = %v, want %v", err, failure)
	}
}

func TestLoad_DuplicateAlias_NamesTheOtherFileApart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "export-csv.md", "login-flow.md")

	_, err := tree.Load(dir, aliasesByFile(map[string][]string{
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

	_, problems := tree.CheckAliases("login-flow.md", "note", "login-flow", []string{"login-flow"})

	if len(problems) != 1 || !errors.Is(problems[0], tree.ErrDuplicate) {
		t.Fatalf("problems = %v, want one wrapping ErrDuplicate", problems)
	}
	if want := `alias "login-flow" repeats the note's own id`; problems[0].Message() != want {
		t.Errorf("Message = %q, want %q", problems[0].Message(), want)
	}
}

func TestLoad_RepeatedAliases_DropOnlyTheOffendingAlias(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md")

	loaded, err := tree.Load(dir, aliasesByFile(map[string][]string{
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
		`alias "login-flow" repeats the node's own id`,
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

	loaded, err := tree.Load(dir, aliasesByFile(map[string][]string{"export-csv.md": {"Bad Alias"}}))

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
	if got := ids(loaded.Nodes()); !reflect.DeepEqual(got, []string{"export-csv", "login-flow"}) {
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

// failingReader returns its content, then fails, so a write breaks after the temp file opens.
type failingReader struct {
	content io.Reader
}

var errReaderFailed = errors.New("reader failed")

func (r *failingReader) Read(p []byte) (int, error) {
	n, err := r.content.Read(p)
	if err == io.EOF {
		return n, errReaderFailed
	}
	return n, err
}

// dirEntries returns the names in dir.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestWriteFileAtomic_Success_ReplacesFileAndLeavesNoTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "login-flow.md")
	writeNodes(t, dir, "login-flow.md")

	err := tree.WriteFileAtomic(path, strings.NewReader("new\ncontent\n"))

	if err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new\ncontent\n" {
		t.Fatalf("content = %q", got)
	}
	if names := dirEntries(t, dir); !reflect.DeepEqual(names, []string{"login-flow.md"}) {
		t.Fatalf("dir holds %v, want only login-flow.md", names)
	}
}

func TestWriteFileAtomic_MissingDirectory_IsCreated(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "login-flow", "login-timeout-race.md")

	err := tree.WriteFileAtomic(path, strings.NewReader("x\n"))

	if err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "x\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestWriteFileAtomic_CRLF_WrittenAsLF(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "login-flow.md")
	content := io.MultiReader(
		strings.NewReader("a\r\nb\r"),
		strings.NewReader("\nlone\rcr\r"),
	)

	err := tree.WriteFileAtomic(path, content)

	if err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "a\nb\nlone\rcr\r" {
		t.Fatalf("content = %q", got)
	}
}

func TestWriteFileAtomic_ReaderFails_OldFileIntactNoTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "login-flow.md")
	writeNodes(t, dir, "login-flow.md")
	reader := &failingReader{content: strings.NewReader(strings.Repeat("partial\n", 10000))}

	err := tree.WriteFileAtomic(path, reader)

	if !errors.Is(err, errReaderFailed) || !strings.Contains(err.Error(), path) {
		t.Fatalf("WriteFileAtomic = %v, want the reader failure naming %s", err, path)
	}
	if got, _ := os.ReadFile(path); string(got) != "x\n" {
		t.Fatalf("old file changed to %q", got)
	}
	if names := dirEntries(t, dir); !reflect.DeepEqual(names, []string{"login-flow.md"}) {
		t.Fatalf("dir holds %v, want only login-flow.md", names)
	}
}
