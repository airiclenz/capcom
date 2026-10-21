package hooks_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/airiclenz/capcom/internal/hooks"
)

// apogeeReaction is the capcom prime reaction MergeApogee adds, as an item of a reactions list
// whose dash sits two spaces in.
const apogeeReaction = `  - id: capcom-prime
    on: [session-started, compaction-finished]
    advise: ["capcom", "prime"]
`

// apogeeCommented is a config whose reactions list carries comments, blank lines and a trailing
// key, so a merge must keep each byte around the item it inserts.
const apogeeCommented = `# project config
allow: [go test]
reactions:
  # lint before a stop
  - id: lint
    on: [stop]   # only at the end
    gate: ["make", "lint"]

  - id: fmt
    on: [edit]
    advise: ["gofmt"]
    # trailing item comment

# tail
model: x
`

// apogeeCommentedMerged is apogeeCommented with the capcom prime reaction after its last item.
const apogeeCommentedMerged = `# project config
allow: [go test]
reactions:
  # lint before a stop
  - id: lint
    on: [stop]   # only at the end
    gate: ["make", "lint"]

  - id: fmt
    on: [edit]
    advise: ["gofmt"]
    # trailing item comment
` + apogeeReaction + `
# tail
model: x
`

func TestMergeApogee_UsableContent_AddsTheReaction(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantMerged string
	}{
		{"empty content", "", "reactions:\n" + apogeeReaction},
		{"only comments", "# nothing yet\n", "# nothing yet\nreactions:\n" + apogeeReaction},
		{"only a document start", "---\n", "---\nreactions:\n" + apogeeReaction},
		{"no reactions, no final line break", "allow: [go test]", "allow: [go test]\nreactions:\n" + apogeeReaction},
		{"existing reactions with comments", apogeeCommented, apogeeCommentedMerged},
		{
			"list dash at the key's column",
			"reactions:\n- id: lint\n  on: [stop]\n  gate: [\"make\"]\nmodel: x\n",
			"reactions:\n- id: lint\n  on: [stop]\n  gate: [\"make\"]\n" +
				"- id: capcom-prime\n  on: [session-started, compaction-finished]\n  advise: [\"capcom\", \"prime\"]\n" +
				"model: x\n",
		},
		{
			"anchored list",
			"reactions: &r\n  - id: lint\n    on: [stop]\nmodel: x\n",
			"reactions: &r\n  - id: lint\n    on: [stop]\n" + apogeeReaction + "model: x\n",
		},
		{
			"tagged list",
			"reactions: !!seq\n  - id: lint\n    on: [stop]\nmodel: x\n",
			"reactions: !!seq\n  - id: lint\n    on: [stop]\n" + apogeeReaction + "model: x\n",
		},
		{
			"anchored list, dash alone on its line",
			"reactions: &r\n  -\n    id: lint\n    on: [stop]\nmodel: x\n",
			"reactions: &r\n  -\n    id: lint\n    on: [stop]\n" + apogeeReaction + "model: x\n",
		},
		{
			"last item on the last line without a line break",
			"reactions:\n  - id: lint\n    on: [stop]",
			"reactions:\n  - id: lint\n    on: [stop]\n" + apogeeReaction,
		},
		{
			"CRLF line endings kept",
			"allow: [go test]\r\nreactions:\r\n  - id: lint\r\n    on: [stop]\r\n",
			"allow: [go test]\r\nreactions:\r\n  - id: lint\r\n    on: [stop]\r\n" +
				"  - id: capcom-prime\r\n    on: [session-started, compaction-finished]\r\n    advise: [\"capcom\", \"prime\"]\r\n",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			before := len(apogeeReactions(t, testCase.content))

			merged, status, err := hooks.MergeApogee([]byte(testCase.content))

			if err != nil || status != hooks.Added || string(merged) != testCase.wantMerged {
				t.Fatalf("MergeApogee = %q, %v; merged:\n%s\nwant added and:\n%s", status, err, merged, testCase.wantMerged)
			}
			reactions := apogeeReactions(t, string(merged))
			if len(reactions) != before+1 {
				t.Fatalf("reactions after merge = %d, want %d", len(reactions), before+1)
			}
			want := []string{"session-started", "compaction-finished"}
			if got := reactionMoments(reactions[len(reactions)-1]); !slices.Equal(got, want) {
				t.Errorf("new reaction on = %q, want %q", got, want)
			}
		})
	}
}

func TestMergeApogee_ReactionAlreadyThere_ReportsPresentAndChangesNothing(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"merged once already", apogeeCommentedMerged},
		{"the id alone", "reactions:\n  - id: capcom-prime\n    on: [stop]\n"},
		{"the advise argv under another id", "reactions:\n  - id: brief\n    on: [session-started]\n    advise: [capcom, prime]\n"},
		{"hand-written as a flow list", "reactions: [{id: capcom-prime, on: [session-started], advise: [capcom, prime]}]\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged, status, err := hooks.MergeApogee([]byte(testCase.content))

			if err != nil || status != hooks.Present || string(merged) != testCase.content {
				t.Errorf("MergeApogee = %q, %v; merged:\n%s\nwant present and the content unchanged", status, err, merged)
			}
		})
	}
}

func TestMergeApogee_UnusableContent_FailsWithPathAndLine(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		wantLine int
		wantText string
	}{
		{"flow reactions list", "allow: [go test]\nreactions: []\n", 2, "reactions must be a block list; add the capcom-prime entry by hand"},
		{"scalar reactions", "reactions: capcom prime\n", 1, "reactions must be a block list; add the capcom-prime entry by hand"},
		{"empty reactions", "reactions:\nmodel: x\n", 1, "reactions must be a block list; add the capcom-prime entry by hand"},
		{"mapping reactions", "reactions:\n  id: lint\n", 1, "reactions must be a block list; add the capcom-prime entry by hand"},
		{"broken YAML", "allow:\n  - go test\nmodel: \"x\n", 3, "invalid config: found unexpected end of stream"},
		{"broken YAML without a line", "a: b: c\n", 0, "invalid config: yaml: mapping values are not allowed in this context"},
		{"document end line", "allow: [go test]\n...\n", 2, "a `...` document end line is not allowed; remove it"},
		{"second document", "a: 1\n---\nb: 2\n", 2, "only one YAML document is allowed"},
		{"flow root mapping", "{allow: [go test]}\n", 1, "the config must be a block mapping; add the capcom-prime entry by hand"},
		{"list document", "- go test\n", 1, "the config must be a mapping of keys"},
		{"explicit null document", "~\n", 1, "the config must be a mapping of keys"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged, status, err := hooks.MergeApogee([]byte(testCase.content))

			var failure *hooks.Error
			want := hooks.Error{Path: hooks.ApogeePath, Line: testCase.wantLine, Message: testCase.wantText}
			if !errors.As(err, &failure) || *failure != want || merged != nil || status != "" {
				t.Errorf("MergeApogee = %q, %q, %v; want nil, %+v", merged, status, err, want)
			}
		})
	}
}

// apogeeConfigPath returns the path of the apogee config under root, its directory created.
func apogeeConfigPath(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(hooks.ApogeePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return path
}

func TestInstallApogee_MissingConfig_CreatesTheFileAndItsDirectory(t *testing.T) {
	root := t.TempDir()

	status, err := hooks.InstallApogee(root)

	content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(hooks.ApogeePath)))
	want := "reactions:\n" + apogeeReaction
	if err != nil || status != hooks.Added || readErr != nil || string(content) != want {
		t.Errorf("InstallApogee = %q, %v; file %v %q; want added and %q", status, err, readErr, content, want)
	}
}

func TestInstallApogee_ExistingConfig_KeepsItsBytesAndMode(t *testing.T) {
	root := t.TempDir()
	path := apogeeConfigPath(t, root)
	if err := os.WriteFile(path, []byte(apogeeCommented), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	status, err := hooks.InstallApogee(root)

	content, readErr := os.ReadFile(path)
	if err != nil || status != hooks.Added || readErr != nil || string(content) != apogeeCommentedMerged {
		t.Errorf("InstallApogee = %q, %v; file %v:\n%s\nwant added and:\n%s", status, err, readErr, content, apogeeCommentedMerged)
	}
	if info, err := os.Stat(path); runtime.GOOS != "windows" && (err != nil || info.Mode().Perm() != 0o600) {
		t.Errorf("config mode = %v %v, want 0600 kept", info, err)
	}
}

func TestInstallApogee_SecondRun_ReportsPresent(t *testing.T) {
	root := t.TempDir()
	if _, err := hooks.InstallApogee(root); err != nil {
		t.Fatalf("first InstallApogee: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(hooks.ApogeePath))
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	status, err := hooks.InstallApogee(root)

	second, readErr := os.ReadFile(path)
	if err != nil || status != hooks.Present || readErr != nil || string(second) != string(first) {
		t.Errorf("second InstallApogee = %q, %v; file %v %q; want present and %q", status, err, readErr, second, first)
	}
}

func TestInstallApogee_RefusedConfig_FailsAndStaysAsItIs(t *testing.T) {
	root := t.TempDir()
	path := apogeeConfigPath(t, root)
	before := "reactions: []\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	status, err := hooks.InstallApogee(root)

	content, readErr := os.ReadFile(path)
	var failure *hooks.Error
	if !errors.As(err, &failure) || failure.Path != hooks.ApogeePath || failure.Line != 1 || status != "" {
		t.Errorf("InstallApogee = %q, %v; want an *Error at %s:1", status, err, hooks.ApogeePath)
	}
	if readErr != nil || string(content) != before {
		t.Errorf("config = %q, %v; want %q untouched", content, readErr, before)
	}
}

func TestInstallApogee_UnusableConfig_FailsWithPathAndStaysAsItIs(t *testing.T) {
	for _, unusable := range unusableFiles {
		t.Run(unusable.name, func(t *testing.T) {
			root := t.TempDir()
			path := apogeeConfigPath(t, root)
			unusable.make(t, path)
			wantType, wantSize := entryMode(t, path)

			status, err := hooks.InstallApogee(root)

			var failure *hooks.Error
			want := hooks.Error{Path: hooks.ApogeePath, Message: unusable.wantMessage}
			if !errors.As(err, &failure) || *failure != want || status != "" {
				t.Errorf("InstallApogee = %q, %v; want %+v", status, err, want)
			}
			if gotType, gotSize := entryMode(t, path); gotType != wantType || gotSize != wantSize {
				t.Errorf("config entry = %v, %d bytes; want %v, %d bytes untouched", gotType, gotSize, wantType, wantSize)
			}
		})
	}
}

// apogeeReactions parses the apogee config content and returns the items of its reactions list,
// none when it has no reactions key.
func apogeeReactions(t *testing.T, content string) []*yaml.Node {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, content)
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	root := document.Content[0]
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == "reactions" {
			return root.Content[index+1].Content
		}
	}
	return nil
}

// reactionMoments returns the values of a reaction's on list.
func reactionMoments(reaction *yaml.Node) []string {
	var moments []string
	for index := 0; index+1 < len(reaction.Content); index += 2 {
		if reaction.Content[index].Value != "on" {
			continue
		}
		for _, moment := range reaction.Content[index+1].Content {
			moments = append(moments, moment.Value)
		}
	}
	return moments
}
