package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// primeConfig declares a doing status, so the Briefing has issues in progress to list.
const primeConfig = `statuses:
  open: todo
  active: doing
  parked: waiting
  done: done
  dropped: dropped
`

// primePlan is the plan the planned fixture issue links, under the default plans directory.
const primePlan = "2026-10-11 - 00 - prime-plan"

// newPrimeStore creates a store with planned, unplanned, in-progress and closed issues and a Note
// with one broken Ref.
func newPrimeStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, nil)
	files := map[string]string{
		".capcom/config.yaml":             primeConfig,
		"docs/plans/" + primePlan + ".md": "# Plan\n",
		"go.mod":                          "module example\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	writeIssueFiles(t, root, map[string]string{
		"plan-work.md":   readyIssue("Planned work", "open", 1, "plan: "+primePlan+"\n"),
		"quick-fix.md":   readyIssue("Quick fix", "open", 0, ""),
		"side-quest.md":  readyIssue("Side quest", "open", 3, ""),
		"busy-work.md":   readyIssue("Busy work", "active", 2, ""),
		"waiting-on.md":  readyIssue("Waiting on vendor", "parked", 2, ""),
		"shipped-it.md":  readyIssue("Shipped", "done", 2, ""),
		"blocked-one.md": readyIssue("Blocked one", "open", 2, "deps: [busy-work]\n"),
	})
	writeNoteFiles(t, root, map[string]string{
		"build-notes.md": "---\ntitle: Build notes\nrefs: [go.mod, gone.go]\n---\n",
	})
	return root
}

func TestPrime_FixtureStore_PrintsTheBriefing(t *testing.T) {
	root := newPrimeStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	want := "ready, planned: 1\n" +
		"  plan-work  open  P1  Planned work\n" +
		"ready, unplanned: 2\n" +
		"  quick-fix  open  P0  Quick fix\n" +
		"  side-quest  open  P3  Side quest\n" +
		"doing: 1\n" +
		"  busy-work  active  P2  Busy work\n" +
		"locks: 0\n" +
		worktreeLockFree + "\n" +
		"broken Note refs: 1\n" +
		"  .capcom/knowledge/build-notes.md:3: ref \"gone.go\": no such file or directory\n" +
		"check: 1 problem; run capcom check\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Fatalf("prime = %d, stderr %q, stdout:\n%s\nwant:\n%s", exitCode, stderr, stdout, want)
	}
}

func TestPrime_JSON_CarriesEverySection(t *testing.T) {
	root := newPrimeStore(t)

	stdout, _, exitCode := runCLI(t, root, "prime", "--json")

	envelope := decodeEnvelope(t, stdout)
	data, isObject := envelope["data"].(map[string]any)
	if exitCode != ExitOK || envelope["kind"] != kindPrime || !isObject {
		t.Fatalf("prime --json = %d\n%s", exitCode, stdout)
	}
	for _, key := range []string{"ready_planned", "ready_unplanned", "doing", "locks", "broken_refs", "check_problems"} {
		if _, isPresent := data[key]; !isPresent {
			t.Errorf("data lacks %s: %v", key, data)
		}
	}
	if count := data["ready_unplanned"].(map[string]any)["count"]; count != 2.0 {
		t.Errorf("ready_unplanned count = %v, want 2", count)
	}
}

// A broken_refs element is a check problem: exactly path, line, code and message, and the line of
// the broken ref, never 0, since every ref of a Note sits on its own line.
func TestPrime_JSONBrokenRef_IsACheckProblemAtItsLine(t *testing.T) {
	root := newPrimeStore(t)

	stdout, _, exitCode := runCLI(t, root, "prime", "--json")

	envelope := decodeEnvelope(t, stdout)
	data, isObject := envelope["data"].(map[string]any)
	if exitCode != ExitOK || !isObject {
		t.Fatalf("prime --json = %d\n%s", exitCode, stdout)
	}
	want := []any{map[string]any{
		"path":    ".capcom/knowledge/build-notes.md",
		"line":    float64(3),
		"code":    "broken-ref",
		"message": `ref "gone.go": no such file or directory`,
	}}
	if !reflect.DeepEqual(data["broken_refs"], want) {
		t.Errorf("broken_refs = %v, want %v", data["broken_refs"], want)
	}
}

func TestPrime_ManyReadyIssues_CapsEachListAndCountsAll(t *testing.T) {
	root := newIssueStore(t, nil)
	files := map[string]string{}
	for number := range primeListCap + 2 {
		files[fmt.Sprintf("task-%02d.md", number)] = readyIssue("Task", "open", 2, "")
	}
	writeIssueFiles(t, root, files)

	stdout, _, exitCode := runCLI(t, root, "prime")

	_, unplanned, _ := strings.Cut(stdout, "ready, unplanned: 17, first 15 shown\n")
	if exitCode != ExitOK || strings.Count(unplanned, "  task-") != primeListCap {
		t.Errorf("prime = %d, stdout:\n%s\nwant 15 of 17 unplanned issues listed", exitCode, stdout)
	}
}

func TestPrime_Locks_ShowAgeAndStaleFlag(t *testing.T) {
	root := newIssueStore(t, nil)
	mustRun(t, root, "lock", "acquire", "--purpose", "refactor the parser", "--holder", "bob")
	recordPath := filepath.Join(root, ".git", "capcom", "locks", "_main.json")
	ageLockRecord(t, recordPath, 3*time.Hour)

	stdout, _, exitCode := runCLI(t, root, "prime")

	pattern := regexp.MustCompile(`(?m)^locks: 1\n  _main: held by bob for 3h0[0-9]m, heartbeat 3h0[0-9]m ago: refactor the parser \(stale\)$`)
	if exitCode != ExitOK || !pattern.MatchString(stdout) {
		t.Errorf("prime = %d, stdout:\n%s\nwant a stale lock line matching %s", exitCode, stdout, pattern)
	}
	if !strings.Contains(stdout, " (stale)\n"+worktreeLockStale+"\n") {
		t.Errorf("prime stdout:\n%s\nwant the stale advice after the locks list", stdout)
	}
}

func TestPrime_FreshLock_AdvisesItIsHeld(t *testing.T) {
	root := newIssueStore(t, nil)
	mustRun(t, root, "lock", "acquire", "--purpose", "refactor the parser", "--holder", "bob")

	stdout, _, exitCode := runCLI(t, root, "prime")

	pattern := regexp.MustCompile(`(?m)^  _main: held by bob for [^\n]*: refactor the parser\n` +
		regexp.QuoteMeta(worktreeLockHeld) + `$`)
	if exitCode != ExitOK || !pattern.MatchString(stdout) {
		t.Errorf("prime = %d, stdout:\n%s\nwant the held advice after the locks list", exitCode, stdout)
	}
}

// Acquire fails on a corrupt record of this worktree, so the Briefing calls the worktree invalid,
// never free.
func TestPrime_GarbageLockRecord_AdvisesItIsInvalid(t *testing.T) {
	root := newLockStore(t)
	writeLockRecord(t, root, "_main", "not json")

	stdout, _, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || !strings.Contains(stdout, "locks: 0\n"+worktreeLockInvalid+"\n") {
		t.Errorf("prime = %d, stdout:\n%s\nwant the invalid advice after the locks list", exitCode, stdout)
	}
	warning := regexp.MustCompile(`(?m)^warning: locks: [^\n]*_main\.json: invalid lock record: `)
	if !warning.MatchString(stdout) {
		t.Errorf("stdout:\n%s\nwant the invalid record warning", stdout)
	}
}

func TestPrime_DirectoryAtTheLockRecord_AdvisesItIsUnreadable(t *testing.T) {
	root := newLockStore(t)
	recordPath := filepath.Join(root, ".git", "capcom", "locks", "_main.json")
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	stdout, _, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || !strings.Contains(stdout, "locks: 0\n"+worktreeLockUnreadable+"\n") {
		t.Errorf("prime = %d, stdout:\n%s\nwant the unreadable advice after the locks list", exitCode, stdout)
	}
	warnings := regexp.MustCompile(`(?m)^warning: locks: [^\n]*_main\.json: `).FindAllString(stdout, -1)
	if len(warnings) != 1 {
		t.Errorf("stdout:\n%s\nwant exactly one warning naming the record", stdout)
	}
}

func TestPrime_UnreadableForeignLockRecord_ListsTheOtherLocksWithOneWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an unreadable file test does not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("an unreadable file test does not run as root")
	}
	root := newLockStore(t)
	mustRun(t, root, "lock", "acquire", "--purpose", "refactor the parser", "--holder", "bob")
	writeLockRecord(t, root, "wt-ghost", "{}")
	foreignPath := filepath.Join(root, ".git", "capcom", "locks", "wt-ghost.json")
	if err := os.Chmod(foreignPath, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	pattern := regexp.MustCompile(`(?m)^locks: 1\n  _main: held by bob for [^\n]*: refactor the parser\n` +
		regexp.QuoteMeta(worktreeLockHeld) + `$`)
	if exitCode != ExitOK || stderr != "" || !pattern.MatchString(stdout) {
		t.Errorf("prime = %d, stderr %q, stdout:\n%s\nwant the held lock and its advice", exitCode, stderr, stdout)
	}
	warnings := regexp.MustCompile(`(?m)^warning: .*$`).FindAllString(stdout, -1)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "warning: locks: ") ||
		!strings.Contains(warnings[0], "wt-ghost.json: ") {
		t.Errorf("warnings = %q, want one locks warning naming wt-ghost.json", warnings)
	}
}

func TestPrime_LockHeldInMainWorktree_AdvisesALinkedWorktreeItIsFree(t *testing.T) {
	root := newLockStore(t)
	worktree := addWorktree(t, root, "feature")
	mustRun(t, root, "lock", "acquire", "--purpose", "refactor the parser", "--holder", "bob")

	stdout, _, exitCode := runCLI(t, worktree, "prime")

	pattern := regexp.MustCompile(`(?m)^locks: 1\n  _main: held by bob for [^\n]*: refactor the parser\n` +
		regexp.QuoteMeta(worktreeLockFree) + `$`)
	if exitCode != ExitOK || !pattern.MatchString(stdout) {
		t.Errorf("prime = %d, stdout:\n%s\nwant the free advice after the locks list", exitCode, stdout)
	}
}

// ageLockRecord moves the since and heartbeat of the lock record at path back by age.
func ageLockRecord(t *testing.T, path string, age time.Duration) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(content, &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	past := time.Now().Add(-age).UTC().Format(time.RFC3339Nano)
	record["since"], record["heartbeat"] = past, past
	if content, err = json.Marshal(record); err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestPrime_BrokenConfig_ExitsZeroWithOneWarning(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	configPath := filepath.Join(root, ".capcom", "config.yaml")
	if err := os.WriteFile(configPath, []byte("statuses:\n  open: nonsense\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || stderr != "" || !strings.HasPrefix(stdout, "warning: store: .capcom/config.yaml:2: ") ||
		strings.Count(stdout, "\n") != 1 {
		t.Errorf("prime = %d, stderr %q, stdout %q; want 0 and one config warning line", exitCode, stderr, stdout)
	}
}

func TestPrime_NewerStoreFormat_ExitsZeroWithOneWarning(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	writeNewerFormatConfig(t, root)

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || stderr != "" || stdout != "warning: store: "+newerFormatMessage+"\n" {
		t.Errorf("prime = %d, stderr %q, stdout %q; want 0 and one too-new store warning", exitCode, stderr, stdout)
	}
}

func TestPrime_GarbageIndex_ExitsZeroWithOneWarningLine(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	mustRun(t, root, "reindex")
	indexPath := filepath.Join(root, ".git", "capcom", "index.db")
	if err := os.WriteFile(indexPath, []byte(strings.Repeat("not a database ", 512)), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	warnings := regexp.MustCompile(`(?m)^warning: `).FindAllString(stdout, -1)
	if exitCode != ExitOK || stderr != "" || len(warnings) != 1 || !strings.Contains(stdout, "warning: index: ") {
		t.Errorf("prime = %d, stderr %q, stdout:\n%s\nwant 0 and exactly one index warning", exitCode, stderr, stdout)
	}
	if !strings.Contains(stdout, "locks: 0\n") || !strings.Contains(stdout, "check: no problems\n") {
		t.Errorf("stdout:\n%s\nwant the sections that do not need the index", stdout)
	}
}

func TestPrime_UnparseableIssue_CountsItInCheckWithoutAWarningLine(t *testing.T) {
	root := newIssueStore(t, map[string]string{
		"valid-issue.md":  validIssue,
		"broken-issue.md": "no frontmatter here\n",
	})

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	warnings := regexp.MustCompile(`(?m)^warning: `).FindAllString(stdout, -1)
	if exitCode != ExitOK || stderr != "" || len(warnings) != 0 {
		t.Errorf("prime = %d, stderr %q, stdout:\n%s\nwant 0 and no warning line", exitCode, stderr, stdout)
	}
	if !strings.Contains(stdout, "check: 1 problem; run capcom check\n") {
		t.Errorf("stdout:\n%s\nwant the broken issue counted in the check line", stdout)
	}
}

func TestPrime_RepeatedID_CountsBothFilesInCheck(t *testing.T) {
	root := newIssueStore(t, map[string]string{"x.md": validIssue, "a.md": validIssue})
	writeIssueFiles(t, root, map[string]string{"a/x.md": validIssue})

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || stderr != "" || !strings.Contains(stdout, "check: 2 problems; run capcom check\n") {
		t.Errorf("prime = %d, stderr %q, stdout:\n%s\nwant 0 and both files of the repeated id counted",
			exitCode, stderr, stdout)
	}
}

func TestPrime_OversizedAgentsGuide_ExitsZeroCountingIt(t *testing.T) {
	root := newIssueStore(t, map[string]string{"valid-issue.md": validIssue})
	guide := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(guide, []byte(strings.Repeat("x", 1<<20+1)), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || stderr != "" || !strings.Contains(stdout, "check: 1 problem; run capcom check\n") {
		t.Errorf("prime = %d, stderr %q, stdout:\n%s\nwant 0 and the oversized guide counted in the check line",
			exitCode, stderr, stdout)
	}
}

func TestPrime_WithoutStore_ExitsZeroWithAWarning(t *testing.T) {
	root := newGitRepo(t)

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || stderr != "" || !strings.HasPrefix(stdout, "warning: store: ") {
		t.Errorf("prime = %d, stderr %q, stdout %q; want 0 and a store warning", exitCode, stderr, stdout)
	}
}
