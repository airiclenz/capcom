package userconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The worktree state file's keys, its first line and the temp file SaveWorktreeState writes.
const (
	keyTab              = "tab"
	keySelected         = "selected"
	keyReady            = "ready"
	keyIssues           = "issues"
	keyKnowledge        = "knowledge"
	keyClosed           = "closed"
	worktreeStateHeader = "# capcom ui worktree state: written by capcom ui, not meant for editing (ADR 0015).\n"
	// worktreeStateTempPattern names the temp file SaveWorktreeState writes before renaming it into
	// place.
	worktreeStateTempPattern = ".ui-state-*.yaml.tmp"
	// yamlStringTag is the tag yaml.v3 resolves a quoted scalar, and a plain one that is no other
	// type, to.
	yamlStringTag = "!!str"
)

// WorktreeState is what capcom ui remembers per worktree between runs, in the git directory's
// ui-state.yaml: the tab it was on and the id selected on each tab. An empty field means none is
// stored. The values are kept as written; whether a tab name or an id still means anything is the
// caller's to decide.
type WorktreeState struct {
	// Tab is the name of the tab capcom ui was on.
	Tab string
	// Ready is the id selected on the Ready tab.
	Ready string
	// Issues is the id selected on the Issues tab.
	Issues string
	// Knowledge is the id selected on the Knowledge tab.
	Knowledge string
	// Closed is the id selected on the Closed tab.
	Closed string
}

// selection pairs a key of the ui.selected mapping with the WorktreeState field it holds.
type selection struct {
	key   string
	field *string
}

// selections returns the ui.selected keys of state in file order, each with its field.
func (s *WorktreeState) selections() []selection {
	return []selection{
		{keyReady, &s.Ready},
		{keyIssues, &s.Issues},
		{keyKnowledge, &s.Knowledge},
		{keyClosed, &s.Closed},
	}
}

// LoadWorktreeState reads the worktree state file at path. A missing file is the zero
// WorktreeState and no error, and is never created. Unknown keys are ignored silently, so a newer
// capcom's state loads in an older one. Malformed YAML, a value that is not a string, and a file
// that cannot be read are an *Error naming the file and, where known, the line; the WorktreeState
// is then the zero WorktreeState, so the caller can run on the defaults.
func LoadWorktreeState(path string) (WorktreeState, error) {
	p, root, err := readDocument(path)
	var state WorktreeState
	if err == nil && root != nil {
		err = p.worktreeStateTopLevel(root, &state)
	}
	if err != nil {
		return WorktreeState{}, err
	}
	return state, nil
}

// SaveWorktreeState writes state to path, creating its directory when missing. It writes a temp
// file in the same directory and renames it over path, so a reader never sees a half-written file.
func SaveWorktreeState(path string, state WorktreeState) error {
	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		return fmt.Errorf("save the worktree state file: %w", err)
	}
	if err := replaceFile(path, encodeWorktreeState(state), worktreeStateTempPattern); err != nil {
		return fmt.Errorf("save the worktree state file: %w", err)
	}
	return nil
}

// encodeWorktreeState renders state as the content of the worktree state file; an empty field and
// an empty mapping are left out. Every value is double-quoted, so an id such as 2026, null or true
// loads back as a string.
func encodeWorktreeState(state WorktreeState) []byte {
	var selected strings.Builder
	for _, entry := range state.selections() {
		if *entry.field != "" {
			fmt.Fprintf(&selected, "    %s: %s\n", entry.key, strconv.Quote(*entry.field))
		}
	}

	var content strings.Builder
	content.WriteString(worktreeStateHeader)
	if state.Tab == "" && selected.Len() == 0 {
		return []byte(content.String())
	}
	content.WriteString(keyUI + ":\n")
	if state.Tab != "" {
		fmt.Fprintf(&content, "  %s: %s\n", keyTab, strconv.Quote(state.Tab))
	}
	if selected.Len() > 0 {
		fmt.Fprintf(&content, "  %s:\n%s", keySelected, selected.String())
	}
	return []byte(content.String())
}

// worktreeStateTopLevel applies the top-level keys of the worktree state file onto state; any key
// but ui is ignored.
func (p *parser) worktreeStateTopLevel(root *yaml.Node, state *WorktreeState) error {
	return p.eachKey(root, func(key, value *yaml.Node) error {
		if key.Value != keyUI {
			return nil
		}
		return p.worktreeStateUI(value, state)
	})
}

// worktreeStateUI applies the worktree state file's ui mapping onto state; a null ui stores
// nothing and an unknown key is ignored.
func (p *parser) worktreeStateUI(node *yaml.Node, state *WorktreeState) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return p.errorAt(node, "%s: expected a mapping of keys", keyUI)
	}
	return p.eachKey(node, func(key, value *yaml.Node) error {
		switch key.Value {
		case keyTab:
			return p.stringValue(value, &state.Tab, keyUI+"."+keyTab)
		case keySelected:
			return p.selected(value, state)
		default:
			return nil
		}
	})
}

// selected applies the ui.selected mapping onto state; a null selected stores nothing and a key
// that names no tab is ignored.
func (p *parser) selected(node *yaml.Node, state *WorktreeState) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return p.errorAt(node, "%s.%s: expected a mapping of tabs to ids", keyUI, keySelected)
	}
	return p.eachKey(node, func(key, value *yaml.Node) error {
		for _, entry := range state.selections() {
			if key.Value == entry.key {
				return p.stringValue(value, entry.field, keyUI+"."+keySelected+"."+entry.key)
			}
		}
		return nil
	})
}

// stringValue stores the string scalar node in target; a null value stores nothing. Any other
// value, an unquoted number, boolean or list among them, is an *Error naming name.
func (p *parser) stringValue(node *yaml.Node, target *string, name string) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.ScalarNode || node.Tag != yamlStringTag {
		return p.errorAt(node, "%s: expected a quoted string", name)
	}
	*target = node.Value
	return nil
}
