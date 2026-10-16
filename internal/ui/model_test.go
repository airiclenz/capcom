package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const validIssue = "---\ntitle: Valid\nstatus: open\ntype: task\npriority: 2\ncreated: 2026-10-11T09:00:00Z\n---\n"

func TestModel_Update_QuitKeys_ReturnQuit(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "q", key: runeKey('q')},
		{name: "ctrl+c", key: ctrlKey('c')},
	}
	ts := newTestStore(t, nil)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, 120, 30)

			_, cmd := m.Update(testCase.key)

			if cmd == nil {
				t.Fatalf("%s: Update returned no command, want tea.Quit", testCase.name)
			}
			if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
				t.Errorf("%s: command returned %T, want tea.QuitMsg", testCase.name, cmd())
			}
		})
	}
}

func TestModel_Update_OtherKey_DoesNotQuit(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)

	_, cmd := m.Update(runeKey('x'))

	if cmd != nil {
		t.Errorf("x: Update returned a command, want none")
	}
}

func TestModel_Update_UnicodeCoreModeReport_SwitchesMarkdownMeasureToGraphemeWidth(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)
	if got := m.markdownTheme.Measure.Method(); got != ansi.WcWidth {
		t.Fatalf("markdown measure before the report = %v, want the painter's starting ansi.WcWidth", got)
	}

	m = send(t, m, tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})

	if got := m.markdownTheme.Measure.Method(); got != ansi.GraphemeWidth {
		t.Errorf("markdown measure after a mode-2027 report = %v, want ansi.GraphemeWidth", got)
	}
}

func TestModel_View_BeforeWindowSize_IsEmpty(t *testing.T) {
	m := newModel(newTestOptions(t, newTestStore(t, nil)))

	view := m.View()

	if view.Content != "" {
		t.Errorf("View before a window size = %q, want empty", view.Content)
	}
}

func TestParseTab_Names(t *testing.T) {
	cases := []struct {
		name    string
		want    Tab
		isValid bool
	}{
		{name: "issues", want: TabIssues, isValid: true},
		{name: "ready", want: TabReady, isValid: true},
		{name: "knowledge", want: TabKnowledge, isValid: true},
		{name: "bogus", want: TabReady, isValid: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ParseTab(testCase.name)

			if (err == nil) != testCase.isValid {
				t.Fatalf("ParseTab(%q) error = %v, want valid %t", testCase.name, err, testCase.isValid)
			}
			if got != testCase.want {
				t.Errorf("ParseTab(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}
