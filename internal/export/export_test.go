package export_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/export"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// markerName is the marker file every export directory holds.
const markerName = ".capcom-export"

// depBody is the body of dep-task: markup that must print escaped, and a Log.
const depBody = "\nNeeds <b>bold</b> handling.\n\n## Acceptance\n\n- It works.\n\n## Log\n\n- 2026-10-12: started\n"

// hrefPattern finds the target of every link a page writes.
var hrefPattern = regexp.MustCompile(`href="([^"]*)"`)

// issueFile is the content of a valid issue file with the given status, priority, extra
// frontmatter lines and body.
func issueFile(status string, priority int, extra string, body string) string {
	closing := ""
	if status == "done" {
		closing = "closed: 2026-10-12T09:00:00Z\nreason: finished\n"
	}
	return fmt.Sprintf(
		"---\ntitle: An issue\nstatus: %s\ntype: task\npriority: %d\n%screated: 2026-10-11T09:00:00Z\n%s---\n%s",
		status, priority, extra, closing, body,
	)
}

// exportFixture indexes a tree in a temp dir and returns its store and index. In tree order:
// base-task (Ready, priority 1), closed-task (done), dep-task (deps on base-task, on old-task, an
// alias of renamed-task, and on missing-task, which names no issue; its body is depBody),
// epic-task (open parent) with its child child-task (Ready, priority 1, and unblocks epic-task, so
// it lists before base-task), and renamed-task (Ready, priority 3, alias old-task).
func exportFixture(t *testing.T) (*store.Store, *index.Index) {
	t.Helper()
	root := t.TempDir()
	st := &store.Store{
		Root:         filepath.Join(root, ".capcom"),
		WorktreeRoot: root,
		GitDir:       filepath.Join(root, ".git"),
		GitCommonDir: filepath.Join(root, ".git"),
		Config:       config.Default(),
	}
	files := map[string]string{
		"base-task.md":            issueFile("open", 1, "", ""),
		"closed-task.md":          issueFile("done", 2, "", ""),
		"dep-task.md":             issueFile("open", 2, "deps: [base-task, old-task, missing-task]\n", depBody),
		"epic-task.md":            issueFile("open", 2, "", ""),
		"epic-task/child-task.md": issueFile("open", 1, "", ""),
		"renamed-task.md":         issueFile("open", 3, "aliases: [old-task]\n", ""),
	}
	for name, content := range files {
		path := filepath.Join(st.IssuesDir(), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	opened, err := index.Open(st, index.ModeRefresh)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := opened.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return st, opened
}

// writeExport exports the fixture into dir and fails the test when the export fails.
func writeExport(t *testing.T, dir string, st *store.Store, ix *index.Index) {
	t.Helper()
	if _, err := export.WriteHTML(dir, st, ix); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
}

// readTree returns every file below dir by its path relative to dir, with forward slashes.
func readTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(walked string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(walked)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, walked)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	return files
}

// readPage returns the page at the slash path below dir.
func readPage(t *testing.T, dir string, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

func TestWriteHTML_PageSet(t *testing.T) {
	st, ix := exportFixture(t)
	dir := filepath.Join(t.TempDir(), "site")

	writeExport(t, dir, st, ix)

	var names []string
	for name := range readTree(t, dir) {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{
		markerName,
		"index.html",
		"issues.html",
		"issues/base-task.html",
		"issues/child-task.html",
		"issues/closed-task.html",
		"issues/dep-task.html",
		"issues/epic-task.html",
		"issues/renamed-task.html",
		"knowledge.html",
	}
	if !slices.Equal(names, want) {
		t.Errorf("files = %v, want %v", names, want)
	}
}

func TestWriteHTML_ReturnsPageCount(t *testing.T) {
	st, ix := exportFixture(t)
	dir := filepath.Join(t.TempDir(), "site")

	pages, err := export.WriteHTML(dir, st, ix)

	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	if want := len(readTree(t, dir)) - 1; pages != want {
		t.Errorf("WriteHTML = %d pages, want %d: every file but the marker", pages, want)
	}
}

func TestWriteHTML_ReadyList(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	content := readPage(t, dir, "index.html")
	positions := []int{
		strings.Index(content, `href="issues/child-task.html"`),
		strings.Index(content, `href="issues/base-task.html"`),
		strings.Index(content, `href="issues/renamed-task.html"`),
	}
	if slices.Contains(positions, -1) || !slices.IsSorted(positions) {
		t.Errorf("Ready rows at %v, want child-task, base-task, renamed-task in that order:\n%s", positions, content)
	}
	for _, absent := range []string{"closed-task", "epic-task", "dep-task"} {
		if strings.Contains(content, absent) {
			t.Errorf("index.html lists %s, which is not Ready:\n%s", absent, content)
		}
	}
	if !strings.Contains(content, `href="issues.html"`) {
		t.Errorf("index.html does not link issues.html:\n%s", content)
	}
}

func TestWriteHTML_ClosedIssue(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	tree := readPage(t, dir, "issues.html")
	if !strings.Contains(tree, `<a href="issues/closed-task.html">closed-task</a> An issue <span class="meta">done · task</span>`) {
		t.Errorf("issues.html has no row for closed-task with its status:\n%s", tree)
	}
	closedPage := readPage(t, dir, "issues/closed-task.html")
	if !strings.Contains(closedPage, `<dt>Status</dt><dd class="closed">done</dd>`) {
		t.Errorf("closed-task's page does not show its status:\n%s", closedPage)
	}
}

func TestWriteHTML_TreeNestsChildren(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	tree := readPage(t, dir, "issues.html")
	nested := regexp.MustCompile(`(?s)<a href="issues/epic-task.html">epic-task</a>[^<]*<span[^>]*>[^<]*</span>\s*<ul class="tree">\s*<li><a href="issues/child-task.html">`)
	if !nested.MatchString(tree) {
		t.Errorf("issues.html does not nest child-task under epic-task:\n%s", tree)
	}
}

func TestWriteHTML_DepLinks(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	depPage := readPage(t, dir, "issues/dep-task.html")
	wants := []string{
		`<a href="base-task.html">base-task</a>`,
		`<a href="renamed-task.html">renamed-task (as old-task)</a>`,
		`, missing-task (no such issue)</dd>`,
	}
	for _, want := range wants {
		if !strings.Contains(depPage, want) {
			t.Errorf("dep-task's page lacks %q:\n%s", want, depPage)
		}
	}
	basePage := readPage(t, dir, "issues/base-task.html")
	if !strings.Contains(basePage, `<dt>Dependents</dt><dd><a href="dep-task.html">dep-task</a></dd>`) {
		t.Errorf("base-task's page does not link its dependent dep-task:\n%s", basePage)
	}
}

func TestWriteHTML_EveryLinkResolves(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	for name, content := range readTree(t, dir) {
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		for _, match := range hrefPattern.FindAllStringSubmatch(string(content), -1) {
			target := filepath.Join(dir, filepath.FromSlash(path.Join(path.Dir(name), match[1])))
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s links %q, which does not resolve: %v", name, match[1], err)
			}
		}
	}
}

func TestWriteHTML_Deterministic(t *testing.T) {
	st, ix := exportFixture(t)
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")

	writeExport(t, first, st, ix)
	before := readTree(t, first)
	writeExport(t, first, st, ix)
	writeExport(t, second, st, ix)

	again := readTree(t, first)
	elsewhere := readTree(t, second)
	for name, content := range before {
		if !bytes.Equal(again[name], content) {
			t.Errorf("%s differs on a second export into the same directory", name)
		}
		if !bytes.Equal(elsewhere[name], content) {
			t.Errorf("%s differs on an export into another directory", name)
		}
	}
	if len(again) != len(before) || len(elsewhere) != len(before) {
		t.Errorf("file counts = %d, %d, %d; want equal", len(before), len(again), len(elsewhere))
	}
}

func TestWriteHTML_RefusesStoreDirs(t *testing.T) {
	st, ix := exportFixture(t)
	outside := t.TempDir()
	link := filepath.Join(outside, "into-store")
	isLinked := os.Symlink(st.Root, link) == nil
	cases := []struct {
		name     string
		dir      string
		isLinked bool
		want     error
	}{
		{name: "the store directory", dir: st.Root, want: export.ErrInsideStore},
		{name: "a new directory inside the store", dir: filepath.Join(st.Root, "site"), want: export.ErrInsideStore},
		{name: "a directory reached through a symlink into the store", dir: filepath.Join(link, "site"), isLinked: true, want: export.ErrInsideStore},
		{name: "the working tree that holds the store", dir: st.WorktreeRoot, want: export.ErrHoldsStore},
	}
	if err := os.WriteFile(filepath.Join(st.WorktreeRoot, markerName), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.isLinked && !isLinked {
				t.Skipf("symlinks unavailable on %s", runtime.GOOS)
			}

			_, err := export.WriteHTML(tc.dir, st, ix)

			if !errors.Is(err, tc.want) {
				t.Fatalf("WriteHTML(%s) = %v, want %v", tc.dir, err, tc.want)
			}
			if _, err := os.Stat(filepath.Join(st.IssuesDir(), "base-task.md")); err != nil {
				t.Errorf("the store lost an issue file: %v", err)
			}
			if _, err := os.Stat(filepath.Join(st.Root, "site")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("the refused export wrote into the store: %v", err)
			}
		})
	}
}

func TestWriteHTML_RefusesForeignDir(t *testing.T) {
	st, ix := exportFixture(t)
	// The error names the resolved directory: on Windows resolving expands an 8.3 short name in
	// the temp path, so the check below compares against the resolved form.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	foreign := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(foreign, []byte("mine"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = export.WriteHTML(dir, st, ix)

	if !errors.Is(err, export.ErrForeignDir) {
		t.Fatalf("WriteHTML = %v, want ErrForeignDir", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error %q does not name %s", err, dir)
	}
	names := readTree(t, dir)
	if len(names) != 1 || string(names["notes.txt"]) != "mine" {
		t.Errorf("the refused export changed the directory: %v", names)
	}
}

func TestWriteHTML_ReplacesMarkedDir(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()
	writeExport(t, dir, st, ix)
	stale := []string{"issues/gone-task.html", "stale.txt"}
	for _, name := range stale {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte("old"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	writeExport(t, dir, st, ix)

	files := readTree(t, dir)
	for _, name := range stale {
		if _, isPresent := files[name]; isPresent {
			t.Errorf("%s survived the export", name)
		}
	}
	if _, isPresent := files["issues/base-task.html"]; !isPresent {
		t.Errorf("the export did not write issues/base-task.html: %v", files)
	}
}

func TestWriteHTML_AcceptsPartialExport(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()
	partial := map[string]string{markerName: "", "index.html": "half"}
	for name, content := range partial {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	writeExport(t, dir, st, ix)

	files := readTree(t, dir)
	if len(files) != 10 || string(files["index.html"]) == "half" {
		t.Errorf("the export did not complete the partial page set: %d files", len(files))
	}
}

func TestWriteHTML_PageContent(t *testing.T) {
	st, ix := exportFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	files := readTree(t, dir)
	for name, content := range files {
		if bytes.Contains(bytes.ToLower(content), []byte("<script")) {
			t.Errorf("%s holds a script", name)
		}
	}
	depPage := string(files["issues/dep-task.html"])
	if strings.Contains(depPage, "<b>") || !strings.Contains(depPage, "Needs &lt;b&gt;bold&lt;/b&gt; handling.") {
		t.Errorf("dep-task's body is not escaped:\n%s", depPage)
	}
	if count := strings.Count(depPage, "## Log"); count != 1 {
		t.Errorf("dep-task's page prints the Log %d times, want 1:\n%s", count, depPage)
	}
}
