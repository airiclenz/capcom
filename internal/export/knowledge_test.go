package export_test

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// alphaBody is the body of the Note alpha: markup that must print escaped.
const alphaBody = "\nUse <i>care</i> here.\n"

// knowledgeFixture indexes a tree in a temp dir holding the Issues base-task and epic-task, with
// its child child-task, and these Notes: alpha (labels rules, refs cmd/a.go and docs/b.md, body
// alphaBody) with its child alpha-child and grandchild alpha-grandchild; beta; old (retired) with
// its child old-child; and quiet (labelled background) with its child quiet-child. It returns
// the store and its index.
func knowledgeFixture(t *testing.T) (*store.Store, *index.Index) {
	t.Helper()
	root := t.TempDir()
	st := &store.Store{
		Root:         filepath.Join(root, ".capcom"),
		WorktreeRoot: root,
		GitDir:       filepath.Join(root, ".git"),
		GitCommonDir: filepath.Join(root, ".git"),
		Config:       config.Default(),
	}
	issues := map[string]string{
		"base-task.md":            issueFile("open", 1, "", ""),
		"epic-task.md":            issueFile("open", 2, "", ""),
		"epic-task/child-task.md": issueFile("open", 1, "", ""),
	}
	notes := map[string]string{
		"alpha.md":                              "---\ntitle: Alpha rules\nlabels: [rules]\nrefs: [cmd/a.go, docs/b.md]\n---\n" + alphaBody,
		"alpha/alpha-child.md":                  "---\ntitle: Alpha child\n---\n\nChild.\n",
		"alpha/alpha-child/alpha-grandchild.md": "---\ntitle: Alpha grandchild\n---\n\nGrandchild.\n",
		"beta.md":                               "---\ntitle: Beta\n---\n\nBeta.\n",
		"old.md":                                "---\ntitle: Old\nretired: true\n---\n\nOld.\n",
		"old/old-child.md":                      "---\ntitle: Old child\n---\n\nOld child.\n",
		"quiet.md":                              "---\ntitle: Quiet\nlabels: [background]\n---\n\nQuiet.\n",
		"quiet/quiet-child.md":                  "---\ntitle: Quiet child\n---\n\nQuiet child.\n",
	}
	writeFiles(t, st.IssuesDir(), issues)
	writeFiles(t, st.KnowledgeDir(), notes)

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

// writeFiles writes every file of files, keyed by its slash path below dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
}

func TestWriteHTML_NotePageSet(t *testing.T) {
	st, ix := knowledgeFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	var names []string
	for name := range readTree(t, dir) {
		if strings.HasPrefix(name, "knowledge") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	want := []string{
		"knowledge.html",
		"knowledge/alpha-child.html",
		"knowledge/alpha-grandchild.html",
		"knowledge/alpha.html",
		"knowledge/beta.html",
	}
	if !slices.Equal(names, want) {
		t.Errorf("Knowledge files = %v, want %v", names, want)
	}
}

func TestWriteHTML_LeavesOutRetiredAndBackgroundNotes(t *testing.T) {
	st, ix := knowledgeFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	tree := readPage(t, dir, "knowledge.html")
	for _, absent := range []string{"old", "old-child", "quiet", "quiet-child"} {
		if strings.Contains(tree, `href="knowledge/`+absent+`.html"`) {
			t.Errorf("knowledge.html lists %s, which the default tree leaves out:\n%s", absent, tree)
		}
		if _, err := os.Stat(filepath.Join(dir, "knowledge", absent+".html")); err == nil {
			t.Errorf("knowledge/%s.html is written, but the default tree leaves %s out", absent, absent)
		}
	}
}

func TestWriteHTML_KnowledgeTreeNestsNotes(t *testing.T) {
	st, ix := knowledgeFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	tree := readPage(t, dir, "knowledge.html")
	nested := `<li><a href="knowledge/alpha.html">alpha</a> Alpha rules
<ul class="tree">
<li><a href="knowledge/alpha-child.html">alpha-child</a> Alpha child
<ul class="tree">
<li><a href="knowledge/alpha-grandchild.html">alpha-grandchild</a> Alpha grandchild
</li>
</ul>
</li>
</ul>
</li>`
	if !strings.Contains(tree, nested) {
		t.Errorf("knowledge.html does not nest alpha-grandchild under alpha-child under alpha:\n%s", tree)
	}
	alpha := strings.Index(tree, `href="knowledge/alpha.html"`)
	beta := strings.Index(tree, `href="knowledge/beta.html"`)
	if alpha == -1 || beta == -1 || alpha > beta {
		t.Errorf("knowledge.html rows at %d, %d, want alpha before beta:\n%s", alpha, beta, tree)
	}
}

func TestWriteHTML_NotePageContent(t *testing.T) {
	st, ix := knowledgeFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	page := readPage(t, dir, "knowledge/alpha.html")
	for _, want := range []string{
		"<h1>Alpha rules</h1>",
		"<dt>Id</dt><dd>alpha</dd>",
		"<dt>Labels</dt><dd>rules</dd>",
		"<dt>Refs</dt><dd><code>cmd/a.go</code>, <code>docs/b.md</code></dd>",
		`<pre class="body">Use &lt;i&gt;care&lt;/i&gt; here.</pre>`,
		`<a href="../index.html">Ready</a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("alpha's page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "<i>") || strings.Contains(strings.ToLower(page), "<script") {
		t.Errorf("alpha's page holds unescaped markup or a script:\n%s", page)
	}
}

func TestWriteHTML_IndexLinksKnowledge(t *testing.T) {
	st, ix := knowledgeFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	content := readPage(t, dir, "index.html")
	if !strings.Contains(content, `<a href="knowledge.html">Knowledge tree</a>`) {
		t.Errorf("index.html does not link knowledge.html:\n%s", content)
	}
}

func TestWriteHTML_EveryPageLinksHomeAndResolves(t *testing.T) {
	st, ix := knowledgeFixture(t)
	dir := t.TempDir()

	writeExport(t, dir, st, ix)

	pages := 0
	for name, content := range readTree(t, dir) {
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		pages++
		isHomeLinked := false
		for _, match := range hrefPattern.FindAllStringSubmatch(string(content), -1) {
			resolved := path.Join(path.Dir(name), match[1])
			if resolved == "index.html" {
				isHomeLinked = true
			}
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(resolved))); err != nil {
				t.Errorf("%s links %q, which does not resolve: %v", name, match[1], err)
			}
		}
		if !isHomeLinked {
			t.Errorf("%s does not link back to index.html", name)
		}
	}
	// index.html, issues.html, three Issue pages, knowledge.html and four Note pages.
	if pages != 10 {
		t.Errorf("checked %d pages, want 10", pages)
	}
}

func TestWriteHTML_KnowledgeDeterministic(t *testing.T) {
	st, ix := knowledgeFixture(t)
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
