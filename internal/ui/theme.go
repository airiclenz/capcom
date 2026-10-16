package ui

import (
	"embed"
	"fmt"
	"image/color"
	"io/fs"
	"math"
	"path"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"gopkg.in/yaml.v3"

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
	roleLockHeld     = "lock-held"
	roleLockFree     = "lock-free"
	statusRolePrefix = "status-"
)

// The lock badge's text colours: black on a light fill, white on a dark one.
const (
	badgeTextOnLight = "#000000"
	badgeTextOnDark  = "#ffffff"
)

// lightFillLuminance is the relative luminance from which a fill counts as light: the point where
// black and white text contrast with it equally, by the WCAG contrast ratio.
const lightFillLuminance = 0.179

// schemeFiles holds the colour schemes, one YAML file per preset under schemes/, named after
// the theme key that picks it: one quoted "#rrggbb" per role of userconfig.Roles.
//
//go:embed schemes/*.yaml
var schemeFiles embed.FS

// The scheme files' glob within schemeFiles and the extension a scheme's name drops.
const (
	schemeFilesPattern = "schemes/*.yaml"
	schemeFileSuffix   = ".yaml"
)

// presets maps each preset a theme key names, auto aside, to its colour for every role of
// userconfig.Roles, written as a user config file writes an override.
var presets = loadSchemes(schemeFiles)

// loadSchemes returns every scheme file of files that schemeFilesPattern matches, keyed by its
// basename without the suffix. The files are compiled in, so one that cannot be read or decoded
// is a defect of this program: loadSchemes panics naming it, and the theme tests decode every
// embedded file so that never happens at start-up.
func loadSchemes(files fs.FS) map[string]map[string]string {
	names, err := fs.Glob(files, schemeFilesPattern)
	if err != nil {
		panic(fmt.Sprintf("%s: %v", schemeFilesPattern, err))
	}
	schemes := make(map[string]map[string]string, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(files, name)
		if err != nil {
			panic(fmt.Sprintf("%s: %v", name, err))
		}
		colors := map[string]string{}
		if err := yaml.Unmarshal(content, &colors); err != nil {
			panic(fmt.Sprintf("%s: %v", name, err))
		}
		schemes[strings.TrimSuffix(path.Base(name), schemeFileSuffix)] = colors
	}
	return schemes
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
	// border paints the frame: the tab bar's separators, the separator between the panes, the
	// rules above and under them, and the connectors of the sidebar trees.
	border lipgloss.Style
	// dividerActive paints the separator between the panes and its joints in the rules while the
	// divider is dragged: accent and bold.
	dividerActive lipgloss.Style
	// hints paints the status bar's key hints, muted.
	hints lipgloss.Style
	// lockHeld paints the status bar's word lock while another session holds the Tree lock, live
	// or stale: a lock-held fill, or Reverse without one. lockFree paints it while the lock is
	// free: a lock-free fill, or nothing without one. Each fill's text is black or white, whichever
	// reads on it.
	lockHeld lipgloss.Style
	lockFree lipgloss.Style
	// selectedRow marks the selected row of a list: a background fill in the selection colour, or
	// Reverse when the palette has none. mutedRow marks a tree row shown only to place another,
	// muted and faint, and mutedSelectedRow one that is both: the fill with faint text, or Reverse.
	selectedRow      lipgloss.Style
	mutedRow         lipgloss.Style
	mutedSelectedRow lipgloss.Style
	// isSelectionFill is whether selectedRow is the background fill, which each styled span of a
	// selected row carries beneath its own style, rather than a Reverse covering the row whole.
	isSelectionFill bool
	// scrollThumb paints a scrollbar's thumb, muted, and scrollTrack its track in the border
	// colour.
	scrollThumb lipgloss.Style
	scrollTrack lipgloss.Style
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
	selectedRow, mutedSelectedRow, isSelectionFill := selectionStyles(colors)
	return theme{
		colors:           colors,
		activeTab:        colors.foreground(roleAccent).Bold(true).Reverse(true),
		border:           colors.foreground(roleBorder),
		dividerActive:    colors.foreground(roleAccent).Bold(true),
		hints:            colors.foreground(roleMuted),
		lockHeld:         badgeStyle(colors, roleLockHeld, lipgloss.NewStyle().Reverse(true)),
		lockFree:         badgeStyle(colors, roleLockFree, lipgloss.NewStyle()),
		selectedRow:      selectedRow,
		mutedRow:         colors.foreground(roleMuted).Faint(true),
		mutedSelectedRow: mutedSelectedRow,
		isSelectionFill:  isSelectionFill,
		scrollThumb:      colors.foreground(roleMuted),
		scrollTrack:      colors.foreground(roleBorder),
		markdown:         markdownTheme(colors),
		input:            inputStyles(colors),
	}
}

// selectionStyles returns the style of a selected row and of a muted selected one, and whether
// they are a background fill: in colors' selection colour, the muted one faint, when colors has
// one, and otherwise Reverse, which needs no colour to show.
func selectionStyles(colors palette) (lipgloss.Style, lipgloss.Style, bool) {
	selection, hasSelection := colors[roleSelection]
	if !hasSelection {
		reverse := lipgloss.NewStyle().Reverse(true)
		return reverse, reverse.Faint(true), false
	}
	fill := lipgloss.NewStyle().Background(selection)
	return fill, fill.Faint(true), true
}

// badgeStyle returns the style of a badge filled in colors' colour for role, its text black or
// white by the fill's luminance, or fallback when colors has no colour for role.
func badgeStyle(colors palette, role string, fallback lipgloss.Style) lipgloss.Style {
	fill, hasFill := colors[role]
	if !hasFill {
		return fallback
	}
	return lipgloss.NewStyle().Background(fill).Foreground(readableTextOn(fill))
}

// readableTextOn returns the text colour that reads on fill: black when fill is light, white when
// it is dark.
func readableTextOn(fill color.Color) color.Color {
	if relativeLuminance(fill) >= lightFillLuminance {
		return lipgloss.Color(badgeTextOnLight)
	}
	return lipgloss.Color(badgeTextOnDark)
}

// relativeLuminance returns the WCAG relative luminance of value, from 0 for black to 1 for white.
func relativeLuminance(value color.Color) float64 {
	red, green, blue, _ := value.RGBA()
	return 0.2126*linearChannel(red) + 0.7152*linearChannel(green) + 0.0722*linearChannel(blue)
}

// linearChannel returns the linear light of one 16-bit sRGB channel, from 0 to 1.
func linearChannel(channel uint32) float64 {
	encoded := float64(channel) / 0xffff
	if encoded <= 0.04045 {
		return encoded / 12.92
	}
	return math.Pow((encoded+0.055)/1.055, 2.4)
}

// lockBadge returns the style the status bar paints the word lock in for badge: plain for none.
func (t theme) lockBadge(badge lockBadge) lipgloss.Style {
	switch badge {
	case lockBadgeHeld:
		return t.lockHeld
	case lockBadgeFree:
		return t.lockFree
	default:
		return lipgloss.NewStyle()
	}
}

// span is one run of a list row's text and the style it is painted in, under the row's mark.
type span struct {
	text  string
	style lipgloss.Style
}

// rowMark is how a list row is marked, in style; the zero rowMark marks nothing. A whole mark
// paints the row's text in style alone, its spans' own styles dropped; any other carries style
// beneath each span's own, since a reset closing one span would end a style laid over the row.
type rowMark struct {
	style   lipgloss.Style
	isWhole bool
}

// markOf returns the mark of a list row that is muted or selected, or both: a muted one whole in
// the muted styles, a selected one carrying the selection fill, or whole in Reverse without one.
func (t theme) markOf(isMuted, isSelected bool) rowMark {
	switch {
	case isMuted && isSelected:
		return rowMark{style: t.mutedSelectedRow, isWhole: true}
	case isMuted:
		return rowMark{style: t.mutedRow, isWhole: true}
	case isSelected:
		return rowMark{style: t.selectedRow, isWhole: !t.isSelectionFill}
	}
	return rowMark{}
}

// render returns the text of spans joined, painted under the mark.
func (rm rowMark) render(spans ...span) string {
	var text strings.Builder
	for _, run := range spans {
		if rm.isWhole {
			text.WriteString(run.text)
			continue
		}
		text.WriteString(run.style.Inherit(rm.style).Render(run.text))
	}
	if rm.isWhole {
		return rm.style.Render(text.String())
	}
	return text.String()
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
