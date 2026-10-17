// Package userconfig loads the per-user config file of capcom ui (ADR 0011),
// `<os.UserConfigDir>/capcom/config.yaml`, and creates it with every key as a commented default
// the first time it is missing. It also owns capcom ui's machine-written state files: `state.yaml`
// beside it (state.go), which LoadState reads and SaveState replaces, and the per-worktree
// `<git-dir>/capcom/ui-state.yaml` (worktree_state.go), which LoadWorktreeState reads and
// SaveWorktreeState replaces; config.yaml itself is never written once created.
//
// Only capcom ui may call Load, LoadState, SaveState, LoadWorktreeState or SaveWorktreeState: every
// other command neither reads nor creates these files, so normal capcom use leaves nothing behind
// in the user's home or the git directory. The config file is read by every capcom binary on
// the machine, of whatever release, so Load is lenient where the store's config is strict: an
// unknown key or colour role is a Warning, never an error. Malformed YAML and a value of the wrong
// type are an *Error naming the file and line.
package userconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The colour presets a theme key may name.
const (
	// ThemeAuto picks ThemeDark or ThemeLight from the terminal background.
	ThemeAuto = "auto"
	// ThemeDark is the preset for a dark terminal background.
	ThemeDark = "dark"
	// ThemeLight is the preset for a light terminal background.
	ThemeLight = "light"
)

// themes lists every value a theme key may take.
var themes = []string{ThemeAuto, ThemeDark, ThemeLight}

// Roles lists every colour role a colors key may override, in the order the template documents
// them. Each status role names a status category, so it covers every status of that category.
var Roles = []string{
	"accent",
	"border",
	"muted",
	"link",
	"selection",
	"heading",
	"code",
	"status-todo",
	"status-doing",
	"status-waiting",
	"status-done",
	"status-dropped",
	"lock-held",
	"lock-free",
}

// The directory and file names under os.UserConfigDir, and the keys Load knows.
const (
	directoryName = "capcom"
	fileName      = "config.yaml"
	keyUI         = "ui"
	keyTheme      = "theme"
	keyColors     = "colors"
	maxColorIndex = 255
	directoryMode = 0o755
	fileMode      = 0o644
)

var (
	// hexColor matches a "#rgb" or "#rrggbb" colour.
	hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	// colorIndex matches the digits of a 256-colour number; its range is checked separately.
	colorIndex = regexp.MustCompile(`^[0-9]{1,3}$`)
	// yamlLine splits a yaml.v3 syntax error into its line and its message.
	yamlLine = regexp.MustCompile(`^yaml: line ([0-9]+): (.*)$`)
)

// Config is the content of the user config file.
type Config struct {
	UI UI
}

// UI holds the settings of capcom ui.
type UI struct {
	// Theme is ThemeAuto, ThemeDark or ThemeLight.
	Theme string
	// Colors maps a role of Roles to its override, a "#rgb" or "#rrggbb" colour or a 256-colour
	// number 0-255, as written in the file. It is never nil; empty means no overrides.
	Colors map[string]string
}

// Default returns the configuration a missing or empty file means: theme auto, no colour
// overrides.
func Default() Config {
	return Config{UI: UI{Theme: ThemeAuto, Colors: map[string]string{}}}
}

// Error is a user config file that cannot be used. Line is the 1-based line of the offending key
// or value, or 0 when no single line is to blame.
type Error struct {
	Path    string
	Line    int
	Message string
}

// Error renders "path:line: message", or "path: message" when the line is unknown.
func (e *Error) Error() string {
	return located(e.Path, e.Line, e.Message)
}

// Warning is a problem Load ran past: an unknown key or colour role, ignored, or a file that could
// not be created. Line is 1-based, or 0 when no single line is to blame.
type Warning struct {
	Path    string
	Line    int
	Message string
}

// String renders "path:line: message", or "path: message" when the line is unknown.
func (w Warning) String() string {
	return located(w.Path, w.Line, w.Message)
}

// located renders a message attributed to path and, when known, line.
func located(path string, line int, message string) string {
	if line > 0 {
		return fmt.Sprintf("%s:%d: %s", path, line, message)
	}
	return fmt.Sprintf("%s: %s", path, message)
}

// Path returns `<os.UserConfigDir>/capcom/config.yaml`. It fails when the platform's user config
// directory is unknown, for example with neither $XDG_CONFIG_HOME nor $HOME set on Linux.
func Path() (string, error) {
	return capcomFile(fileName)
}

// capcomFile returns `<os.UserConfigDir>/capcom/<name>`, failing when the platform's user config
// directory is unknown.
func capcomFile(name string) (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(directory, directoryName, name), nil
}

// Load reads the user config file at path. A missing file is first created, its directory too,
// with every key as a commented default; when that fails Load warns and returns Default. An
// existing file is never written, so loading it again leaves it byte-identical.
//
// Keys the file leaves out take their Default value. An unknown key or colour role is a Warning
// and is ignored. Malformed YAML, a theme other than auto, dark or light, a colour that is not
// "#rgb", "#rrggbb" or a number 0-255, and a file that cannot be read are an *Error; the Config is
// then Default, so the caller can run on it.
func Load(path string) (Config, []Warning, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return create(path)
	}
	if err != nil {
		return Default(), nil, &Error{Path: path, Message: fmt.Sprintf("cannot read the file: %v", err)}
	}
	return parse(path, data)
}

// create writes the commented template at path and returns Default. A failure to create the
// directory or the file is a Warning. A file another process created first is loaded instead.
func create(path string) (Config, []Warning, error) {
	err := os.MkdirAll(filepath.Dir(path), directoryMode)
	if err == nil {
		err = writeNew(path, []byte(template))
	}
	if errors.Is(err, fs.ErrExist) {
		return Load(path)
	}
	if err != nil {
		message := fmt.Sprintf("cannot create the config file, running on the defaults: %v", err)
		return Default(), []Warning{{Path: path, Message: message}}, nil
	}
	return Default(), nil, nil
}

// writeNew writes data to path, failing with fs.ErrExist rather than replacing an existing file.
func writeNew(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// parse decodes the content of the user config file at path; Load describes the rules.
func parse(path string, data []byte) (Config, []Warning, error) {
	p := &parser{path: path}
	config := Default()

	root, err := p.document(data)
	if err == nil && root != nil {
		err = p.topLevel(root, &config)
	}
	if err != nil {
		return Default(), nil, err
	}
	return config, p.warnings, nil
}

// parser decodes one user config file, attributing every problem to its path and line.
type parser struct {
	path     string
	warnings []Warning
}

// errorAt returns an *Error at node's line.
func (p *parser) errorAt(node *yaml.Node, format string, args ...any) *Error {
	return &Error{Path: p.path, Line: node.Line, Message: fmt.Sprintf(format, args...)}
}

// warnAt records a Warning at node's line.
func (p *parser) warnAt(node *yaml.Node, format string, args ...any) {
	p.warnings = append(p.warnings, Warning{Path: p.path, Line: node.Line, Message: fmt.Sprintf(format, args...)})
}

// document parses data into its top-level mapping node, or nil for an empty or all-comment
// document. A second YAML document is an error, so no key escapes the checks behind a `---` line.
func (p *parser) document(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, p.yamlError(err)
	}
	var second yaml.Node
	if err := decoder.Decode(&second); err != io.EOF {
		if err != nil {
			return nil, p.yamlError(err)
		}
		return nil, p.errorAt(&second, "only one YAML document is allowed")
	}
	if len(document.Content) == 0 || isNull(document.Content[0]) {
		return nil, nil
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, p.errorAt(root, "the config must be a mapping of keys")
	}
	return root, nil
}

// yamlError turns a yaml.v3 syntax error into an *Error, lifting its line out of the message.
func (p *parser) yamlError(err error) *Error {
	failure := &Error{Path: p.path, Message: "invalid YAML: " + strings.TrimPrefix(err.Error(), "yaml: ")}
	if match := yamlLine.FindStringSubmatch(err.Error()); match != nil {
		line, convErr := strconv.Atoi(match[1])
		if convErr == nil {
			failure.Line = line
			failure.Message = "invalid YAML: " + match[2]
		}
	}
	return failure
}

// topLevel applies every top-level key of root onto config.
func (p *parser) topLevel(root *yaml.Node, config *Config) error {
	return p.eachKey(root, func(key, value *yaml.Node) error {
		if key.Value != keyUI {
			p.warnAt(key, "unknown key %q, ignored", key.Value)
			return nil
		}
		return p.ui(value, &config.UI)
	})
}

// ui applies the ui mapping onto settings; a null ui leaves the defaults.
func (p *parser) ui(node *yaml.Node, settings *UI) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return p.errorAt(node, "%s: expected a mapping of keys", keyUI)
	}
	return p.eachKey(node, func(key, value *yaml.Node) error {
		switch key.Value {
		case keyTheme:
			return p.theme(value, settings)
		case keyColors:
			return p.colors(value, settings)
		default:
			p.warnAt(key, "unknown key %q in %s, ignored", key.Value, keyUI)
			return nil
		}
	})
}

// theme applies ui.theme onto settings; a null theme leaves the default.
func (p *parser) theme(node *yaml.Node, settings *UI) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.ScalarNode || !slices.Contains(themes, node.Value) {
		return p.errorAt(node, "%s.%s: expected %s, %s or %s", keyUI, keyTheme, ThemeAuto, ThemeDark, ThemeLight)
	}
	settings.Theme = node.Value
	return nil
}

// colors applies the ui.colors mapping onto settings; a null colors leaves no overrides.
func (p *parser) colors(node *yaml.Node, settings *UI) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return p.errorAt(node, "%s.%s: expected a mapping of colour roles", keyUI, keyColors)
	}
	return p.eachKey(node, func(key, value *yaml.Node) error {
		if !slices.Contains(Roles, key.Value) {
			p.warnAt(key, "unknown colour role %q, ignored", key.Value)
			return nil
		}
		if !isColor(value) {
			return p.errorAt(value, "%s.%s.%s: expected a quoted \"#rgb\" or \"#rrggbb\" colour or a number 0-255"+
				" (an unquoted # starts a comment)", keyUI, keyColors, key.Value)
		}
		settings.Colors[key.Value] = value.Value
		return nil
	})
}

// eachKey calls apply for every key and value of mapping in file order, stopping at the first
// error. A key that is not a plain scalar is an error.
func (p *parser) eachKey(mapping *yaml.Node, apply func(key, value *yaml.Node) error) error {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		if key.Kind != yaml.ScalarNode {
			return p.errorAt(key, "a key must be a plain name")
		}
		if err := apply(key, value); err != nil {
			return err
		}
	}
	return nil
}

// isNull reports whether node is an absent value: `key:` with nothing after it, or `null`.
func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == "!!null"
}

// isColor reports whether node is a "#rgb" or "#rrggbb" colour or a 256-colour number 0-255.
func isColor(node *yaml.Node) bool {
	if node.Kind != yaml.ScalarNode || isNull(node) {
		return false
	}
	if hexColor.MatchString(node.Value) {
		return true
	}
	if !colorIndex.MatchString(node.Value) {
		return false
	}
	number, err := strconv.Atoi(node.Value)
	return err == nil && number <= maxColorIndex
}
