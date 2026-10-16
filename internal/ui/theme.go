package ui

import (
	"image/color"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/ui/markdown"
	"github.com/airiclenz/capcom/internal/userconfig"
)

// The colour roles of userconfig.Roles the UI paints with. A status role is statusRolePrefix
// followed by a config.Category, so status-doing covers every status of the doing category.
const (
	roleAccent       = "accent"
	roleBorder       = "border"
	roleMuted        = "muted"
	roleLink         = "link"
	roleSelection    = "selection"
	roleHeading      = "heading"
	roleCode         = "code"
	statusRolePrefix = "status-"
)

// presets maps each preset a theme key names, auto aside, to its colour for every role of
// userconfig.Roles, written as a user config file writes an override.
var presets = map[string]map[string]string{
	userconfig.ThemeDark: {
		roleAccent:    "#bb9af7",
		roleBorder:    "#565f89",
		roleMuted:     "#737aa2",
		roleLink:      "#7dcfff",
		roleSelection: "#7aa2f7",
		roleHeading:   "#e0af68",
		roleCode:      "#9ece6a",
		statusRolePrefix + string(config.CategoryTodo):    "#a9b1d6",
		statusRolePrefix + string(config.CategoryDoing):   "#e0af68",
		statusRolePrefix + string(config.CategoryWaiting): "#ff9e64",
		statusRolePrefix + string(config.CategoryDone):    "#9ece6a",
		statusRolePrefix + string(config.CategoryDropped): "#565f89",
	},
	userconfig.ThemeLight: {
		roleAccent:    "#9854f1",
		roleBorder:    "#a8aecb",
		roleMuted:     "#6172b0",
		roleLink:      "#007197",
		roleSelection: "#2e7de9",
		roleHeading:   "#8c6c3e",
		roleCode:      "#587539",
		statusRolePrefix + string(config.CategoryTodo):    "#3760bf",
		statusRolePrefix + string(config.CategoryDoing):   "#8c6c3e",
		statusRolePrefix + string(config.CategoryWaiting): "#b15c00",
		statusRolePrefix + string(config.CategoryDone):    "#587539",
		statusRolePrefix + string(config.CategoryDropped): "#a8aecb",
	},
}

// palette maps a colour role to its colour. A role it lacks has no colour.
type palette map[string]color.Color

// newPalette returns the colours of settings over a terminal background that is light or not: the
// preset its theme names, or for auto the background's, with its overrides on top. With isNoColor
// the palette is empty, whatever settings says.
func newPalette(settings userconfig.UI, isLightBackground, isNoColor bool) palette {
	colors := palette{}
	if isNoColor {
		return colors
	}
	for role, value := range presets[presetName(settings.Theme, isLightBackground)] {
		colors[role] = lipgloss.Color(value)
	}
	for role, value := range settings.Colors {
		colors[role] = lipgloss.Color(value)
	}
	return colors
}

// presetName returns the preset a theme key picks: dark or light as it says, and for auto (or
// any other value) the light preset over a light background and the dark one otherwise.
func presetName(configured string, isLightBackground bool) string {
	switch {
	case configured == userconfig.ThemeDark || configured == userconfig.ThemeLight:
		return configured
	case isLightBackground:
		return userconfig.ThemeLight
	}
	return userconfig.ThemeDark
}

// of returns role's colour, and lipgloss.NoColor, which paints nothing, for a role without one.
func (p palette) of(role string) color.Color {
	if value, hasColor := p[role]; hasColor {
		return value
	}
	return lipgloss.NoColor{}
}

// foreground returns a style painting text in role's colour.
func (p palette) foreground(role string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.of(role))
}

// theme is every style the UI paints with, built from one palette. Bold, faint, reverse and
// underline marks do not depend on the palette, so a theme without colours still shows the open
// tab, the selection, revealed rows, headings and links.
type theme struct {
	// colors is the palette the styles were built from.
	colors palette
	// activeTab marks the open tab on the tab bar, in the accent colour.
	activeTab lipgloss.Style
	// border paints the frame: the tab bar's separators, the separator between the panes and the
	// rules above and under them.
	border lipgloss.Style
	// hints paints the status bar's key hints, muted.
	hints lipgloss.Style
	// selectedRow marks the selected row of a list in the selection colour, mutedRow a tree row
	// shown only to place another, and mutedSelectedRow one that is both.
	selectedRow      lipgloss.Style
	mutedRow         lipgloss.Style
	mutedSelectedRow lipgloss.Style
	// markdown is what issue and Note bodies render with, its Measure left at the zero value: the
	// model keeps the painter's.
	markdown markdown.Theme
	// input styles the filter line and the text prompts: an accent prompt and cursor.
	input textinput.Styles
}

// newTheme returns the theme of settings over a terminal background that is light or not, with
// no colour at all when isNoColor: see newPalette.
func newTheme(settings userconfig.UI, isLightBackground, isNoColor bool) theme {
	colors := newPalette(settings, isLightBackground, isNoColor)
	return theme{
		colors:           colors,
		activeTab:        colors.foreground(roleAccent).Bold(true).Reverse(true),
		border:           colors.foreground(roleBorder),
		hints:            colors.foreground(roleMuted),
		selectedRow:      colors.foreground(roleSelection).Reverse(true),
		mutedRow:         colors.foreground(roleMuted).Faint(true),
		mutedSelectedRow: colors.foreground(roleSelection).Faint(true).Reverse(true),
		markdown:         markdownTheme(colors),
		input:            inputStyles(colors),
	}
}

// markdownTheme returns the markdown styles of colors, its Measure the zero value.
func markdownTheme(colors palette) markdown.Theme {
	return markdown.Theme{
		Bold:      lipgloss.NewStyle().Bold(true),
		Underline: lipgloss.NewStyle().Underline(true),
		Heading:   colors.foreground(roleHeading).Bold(true),
		Code:      colors.foreground(roleCode),
		CodeBlock: colors.foreground(roleCode),
		Rule:      colors.foreground(roleBorder),
		Link:      colors.foreground(roleLink).Underline(true),
		Inert:     colors.foreground(roleMuted),
		Quote:     colors.foreground(roleMuted),
	}
}

// inputStyles returns the textinput styles of colors, focused and blurred alike: an accent prompt
// and cursor, muted placeholder and suggestion, plain text. They replace bubbles' defaults, which
// carry colours of their own.
func inputStyles(colors palette) textinput.Styles {
	state := textinput.StyleState{
		Text:        lipgloss.NewStyle(),
		Placeholder: colors.foreground(roleMuted),
		Suggestion:  colors.foreground(roleMuted),
		Prompt:      colors.foreground(roleAccent),
	}
	return textinput.Styles{
		Focused: state,
		Blurred: state,
		Cursor:  textinput.CursorStyle{Color: colors.of(roleAccent), Shape: tea.CursorBlock},
	}
}

// status returns the style of a status of category: the colour of its status role.
func (t theme) status(category config.Category) lipgloss.Style {
	return t.colors.foreground(statusRolePrefix + string(category))
}

// themeOf returns the theme the model's user config, terminal background and NO_COLOR setting
// pick.
func (m model) themeOf() theme {
	return newTheme(m.userConfig.UI, m.isLightBackground, m.isNoColor)
}

// applyTheme repaints the UI in th: the panes, the shown issue or Note, rendered again in the
// painter's measure, and an open filter line or text prompt.
func (m model) applyTheme(th theme) model {
	m.theme = th
	measure := m.markdownTheme.Measure
	m.markdownTheme = th.markdown
	m.markdownTheme.Measure = measure
	if m.isFilterOpen {
		m.filterLine.setStyles(th.input)
	}
	if m.textPrompt.isOpen {
		m.textPrompt.line.setStyles(th.input)
	}
	m.issue.renderedWidth = 0
	return m.paintSidebar().renderDetail().paintDeps()
}

// applyBackground records the terminal background the terminal reported and repaints the UI in
// the theme it picks, which changes only under theme auto.
func (m model) applyBackground(msg tea.BackgroundColorMsg) model {
	m.isLightBackground = !msg.IsDark()
	return m.applyTheme(m.themeOf())
}
