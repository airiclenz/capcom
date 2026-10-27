package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// readStoredLayout returns the layout choice the state file at path holds, "" for none.
func readStoredLayout(t *testing.T, path string) string {
	t.Helper()
	state, err := userconfig.LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	return state.UI.Layout
}

// pressForCommand hands msg to the model's Update and returns the updated model with the command
// it returned, undriven.
func pressForCommand(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	next, isModel := updated.(model)
	if !isModel {
		t.Fatalf("Update returned %T, want model", updated)
	}
	return next, cmd
}

func TestKeys_LayoutKey_TogglesSplitAndStackedAndSavesEach(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	m := startModel(t, options, 120, 30)
	split := m.View().Content

	m = press(t, m, runeKey('v'))
	isStackedAfterFirst := hasStackedDivider(m)
	storedAfterFirst := readStoredLayout(t, options.UserStatePath)
	m = press(t, m, runeKey('v'))

	if !isStackedAfterFirst {
		t.Errorf("after one v: want the stacked layout's divider on screen")
	}
	if storedAfterFirst != userconfig.LayoutStacked {
		t.Errorf("stored layout after one v = %q, want %q", storedAfterFirst, userconfig.LayoutStacked)
	}
	if got := m.View().Content; got != split {
		t.Errorf("after two v:\n%s\nwant the split layout\n%s", got, split)
	}
	if got := readStoredLayout(t, options.UserStatePath); got != userconfig.LayoutSplit {
		t.Errorf("stored layout after two v = %q, want %q", got, userconfig.LayoutSplit)
	}
}

func TestKeys_LayoutKey_WhereOnlyOneLayoutFits_ChangesNothing(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "80x40 too narrow", width: 80, height: 40},
		{name: "120x24 too short", width: 120, height: 24},
		{name: "80x24 one pane", width: 80, height: 24},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			m := startModel(t, options, testCase.width, testCase.height)
			before := m.View().Content

			m, cmd := pressForCommand(t, m, runeKey('v'))

			if cmd != nil {
				t.Errorf("v returned a command, want none")
			}
			if m.layoutChoice != "" {
				t.Errorf("layoutChoice = %q, want none", m.layoutChoice)
			}
			if got := m.View().Content; got != before {
				t.Errorf("after v:\n%s\nwant the screen unchanged\n%s", got, before)
			}
		})
	}
}

func TestKeys_LayoutKey_WhileAnotherViewTakesKeys_LeavesTheLayout(t *testing.T) {
	cases := []struct {
		name string
		open func(t *testing.T, m model) model
		want func(t *testing.T, m model)
	}{
		{
			name: "help screen",
			open: func(t *testing.T, m model) model { return press(t, m, runeKey('?')) },
			want: func(t *testing.T, m model) {
				if !m.isHelpOpen {
					t.Errorf("help screen closed by v, want it open")
				}
			},
		},
		{
			name: "confirm prompt",
			open: func(t *testing.T, m model) model {
				m.prompt = newConfirmPrompt("go on?", nil)
				return m
			},
			want: func(t *testing.T, m model) {
				if !m.prompt.isOpen {
					t.Errorf("prompt closed by v, want it open")
				}
			},
		},
		{
			name: "text prompt",
			open: func(t *testing.T, m model) model { return press(t, m, runeKey('L')) },
			want: func(t *testing.T, m model) {
				if got := m.textPrompt.line.value(); got != "v" {
					t.Errorf("text prompt = %q, want v typed into it", got)
				}
			},
		},
		{
			name: "filter line",
			open: func(t *testing.T, m model) model { return press(t, m, runeKey('/')) },
			want: func(t *testing.T, m model) {
				if got := m.filterLine.value(); got != "v" {
					t.Errorf("filter line = %q, want v typed into it", got)
				}
			},
		},
		{
			name: "status picker",
			open: func(t *testing.T, m model) model { return press(t, m, runeKey('s')) },
			want: func(t *testing.T, m model) {
				if !m.picker.isOpen {
					t.Errorf("status picker closed by v, want it open")
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			m := testCase.open(t, startModel(t, options, 120, 30))

			m = press(t, m, runeKey('v'))

			testCase.want(t, m)
			if m.layoutChoice != "" {
				t.Errorf("layoutChoice = %q, want none", m.layoutChoice)
			}
			if _, err := os.Stat(options.UserStatePath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("state file after v: Stat error %v, want it not written", err)
			}
		})
	}
}

func TestKeys_LayoutKey_UnwritableStateFile_ShowsTheLayoutSaveNoticeAndStillToggles(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	writeTestFile(t, parent, "a regular file\n")
	options.UserStatePath = filepath.Join(parent, "state.yaml")
	m := startModel(t, options, 120, 30)

	m = press(t, m, runeKey('v'))

	if !hasStackedDivider(m) {
		t.Errorf("after v: want the stacked layout though it cannot be saved")
	}
	if !strings.HasPrefix(m.stateNotice, "cannot save the layout: ") {
		t.Errorf("state notice = %q, want it to open with %q", m.stateNotice, "cannot save the layout: ")
	}
}
