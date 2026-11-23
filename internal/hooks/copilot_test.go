package hooks_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/airiclenz/capcom/internal/hooks"
)

// copilotPrimeEntry is the sessionStart entry MergeCopilot adds, as it is laid out inside the
// sessionStart array.
const copilotPrimeEntry = `      {
        "type": "command",
        "command": "capcom prime --additional-context"
      }`

// copilotCreated is the Copilot hooks file MergeCopilot writes for empty content.
const copilotCreated = `{
  "version": 1,
  "hooks": {
    "sessionStart": [
` + copilotPrimeEntry + `
    ]
  }
}
`

func TestMergeCopilot_UsableContent_AddsTheHook(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantMerged string
	}{
		{"empty content", "", copilotCreated},
		{"null", " null\n", copilotCreated},
		{
			"member order and value bytes kept",
			`{"z": 1.50, "version": 1, "hooks": {"stop": [{"x":1}]}, "a": "<&>"}`,
			`{
  "z": 1.50,
  "version": 1,
  "hooks": {
    "stop": [
      {
        "x": 1
      }
    ],
    "sessionStart": [
` + copilotPrimeEntry + `
    ]
  },
  "a": "<&>"
}
`,
		},
		{
			"no version, other members kept after it",
			`{"z": 1.50, "a": "<&>"}`,
			`{
  "version": 1,
  "z": 1.50,
  "a": "<&>",
  "hooks": {
    "sessionStart": [
` + copilotPrimeEntry + `
    ]
  }
}
`,
		},
		{
			"existing sessionStart entries kept, ours appended",
			`{"version": 1, "hooks": {"sessionStart": [{"type": "command", "bash": "./setup.sh", "timeoutSec": 30}, "not an entry"]}}`,
			`{
  "version": 1,
  "hooks": {
    "sessionStart": [
      {
        "type": "command",
        "bash": "./setup.sh",
        "timeoutSec": 30
      },
      "not an entry",
` + copilotPrimeEntry + `
    ]
  }
}
`,
		},
		{
			"plain capcom prime is not the hook",
			`{"version": 1, "hooks": {"sessionStart": [{"type": "command", "command": "capcom prime"}]}}`,
			`{
  "version": 1,
  "hooks": {
    "sessionStart": [
      {
        "type": "command",
        "command": "capcom prime"
      },
` + copilotPrimeEntry + `
    ]
  }
}
`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged, status, err := hooks.MergeCopilot([]byte(testCase.content))

			if err != nil || status != hooks.Added || string(merged) != testCase.wantMerged {
				t.Errorf("MergeCopilot = %q, %v, merged:\n%s\nwant added, nil, merged:\n%s", status, err, merged, testCase.wantMerged)
			}
		})
	}
}

func TestMergeCopilot_HookAlreadyThere_ReportsPresentAndChangesNothing(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"merged once already", copilotCreated},
		{"the command", `{"version": 1, "hooks": {"sessionStart": [{"type": "command", "command": "capcom prime --additional-context"}]}}`},
		{"the bash command", `{"version": 1, "hooks": {"sessionStart": [{"type": "command", "bash": "capcom prime --additional-context"}]}}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged, status, err := hooks.MergeCopilot([]byte(testCase.content))

			if err != nil || status != hooks.Present || string(merged) != testCase.content {
				t.Errorf("MergeCopilot = %q, %v; merged:\n%s\nwant present and the content unchanged", status, err, merged)
			}
		})
	}
}

func TestMergeCopilot_SecondMerge_ChangesNothing(t *testing.T) {
	first, _, err := hooks.MergeCopilot([]byte(`{"hooks": {"sessionStart": [{"bash": "./setup.sh"}]}}`))
	if err != nil {
		t.Fatalf("first MergeCopilot: %v", err)
	}

	second, status, err := hooks.MergeCopilot(first)

	if err != nil || status != hooks.Present || string(second) != string(first) {
		t.Errorf("second MergeCopilot = %q, %v, merged:\n%s\nwant present, nil and the first result", status, err, second)
	}
}

// TestMergeCopilot_UnusableContent_FailsWithPathAndLine pins capcom's own wording. A syntax error's
// message carries the JSON decoder's prose after capcom's prefix, which differs between the
// encoding/json v1 and v2 decoders, so that case matches the prefix only: isPrefix marks it.
func TestMergeCopilot_UnusableContent_FailsWithPathAndLine(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		want     hooks.Error
		isPrefix bool
	}{
		{
			"syntax error",
			"{\n  \"version\": 1,\n  oops\n}\n",
			hooks.Error{Path: ".github/hooks/capcom.json", Line: 3, Message: "invalid settings: "},
			true,
		},
		{
			"not an object",
			"[1, 2]\n",
			hooks.Error{Path: ".github/hooks/capcom.json", Message: "invalid settings: must be a JSON object"},
			false,
		},
		{
			"version 2",
			`{"version": 2, "hooks": {}}`,
			hooks.Error{Path: ".github/hooks/capcom.json", Message: "version must be 1"},
			false,
		},
		{
			"version a string",
			`{"version": "1"}`,
			hooks.Error{Path: ".github/hooks/capcom.json", Message: "version must be 1"},
			false,
		},
		{
			"hooks an array",
			`{"version": 1, "hooks": []}`,
			hooks.Error{Path: ".github/hooks/capcom.json", Message: "hooks must be a JSON object"},
			false,
		},
		{
			"sessionStart an object",
			`{"version": 1, "hooks": {"sessionStart": {}}}`,
			hooks.Error{Path: ".github/hooks/capcom.json", Message: "hooks.sessionStart must be an array of hooks"},
			false,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged, status, err := hooks.MergeCopilot([]byte(testCase.content))

			var failure *hooks.Error
			if !errors.As(err, &failure) || merged != nil || status != "" ||
				failure.Path != testCase.want.Path || failure.Line != testCase.want.Line ||
				!messageMatches(failure.Message, testCase.want.Message, testCase.isPrefix) {
				t.Errorf("MergeCopilot = %q, %q, %v; want nil, %+v (message prefix only: %v)", merged, status, err, testCase.want, testCase.isPrefix)
			}
		})
	}
}

// copilotHooksPath returns the path of the Copilot hooks file under root, its directory created.
func copilotHooksPath(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(hooks.CopilotPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return path
}

func TestInstallCopilot_MissingFile_CreatesTheFileAndItsDirectory(t *testing.T) {
	root := t.TempDir()

	status, err := hooks.InstallCopilot(root)

	content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(hooks.CopilotPath)))
	if err != nil || status != hooks.Added || readErr != nil || string(content) != copilotCreated {
		t.Errorf("InstallCopilot = %q, %v; file %v:\n%s\nwant added and:\n%s", status, err, readErr, content, copilotCreated)
	}
}

func TestInstallCopilot_ExistingFile_KeepsItsMode(t *testing.T) {
	root := t.TempDir()
	path := copilotHooksPath(t, root)
	before := []byte("{\"version\": 1, \"hooks\": {\"stop\": []}}\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	want, _, err := hooks.MergeCopilot(before)
	if err != nil {
		t.Fatalf("MergeCopilot: %v", err)
	}

	status, err := hooks.InstallCopilot(root)

	content, readErr := os.ReadFile(path)
	if err != nil || status != hooks.Added || readErr != nil || string(content) != string(want) {
		t.Errorf("InstallCopilot = %q, %v; file %v %q; want added and %q", status, err, readErr, content, want)
	}
	if info, err := os.Stat(path); runtime.GOOS != "windows" && (err != nil || info.Mode().Perm() != 0o600) {
		t.Errorf("hooks file mode = %v %v, want 0600 kept", info, err)
	}
}

func TestInstallCopilot_SecondRun_ReportsPresent(t *testing.T) {
	root := t.TempDir()
	if _, err := hooks.InstallCopilot(root); err != nil {
		t.Fatalf("first InstallCopilot: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(hooks.CopilotPath))
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	status, err := hooks.InstallCopilot(root)

	second, readErr := os.ReadFile(path)
	if err != nil || status != hooks.Present || readErr != nil || string(second) != string(first) {
		t.Errorf("second InstallCopilot = %q, %v; file %v %q; want present and %q", status, err, readErr, second, first)
	}
}

func TestInstallCopilot_RefusedFile_FailsAndStaysAsItIs(t *testing.T) {
	root := t.TempDir()
	path := copilotHooksPath(t, root)
	before := "{\"version\": 2}\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	status, err := hooks.InstallCopilot(root)

	content, readErr := os.ReadFile(path)
	var failure *hooks.Error
	want := hooks.Error{Path: hooks.CopilotPath, Message: "version must be 1"}
	if !errors.As(err, &failure) || *failure != want || status != "" {
		t.Errorf("InstallCopilot = %q, %v; want %+v", status, err, want)
	}
	if readErr != nil || string(content) != before {
		t.Errorf("hooks file = %q, %v; want %q untouched", content, readErr, before)
	}
}

func TestInstallCopilot_UnusableFile_FailsWithPathAndStaysAsItIs(t *testing.T) {
	for _, unusable := range unusableFiles {
		t.Run(unusable.name, func(t *testing.T) {
			root := t.TempDir()
			path := copilotHooksPath(t, root)
			unusable.make(t, path)
			wantType, wantSize := entryMode(t, path)

			status, err := hooks.InstallCopilot(root)

			var failure *hooks.Error
			want := hooks.Error{Path: hooks.CopilotPath, Message: unusable.wantMessage}
			if !errors.As(err, &failure) || *failure != want || status != "" {
				t.Errorf("InstallCopilot = %q, %v; want %+v", status, err, want)
			}
			if gotType, gotSize := entryMode(t, path); gotType != wantType || gotSize != wantSize {
				t.Errorf("hooks file entry = %v, %d bytes; want %v, %d bytes untouched", gotType, gotSize, wantType, wantSize)
			}
		})
	}
}
