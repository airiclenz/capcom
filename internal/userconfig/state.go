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
	stateHeader     = "# capcom ui state: written by capcom ui, not meant for editing (ADR 0014).\n"
	// stateTempPattern names the temp file SaveState writes before renaming it into place.
	stateTempPattern = ".state-*.yaml.tmp"
	// yamlFloatTag is the tag yaml.v3 resolves a plain decimal scalar to.
	yamlFloatTag = "!!float"
)

// State is what capcom ui remembers between runs. Unlike Config it is written by capcom ui itself,
// never by the user.
type State struct {
	UI UIState
}

// UIState is the remembered layout of capcom ui.
type UIState struct {
	// SidebarRatio is the sidebar's width as the user last set it, as a fraction of the terminal's
	// width, so the split holds at every terminal width; 0 means none is stored and the default
	// applies. It is 0 or more and less than 1.
	SidebarRatio float64
}

// StatePath returns `<os.UserConfigDir>/capcom/state.yaml`, beside Path's config file. It fails
// when the platform's user config directory is unknown.
func StatePath() (string, error) {
	return capcomFile(stateFileName)
}

// LoadState reads the state file at path. A missing file is the zero State and no error, and is
// never created. Unknown keys are ignored silently, so a newer capcom's state loads in an older
// one. Malformed YAML, a sidebar_ratio that is not a fraction more than 0 and less than 1, and a
// file that cannot be read are an *Error naming the file and, where known, the line; the State is then the
// zero State, so the caller can run on the defaults.
func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, &Error{Path: path, Message: fmt.Sprintf("cannot read the file: %v", err)}
	}

	p := &parser{path: path}
	var state State
	root, err := p.document(data)
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
// SidebarRatio below 0, of 1 or more, or NaN is an error and leaves the file untouched.
func SaveState(path string, state State) error {
	if ratio := state.UI.SidebarRatio; !isStorableRatio(ratio) {
		return fmt.Errorf("save the state file: sidebar ratio %v is not 0 or more and less than 1", ratio)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return fmt.Errorf("save the state file: %w", err)
	}
	if err := replaceFile(path, encodeState(state)); err != nil {
		return fmt.Errorf("save the state file: %w", err)
	}
	return nil
}

// encodeState renders state as the content of the state file; a zero field and an empty mapping
// are left out.
func encodeState(state State) []byte {
	var content strings.Builder
	content.WriteString(stateHeader)
	if ratio := state.UI.SidebarRatio; ratio > 0 {
		// The shortest form that parses back to the same float64, so a save and a load round trip.
		formatted := strconv.FormatFloat(ratio, 'f', -1, 64)
		fmt.Fprintf(&content, "%s:\n  %s: %s\n", keyUI, keySidebarRatio, formatted)
	}
	return []byte(content.String())
}

// isStorableRatio reports whether ratio is a SidebarRatio SaveState accepts: 0 for none, or a
// fraction less than 1. NaN fails every comparison and so is never storable.
func isStorableRatio(ratio float64) bool {
	return ratio >= 0 && ratio < 1
}

// replaceFile writes data to a temp file beside path and renames it over path, removing the temp
// file when any step fails.
func replaceFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), stateTempPattern)
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
		if key.Value != keySidebarRatio {
			return nil
		}
		return p.sidebarRatio(value, settings)
	})
}

// sidebarRatio applies ui.sidebar_ratio onto settings; a null value stores nothing. Only a
// decimal more than 0 and less than 1 is accepted: 0 and 1, an integer, NaN and an infinity are not.
func (p *parser) sidebarRatio(node *yaml.Node, settings *UIState) error {
	if isNull(node) {
		return nil
	}
	var ratio float64
	if node.Kind != yaml.ScalarNode || node.Tag != yamlFloatTag || node.Decode(&ratio) != nil ||
		ratio <= 0 || !isStorableRatio(ratio) {
		return p.errorAt(
			node,
			"%s.%s: expected a fraction of the terminal's width, more than 0 and less than 1",
			keyUI,
			keySidebarRatio,
		)
	}
	settings.SidebarRatio = ratio
	return nil
}
