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
	keySidebarWidth = "sidebar_width"
	stateHeader     = "# capcom ui state: written by capcom ui, not meant for editing (ADR 0014).\n"
	// stateTempPattern names the temp file SaveState writes before renaming it into place.
	stateTempPattern = ".state-*.yaml.tmp"
	// yamlIntTag is the tag yaml.v3 resolves a plain integer scalar to.
	yamlIntTag = "!!int"
)

// State is what capcom ui remembers between runs. Unlike Config it is written by capcom ui itself,
// never by the user.
type State struct {
	UI UIState
}

// UIState is the remembered layout of capcom ui.
type UIState struct {
	// SidebarWidth is the sidebar's width in columns, as the user last set it; 0 means none is
	// stored and the default applies. It is never negative.
	SidebarWidth int
}

// StatePath returns `<os.UserConfigDir>/capcom/state.yaml`, beside Path's config file. It fails
// when the platform's user config directory is unknown.
func StatePath() (string, error) {
	return capcomFile(stateFileName)
}

// LoadState reads the state file at path. A missing file is the zero State and no error, and is
// never created. Unknown keys are ignored silently, so a newer capcom's state loads in an older
// one. Malformed YAML, a sidebar_width that is not a whole number of 1 or more, and a file that
// cannot be read are an *Error naming the file and, where known, the line; the State is then the
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
// negative SidebarWidth is an error and leaves the file untouched.
func SaveState(path string, state State) error {
	if state.UI.SidebarWidth < 0 {
		return fmt.Errorf("save the state file: sidebar width %d is negative", state.UI.SidebarWidth)
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
	if state.UI.SidebarWidth > 0 {
		fmt.Fprintf(&content, "%s:\n  %s: %d\n", keyUI, keySidebarWidth, state.UI.SidebarWidth)
	}
	return []byte(content.String())
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
		if key.Value != keySidebarWidth {
			return nil
		}
		return p.sidebarWidth(value, settings)
	})
}

// sidebarWidth applies ui.sidebar_width onto settings; a null value stores nothing.
func (p *parser) sidebarWidth(node *yaml.Node, settings *UIState) error {
	if isNull(node) {
		return nil
	}
	width, err := strconv.Atoi(node.Value)
	if node.Kind != yaml.ScalarNode || node.Tag != yamlIntTag || err != nil || width < 1 {
		return p.errorAt(node, "%s.%s: expected a whole number of columns, 1 or more", keyUI, keySidebarWidth)
	}
	settings.SidebarWidth = width
	return nil
}
