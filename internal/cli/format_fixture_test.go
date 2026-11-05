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

// copyFixtureStore copies the hand-written store testdata/stores/format-<format> into the .capcom
// directory of a fresh git repository and commits it, returning the repository's root.
func copyFixtureStore(t *testing.T, format int) string {
	t.Helper()
	source, err := filepath.Abs(fixtureStoreDir(format))
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("no fixture store for format %d: %v", format, err)
	}
	root := newGitRepo(t)
	target := filepath.Join(root, ".capcom")
	if err := os.CopyFS(target, os.DirFS(source)); err != nil {
		t.Fatalf("CopyFS %s: %v", source, err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", fmt.Sprintf("format %d fixture", format))
	return root
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
