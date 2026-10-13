// Package hooks edits the files capcom init hooks capcom into. The agent-settings files run capcom
// prime: the Claude Code settings and the Codex hooks file, which share one hooks shape. Merge is
// the pure JSON edit; Install reads, merges and writes one Target in a working tree. Only the hooks
// member is ever rewritten: every other member keeps its key order and its value its bytes. The
// .gitattributes file hands issue files to the capcom-log merge driver: AppendMergeAttribute is the
// pure line edit, InstallMergeAttribute reads, appends and writes it in a working tree.
package hooks

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// PrimeCommand is the command the Claude Code and Codex hooks run.
const PrimeCommand = "capcom prime"

// Keys and layout of the settings files capcom writes.
const (
	settingsHooksKey  = "hooks"
	hookTypeCommand   = "command"
	settingsIndent    = "  "
	settingsSeparator = ","
)

// Status is what Install did to a settings file.
type Status string

// Status values of Install.
const (
	// Added means the file was written: at least one event gained the capcom prime group.
	Added Status = "added"
	// Present means every event already ran capcom prime; the file was left as it is.
	Present Status = "present"
)

// Error is a file that cannot be read, a .gitattributes that cannot be replaced, or a settings file
// that is not shaped as Claude Code and Codex read it. Path is the file relative to the working tree
// root, slash-separated; Line is the 1-based line of a JSON syntax error, or 0 when no single line is
// to blame.
type Error struct {
	Path    string
	Line    int
	Message string
}

// Error renders "path:line: message", or "path: message" when the line is unknown.
func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// settingsHook is one hook of a hook group.
type settingsHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookGroup is one entry of a hook event: a matcher and the hooks it runs.
type hookGroup struct {
	Matcher string         `json:"matcher,omitempty"`
	Hooks   []settingsHook `json:"hooks"`
}

// hookEvent is a hook event and the group capcom adds to it.
type hookEvent struct {
	name  string
	group hookGroup
}

// Target is a settings file the capcom prime hooks are merged into. Path is relative to the
// working tree root, slash-separated.
type Target struct {
	Path   string
	events []hookEvent
}

// Claude is the Claude Code settings file. Its hooks run capcom prime at every session start,
// including a resume, a /clear and the restart after compaction, and at the moment before
// compaction.
var Claude = Target{Path: ".claude/settings.json", events: []hookEvent{
	{name: "SessionStart", group: hookGroup{
		Matcher: "startup|resume|clear|compact",
		Hooks:   []settingsHook{{Type: hookTypeCommand, Command: PrimeCommand}},
	}},
	{name: "PreCompact", group: hookGroup{
		Hooks: []settingsHook{{Type: hookTypeCommand, Command: PrimeCommand}},
	}},
}}

// Codex is the Codex hooks file. Its hooks run capcom prime at every session start, including a
// resume and a /clear, and at the moment before a manual or automatic compaction.
var Codex = Target{Path: ".codex/hooks.json", events: []hookEvent{
	{name: "SessionStart", group: hookGroup{
		Matcher: "startup|resume|clear",
		Hooks:   []settingsHook{{Type: hookTypeCommand, Command: PrimeCommand}},
	}},
	{name: "PreCompact", group: hookGroup{
		Matcher: "manual|auto",
		Hooks:   []settingsHook{{Type: hookTypeCommand, Command: PrimeCommand}},
	}},
}}

// jsonMember is one member of a JSON object, its value kept byte for byte.
type jsonMember struct {
	key   string
	value json.RawMessage
}

// Snippet returns the settings that make target's agent run capcom prime, as a JSON object holding
// only the hooks member, indented two spaces and ending in a line break.
func Snippet(target Target) []byte {
	var hooks []jsonMember
	for _, event := range target.events {
		hooks = append(hooks, jsonMember{event.name, jsonArray([]json.RawMessage{encodeJSON(event.group)})})
	}
	return indentJSON(encodeObject([]jsonMember{{settingsHooksKey, encodeObject(hooks)}}))
}

// Install merges the capcom prime hooks into target's file in the working tree at worktreeRoot,
// creating the file and its directory when missing, and reports whether it wrote the file. A file
// that cannot be read, or that Merge refuses, fails as *Error naming target.Path and leaves the
// file untouched; a failed write returns nodefile.WriteFileAtomic's error as is.
func Install(worktreeRoot string, target Target) (Status, error) {
	path := filepath.Join(worktreeRoot, filepath.FromSlash(target.Path))
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", &Error{Path: target.Path, Message: err.Error()}
	}
	merged, isAdded, err := Merge(content, target)
	if err != nil {
		return "", err
	}
	if !isAdded {
		return Present, nil
	}
	if err := nodefile.WriteFileAtomic(path, bytes.NewReader(merged)); err != nil {
		return "", err
	}
	return Added, nil
}

// Merge adds the capcom prime hook group to each event of target in the settings content, which may
// be empty or null for a missing file. An event that already runs capcom prime is left as it is;
// isAdded reports whether any event changed, and when it is false merged is content unchanged, so a
// second merge changes nothing. The merged file keeps every member other than hooks in key order
// with its value's bytes, re-indented two spaces. It fails as *Error naming target.Path on content
// that is not a JSON object, or whose hooks are not an object of arrays; a syntax error carries
// its line.
func Merge(content []byte, target Target) (merged []byte, isAdded bool, err error) {
	settings, err := decodeObject(content)
	if err != nil {
		return nil, false, settingsFailure(target.Path, content, "", err)
	}
	position := slices.IndexFunc(settings, func(member jsonMember) bool { return member.key == settingsHooksKey })
	var hooks []jsonMember
	if position >= 0 {
		if hooks, err = decodeObject(settings[position].value); err != nil {
			return nil, false, settingsFailure(target.Path, nil, settingsHooksKey, err)
		}
	}

	for _, event := range target.events {
		isEventAdded, err := addHookGroup(target.Path, &hooks, event)
		if err != nil {
			return nil, false, err
		}
		isAdded = isAdded || isEventAdded
	}
	if !isAdded {
		return content, false, nil
	}
	if position >= 0 {
		settings[position].value = encodeObject(hooks)
	} else {
		settings = append(settings, jsonMember{settingsHooksKey, encodeObject(hooks)})
	}
	return indentJSON(encodeObject(settings)), true, nil
}

// addHookGroup appends event's group to the event's array in hooks, read from the settings file at
// path, unless one of its groups already runs capcom prime. Existing groups are kept byte for byte.
func addHookGroup(path string, hooks *[]jsonMember, event hookEvent) (bool, error) {
	position := slices.IndexFunc(*hooks, func(member jsonMember) bool { return member.key == event.name })
	var groups []json.RawMessage
	if position >= 0 {
		if err := json.Unmarshal((*hooks)[position].value, &groups); err != nil {
			return false, &Error{
				Path:    path,
				Message: settingsHooksKey + "." + event.name + " must be an array of hook groups",
			}
		}
	}
	for _, raw := range groups {
		var group hookGroup
		if json.Unmarshal(raw, &group) != nil {
			continue // a group of another shape is not capcom's
		}
		if slices.ContainsFunc(group.Hooks, func(hook settingsHook) bool { return hook.Command == PrimeCommand }) {
			return false, nil
		}
	}

	merged := jsonArray(append(groups, encodeJSON(event.group)))
	if position >= 0 {
		(*hooks)[position].value = merged
	} else {
		*hooks = append(*hooks, jsonMember{event.name, merged})
	}
	return true, nil
}

// errNotObject reports JSON that is not an object.
var errNotObject = errors.New("must be a JSON object")

// decodeObject reads the members of the JSON object content in file order, their values byte for
// byte. Empty content, or null, is an object without members.
func decodeObject(content []byte) ([]jsonMember, error) {
	if trimmed := bytes.TrimSpace(content); len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, cmp.Or(err, errNotObject)
	}
	var members []jsonMember
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		members = append(members, jsonMember{token.(string), value}) // an object key is a string
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the JSON object")
	}
	return members, nil
}

// settingsFailure types a settings file that cannot be read as an object: an *Error at path, at
// the line of a syntax error in content. member names the member at fault, if any.
func settingsFailure(path string, content []byte, member string, err error) *Error {
	failure := &Error{Path: path, Message: "invalid settings: " + err.Error()}
	if member != "" {
		failure.Message = member + " " + err.Error()
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) && content != nil {
		failure.Line = 1 + bytes.Count(content[:min(int(syntax.Offset), len(content))], []byte("\n"))
	}
	return failure
}

// encodeObject writes members as a compact JSON object, values verbatim.
func encodeObject(members []jsonMember) json.RawMessage {
	var object bytes.Buffer
	object.WriteString("{")
	for i, member := range members {
		if i > 0 {
			object.WriteString(settingsSeparator)
		}
		object.Write(encodeJSON(member.key))
		object.WriteString(":")
		object.Write(member.value)
	}
	object.WriteString("}")
	return object.Bytes()
}

// jsonArray writes values as a compact JSON array, each verbatim.
func jsonArray(values []json.RawMessage) json.RawMessage {
	array := []byte("[")
	for i, value := range values {
		if i > 0 {
			array = append(array, settingsSeparator...)
		}
		array = append(array, value...)
	}
	return append(array, ']')
}

// encodeJSON encodes value without escaping <, > and &, which settings commands use.
func encodeJSON(value any) json.RawMessage {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	// The values encoded here are strings and hook groups, which always encode.
	_ = encoder.Encode(value)
	return bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))
}

// indentJSON lays out valid compact or indented JSON two spaces per level, ending in a line break;
// the bytes inside strings and numbers are kept.
func indentJSON(compact json.RawMessage) []byte {
	var indented bytes.Buffer
	// compact is built from valid JSON values, so it always indents.
	_ = json.Indent(&indented, compact, "", settingsIndent)
	indented.WriteString("\n")
	return indented.Bytes()
}
