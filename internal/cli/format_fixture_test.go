package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
)

// fixtureReadyIssue is the issue every format fixture holds Ready: its only dependency is closed.
const fixtureReadyIssue = "release-script"

// fixtureEnvelope is the part of a --json envelope the format fixture tests read: the data, and
// the warnings, which a clean fixture store never draws.
type fixtureEnvelope struct {
	Data     json.RawMessage `json:"data"`
	Warnings []string        `json:"warnings"`
}

// fixtureNode is the id of one issue or Note in a --json list.
type fixtureNode struct {
	ID string `json:"id"`
}

// newerStoreDir is the package-relative directory of the store a newer capcom wrote: it holds no
// config.yaml, since its format key is written at test time as newerFormat, and it sits outside
// the format-<n> namespace, so a raised config.FormatVersion never takes it for its fixture.
var newerStoreDir = filepath.Join("testdata", "stores", "newer")

// copyFixtureStore copies the hand-written store testdata/stores/format-<format> into the .capcom
// directory of a fresh git repository and commits it, returning the repository's root.
func copyFixtureStore(t *testing.T, format int) string {
	t.Helper()
	root := copyStoreDir(t, fixtureStoreDir(format))
	commitFixtureStore(t, root, fmt.Sprintf("format %d fixture", format))
	return root
}

// copyStoreDir copies the package-relative store directory dir into the .capcom directory of a
// fresh git repository, uncommitted, and returns the repository's root.
func copyStoreDir(t *testing.T, dir string) string {
	t.Helper()
	source, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("no fixture store %s: %v", dir, err)
	}
	root := newGitRepo(t)
	target := filepath.Join(root, ".capcom")
	if err := os.CopyFS(target, os.DirFS(source)); err != nil {
		t.Fatalf("CopyFS %s: %v", source, err)
	}
	return root
}

// commitFixtureStore commits everything in the repository at root with message.
func commitFixtureStore(t *testing.T, root, message string) {
	t.Helper()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", message)
}

// fixtureStoreDir is the package-relative directory of the fixture store for format.
func fixtureStoreDir(format int) string {
	return filepath.Join("testdata", "stores", fmt.Sprintf("format-%d", format))
}

// fixtureIDs returns the sorted ids of the markdown files below the fixture store's sub directory
// (issues or knowledge): a node's id is its file name without .md.
func fixtureIDs(t *testing.T, format int, sub string) []string {
	t.Helper()
	dir := filepath.Join(fixtureStoreDir(format), sub)
	var ids []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".md"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir %s: %v", dir, err)
	}
	slices.Sort(ids)
	return ids
}

// runFixtureJSON runs a --json command in root, requires exit 0 and an envelope without warnings,
// and returns the envelope's data.
func runFixtureJSON(t *testing.T, root string, args ...string) json.RawMessage {
	t.Helper()
	stdout := mustRun(t, root, append(args, "--json")...)
	var envelope fixtureEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("%v: decode: %v\n%s", args, err, stdout)
	}
	if len(envelope.Warnings) > 0 {
		t.Errorf("%v warnings = %q, want none", args, envelope.Warnings)
	}
	return envelope.Data
}

// fixtureNodeIDs decodes a --json list of issues or Notes into their sorted ids.
func fixtureNodeIDs(t *testing.T, data json.RawMessage) []string {
	t.Helper()
	var nodes []fixtureNode
	if err := json.Unmarshal(data, &nodes); err != nil {
		t.Fatalf("decode node list: %v\n%s", err, data)
	}
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	slices.Sort(ids)
	return ids
}

// TestFormatFixture_Reads proves capcom still reads a store of every format it ever wrote: each
// hand-written fixture, using only the keys its format allows, checks clean, reindexes, and lists
// its issues and Notes through list, ready, prime and know tree. It loops to config.FormatVersion,
// so raising the format without adding its fixture fails here.
func TestFormatFixture_Reads(t *testing.T) {
	for format := config.MissingFormat; format <= config.FormatVersion; format++ {
		t.Run(fmt.Sprintf("format-%d", format), func(t *testing.T) {
			root := copyFixtureStore(t, format)
			wantIssues := fixtureIDs(t, format, "issues")
			wantNotes := fixtureIDs(t, format, "knowledge")

			if problems := runFixtureJSON(t, root, "check"); string(problems) != "[]" {
				t.Errorf("check problems = %s, want none", problems)
			}

			var reindex struct {
				Issues int `json:"issues"`
			}
			if err := json.Unmarshal(runFixtureJSON(t, root, "reindex"), &reindex); err != nil {
				t.Fatalf("decode reindex: %v", err)
			}
			if reindex.Issues != len(wantIssues) {
				t.Errorf("reindex issues = %d, want %d", reindex.Issues, len(wantIssues))
			}

			if got := fixtureNodeIDs(t, runFixtureJSON(t, root, "list", "--all")); !slices.Equal(got, wantIssues) {
				t.Errorf("list --all ids = %q, want %q", got, wantIssues)
			}

			ready := fixtureNodeIDs(t, runFixtureJSON(t, root, "ready"))
			if !slices.Contains(ready, fixtureReadyIssue) {
				t.Errorf("ready ids = %q, want them to hold %q", ready, fixtureReadyIssue)
			}
			for _, id := range ready {
				if !slices.Contains(wantIssues, id) {
					t.Errorf("ready id %q is not a fixture issue", id)
				}
			}

			if prime := runFixtureJSON(t, root, "prime"); !strings.Contains(string(prime), `"id":"`+fixtureReadyIssue+`"`) {
				t.Errorf("prime = %s, want it to name %q", prime, fixtureReadyIssue)
			}

			if got := fixtureNodeIDs(t, runFixtureJSON(t, root, "know", "tree", "--retired")); !slices.Equal(got, wantNotes) {
				t.Errorf("know tree --retired ids = %q, want %q", got, wantNotes)
			}
		})
	}
}

// TestFormatFixture_LogKeepsFormat proves a write that needs no newer format leaves a store at its
// own: capcom log on each fixture, the oldest included, keeps config.yaml byte for byte and warns
// of no raise.
func TestFormatFixture_LogKeepsFormat(t *testing.T) {
	for format := config.MissingFormat; format <= config.FormatVersion; format++ {
		t.Run(fmt.Sprintf("format-%d", format), func(t *testing.T) {
			root := copyFixtureStore(t, format)
			before := readStoreConfig(t, root)

			stdout := mustRun(t, root, "log", fixtureReadyIssue, "Drafted the release script.")

			if after := readStoreConfig(t, root); after != before {
				t.Errorf("config.yaml after log =\n%s\nwant it unchanged:\n%s", after, before)
			}
			if strings.Contains(stdout, "store format raised") {
				t.Errorf("log stdout = %q, want no format raise warning", stdout)
			}
		})
	}
}

// TestFormatFixture_StandingRaisesFormat pins today's raise rule on a real store of the format just
// below config.FormatStanding: creating a standing issue raises the store to FormatStanding,
// rewriting only the format line, and warns once that older releases can no longer read it.
func TestFormatFixture_StandingRaisesFormat(t *testing.T) {
	const fromFormat = config.FormatStanding - 1
	root := copyFixtureStore(t, fromFormat)
	before := readStoreConfig(t, root)

	stdout := mustRun(t, root, "create", "--id", "feature-requests", "--title", "Feature requests", "--standing")

	want := "warning: " + fmt.Sprintf(formatRaisedWarning, ".capcom/config.yaml", config.FormatStanding, fromFormat) + "\n"
	if count := strings.Count(stdout, want); count != 1 {
		t.Errorf("create stdout = %q, want the warning %q once, got %d", stdout, want, count)
	}
	wantConfig := strings.Replace(before, fmt.Sprintf("\nformat: %d\n", fromFormat), fmt.Sprintf("\nformat: %d\n", config.FormatStanding), 1)
	if after := readStoreConfig(t, root); wantConfig == before || after != wantConfig {
		t.Errorf("config.yaml after create =\n%s\nwant only its format line raised to %d:\n%s", after, config.FormatStanding, wantConfig)
	}
}

// TestFormatFixture_NewerFormatRefused proves every command refuses a store one format past
// config.FormatVersion with the same message: prime as its one warning, exiting 0; check as its one
// config problem, exiting 1; every other command as its error, writing nothing.
func TestFormatFixture_NewerFormatRefused(t *testing.T) {
	const issueFile = "newer-issue.md"
	tests := []struct {
		name string
		args []string
	}{
		{name: "reindex", args: []string{"reindex"}},
		{name: "list", args: []string{"list"}},
		{name: "ready", args: []string{"ready"}},
		{name: "know tree", args: []string{"know", "tree"}},
		{name: "show", args: []string{"show", "newer-issue"}},
		{name: "log", args: []string{"log", "newer-issue", "A note a newer store must not take."}},
		{name: "create", args: []string{"create", "--id", "new-issue", "--title", "New issue"}},
	}
	root := copyStoreDir(t, newerStoreDir)
	writeNewerFormatConfig(t, root)
	commitFixtureStore(t, root, fmt.Sprintf("format %d store", newerFormat))
	issueBefore := readStoreFile(t, root, issueFile)

	t.Run("prime", func(t *testing.T) {
		stdout, stderr, exitCode := runCLI(t, root, "prime")

		if exitCode != ExitOK || stderr != "" || stdout != "warning: store: "+newerFormatMessage+"\n" {
			t.Errorf("prime = %d, stdout %q, stderr %q; want 0 and one too-new store warning", exitCode, stdout, stderr)
		}
	})
	t.Run("check", func(t *testing.T) {
		stdout, stderr, exitCode := runCLI(t, root, "check")

		if exitCode != ExitError || stdout != newerFormatMessage+"\n" || stderr != "" {
			t.Errorf("check = %d, stdout %q, stderr %q; want 1 and the too-new config problem", exitCode, stdout, stderr)
		}
	})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, root, test.args...)

			if exitCode == ExitOK || stdout != "" || !strings.HasPrefix(stderr, "capcom: "+newerFormatMessage+"\n") {
				t.Errorf("%v = %d, stdout %q, stderr %q; want a failure carrying %q", test.args, exitCode, stdout, stderr, newerFormatMessage)
			}
		})
	}

	if issueAfter := readStoreFile(t, root, issueFile); issueAfter != issueBefore {
		t.Errorf("%s after the refused commands =\n%s\nwant it unchanged:\n%s", issueFile, issueAfter, issueBefore)
	}
	if _, err := os.Stat(filepath.Join(root, ".capcom", "issues", "new-issue.md")); !os.IsNotExist(err) {
		t.Errorf("new-issue.md stat err = %v, want the refused create to write no file", err)
	}
}

// TestFormatFixture_CurrentFormatHasFixture fails when the newest store format has no fixture: a
// raised config.FormatVersion needs its testdata/stores/format-<n> store, declaring that format.
func TestFormatFixture_CurrentFormatHasFixture(t *testing.T) {
	path := filepath.Join(fixtureStoreDir(config.FormatVersion), "config.yaml")

	data, err := os.ReadFile(path)

	if err != nil {
		t.Fatalf("no fixture store for store format %d: %v", config.FormatVersion, err)
	}
	if want := fmt.Sprintf("\nformat: %d\n", config.FormatVersion); !strings.Contains(string(data), want) {
		t.Errorf("%s does not declare format %d", path, config.FormatVersion)
	}
}
