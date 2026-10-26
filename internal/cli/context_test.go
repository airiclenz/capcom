package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contextIssue is the issue main of newContextStore: titled for the search on rotation and
// cadence, mentioning one file and one directory that Note refs name.
const contextIssue = "---\ntitle: Rotation cadence\nstatus: open\ntype: task\npriority: 1\n" +
	"deps: [base, ghost]\nplan: login-plan\ncreated: 2026-10-11T09:00:00Z\n---\n\n" +
	"Change internal/auth/token.go and the files under internal/store/ here.\n"

// newContextStore creates a store whose issue main has a dep, a missing dep, a dependent, a plan
// file, two Notes whose refs name paths its body mentions, one whose ref only shares a prefix,
// a retired one whose ref matches, and one Note its title search finds.
func newContextStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"main.md":  contextIssue,
		"base.md":  readyIssue("Base", "open", 2, ""),
		"child.md": readyIssue("Child", "open", 2, "deps: [main]\n"),
	})
	writeNoteFiles(t, root, map[string]string{
		"token-rules.md": "---\ntitle: Token rules\nrefs: [internal/auth/token.go#Sign]\n---\n\nSign with the shared key.\n",
		"store-rules.md": "---\ntitle: Store rules\nrefs: [internal/store]\n---\n\nWrites go through ops.\n",
		"prefix-only.md": "---\ntitle: Prefix only\nrefs: [internal/auth/token.go.bak]\n---\n\nUnrelated.\n",
		"old-token.md":   "---\ntitle: Old token\nretired: true\nrefs: [internal/auth/token.go]\n---\n\nGone.\n",
		"cadence.md":     "---\ntitle: Rotation cadence notes\n---\n\nEvery ninety days.\n",
	})
	plans := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(plans, "login-plan.md"), []byte("# Plan\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return root
}

func TestContext_FullIssue_PrintsShowThenEverySection(t *testing.T) {
	root := newContextStore(t)
	shown, _, _ := runCLI(t, root, "show", "main", "--no-elsewhere")
	want := shown + "\n" +
		"deps:\n  base  open  P2  Base\n  ghost  missing\n" +
		"dependents:\n  child  open  P2  Child\n" +
		"plan: docs/plans/login-plan.md\n" +
		"notes:\n" +
		"  store-rules  Store rules  (ref internal/store)\n" +
		"  token-rules  Token rules  (ref internal/auth/token.go#Sign)\n" +
		"  cadence  Rotation cadence notes  (search)\n" +
		"\n--- store-rules: Store rules\nWrites go through ops.\n" +
		"\n--- token-rules: Token rules\nSign with the shared key.\n" +
		"\n--- cadence: Rotation cadence notes\nEvery ninety days.\n"

	stdout, stderr, exitCode := runCLI(t, root, "context", "main")

	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestContext_NothingAround_PrintsNoneForEachSection(t *testing.T) {
	root := newIssueStore(t, map[string]string{"lone.md": readyIssue("Lone", "open", 2, "")})
	shown, _, _ := runCLI(t, root, "show", "lone", "--no-elsewhere")
	want := shown + "\ndeps: none\ndependents: none\nplan: none\nnotes: none\n"

	stdout, stderr, exitCode := runCLI(t, root, "context", "lone")

	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestContext_PlanWithoutFile_PrintsItNotFound(t *testing.T) {
	root := newIssueStore(t, map[string]string{"lone.md": readyIssue("Lone", "open", 2, "plan: absent-plan\n")})

	stdout, _, exitCode := runCLI(t, root, "context", "lone")

	if exitCode != ExitOK || !strings.Contains(stdout, "\nplan: absent-plan (not found)\n") {
		t.Errorf("exit = %d\nstdout = %q", exitCode, stdout)
	}
}

func TestContext_LongSections_CutWithTheCommandThatPrintsTheRest(t *testing.T) {
	files := map[string]string{"hub.md": readyIssue("Hub", "open", 1, "") + "\nSee notes/long.md for the rules.\n"}
	for number := range contextDepLimit + 2 {
		files[fmt.Sprintf("leaf-%02d.md", number)] = readyIssue("Leaf", "open", 2, "deps: [hub]\n")
	}
	root := newIssueStore(t, files)
	var body strings.Builder
	for number := range contextBodyLineLimit + 5 {
		fmt.Fprintf(&body, "line %d\n", number+1)
	}
	writeNoteFiles(t, root, map[string]string{
		"long-note.md": "---\ntitle: Long note\nrefs: [notes/long.md]\n---\n\n" + body.String(),
	})

	stdout, _, exitCode := runCLI(t, root, "context", "hub")

	wantLines := []string{
		"  leaf-14  open  P2  Leaf\n  ... 2 more: capcom dep list hub\nplan: none\n",
		fmt.Sprintf("line %d\n... cut: capcom know show long-note\n", contextBodyLineLimit),
	}
	for _, want := range wantLines {
		if exitCode != ExitOK || !strings.Contains(stdout, want) {
			t.Errorf("exit = %d, stdout lacks %q:\n%s", exitCode, want, stdout)
		}
	}
	if strings.Contains(stdout, "  leaf-15  open") || strings.Contains(stdout, fmt.Sprintf("line %d\n", contextBodyLineLimit+1)) {
		t.Errorf("stdout prints past a cap:\n%s", stdout)
	}
}

func TestContext_Brief_PrintsTheIssueLineAndNoBodies(t *testing.T) {
	root := newContextStore(t)

	stdout, _, exitCode := runCLI(t, root, "context", "main", "--brief")

	wantStart := "main  open  P1  Rotation cadence\n\ndeps:\n"
	wantEnd := "  cadence  Rotation cadence notes  (search)\n"
	if exitCode != ExitOK || !strings.HasPrefix(stdout, wantStart) || !strings.HasSuffix(stdout, wantEnd) {
		t.Errorf("exit = %d\nstdout = %q\nwant it to start %q and end %q", exitCode, stdout, wantStart, wantEnd)
	}
}

func TestContext_JSON_GivesIssueDepsPlanAndNotes(t *testing.T) {
	root := newContextStore(t)

	stdout, _, exitCode := runCLI(t, root, "context", "main", "--json")

	envelope := decodeEnvelope(t, stdout)
	data, _ := envelope["data"].(map[string]any)
	issue, _ := data["issue"].(map[string]any)
	deps, _ := data["deps"].([]any)
	dependents, _ := data["dependents"].([]any)
	plan, _ := data["plan"].(map[string]any)
	notes, _ := data["notes"].([]any)
	if exitCode != ExitOK || envelope["kind"] != "context" || issue["id"] != "main" || issue["body"] == nil ||
		len(deps) != 1 || len(dependents) != 1 || plan["path"] != "docs/plans/login-plan.md" || len(notes) != 3 {
		t.Fatalf("exit = %d\nstdout = %s", exitCode, stdout)
	}
	first, _ := notes[0].(map[string]any)
	last, _ := notes[2].(map[string]any)
	if first["id"] != "store-rules" || first["source"] != "ref" || first["ref"] != "internal/store" ||
		first["body"] != "\nWrites go through ops.\n" || last["source"] != "search" || last["ref"] != nil {
		t.Errorf("notes = %v", notes)
	}
}

func TestContext_UnknownID_FailsNotFound(t *testing.T) {
	root := newIssueStore(t, map[string]string{"lone.md": readyIssue("Lone", "open", 2, "")})

	_, stderr, exitCode := runCLI(t, root, "context", "nosuch")

	if exitCode != ExitNotFound || !strings.Contains(stderr, `"nosuch" not found`) {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
}

func TestMentionsPath_WholePathOrDirectory_Counts(t *testing.T) {
	cases := []struct {
		text string
		path string
		want bool
	}{
		{"edit internal/auth/token.go now", "internal/auth/token.go", true},
		{"edit internal/auth/token.go.", "internal/auth/token.go", true},
		{"(internal/auth/token.go)", "internal/auth/token.go", true},
		{"under internal/store/ops.go", "internal/store", true},
		{"under internal/store/", "internal/store/", true},
		{"edit internal/auth/token.go.bak", "internal/auth/token.go", false},
		{"edit vendor/internal/store/x.go", "internal/store", false},
		{"edit internal/storefront", "internal/store", false},
		{"anything", "", false},
	}

	for _, current := range cases {
		got := mentionsPath(current.text, current.path)

		if got != current.want {
			t.Errorf("mentionsPath(%q, %q) = %v, want %v", current.text, current.path, got, current.want)
		}
	}
}
