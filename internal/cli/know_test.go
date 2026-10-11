package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeNoteFiles writes Note files, named by slash paths below .capcom/knowledge/, creating their
// directories.
func writeNoteFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, ".capcom", "knowledge", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
}

// newKnowStore creates a store holding a three-level Knowledge tree and one issue sharing a
// Note's id.
func newKnowStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"release-flow.md": readIssue("Release flow issue", "open", "", "\nShip the tag.\n"),
	})
	writeNoteFiles(t, root, map[string]string{
		"testing-rules.md":                       "---\ntitle: Testing rules\nlabels: [invariant]\n---\n\nTests use real temp dirs.\n",
		"testing-rules/cli-tests.md":             "---\ntitle: CLI tests\n---\n\nThrough runCLI.\n",
		"testing-rules/cli-tests/no-parallel.md": "---\ntitle: No parallel\nrefs: [go.mod]\n---\n\nThe harness changes directory.\n",
		"testing-rules/git-fixtures.md":          "---\ntitle: Git fixtures\n---\n",
		"release-flow.md":                        "---\ntitle: Release flow\nlabels: [process]\n---\n\nTag, then publish.\n",
	})
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return root
}

func TestKnowTree_DepthAndStart_LimitTheOutline(t *testing.T) {
	root := newKnowStore(t)
	cases := []struct {
		args []string
		want string
	}{
		{nil, "release-flow  Release flow\ntesting-rules  Testing rules\n  cli-tests  CLI tests\n" +
			"    no-parallel  No parallel\n  git-fixtures  Git fixtures\n"},
		{[]string{"--depth", "1"}, "release-flow  Release flow\ntesting-rules  Testing rules\n"},
		{[]string{"testing", "--depth", "1"}, "testing-rules  Testing rules\n  cli-tests  CLI tests\n  git-fixtures  Git fixtures\n"},
		{[]string{"cli-tests"}, "cli-tests  CLI tests\n  no-parallel  No parallel\n"},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"know", "tree"}, current.args...)...)

		if exitCode != ExitOK || stdout != current.want {
			t.Errorf("know tree %v = %d\n%s%s\nwant\n%s", current.args, exitCode, stdout, stderr, current.want)
		}
	}
}

func TestKnowTree_NegativeDepthOrUnknownNote_Fails(t *testing.T) {
	root := newKnowStore(t)

	_, _, depthExit := runCLI(t, root, "know", "tree", "--depth", "-1")
	_, _, unknownExit := runCLI(t, root, "know", "tree", "absent-note")

	if depthExit != ExitUsage || unknownExit != ExitNotFound {
		t.Errorf("exits = %d, %d; want usage then not_found", depthExit, unknownExit)
	}
}

func TestKnowShow_WithChildren_ShowsTheNoteThenItsDirectChildren(t *testing.T) {
	root := newKnowStore(t)

	stdout, _, exitCode := runCLI(t, root, "know", "show", "testing-rules", "--with-children", "--json")

	if exitCode != ExitOK {
		t.Fatalf("know show exit = %d\n%s", exitCode, stdout)
	}
	if got := strings.Join(envelopeIDs(t, stdout), ","); got != "testing-rules,cli-tests,git-fixtures" {
		t.Errorf("shown ids = %s", got)
	}
}

func TestKnowShow_Text_PrintsFrontmatterThenBody(t *testing.T) {
	root := newKnowStore(t)

	stdout, _, exitCode := runCLI(t, root, "know", "show", "no-parallel")

	want := "id: no-parallel\npath: .capcom/knowledge/testing-rules/cli-tests/no-parallel.md\nparent: cli-tests\n" +
		"title: No parallel\nrefs: [go.mod]\n\nThe harness changes directory.\n"
	if exitCode != ExitOK || stdout != want {
		t.Errorf("know show = %d\n%s\nwant\n%s", exitCode, stdout, want)
	}
}

func TestKnowAdd_Nested_WritesTheNoteBelowItsParent(t *testing.T) {
	root := newKnowStore(t)
	bodyFile := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(bodyFile, []byte("Seed the fixture with git init."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "seed-repos", "--title", "Seed repos",
		"--parent", "git-fixtures", "--label", "cli,cli", "--ref", "go.mod", "--ref", "go.mod#module", "--body-file", bodyFile)

	if exitCode != ExitOK {
		t.Fatalf("know add exit = %d\n%s%s", exitCode, stdout, stderr)
	}
	name := "testing-rules/git-fixtures/seed-repos.md"
	data, err := os.ReadFile(filepath.Join(root, ".capcom", "knowledge", filepath.FromSlash(name)))
	want := "---\ntitle: Seed repos\nlabels: [cli]\nrefs: [go.mod, go.mod#module]\n---\n\nSeed the fixture with git init.\n"
	if err != nil || string(data) != want {
		t.Errorf("note file = %q, %v; want %q", data, err, want)
	}
	if !strings.Contains(stdout, "path: .capcom/knowledge/"+name) {
		t.Errorf("stdout = %s", stdout)
	}
	if tree := mustRun(t, root, "know", "tree", "git-fixtures"); tree != "git-fixtures  Git fixtures\n  seed-repos  Seed repos\n" {
		t.Errorf("know tree after add = %s", tree)
	}
}

func TestKnowAdd_RefusedInputs_FailWithoutWriting(t *testing.T) {
	root := newKnowStore(t)
	cases := []struct {
		args     []string
		exitCode int
	}{
		{[]string{"--id", "cli-tests", "--title", "Again"}, ExitError},
		{[]string{"--id", "release-flow", "--title", "Clashes with the note, not the issue"}, ExitError},
		{[]string{"--id", "escaping-ref", "--title", "Out", "--ref", "../outside.go"}, ExitError},
		{[]string{"--id", "bad", "--title", "Short id"}, ExitError},
		{[]string{"--id", "orphan-note", "--title", "Orphan", "--parent", "absent-note"}, ExitNotFound},
	}

	for _, current := range cases {
		stdout, _, exitCode := runCLI(t, root, append([]string{"know", "add", "--json"}, current.args...)...)

		if exitCode != current.exitCode {
			t.Errorf("know add %v exit = %d, want %d\n%s", current.args, exitCode, current.exitCode, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "knowledge", "escaping-ref.md")); !os.IsNotExist(err) {
		t.Errorf("escaping-ref.md was written: %v", err)
	}
}

func TestKnowVerify_BrokenRefs_ListsEachWithItsLineAndFails(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"stale-refs.md": "---\ntitle: Stale\nrefs:\n  - go.mod\n  - gone.go\n  - go.mod#absentSymbol\n  - ../escape.go\n---\n",
	})

	stdout, stderr, exitCode := runCLI(t, root, "know", "verify")

	want := ".capcom/knowledge/stale-refs.md:5: ref \"gone.go\": no such file or directory\n" +
		".capcom/knowledge/stale-refs.md:6: ref \"go.mod#absentSymbol\": symbol \"absentSymbol\" is not in the file\n" +
		".capcom/knowledge/stale-refs.md:7: ref \"../escape.go\": escapes the working tree\n"
	if exitCode != ExitError || stdout != want || !strings.Contains(stderr, "3 note refs do not resolve") {
		t.Errorf("know verify = %d\n%s%s\nwant\n%s", exitCode, stdout, stderr, want)
	}
}

func TestKnowVerify_RefBelowAFile_IsListedAsBroken(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"below-a-file.md": "---\ntitle: Below a file\nrefs:\n  - go.mod/sub\n  - gone.go\n---\n",
	})

	stdout, stderr, exitCode := runCLI(t, root, "know", "verify")

	want := ".capcom/knowledge/below-a-file.md:4: ref \"go.mod/sub\": no such file or directory\n" +
		".capcom/knowledge/below-a-file.md:5: ref \"gone.go\": no such file or directory\n"
	if exitCode != ExitError || stdout != want || !strings.Contains(stderr, "2 note refs do not resolve") {
		t.Errorf("know verify = %d\n%s%s\nwant\n%s", exitCode, stdout, stderr, want)
	}
}

func TestKnowVerify_EveryRefResolves_PrintsNothing(t *testing.T) {
	root := newKnowStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "know", "verify", "--json")

	data, _ := decodeEnvelope(t, stdout)["data"].([]any)
	if exitCode != ExitOK || len(data) != 0 {
		t.Errorf("know verify = %d, data %v\n%s", exitCode, data, stderr)
	}
}

func TestKnowSearch_WordsAndLabel_FindNotesOnly(t *testing.T) {
	root := newKnowStore(t)

	byWord := mustRun(t, root, "know", "search", "harness")
	byLabel, _, labelExit := runCLI(t, root, "know", "search", "tag", "--label", "invariant")
	issueOnly, _, issueExit := runCLI(t, root, "know", "search", "ship")

	if byWord != "no-parallel  No parallel\n" {
		t.Errorf("know search harness = %q", byWord)
	}
	if labelExit != ExitNotFound || issueExit != ExitNotFound {
		t.Errorf("exits = %d, %d; want not_found for a label miss and an issue-only word\n%s%s",
			labelExit, issueExit, byLabel, issueOnly)
	}
}

func TestKnow_IssueAndNoteShareAnID_EachCommandFindsItsOwn(t *testing.T) {
	root := newKnowStore(t)

	issueText := mustRun(t, root, "show", "release-flow")
	noteText := mustRun(t, root, "know", "show", "release-flow")

	if !strings.Contains(issueText, "title: Release flow issue") || !strings.Contains(noteText, "title: Release flow\n") {
		t.Errorf("issue show =\n%s\nknow show =\n%s", issueText, noteText)
	}
}

func TestKnow_IssueSearchForAWordOnlyInANote_IsNotFound(t *testing.T) {
	root := newKnowStore(t)

	stdout, _, exitCode := runCLI(t, root, "search", "harness", "--json")

	if exitCode != ExitNotFound || !strings.Contains(stdout, `"code":"not_found"`) {
		t.Errorf("search harness = %d\n%s", exitCode, stdout)
	}
}

func TestKnowAdd_TopLevel_RefreshesAnExistingAgentsBlock(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "agents")

	nested := mustRun(t, root, "know", "add", "--id", "seed-repos", "--title", "Seed repos", "--parent", "git-fixtures")
	topLevel := mustRun(t, root, "know", "add", "--id", "api-design", "--title", "API design")

	if strings.Contains(nested, "refreshed") || !strings.HasSuffix(topLevel, "refreshed: AGENTS.md\n") {
		t.Errorf("know add output = %q then %q; want only the top-level add to refresh", nested, topLevel)
	}
	if !strings.Contains(readAgentsGuide(t, root), "- `api-design`: API design\n") {
		t.Errorf("AGENTS.md =\n%s\nwant the new top-level Note listed", readAgentsGuide(t, root))
	}
	if stdout, _, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after know add = %d\n%s", exitCode, stdout)
	}
}

func TestKnowAdd_TopLevelWithoutAgentsBlock_LeavesAgentsAlone(t *testing.T) {
	root := newKnowStore(t)
	writeAgentsGuide(t, root, "# Guide\n")

	stdout := mustRun(t, root, "know", "add", "--id", "api-design", "--title", "API design")

	if strings.Contains(stdout, "refreshed") || readAgentsGuide(t, root) != "# Guide\n" {
		t.Errorf("know add = %q, AGENTS.md = %q; want AGENTS.md untouched", stdout, readAgentsGuide(t, root))
	}
}
