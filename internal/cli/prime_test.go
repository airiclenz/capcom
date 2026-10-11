package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		"broken note refs: 1\n" +
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

func TestPrime_WithoutStore_ExitsZeroWithAWarning(t *testing.T) {
	root := newGitRepo(t)

	stdout, stderr, exitCode := runCLI(t, root, "prime")

	if exitCode != ExitOK || stderr != "" || !strings.HasPrefix(stdout, "warning: store: ") {
		t.Errorf("prime = %d, stderr %q, stdout %q; want 0 and a store warning", exitCode, stderr, stdout)
	}
}
