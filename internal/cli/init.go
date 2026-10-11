package cli

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// initData is the --json data of capcom init.
type initData struct {
	// Root is the absolute store directory.
	Root string `json:"root"`
	// Created lists the paths init created, relative to the working tree root; empty on a re-run.
	Created []string `json:"created"`
	// ClaudeHooks is what --claude-hooks did to the Claude Code settings: added or present.
	ClaudeHooks string `json:"claude_hooks,omitempty"`
}

// flagClaudeHooks makes init add the capcom prime hooks to the Claude Code settings.
const flagClaudeHooks = "claude-hooks"

// ClaudeHooks values of initData.
const (
	claudeHooksAdded   = "added"
	claudeHooksPresent = "present"
)

// newInitCommand creates the store in the current git working tree, or does nothing when it
// already exists. With --claude-hooks it also merges the hooks running capcom prime into
// .claude/settings.json; without it, text mode prints those hooks for the user to add.
func (a *app) newInitCommand() *cobra.Command {
	isClaudeHooks := false
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the capcom store (.capcom/) in this git repository",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opened, created, err := store.Init(cwd)
			if err != nil {
				return storeFailure(err)
			}
			if created == nil {
				created = []string{}
			}
			data := initData{Root: opened.Root, Created: created}
			text := initText(opened.Root, created)
			if !isClaudeHooks {
				return a.emit(cmd, Result{Kind: "init", Data: data, Text: text + "\n" + claudeHooksSnippet()})
			}
			isAdded, err := mergeClaudeHooks(opened.WorktreeRoot)
			if err != nil {
				return err
			}
			if isAdded {
				data.ClaudeHooks = claudeHooksAdded
				text += "added the " + primeCommand + " hooks to " + claudeSettingsPath + "\n"
			} else {
				data.ClaudeHooks = claudeHooksPresent
				text += claudeSettingsPath + " already runs " + primeCommand + "\n"
			}
			return a.emit(cmd, Result{Kind: "init", Data: data, Text: text})
		}),
	}
	cmd.Flags().BoolVar(&isClaudeHooks, flagClaudeHooks, false,
		"also add SessionStart and PreCompact hooks running capcom prime to "+claudeSettingsPath)
	return cmd
}

func initText(root string, created []string) string {
	if len(created) == 0 {
		return "capcom store already initialized at " + root + "\n"
	}
	var text strings.Builder
	text.WriteString("initialized capcom store at " + root + "\n")
	for _, path := range created {
		text.WriteString("created " + path + "\n")
	}
	return text.String()
}

// storeFailure types a failure to open or create the store: a bad config.yaml is invalid at its
// file and line, a missing store is not_initialized, and a directory outside git is invalid.
func storeFailure(err error) *Error {
	var configFailure *config.Error
	switch {
	case errors.As(err, &configFailure):
		return &Error{
			Code:    CodeInvalid,
			Message: configFailure.Message,
			Path:    configFailure.Path,
			Line:    configFailure.Line,
		}
	case errors.Is(err, store.ErrNotInitialized):
		return Errorf(CodeNotInitialized, "%s", err.Error())
	default:
		return Errorf(CodeInvalid, "%s", err.Error())
	}
}

// The Claude Code settings file init --claude-hooks merges into, relative to the working tree root.
const claudeSettingsPath = ".claude/settings.json"

// primeCommand is the command the Claude Code hooks run.
const primeCommand = "capcom prime"

// Keys of the Claude Code settings capcom writes.
const (
	settingsHooksKey  = "hooks"
	hookTypeCommand   = "command"
	settingsIndent    = "  "
	settingsSeparator = ","
)

// claudeHook is one hook of a Claude Code hook group.
type claudeHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// claudeHookGroup is one entry of a Claude Code hook event: a matcher and the hooks it runs.
type claudeHookGroup struct {
	Matcher string       `json:"matcher,omitempty"`
	Hooks   []claudeHook `json:"hooks"`
}

// claudeHookEvent is a Claude Code hook event and the group capcom adds to it.
type claudeHookEvent struct {
	name  string
	group claudeHookGroup
}

// claudeHookEvents are the events whose hooks run capcom prime: every session start, including a
// resume, a /clear and the restart after compaction, and the moment before compaction.
var claudeHookEvents = []claudeHookEvent{
	{name: "SessionStart", group: claudeHookGroup{
		Matcher: "startup|resume|clear|compact",
		Hooks:   []claudeHook{{Type: hookTypeCommand, Command: primeCommand}},
	}},
	{name: "PreCompact", group: claudeHookGroup{
		Hooks: []claudeHook{{Type: hookTypeCommand, Command: primeCommand}},
	}},
}

// jsonMember is one member of a JSON object, its value kept byte for byte.
type jsonMember struct {
	key   string
	value json.RawMessage
}

// claudeHooksSnippet is what plain init prints: the settings that make Claude Code run capcom
// prime, for the user to add by hand or through init --claude-hooks.
func claudeHooksSnippet() string {
	var hooks []jsonMember
	for _, event := range claudeHookEvents {
		hooks = append(hooks, jsonMember{event.name, jsonArray([]json.RawMessage{encodeJSON(event.group)})})
	}
	settings := []jsonMember{{settingsHooksKey, encodeObject(hooks)}}
	return "To brief Claude Code at every session start and before compaction, add to " +
		claudeSettingsPath + " (or run capcom init --" + flagClaudeHooks + "):\n" + string(indentJSON(encodeObject(settings)))
}

// mergeClaudeHooks adds the capcom prime hook group to each event of claudeHookEvents in the Claude
// Code settings of the working tree at worktreeRoot, creating the file when missing. An event that
// already runs capcom prime is left as it is, so a re-run changes nothing; isAdded reports whether
// the file was written. Only the hooks member is rewritten: every other member keeps its key order
// and its values their bytes, re-indented two spaces. It fails, naming the file, on a settings file
// that is not a JSON object or whose hooks are not shaped as Claude Code reads them.
func mergeClaudeHooks(worktreeRoot string) (isAdded bool, err error) {
	path := filepath.Join(worktreeRoot, filepath.FromSlash(claudeSettingsPath))
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, &Error{Code: CodeInvalid, Path: claudeSettingsPath, Message: err.Error()}
	}
	settings, err := decodeObject(content)
	if err != nil {
		return false, settingsFailure(content, "", err)
	}
	position := slices.IndexFunc(settings, func(member jsonMember) bool { return member.key == settingsHooksKey })
	var hooks []jsonMember
	if position >= 0 {
		if hooks, err = decodeObject(settings[position].value); err != nil {
			return false, settingsFailure(nil, settingsHooksKey, err)
		}
	}

	for _, event := range claudeHookEvents {
		isEventAdded, err := addHookGroup(&hooks, event)
		if err != nil {
			return false, err
		}
		isAdded = isAdded || isEventAdded
	}
	if !isAdded {
		return false, nil
	}
	if position >= 0 {
		settings[position].value = encodeObject(hooks)
	} else {
		settings = append(settings, jsonMember{settingsHooksKey, encodeObject(hooks)})
	}
	if err := tree.WriteFileAtomic(path, bytes.NewReader(indentJSON(encodeObject(settings)))); err != nil {
		return false, asError(err)
	}
	return true, nil
}

// addHookGroup appends event's group to the event's array in hooks, unless one of its groups
// already runs capcom prime. Existing groups are kept byte for byte.
func addHookGroup(hooks *[]jsonMember, event claudeHookEvent) (bool, error) {
	position := slices.IndexFunc(*hooks, func(member jsonMember) bool { return member.key == event.name })
	var groups []json.RawMessage
	if position >= 0 {
		if err := json.Unmarshal((*hooks)[position].value, &groups); err != nil {
			return false, &Error{
				Code:    CodeInvalid,
				Path:    claudeSettingsPath,
				Message: settingsHooksKey + "." + event.name + " must be an array of hook groups",
			}
		}
	}
	for _, raw := range groups {
		var group claudeHookGroup
		if json.Unmarshal(raw, &group) != nil {
			continue // a group of another shape is not capcom's
		}
		if slices.ContainsFunc(group.Hooks, func(hook claudeHook) bool { return hook.Command == primeCommand }) {
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

// settingsFailure types a settings file that cannot be read as an object: invalid at the file,
// and at the line of a syntax error in content. member names the member at fault, if any.
func settingsFailure(content []byte, member string, err error) *Error {
	failure := &Error{Code: CodeInvalid, Path: claudeSettingsPath, Message: "invalid settings: " + err.Error()}
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
