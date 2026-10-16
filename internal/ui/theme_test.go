package ui

import (
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// themeFiles is a store whose Ready queue is alpha (P1) then beta (P2), both open; alpha's body
// has a heading, a link to beta, an inert URL, a quote and a code block.
var themeFiles = map[string]string{
	"issues/alpha.md": issueFile(
		"title: Alpha task\nstatus: open\npriority: 1\n",
		"## Plan\n\nSee `beta` and [the docs](https://example.com).\n\n> quoted\n\n```\ncode block\n```\n",
	),
	"issues/beta.md": issueFile("title: Beta task\nstatus: open\npriority: 2\n", "Beta's body.\n"),
}

// sgrSequence matches one SGR escape sequence, its parameters captured.
var sgrSequence = regexp.MustCompile("\x1b\\[([0-9;:]*)m")

// newThemeModel returns a started model over themeFiles on the Ready tab at 120 by 30, its user
// config file holding configText and NO_COLOR set when isNoColor.
func newThemeModel(t *testing.T, configText string, isNoColor bool) model {
	t.Helper()
	options := newTestOptions(t, newTestStore(t, themeFiles))
	if err := os.MkdirAll(filepath.Dir(options.UserConfigPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(options.UserConfigPath, []byte(configText), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	options.NoColor = isNoColor
	return startModel(t, options, 120, 30)
}

// colourSequences returns the SGR sequences of view that set a foreground or background colour:
// those with a 38 or 48 parameter.
func colourSequences(view string) []string {
	var found []string
	for _, match := range sgrSequence.FindAllStringSubmatch(view, -1) {
		for _, parameter := range strings.Split(match[1], ";") {
			introducer, _, _ := strings.Cut(parameter, ":")
			if introducer == "38" || introducer == "48" {
				found = append(found, match[0])
				break
			}
		}
	}
	return found
}

// detailLine returns the detail pane line whose text, styling stripped, is text, failing the test
// when there is none.
func detailLine(t *testing.T, m model, text string) string {
	t.Helper()
	for _, line := range m.detail {
		if strings.TrimSpace(ansi.Strip(line)) == text {
			return line
		}
	}
	t.Fatalf("no detail line %q in %q", text, detailText(m))
	return ""
}

func TestTheme_Presets_MatchANSIGoldens(t *testing.T) {
	testCases := []struct {
		name       string
		configText string
		isNoColor  bool
	}{
		{name: "theme-dark-120x30", configText: "ui:\n  theme: dark\n"},
		{name: "theme-light-120x30", configText: "ui:\n  theme: light\n"},
		{name: "theme-no-color-120x30", configText: "ui:\n  theme: light\n", isNoColor: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newThemeModel(t, testCase.configText, testCase.isNoColor)

			goldenANSI(t, testCase.name, m.View())
		})
	}
}

func TestTheme_Presets_ColourEveryRole(t *testing.T) {
	want := slices.Clone(userconfig.Roles)
	sort.Strings(want)

	for name, preset := range presets {
		got := make([]string, 0, len(preset))
		for role := range preset {
			got = append(got, role)
		}
		sort.Strings(got)

		if !slices.Equal(got, want) {
			t.Errorf("preset %s colours %v, want every role of userconfig.Roles %v", name, got, want)
		}
	}
}

func TestTheme_BackgroundColorMsg_PicksThePresetUnderAuto(t *testing.T) {
	testCases := []struct {
		name       string
		configText string
		background color.Color
		wantPreset string
	}{
		{name: "auto on light", configText: "ui:\n  theme: auto\n", background: color.White, wantPreset: "light"},
		{name: "auto on dark", configText: "ui:\n  theme: auto\n", background: color.Black, wantPreset: "dark"},
		{name: "dark on light", configText: "ui:\n  theme: dark\n", background: color.White, wantPreset: "dark"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newThemeModel(t, testCase.configText, false)
			want := newThemeModel(t, "ui:\n  theme: "+testCase.wantPreset+"\n", false).View().Content

			got := press(t, m, tea.BackgroundColorMsg{Color: testCase.background}).View().Content

			if got != want {
				t.Errorf("view after a %s background differs from the %s preset's\n--- got ---\n%q\n--- want ---\n%q",
					testCase.name, testCase.wantPreset, got, want)
			}
		})
	}
}

func TestTheme_BeforeTheBackgroundAnswers_AutoIsDark(t *testing.T) {
	want := newThemeModel(t, "ui:\n  theme: dark\n", false).View().Content

	got := newThemeModel(t, "ui:\n  theme: auto\n", false).View().Content

	if got != want {
		t.Errorf("auto before the background answers differs from the dark preset\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestTheme_AccentOverride_PaintsTheTabBar(t *testing.T) {
	testCases := []struct {
		value    string
		wantSGR  string
		describe string
	}{
		{value: `"#123456"`, wantSGR: "38;2;18;52;86", describe: "hex"},
		{value: "212", wantSGR: "38;5;212", describe: "256-colour"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.describe, func(t *testing.T) {
			m := newThemeModel(t, "ui:\n  colors:\n    accent: "+testCase.value+"\n", false)

			tabBar := strings.Split(m.View().Content, "\n")[0]

			if !strings.Contains(tabBar, testCase.wantSGR) {
				t.Errorf("tab bar %q lacks the accent override's %s", tabBar, testCase.wantSGR)
			}
		})
	}
}

func TestTheme_HeadingOverride_PaintsTheMarkdownHeading(t *testing.T) {
	m := newThemeModel(t, "ui:\n  colors:\n    heading: \"#654321\"\n", false)

	heading := detailLine(t, m, "Plan")

	if !strings.Contains(heading, "38;2;101;67;33") {
		t.Errorf("heading line %q lacks the heading override's 38;2;101;67;33", heading)
	}
}

func TestTheme_StatusOverride_PaintsTheStatusByCategory(t *testing.T) {
	m := newThemeModel(t, "ui:\n  colors:\n    status-todo: \"#010203\"\n", false)

	// beta, open (todo), is the unselected second row.
	row := m.sidebar[1]

	if !strings.Contains(row, "38;2;1;2;3") {
		t.Errorf("sidebar row %q lacks the status-todo override's 38;2;1;2;3", row)
	}
}

func TestTheme_NoColor_DrawsNoColourSequences(t *testing.T) {
	configText := "ui:\n  theme: light\n  colors:\n    accent: \"#123456\"\n    status-todo: \"212\"\n"
	if found := colourSequences(newThemeModel(t, configText, false).View().Content); len(found) == 0 {
		t.Fatalf("the coloured view has no colour sequences: the check cannot tell")
	}
	m := newThemeModel(t, configText, true)
	withPrompt := press(t, m, runeKey('L'))
	if !withPrompt.textPrompt.isOpen {
		t.Fatalf("L opened no text prompt: status row %q", statusRow(withPrompt))
	}

	frames := map[string]string{
		"panes":       m.View().Content,
		"filter line": press(t, m, runeKey('/')).View().Content,
		"text prompt": withPrompt.View().Content,
	}

	for name, frame := range frames {
		if found := colourSequences(frame); len(found) > 0 {
			t.Errorf("%s frame under NO_COLOR has colour sequences %q", name, found)
		}
	}
}
