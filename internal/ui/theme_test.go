package ui

import (
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"gopkg.in/yaml.v3"

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

// schemeColour matches the one colour form a scheme file writes, "#rrggbb" in either case.
var schemeColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func TestTheme_SchemeFiles_ColourEveryRoleInHex(t *testing.T) {
	want := slices.Clone(userconfig.Roles)
	sort.Strings(want)
	names, err := fs.Glob(schemeFiles, schemeFilesPattern)
	if err != nil || len(names) == 0 {
		t.Fatalf("Glob(%q) = %v, %v; want the embedded scheme files", schemeFilesPattern, names, err)
	}

	for _, name := range names {
		content, err := fs.ReadFile(schemeFiles, name)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		colors := map[string]string{}
		if err := yaml.Unmarshal(content, &colors); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := make([]string, 0, len(colors))
		for role, value := range colors {
			got = append(got, role)
			if !schemeColour.MatchString(value) {
				t.Errorf("%s: %s is %q, want a \"#rrggbb\" colour", name, role, value)
			}
		}
		sort.Strings(got)

		if !slices.Equal(got, want) {
			t.Errorf("%s names roles %v, want exactly userconfig.Roles %v", name, got, want)
		}
	}
}

func TestTheme_Presets_AreTheDarkAndLightSchemes(t *testing.T) {
	got := make([]string, 0, len(presets))
	for name := range presets {
		got = append(got, name)
	}
	sort.Strings(got)

	want := []string{userconfig.ThemeDark, userconfig.ThemeLight}
	if !slices.Equal(got, want) {
		t.Errorf("presets = %v, want %v", got, want)
	}
}

func TestTheme_Presets_DrawInApogeesColours(t *testing.T) {
	testCases := []struct {
		theme string
		role  string
		want  string
	}{
		{theme: userconfig.ThemeDark, role: roleAccent, want: "#c396ff"},
		{theme: userconfig.ThemeDark, role: roleSelection, want: "#3a5fcd"},
		{theme: userconfig.ThemeDark, role: "status-doing", want: "#58a6ff"},
		{theme: userconfig.ThemeDark, role: "status-done", want: "#43d389"},
		{theme: userconfig.ThemeLight, role: roleAccent, want: "#8250df"},
		{theme: userconfig.ThemeLight, role: roleSelection, want: "#b3d1ff"},
		{theme: userconfig.ThemeLight, role: "status-doing", want: "#0969da"},
		{theme: userconfig.ThemeLight, role: "status-done", want: "#115e59"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.theme+"/"+testCase.role, func(t *testing.T) {
			colors := newPalette(userconfig.UI{Theme: testCase.theme}, false, false)

			gotR, gotG, gotB, gotA := colors.of(testCase.role).RGBA()
			wantR, wantG, wantB, wantA := lipgloss.Color(testCase.want).RGBA()
			if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
				t.Errorf("theme %s %s = %v, want %s", testCase.theme, testCase.role, colors.of(testCase.role), testCase.want)
			}
		})
	}
}

func TestTheme_LoadSchemes_UndecodableFile_PanicsNamingIt(t *testing.T) {
	files := fstest.MapFS{"schemes/broken.yaml": {Data: []byte("accent: [\n")}}
	defer func() {
		recovered := recover()
		if message, isString := recovered.(string); !isString || !strings.Contains(message, "schemes/broken.yaml") {
			t.Errorf("loadSchemes panicked with %v, want a message naming schemes/broken.yaml", recovered)
		}
	}()

	loadSchemes(files)
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

// sgrBefore returns the parameters of the last SGR sequence in row before the first text, failing
// the test when row lacks text.
func sgrBefore(t *testing.T, row, text string) string {
	t.Helper()
	end := strings.Index(row, text)
	if end < 0 {
		t.Fatalf("row %q lacks %q", row, text)
	}
	matches := sgrSequence.FindAllStringSubmatch(row[:end], -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

func TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours(t *testing.T) {
	// dark's selection is #3a5fcd and its status-todo #b2b2b2.
	const (
		fill     = "48;2;58;95;205"
		todoText = "38;2;178;178;178"
	)
	testCases := []struct {
		name string
		keys []tea.KeyPressMsg
	}{
		{name: "ready"},
		{name: "issues", keys: []tea.KeyPressMsg{runeKey('2')}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pressKeys(t, newThemeModel(t, "ui:\n  theme: dark\n", false), testCase.keys...)

			// alpha, open (todo), is the selected first row.
			row := m.sidebar[0]

			for _, segment := range []struct{ text, want string }{
				{text: "alpha", want: fill},
				{text: "open", want: todoText},
				{text: "open", want: fill},
				{text: "Alpha task", want: fill},
			} {
				if got := sgrBefore(t, row, segment.text); !strings.Contains(got, segment.want) {
					t.Errorf("%q in row %q is drawn in %q, want it to carry %s", segment.text, row, got, segment.want)
				}
			}
		})
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
