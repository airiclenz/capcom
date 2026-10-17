package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
)

// readyConfig declares a custom todo status and hides owner-run issues from ready.
const readyConfig = `statuses:
  open: todo
  triage: todo
  parked: waiting
  done: done
  dropped: dropped
ready:
  hide: [owner-run]
`

// readyIssue is the content of a valid issue file with the given priority, extra frontmatter
// lines (between priority and created) and no body.
func readyIssue(title, status string, priority int, extra string) string {
	closing := ""
	if status == "done" || status == "dropped" {
		closing = "closed: 2026-10-12T09:00:00Z\nreason: finished\n"
	}
	return fmt.Sprintf(
		"---\ntitle: %s\nstatus: %s\ntype: task\npriority: %d\n%screated: 2026-10-11T09:00:00Z\n%s---\n",
		title, status, priority, extra, closing,
	)
}

// newReadyStore creates a store whose issues exercise every ready rule.
func newReadyStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, nil)
	configPath := filepath.Join(root, ".capcom", "config.yaml")
	if err := os.WriteFile(configPath, []byte(readyConfig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writeIssueFiles(t, root, map[string]string{
		"api-tokens.md":                             readyIssue("API tokens", "open", 0, "deps: [old-api]\n"),
		"old-api.md":                                readyIssue("Old API", "dropped", 2, ""),
		"billing-system.md":                         readyIssue("Billing", "parked", 2, ""),
		"billing-system/invoice-run.md":             readyIssue("Invoice", "open", 2, ""),
		"billing-system/invoice-run/invoice-pdf.md": readyIssue("Invoice PDF", "open", 2, ""),
		"deploy-prod.md":                            readyIssue("Deploy", "open", 2, "labels: [owner-run]\n"),
		"login-flow.md":                             readyIssue("Login flow", "open", 2, ""),
		"login-flow/session-refresh.md":             readyIssue("Session refresh", "open", 2, "labels: [auth]\n"),
		"needs-gone.md":                             readyIssue("Needs gone", "open", 2, "deps: [nothing-here]\n"),
		"db-schema.md":                              readyIssue("Schema", "done", 2, "aliases: [apogee-kk0.4]\n"),
		"triage-me.md":                              readyIssue("Triage me", "triage", 2, ""),
		"uses-alias.md": readyIssue("Uses alias", "open", 2,
			"deps: [apogee-kk0.4]\nplan: 2026-10-11 - 00 - alias-plan\n"),
	})
	return root
}

// newStandingParentStore creates a store, declaring the format that allows the standing key, with two
// standing Parents: area, whose only child is closed, and epic, whose child child-task is open.
func newStandingParentStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, nil)
	writeStoreConfig(t, root, fmt.Sprintf("format: %d\n%s", config.FormatStanding, readyConfig))
	writeIssueFiles(t, root, map[string]string{
		"area.md":            readyIssue("Area", "open", 1, "standing: true\n"),
		"area/old-task.md":   readyIssue("Old task", "done", 2, ""),
		"epic.md":            readyIssue("Epic", "open", 1, "standing: true\n"),
		"epic/child-task.md": readyIssue("Child task", "open", 2, ""),
	})
	return root
}

// readyEnvelope runs capcom ready --json with args and returns the decoded envelope.
func readyEnvelope(t *testing.T, root string, args ...string) map[string]any {
	t.Helper()
	stdout, _, exitCode := runCLI(t, root, append([]string{"ready", "--json"}, args...)...)
	if exitCode != ExitOK {
		t.Fatalf("ready %v exit = %d\n%s", args, exitCode, stdout)
	}
	return decodeEnvelope(t, stdout)
}

func TestReady_Flags_SelectReadyIssuesMostUrgentFirst(t *testing.T) {
	root := newReadyStore(t)
	cases := []struct {
		args []string
		want []string
	}{
		{nil, []string{"api-tokens", "session-refresh", "triage-me", "uses-alias"}},
		{[]string{"--show-hidden"}, []string{"api-tokens", "session-refresh", "deploy-prod", "triage-me", "uses-alias"}},
		{[]string{"--planned"}, []string{"uses-alias"}},
		{[]string{"--unplanned"}, []string{"api-tokens", "session-refresh", "triage-me"}},
		{[]string{"--label", "auth"}, []string{"session-refresh"}},
		{[]string{"--no-label", "auth"}, []string{"api-tokens", "triage-me", "uses-alias"}},
		{[]string{"--limit", "2"}, []string{"api-tokens", "session-refresh"}},
		{[]string{"--all"}, []string{
			"api-tokens", "session-refresh", "invoice-run", "invoice-pdf", "login-flow", "needs-gone", "triage-me", "uses-alias",
		}},
	}

	for _, current := range cases {
		got := readyIDs(t, readyEnvelope(t, root, current.args...))

		if !slices.Equal(got, current.want) {
			t.Errorf("ready %v = %v, want %v", current.args, got, current.want)
		}
	}
}

// readyIDs returns the ids of the issues in a decoded envelope's data list.
func readyIDs(t *testing.T, envelope map[string]any) []string {
	t.Helper()
	issues, _ := envelope["data"].([]any)
	ids := make([]string, 0, len(issues))
	for _, item := range issues {
		ids = append(ids, item.(map[string]any)["id"].(string))
	}
	return ids
}

func TestReady_All_JSONCarriesWhatBlocks(t *testing.T) {
	root := newReadyStore(t)

	envelope := readyEnvelope(t, root, "--all")

	byID := map[string]map[string]any{}
	for _, item := range envelope["data"].([]any) {
		entry := item.(map[string]any)
		byID[entry["id"].(string)] = entry
	}
	want := map[string]map[string]any{
		"needs-gone":  {"ready": false, "blocking": []any{map[string]any{"dep": "nothing-here", "reason": "missing"}}},
		"invoice-run": {"ready": false, "open_children": []any{"invoice-pdf"}, "held_by": "billing-system"},
		"invoice-pdf": {"ready": false, "held_by": "billing-system"},
		"login-flow":  {"ready": false, "open_children": []any{"session-refresh"}},
		"uses-alias":  {"ready": true},
	}
	for id, fields := range want {
		for _, key := range []string{"ready", "blocking", "open_children", "held_by"} {
			if got := byID[id][key]; !reflect.DeepEqual(got, fields[key]) {
				t.Errorf("%s.%s = %#v, want %#v", id, key, got, fields[key])
			}
		}
	}
}

func TestReady_DroppedDep_ReadyWithWarning(t *testing.T) {
	root := newReadyStore(t)

	envelope := readyEnvelope(t, root)

	want := []any{`.capcom/issues/api-tokens.md: dep "old-api" is dropped; counted as satisfied`}
	if got := envelope["warnings"]; !reflect.DeepEqual(got, want) {
		t.Errorf("warnings = %#v, want %#v", got, want)
	}
}

func TestReady_Text_ListsReadyLinesAndReasonsUnderAll(t *testing.T) {
	root := newReadyStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "ready", "--all", "--no-label", "auth")

	want := "api-tokens  open  P0  API tokens\n" +
		"invoice-run  open  P2  Invoice  -- not ready: open children: invoice-pdf; held by billing-system\n" +
		"invoice-pdf  open  P2  Invoice PDF  -- not ready: held by billing-system\n" +
		"login-flow  open  P2  Login flow  -- not ready: open children: session-refresh\n" +
		"needs-gone  open  P2  Needs gone  -- not ready: blocked by nothing-here (missing)\n" +
		"triage-me  triage  P2  Triage me\n" +
		"uses-alias  open  P2  Uses alias\n" +
		"warning: .capcom/issues/api-tokens.md: dep \"old-api\" is dropped; counted as satisfied\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestReady_Text_ReadyLineEndsWithWhatItUnblocks(t *testing.T) {
	root := newReadyStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "ready", "--show-hidden")

	want := "api-tokens  open  P0  API tokens\n" +
		"session-refresh  open  P2  Session refresh  -- unblocks 1\n" +
		"deploy-prod  open  P2  Deploy\n" +
		"triage-me  triage  P2  Triage me\n" +
		"uses-alias  open  P2  Uses alias\n" +
		"warning: .capcom/issues/api-tokens.md: dep \"old-api\" is dropped; counted as satisfied\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

func TestReady_All_JSONCarriesWhatAReadyIssueUnblocks(t *testing.T) {
	root := newReadyStore(t)

	envelope := readyEnvelope(t, root, "--all")

	want := map[string]any{"session-refresh": []any{"login-flow"}}
	for _, item := range envelope["data"].([]any) {
		entry := item.(map[string]any)
		id := entry["id"].(string)
		if got := entry["unblocks"]; !reflect.DeepEqual(got, want[id]) {
			t.Errorf("%s.unblocks = %#v, want %#v", id, got, want[id])
		}
	}
}

func TestReady_BadFlags_ExitUsage(t *testing.T) {
	root := newReadyStore(t)

	for _, args := range [][]string{{"--limit", "-1"}, {"--planned", "--unplanned"}} {
		_, _, exitCode := runCLI(t, root, append([]string{"ready"}, args...)...)

		if exitCode != ExitUsage {
			t.Errorf("ready %v exit = %d, want %d", args, exitCode, ExitUsage)
		}
	}
}

func TestReady_StandingParents_OnlyTheOpenChildIsListed(t *testing.T) {
	root := newStandingParentStore(t)

	envelope := readyEnvelope(t, root)

	if got := readyIDs(t, envelope); !reflect.DeepEqual(got, []string{"child-task"}) {
		t.Errorf("ready = %v, want [child-task]", got)
	}
}

func TestReady_All_Text_StandingReasonComesFirst(t *testing.T) {
	root := newStandingParentStore(t)

	stdout, stderr, exitCode := runCLI(t, root, "ready", "--all")

	want := "area  open  P1  Area  -- not ready: standing: holds children\n" +
		"epic  open  P1  Epic  -- not ready: standing: holds children; open children: child-task\n" +
		"child-task  open  P2  Child task\n"
	if exitCode != ExitOK || stderr != "" || stdout != want {
		t.Errorf("exit = %d stderr = %q\nstdout = %q\nwant     %q", exitCode, stderr, stdout, want)
	}
}

// The JSON of ready --all carries the mark as the issue's standing field and adds no reason key.
func TestReady_All_JSON_StandingParentIsNotReady(t *testing.T) {
	root := newStandingParentStore(t)

	envelope := readyEnvelope(t, root, "--all")

	byID := map[string]map[string]any{}
	for _, item := range envelope["data"].([]any) {
		entry := item.(map[string]any)
		byID[entry["id"].(string)] = entry
	}
	want := map[string]map[string]any{
		"area":       {"ready": false, "standing": true},
		"epic":       {"ready": false, "standing": true, "open_children": []any{"child-task"}},
		"child-task": {"ready": true},
	}
	for id, fields := range want {
		if !reflect.DeepEqual(readyFieldsOf(byID[id]), fields) {
			t.Errorf("%s = %#v, want ready fields %#v", id, byID[id], fields)
		}
	}
}

// readyFieldsOf returns the keys of a ready --json entry beyond the issue's identity and contents:
// whether it is Ready, its standing mark and any reason key.
func readyFieldsOf(entry map[string]any) map[string]any {
	fields := map[string]any{}
	for key, value := range entry {
		switch key {
		case "ready", "standing", "open_children", "blocking", "held_by", "unblocks", "reason", "reasons":
			fields[key] = value
		}
	}
	return fields
}
