package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/edit"
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

// noteFileETag returns the etag of the Note file at a slash path below .capcom/knowledge/.
func noteFileETag(t *testing.T, root string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".capcom", "knowledge", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return edit.ETag(data)
}

// textETag returns the value of the etag: line of a text output, empty when it has none.
func textETag(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if value, isEtag := strings.CutPrefix(line, "etag: "); isEtag {
			return value
		}
	}
	return ""
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
		{[]string{"--depth", "1"}, "release-flow  Release flow\ntesting-rules  Testing rules\n  ... 2 more (--depth 2)\n"},
		{[]string{"testing", "--depth", "1"}, "testing-rules  Testing rules\n  cli-tests  CLI tests\n" +
			"    ... 1 more (--depth 2)\n  git-fixtures  Git fixtures\n"},
		{[]string{"--depth", "2"}, "release-flow  Release flow\ntesting-rules  Testing rules\n  cli-tests  CLI tests\n" +
			"    ... 1 more (--depth 3)\n  git-fixtures  Git fixtures\n"},
		{[]string{"--depth", "3"}, "release-flow  Release flow\ntesting-rules  Testing rules\n  cli-tests  CLI tests\n" +
			"    no-parallel  No parallel\n  git-fixtures  Git fixtures\n"},
		{[]string{"cli-tests"}, "cli-tests  CLI tests\n  no-parallel  No parallel\n"},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"know", "tree"}, current.args...)...)

		if exitCode != ExitOK || stdout != current.want {
			t.Errorf("know tree %v = %d\n%s%s\nwant\n%s", current.args, exitCode, stdout, stderr, current.want)
		}
	}
}

func TestKnowTree_DepthCutJSON_CarriesMoreOnTheCutNote(t *testing.T) {
	root := newKnowStore(t)

	data, _ := decodeEnvelope(t, mustRun(t, root, "know", "tree", "--depth", "1", "--json"))["data"].([]any)

	more := map[string]any{}
	for _, item := range data {
		line, _ := item.(map[string]any)
		more[line["id"].(string)] = line["more"]
	}
	if more["testing-rules"] != float64(2) || more["release-flow"] != nil {
		t.Errorf("know tree --depth 1 --json more = %v; want testing-rules 2, release-flow absent", more)
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

	want := "id: no-parallel\npath: .capcom/knowledge/testing-rules/cli-tests/no-parallel.md\n" +
		"etag: " + noteFileETag(t, root, "testing-rules/cli-tests/no-parallel.md") + "\nparent: cli-tests\n" +
		"title: No parallel\nrefs: [go.mod]\n\nThe harness changes directory.\n"
	if exitCode != ExitOK || stdout != want {
		t.Errorf("know show = %d\n%s\nwant\n%s", exitCode, stdout, want)
	}
}

func TestKnowShow_CitationSpelling_Resolves(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"adr-0076.md": "---\ntitle: ADR 0076\n---\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "show", "ADR 0076")

	if exitCode != ExitOK || !strings.HasPrefix(stdout, "id: adr-0076\n") {
		t.Errorf("know show \"ADR 0076\" = %d\n%s%s", exitCode, stdout, stderr)
	}
}

func TestKnowShow_CitationSpellingNotFound_QuotesTheArgumentAsTyped(t *testing.T) {
	root := newKnowStore(t)

	_, stderr, exitCode := runCLI(t, root, "know", "show", "ADR 9999")

	if exitCode != ExitNotFound || stderr != "capcom: id \"ADR 9999\" not found\n" {
		t.Errorf("know show \"ADR 9999\" = %d, stderr %q", exitCode, stderr)
	}
}

func TestKnowShow_PrintsEtag_MatchingTheFile(t *testing.T) {
	root := newKnowStore(t)
	want := noteFileETag(t, root, "testing-rules.md")

	textOutput := mustRun(t, root, "know", "show", "testing-rules")
	jsonData, _ := decodeEnvelope(t, mustRun(t, root, "know", "show", "testing-rules", "--json"))["data"].([]any)

	jsonEtag, _ := jsonData[0].(map[string]any)["etag"].(string)
	if textEtag := textETag(textOutput); textEtag != want || jsonEtag != want {
		t.Errorf("etags = text %q json %q; want %q", textEtag, jsonEtag, want)
	}
}

func TestKnowSearch_JSON_CarriesNoEtag(t *testing.T) {
	root := newKnowStore(t)

	jsonData, _ := decodeEnvelope(t, mustRun(t, root, "know", "search", "harness", "--json"))["data"].([]any)

	if _, hasEtag := jsonData[0].(map[string]any)["etag"]; hasEtag {
		t.Errorf("know search --json data = %v; want no etag", jsonData[0])
	}
}

// newRetiredKnowStore extends newKnowStore with a retired top-level Note superseded by
// testing-rules, its current child, and a retired leaf under testing-rules.
func newRetiredKnowStore(t *testing.T) string {
	t.Helper()
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"old-harness.md": "---\ntitle: Old harness\nretired: true\nsuperseded-by: [testing-rules]\n---\n\n" +
			"The harness of old.\n",
		"old-harness/old-fixtures.md":    "---\ntitle: Old fixtures\n---\n\nFixtures of the harness.\n",
		"testing-rules/legacy-runner.md": "---\ntitle: Legacy runner\nretired: true\n---\n",
	})
	return root
}

func TestKnowTree_RetiredSubtrees_LeftOutUnlessRetired(t *testing.T) {
	root := newRetiredKnowStore(t)
	cases := []struct {
		args []string
		want string
	}{
		{nil, "release-flow  Release flow\ntesting-rules  Testing rules\n  cli-tests  CLI tests\n" +
			"    no-parallel  No parallel\n  git-fixtures  Git fixtures\n"},
		{[]string{"--retired"}, "old-harness  Old harness\n  old-fixtures  Old fixtures\n" +
			"release-flow  Release flow\ntesting-rules  Testing rules\n  cli-tests  CLI tests\n" +
			"    no-parallel  No parallel\n  git-fixtures  Git fixtures\n  legacy-runner  Legacy runner\n"},
		{[]string{"old-harness"}, "old-harness  Old harness\n"},
		{[]string{"old-harness", "--retired"}, "old-harness  Old harness\n  old-fixtures  Old fixtures\n"},
		{[]string{"--depth", "1"}, "release-flow  Release flow\ntesting-rules  Testing rules\n  ... 2 more (--depth 2)\n"},
		{[]string{"--depth", "1", "--retired"}, "old-harness  Old harness\n  ... 1 more (--depth 2)\n" +
			"release-flow  Release flow\ntesting-rules  Testing rules\n  ... 3 more (--depth 2)\n"},
	}

	for _, current := range cases {
		stdout, stderr, exitCode := runCLI(t, root, append([]string{"know", "tree"}, current.args...)...)

		if exitCode != ExitOK || stdout != current.want {
			t.Errorf("know tree %v = %d\n%s%s\nwant\n%s", current.args, exitCode, stdout, stderr, current.want)
		}
	}
}

func TestKnowSearch_RetiredSubtrees_LeftOutUnlessRetired(t *testing.T) {
	root := newRetiredKnowStore(t)

	current := envelopeIDs(t, mustRun(t, root, "know", "search", "harness", "--json"))
	withRetired := envelopeIDs(t, mustRun(t, root, "know", "search", "harness", "--retired", "--json"))

	slices.Sort(withRetired)
	if got := strings.Join(current, ","); got != "no-parallel" {
		t.Errorf("know search harness ids = %s; want no-parallel", got)
	}
	if got := strings.Join(withRetired, ","); got != "no-parallel,old-fixtures,old-harness" {
		t.Errorf("know search harness --retired ids = %s; want no-parallel,old-fixtures,old-harness", got)
	}
}

func TestKnowShow_WithChildren_RetiredChildrenLeftOutUnlessRetired(t *testing.T) {
	root := newRetiredKnowStore(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"testing-rules"}, "testing-rules,cli-tests,git-fixtures"},
		{[]string{"testing-rules", "--retired"}, "testing-rules,cli-tests,git-fixtures,legacy-runner"},
		{[]string{"old-harness"}, "old-harness"},
		{[]string{"old-harness", "--retired"}, "old-harness,old-fixtures"},
	}

	for _, current := range cases {
		args := append([]string{"know", "show", "--with-children", "--json"}, current.args...)
		stdout, stderr, exitCode := runCLI(t, root, args...)

		if exitCode != ExitOK {
			t.Fatalf("know show %v exit = %d\n%s%s", current.args, exitCode, stdout, stderr)
		}
		if got := strings.Join(envelopeIDs(t, stdout), ","); got != current.want {
			t.Errorf("know show %v ids = %s; want %s", current.args, got, current.want)
		}
	}
}

func TestKnowShow_RetiredNote_PrintsRetiredAndSupersededBy(t *testing.T) {
	root := newRetiredKnowStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "know", "show", "old-harness")

	want := "id: old-harness\npath: .capcom/knowledge/old-harness.md\n" +
		"etag: " + noteFileETag(t, root, "old-harness.md") + "\ntitle: Old harness\n" +
		"retired: true\nsuperseded-by: [testing-rules]\n\nThe harness of old.\n"
	if exitCode != ExitOK || stdout != want {
		t.Errorf("know show = %d\n%s%s\nwant\n%s", exitCode, stdout, stderr, want)
	}
}

func TestKnowShow_JSON_CarriesRetiredAndSupersededBy(t *testing.T) {
	root := newRetiredKnowStore(t)

	retiredData, _ := decodeEnvelope(t, mustRun(t, root, "know", "show", "old-harness", "--json"))["data"].([]any)
	currentData, _ := decodeEnvelope(t, mustRun(t, root, "know", "show", "testing-rules", "--json"))["data"].([]any)

	retired := retiredData[0].(map[string]any)
	current := currentData[0].(map[string]any)
	if retired["retired"] != true || fmt.Sprint(retired["superseded_by"]) != "[testing-rules]" {
		t.Errorf("retired note data = %v; want retired true, superseded_by [testing-rules]", retired)
	}
	supersededBy, isList := current["superseded_by"].([]any)
	if _, hasRetired := current["retired"]; hasRetired || !isList || len(supersededBy) != 0 {
		t.Errorf("current note data = %v; want no retired, superseded_by []", current)
	}
}

func TestKnowAdd_PrintsEtag(t *testing.T) {
	root := newKnowStore(t)

	textOutput := mustRun(t, root, "know", "add", "--id", "seed-repos", "--title", "Seed repos", "--parent", "git-fixtures")
	jsonOutput := mustRun(t, root, "know", "add", "--id", "api-design", "--title", "API design", "--json")

	textWant := noteFileETag(t, root, "testing-rules/git-fixtures/seed-repos.md")
	jsonData, _ := decodeEnvelope(t, jsonOutput)["data"].(map[string]any)
	jsonEtag, _ := jsonData["etag"].(string)
	if textEtag := textETag(textOutput); textEtag != textWant {
		t.Errorf("know add text etag = %q; want %q\n%s", textEtag, textWant, textOutput)
	}
	if jsonWant := noteFileETag(t, root, "api-design.md"); jsonEtag != jsonWant {
		t.Errorf("know add --json etag = %q; want %q\n%s", jsonEtag, jsonWant, jsonOutput)
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

func TestKnowAdd_OneWordID_Writes(t *testing.T) {
	root := newKnowStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "turn", "--title", "Turn")

	if exitCode != ExitOK {
		t.Fatalf("know add exit = %d\n%s%s", exitCode, stdout, stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, ".capcom", "knowledge", "turn.md"))
	if want := "---\ntitle: Turn\n---\n"; err != nil || string(data) != want {
		t.Errorf("turn.md = %q, %v; want %q", data, err, want)
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
		{[]string{"--id", "fix-the-login-timeout-race", "--title", "Five-word id"}, ExitError},
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

func TestKnowAdd_BlankTitle_FailsInvalidWithoutWriting(t *testing.T) {
	root := newKnowStore(t)

	stdout, _, exitCode := runCLI(t, root, "know", "add", "--json", "--id", "blank-title", "--title", " ")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeInvalid) ||
		failure["path"] != ".capcom/knowledge/blank-title.md" || failure["message"] != "title is required" {
		t.Errorf("exit = %d error = %v, want invalid at blank-title.md with %q", exitCode, failure, "title is required")
	}
	if noteFileExists(t, root, "blank-title.md") {
		t.Error("blank-title.md was written although its title is blank")
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

func TestKnowVerify_BrokenRefsInRetiredSubtree_AreNotListed(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{
		"old-harness.md":          "---\ntitle: Old harness\nretired: true\nrefs: [gone.go]\n---\n",
		"old-harness/old-step.md": "---\ntitle: Old step\nrefs: [gone.go]\n---\n",
		"live-notes.md":           "---\ntitle: Live notes\nrefs: [gone.go]\n---\n",
	})

	stdout, stderr, exitCode := runCLI(t, root, "know", "verify")

	want := ".capcom/knowledge/live-notes.md:3: ref \"gone.go\": no such file or directory\n"
	if exitCode != ExitError || stdout != want || !strings.Contains(stderr, "1 note refs do not resolve") {
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

func TestKnowAdd_UnrelatedBadFileName_WritesAndWarnsOnce(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"Bad_Name.md": "---\ntitle: Bad name\n---\n"})

	stdout, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "good-note", "--title", "Good note")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if !noteFileExists(t, root, "good-note.md") {
		t.Error("good-note.md was not written")
	}
	if count := strings.Count(stdout+stderr, ".capcom/knowledge/Bad_Name.md"); count != 1 {
		t.Errorf("Bad_Name.md named %d times, want once\nstdout = %q\nstderr = %q", count, stdout, stderr)
	}
}

func TestKnowAdd_UnderParentInOrphanDirectory_Fails(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"lost-dir/parent-note.md": "---\ntitle: Parent note\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "child-note", "--title", "Child", "--parent", "parent-note")

	if exitCode == ExitOK {
		t.Errorf("exit = %d stderr = %q, want a failure", exitCode, stderr)
	}
	if noteFileExists(t, root, "lost-dir/parent-note/child-note.md") {
		t.Error("child-note.md was written below the orphaned parent")
	}
}

func TestKnowAdd_BesideOrphanChildrenDirectory_Fails(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"foo-bar/stray-child.md": "---\ntitle: Stray child\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "foo-bar", "--title", "Foo bar")

	want := ".capcom/knowledge/foo-bar: directory without its node file"
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if noteFileExists(t, root, "foo-bar.md") {
		t.Error("foo-bar.md was written beside its orphan directory")
	}
}

func TestKnowAdd_IDInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"lost-dir/deeper-dir/dup-id.md": "---\ntitle: Lost note\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "dup-id", "--title", "Duplicate")

	want := `.capcom/knowledge/lost-dir/deeper-dir/dup-id.md: id "dup-id" is already taken by this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if noteFileExists(t, root, "dup-id.md") {
		t.Error("dup-id.md was written beside the orphaned file of the same id")
	}
}

func TestKnowAdd_AliasInsideOrphanDirectory_FailsNamingTheFile(t *testing.T) {
	root := newKnowStore(t)
	writeNoteFiles(t, root, map[string]string{"lost-dir/other-id.md": "---\ntitle: Lost note\naliases: [dup-id]\n---\n"})

	_, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "dup-id", "--title", "Duplicate")

	want := `.capcom/knowledge/lost-dir/other-id.md: id "dup-id" is already an alias of this file`
	if exitCode != ExitError || !strings.Contains(stderr, want) {
		t.Errorf("exit = %d stderr = %q, want it to contain %q", exitCode, stderr, want)
	}
	if noteFileExists(t, root, "dup-id.md") {
		t.Error("dup-id.md was written although an orphaned Note has it as an alias")
	}
}

// breakNoteFile puts the title line of the Note file at a slash path below .capcom/knowledge/ inside
// a git conflict hunk, so the file does not parse.
func breakNoteFile(t *testing.T, root string, name string) {
	t.Helper()
	writeNoteFiles(t, root, map[string]string{name: withConflictAroundTitle(readNoteFile(t, root, name))})
}

func TestKnowAdd_AliasOfUnparseableNote_FailsExistsNamingTheFile(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "know", "mv", "release-flow", "release-steps")
	breakNoteFile(t, root, "release-steps.md")

	stdout, _, exitCode := runCLI(t, root, "know", "add", "--json", "--id", "release-flow", "--title", "Duplicate")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	wantMessage := `id "release-flow" may be an alias of note release-steps, whose file does not parse: fix it first`
	if exitCode != ExitError || failure["code"] != string(CodeExists) ||
		failure["path"] != ".capcom/knowledge/release-steps.md" || failure["message"] != wantMessage {
		t.Errorf("exit = %d error = %v, want exists at release-steps.md with %q", exitCode, failure, wantMessage)
	}
	if noteFileExists(t, root, "release-flow.md") {
		t.Error("release-flow.md was written although an unparseable Note lists it as an alias")
	}
}

func TestKnowAdd_BesideUnparseableNoteWithOtherAlias_WritesAndWarnsOnce(t *testing.T) {
	root := newKnowStore(t)
	mustRun(t, root, "know", "mv", "release-flow", "release-steps")
	breakNoteFile(t, root, "release-steps.md")

	stdout, stderr, exitCode := runCLI(t, root, "know", "add", "--id", "good-note", "--title", "Good note")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if !noteFileExists(t, root, "good-note.md") {
		t.Error("good-note.md was not written")
	}
	assertWarnedOnce(t, stdout+stderr, "warning: .capcom/knowledge/release-steps.md:")
}
