package userconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The state file's name under the capcom user config directory, the keys it holds and its first
// line.
const (
	stateFileName   = "state.yaml"
	keySidebarRatio = "sidebar_ratio"
	keyStackedRatio = "stacked_ratio"
	keyLayout       = "layout"
	stateHeader     = "# capcom ui state: written by capcom ui, not meant for editing (ADR 0014).\n"
	// stateTempPattern names the temp file SaveState writes before renaming it into place.
	stateTempPattern = ".state-*.yaml.tmp"
	// yamlFloatTag is the tag yaml.v3 resolves a plain decimal scalar to.
	yamlFloatTag = "!!float"
)

// The measures a load error names for each ratio key: the quantity the ratio is a fraction of.
const (
	sidebarRatioMeasure = "terminal's width"
	stackedRatioMeasure = "body's height"
)

// The layout choices UIState.Layout holds and the state file stores under ui.layout; "" is no
// choice.
const (
	// LayoutSplit is the choice of the split layout, the sidebar beside the detail pane.
	LayoutSplit = "split"
	// LayoutStacked is the choice of the stacked layout, the sidebar above the detail pane.
	LayoutStacked = "stacked"
)

// State is what capcom ui remembers between runs for every worktree of the user, in state.yaml:
// the divider positions of the side-by-side and the stacked layout, and the layout choice. What it
// remembers per worktree, the tab and its selections, is WorktreeState in the git directory's
// ui-state.yaml. Unlike Config it is written by capcom ui itself, never by the user.
type State struct {
	UI UIState
}

// UIState is the layout capcom ui remembers for every worktree: the two divider positions
// SidebarRatio and StackedRatio, each stored apart from the other, and the layout choice Layout;
// the tab and selections it remembers per worktree are WorktreeState.
type UIState struct {
	// SidebarRatio is the sidebar's width as the user last set it, as a fraction of the terminal's
	// width, so the split holds at every terminal width; 0 means none is stored and the default
	// applies. It is 0 or more and less than 1.
	SidebarRatio float64
	// StackedRatio is the sidebar's height in the stacked layout as the user last set it, as a
	// fraction of the body's height (the sidebar's rows over the body's), so the divider holds at
	// every terminal height; 0 means none is stored and the default applies. It is 0 or more and
	// less than 1.
	StackedRatio float64
	// Layout is the layout choice, the layout the user last switched to: LayoutSplit,
	// LayoutStacked, or "" when none is stored. It picks the layout only where both fit.
	Layout string
}

// storedRatio is one of UIState's ratios with the key the state file stores it under and the
// name SaveState's error gives it.
type storedRatio struct {
	key   string
	name  string
	value float64
}

// storedRatios returns settings' ratios in the order the state file writes them, sidebar_ratio
// first.
func storedRatios(settings UIState) []storedRatio {
	return []storedRatio{
		{key: keySidebarRatio, name: "sidebar ratio", value: settings.SidebarRatio},
		{key: keyStackedRatio, name: "stacked ratio", value: settings.StackedRatio},
	}
}

// StatePath returns `<os.UserConfigDir>/capcom/state.yaml`, beside Path's config file. It fails
// when the platform's user config directory is unknown.
func StatePath() (string, error) {
	return capcomFile(stateFileName)
}

// LoadState reads the state file at path. A missing file is the zero State and no error, and is
// never created. Unknown keys are ignored silently, so a newer capcom's state loads in an older
// one. A file without one of the ratio keys loads that ratio as 0. A layout that is not split or
// stacked, or not a string, loads as no layout choice, "", with no error and the ratios kept.
// Malformed YAML, a sidebar_ratio or stacked_ratio that is not a fraction more than 0 and less
// than 1, and a file that cannot be read are an *Error naming the file and, where known, the line;
// the State is then the zero State, so the caller can run on the defaults.
func LoadState(path string) (State, error) {
	p, root, err := readDocument(path)
	var state State
	if err == nil && root != nil {
		err = p.stateTopLevel(root, &state)
	}
	if err != nil {
		return State{}, err
	}
	return state, nil
}

// SaveState writes state to path, creating its directory when missing. It writes a temp file in
// the same directory and renames it over path, so a reader never sees a half-written file. A
// SidebarRatio or StackedRatio below 0, of 1 or more, or NaN is an error naming that ratio, and a
// Layout other than LayoutSplit, LayoutStacked or "" is an error naming the layout; either leaves
// the file untouched.
func SaveState(path string, state State) error {
	for _, ratio := range storedRatios(state.UI) {
		if !isStorableRatio(ratio.value) {
			return fmt.Errorf(
				"save the state file: %s %v is not 0 or more and less than 1",
				ratio.name,
				ratio.value,
			)
		}
	}
	if state.UI.Layout != "" && !isLayout(state.UI.Layout) {
		return fmt.Errorf("save the state file: layout %q is not split or stacked", state.UI.Layout)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return fmt.Errorf("save the state file: %w", err)
	}
	if err := replaceFile(path, encodeState(state), stateTempPattern); err != nil {
		return fmt.Errorf("save the state file: %w", err)
	}
	return nil
}

// encodeState renders state as the content of the state file: ui written once, holding each
// non-zero ratio, sidebar_ratio first, then a non-empty layout; a zero field and an empty mapping
// are left out.
func encodeState(state State) []byte {
	var ui strings.Builder
	for _, ratio := range storedRatios(state.UI) {
		if ratio.value > 0 {
			// The shortest form that parses back to the same float64, so a save and a load round trip.
			formatted := strconv.FormatFloat(ratio.value, 'f', -1, 64)
			fmt.Fprintf(&ui, "  %s: %s\n", ratio.key, formatted)
		}
	}
	if state.UI.Layout != "" {
		fmt.Fprintf(&ui, "  %s: %s\n", keyLayout, state.UI.Layout)
	}

	var content strings.Builder
	content.WriteString(stateHeader)
	if ui.Len() > 0 {
		fmt.Fprintf(&content, "%s:\n%s", keyUI, ui.String())
	}
	return []byte(content.String())
}

// isStorableRatio reports whether ratio is a SidebarRatio or StackedRatio SaveState accepts: 0
// for none, or a fraction less than 1. NaN fails every comparison and so is never storable.
func isStorableRatio(ratio float64) bool {
	return ratio >= 0 && ratio < 1
}

// isLayout reports whether layout is a layout choice the state file stores, LayoutSplit or
// LayoutStacked.
func isLayout(layout string) bool {
	return layout == LayoutSplit || layout == LayoutStacked
}

// readDocument reads the machine-written file at path and parses it into its top-level mapping
// node, attributing problems to path. A missing file, an empty file and an all-comment file are a
// nil node and no error; a missing file is never created. A file that cannot be read and malformed
// YAML are an *Error.
func readDocument(path string) (*parser, *yaml.Node, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, &Error{Path: path, Message: fmt.Sprintf("cannot read the file: %v", err)}
	}
	p := &parser{path: path}
	root, err := p.document(data)
	return p, root, err
}

// replaceFile writes data to a temp file beside path, named after tempPattern (an os.CreateTemp
// pattern), and renames it over path, removing the temp file when any step fails.
func replaceFile(path string, data []byte, tempPattern string) error {
	file, err := os.CreateTemp(filepath.Dir(path), tempPattern)
	if err != nil {
		return err
	}
	temporary := file.Name()
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	err = errors.Join(writeErr, closeErr)
	if err == nil {
		err = os.Chmod(temporary, fileMode)
	}
	if err == nil {
		err = os.Rename(temporary, path)
	}
	if err != nil {
		return errors.Join(err, removeIfPresent(temporary))
	}
	return nil
}

// removeIfPresent deletes path, treating an already missing file as success.
func removeIfPresent(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// stateTopLevel applies the top-level keys of the state file onto state; any key but ui is
// ignored.
func (p *parser) stateTopLevel(root *yaml.Node, state *State) error {
	return p.eachKey(root, func(key, value *yaml.Node) error {
		if key.Value != keyUI {
			return nil
		}
		return p.stateUI(value, &state.UI)
	})
}

// stateUI applies the state file's ui mapping onto settings; a null ui stores nothing and an
// unknown key is ignored.
func (p *parser) stateUI(node *yaml.Node, settings *UIState) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return p.errorAt(node, "%s: expected a mapping of keys", keyUI)
	}
	return p.eachKey(node, func(key, value *yaml.Node) error {
		switch key.Value {
		case keySidebarRatio:
			return p.ratio(value, keySidebarRatio, sidebarRatioMeasure, &settings.SidebarRatio)
		case keyStackedRatio:
			return p.ratio(value, keyStackedRatio, stackedRatioMeasure, &settings.StackedRatio)
		case keyLayout:
			layout(value, &settings.Layout)
		}
		return nil
	})
}

// layout applies ui.layout onto target when it is a string naming a layout choice, split or
// stacked. Any other value, a null and a node that is not a string store nothing and are no
// error, so a layout this capcom does not know loads as no choice.
func layout(node *yaml.Node, target *string) {
	if node.Kind == yaml.ScalarNode && node.Tag == yamlStringTag && isLayout(node.Value) {
		*target = node.Value
	}
}

// ratio applies the ratio under ui.<key>, ui.sidebar_ratio or ui.stacked_ratio, onto target; a
// null value stores nothing. Only a decimal more than 0 and less than 1 is accepted: 0 and 1, an
// integer, NaN and an infinity are not, and the error names the key and measure, the quantity the
// ratio is a fraction of.
func (p *parser) ratio(node *yaml.Node, key, measure string, target *float64) error {
	if isNull(node) {
		return nil
	}
	var ratio float64
	if node.Kind != yaml.ScalarNode || node.Tag != yamlFloatTag || node.Decode(&ratio) != nil ||
		ratio <= 0 || !isStorableRatio(ratio) {
		return p.errorAt(
			node,
			"%s.%s: expected a fraction of the %s, more than 0 and less than 1",
			keyUI,
			key,
			measure,
		)
	}
	*target = ratio
	return nil
}
