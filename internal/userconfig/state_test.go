package userconfig_test

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// stateHeader is the first line SaveState writes.
const stateHeader = "# capcom ui state: written by capcom ui, not meant for editing (ADR 0014).\n"

// ratioMessage is the error LoadState reports for a sidebar_ratio it cannot use.
const ratioMessage = "ui.sidebar_ratio: expected a fraction of the terminal's width, more than 0 and less than 1"

// stackedRatioMessage is the error LoadState reports for a stacked_ratio it cannot use.
const stackedRatioMessage = "ui.stacked_ratio: expected a fraction of the body's height, more than 0 and less than 1"

// statePath returns a state file path inside a fresh temp dir whose capcom directory does not exist
// yet.
func statePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "capcom", "state.yaml")
}

// writeState writes content as the state file at a fresh path and returns that path.
func writeState(t *testing.T, content string) string {
	t.Helper()
	path := statePath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestStatePath_UserConfigDir_EndsInCapcomState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := userconfig.StatePath()

	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	if want := filepath.Join("capcom", "state.yaml"); !strings.HasSuffix(path, want) {
		t.Errorf("StatePath() = %q, want a path ending in %q", path, want)
	}
}

func TestSaveState_LoadState_RoundTrips(t *testing.T) {
	tests := []struct {
		name    string
		ui      userconfig.UIState
		content string
	}{
		{"no ratio stored", userconfig.UIState{}, stateHeader},
		{
			"only a sidebar ratio",
			userconfig.UIState{SidebarRatio: 44.0 / 120.0},
			stateHeader + "ui:\n  sidebar_ratio: 0.36666666666666664\n",
		},
		{
			"only a stacked ratio",
			userconfig.UIState{StackedRatio: 12.0 / 36.0},
			stateHeader + "ui:\n  stacked_ratio: 0.3333333333333333\n",
		},
		{
			"both ratios",
			userconfig.UIState{SidebarRatio: 0.3, StackedRatio: 0.45},
			stateHeader + "ui:\n  sidebar_ratio: 0.3\n  stacked_ratio: 0.45\n",
		},
		{
			"only a layout choice",
			userconfig.UIState{Layout: userconfig.LayoutStacked},
			stateHeader + "ui:\n  layout: stacked\n",
		},
		{
			"both ratios and a layout choice, layout last",
			userconfig.UIState{SidebarRatio: 0.3, StackedRatio: 0.45, Layout: userconfig.LayoutSplit},
			stateHeader + "ui:\n  sidebar_ratio: 0.3\n  stacked_ratio: 0.45\n  layout: split\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := statePath(t)
			saved := userconfig.State{UI: test.ui}

			saveErr := userconfig.SaveState(path, saved)
			loaded, loadErr := userconfig.LoadState(path)

			if saveErr != nil || loadErr != nil {
				t.Fatalf("SaveState() error = %v, LoadState() error = %v", saveErr, loadErr)
			}
			if loaded != saved {
				t.Errorf("LoadState() = %#v, want %#v", loaded, saved)
			}
			if got := readFile(t, path); got != test.content {
				t.Errorf("state file = %q, want %q", got, test.content)
			}
		})
	}
}

func TestSaveState_ExistingFile_ReplacesItAndLeavesNoTempFile(t *testing.T) {
	path := writeState(t, "ui:\n  sidebar_ratio: 0.25\n")
	want := userconfig.State{UI: userconfig.UIState{SidebarRatio: 0.4}}

	err := userconfig.SaveState(path, want)

	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if got, err := userconfig.LoadState(path); err != nil || got != want {
		t.Errorf("LoadState() = %#v, %v; want %#v", got, err, want)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.yaml" {
		t.Errorf("directory holds %v, want only state.yaml", entries)
	}
}

func TestSaveState_MissingDirectory_CreatesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "capcom", "state.yaml")

	err := userconfig.SaveState(path, userconfig.State{UI: userconfig.UIState{SidebarRatio: 0.3}})

	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Stat(%s) error = %v, want the file", path, err)
	}
}

func TestSaveState_ParentIsAFile_Errors(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "capcom")
	if err := os.WriteFile(parent, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := userconfig.SaveState(
		filepath.Join(parent, "state.yaml"),
		userconfig.State{UI: userconfig.UIState{SidebarRatio: 0.3}},
	)

	if err == nil {
		t.Errorf("SaveState() error = nil, want an error")
	}
}

func TestSaveState_RatioOutOfRange_ErrorsAndLeavesTheFile(t *testing.T) {
	tests := []struct {
		name  string
		ui    userconfig.UIState
		field string
	}{
		{"sidebar ratio one", userconfig.UIState{SidebarRatio: 1}, "sidebar ratio"},
		{"sidebar ratio more than one", userconfig.UIState{SidebarRatio: 1.5}, "sidebar ratio"},
		{"sidebar ratio negative", userconfig.UIState{SidebarRatio: -0.2}, "sidebar ratio"},
		{"sidebar ratio NaN", userconfig.UIState{SidebarRatio: math.NaN()}, "sidebar ratio"},
		{"stacked ratio one", userconfig.UIState{SidebarRatio: 0.3, StackedRatio: 1}, "stacked ratio"},
		{"stacked ratio NaN", userconfig.UIState{StackedRatio: math.NaN()}, "stacked ratio"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const existing = "ui:\n  sidebar_ratio: 0.25\n  stacked_ratio: 0.4\n"
			path := writeState(t, existing)

			err := userconfig.SaveState(path, userconfig.State{UI: test.ui})

			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Errorf("SaveState() error = %v, want an error naming %q", err, test.field)
			}
			if got := readFile(t, path); got != existing {
				t.Errorf("state file = %q, want it left as %q", got, existing)
			}
		})
	}
}

func TestSaveState_UnknownLayout_ErrorsAndLeavesTheFile(t *testing.T) {
	const existing = "ui:\n  sidebar_ratio: 0.25\n  layout: split\n"
	path := writeState(t, existing)

	err := userconfig.SaveState(
		path,
		userconfig.State{UI: userconfig.UIState{SidebarRatio: 0.3, Layout: "auto"}},
	)

	const want = `save the state file: layout "auto" is not split or stacked`
	if err == nil || err.Error() != want {
		t.Errorf("SaveState() error = %v, want %q", err, want)
	}
	if got := readFile(t, path); got != existing {
		t.Errorf("state file = %q, want it left as %q", got, existing)
	}
}

func TestLoadState_UnknownLayout_LoadsNoChoiceAndKeepsTheRatios(t *testing.T) {
	tests := []struct {
		name   string
		layout string
	}{
		{"an unknown name", "auto"},
		{"a number", "3"},
		{"a list", "[a]"},
		{"a mapping", "{split: true}"},
		{"null", "~"},
		{"a known name in the wrong case", "Stacked"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeState(
				t,
				stateHeader+"ui:\n  sidebar_ratio: 0.3\n  stacked_ratio: 0.45\n  layout: "+test.layout+"\n",
			)

			state, err := userconfig.LoadState(path)

			want := userconfig.State{UI: userconfig.UIState{SidebarRatio: 0.3, StackedRatio: 0.45}}
			if err != nil || state != want {
				t.Errorf("LoadState() = %#v, %v; want %#v and no error", state, err, want)
			}
		})
	}
}

func TestLoadState_QuotedLayout_LoadsTheChoice(t *testing.T) {
	path := writeState(t, stateHeader+"ui:\n  layout: \"stacked\"\n")

	state, err := userconfig.LoadState(path)

	want := userconfig.State{UI: userconfig.UIState{Layout: userconfig.LayoutStacked}}
	if err != nil || state != want {
		t.Errorf("LoadState() = %#v, %v; want %#v and no error", state, err, want)
	}
}

func TestLoadState_MissingFile_ZeroAndNothingCreated(t *testing.T) {
	path := statePath(t)

	state, err := userconfig.LoadState(path)

	if err != nil || state != (userconfig.State{}) {
		t.Fatalf("LoadState() = %#v, %v; want the zero State and no error", state, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(%s) error = %v, want nothing created", filepath.Dir(path), err)
	}
}

func TestLoadState_UnknownKeys_IgnoredAndRatioKept(t *testing.T) {
	path := writeState(t, "future: 1\nui:\n  sidebar_ratio: 0.3\n  theme: dark\n")

	state, err := userconfig.LoadState(path)

	if want := (userconfig.State{UI: userconfig.UIState{SidebarRatio: 0.3}}); err != nil || state != want {
		t.Errorf("LoadState() = %#v, %v; want %#v and no error", state, err, want)
	}
}

func TestLoadState_OnlyASidebarRatio_LoadsNoStackedRatio(t *testing.T) {
	path := writeState(t, stateHeader+"ui:\n  sidebar_ratio: 0.3\n")

	state, err := userconfig.LoadState(path)

	if want := (userconfig.State{UI: userconfig.UIState{SidebarRatio: 0.3}}); err != nil || state != want {
		t.Errorf("LoadState() = %#v, %v; want %#v and no error", state, err, want)
	}
}

func TestLoadState_OnlyASidebarWidth_LoadsTheZeroState(t *testing.T) {
	path := writeState(t, "ui:\n  sidebar_width: 36\n")

	state, err := userconfig.LoadState(path)

	if err != nil || state != (userconfig.State{}) {
		t.Errorf("LoadState() = %#v, %v; want the zero State and no error", state, err)
	}
}

func TestLoadState_InvalidFile_ErrorsWithLineOnZero(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    int
		message string
	}{
		{"malformed YAML", "ui:\n  sidebar_ratio: 0.3\n  other: [\n", 3, "invalid YAML"},
		{"ratio not a number", "ui:\n  sidebar_ratio: wide\n", 2, ratioMessage},
		{"ratio zero", "ui:\n  sidebar_ratio: 0\n", 2, ratioMessage},
		{"ratio one", "ui:\n  sidebar_ratio: 1\n", 2, ratioMessage},
		{"ratio more than one", "ui:\n  sidebar_ratio: 1.5\n", 2, ratioMessage},
		{"ratio negative", "ui:\n  sidebar_ratio: -0.2\n", 2, ratioMessage},
		{"ratio NaN", "ui:\n  sidebar_ratio: .nan\n", 2, ratioMessage},
		{"ratio infinite", "ui:\n  sidebar_ratio: .inf\n", 2, ratioMessage},
		{"ratio quoted", "ui:\n  sidebar_ratio: \"0.3\"\n", 2, ratioMessage},
		{"ratio a list", "ui:\n  sidebar_ratio: [0.3]\n", 2, ratioMessage},
		{"stacked ratio zero", "ui:\n  sidebar_ratio: 0.3\n  stacked_ratio: 0\n", 3, stackedRatioMessage},
		{"stacked ratio one", "ui:\n  stacked_ratio: 1\n", 2, stackedRatioMessage},
		{"stacked ratio two", "ui:\n  stacked_ratio: 2\n", 2, stackedRatioMessage},
		{"stacked ratio NaN", "ui:\n  stacked_ratio: .nan\n", 2, stackedRatioMessage},
		{"stacked ratio quoted", "ui:\n  stacked_ratio: \"0.3\"\n", 2, stackedRatioMessage},
		{"ui of the wrong type", "ui: 36\n", 1, "ui: expected a mapping"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeState(t, stateHeader+test.content)

			state, err := userconfig.LoadState(path)

			var failure *userconfig.Error
			if !errors.As(err, &failure) {
				t.Fatalf("LoadState() error = %v, want *userconfig.Error", err)
			}
			prefix := path + ":" + strconv.Itoa(test.line+1) + ": "
			if !strings.HasPrefix(err.Error(), prefix) || !strings.Contains(err.Error(), test.message) {
				t.Errorf("LoadState() error = %q, want prefix %q containing %q", err, prefix, test.message)
			}
			if state != (userconfig.State{}) {
				t.Errorf("LoadState() = %#v, want the zero State", state)
			}
		})
	}
}
