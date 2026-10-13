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

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
)

// initData is the --json data of capcom init.
type initData struct {
	// Root is the absolute store directory.
	Root string `json:"root"`
	// Created lists the paths init created, relative to the working tree root; empty on a re-run.
	Created []string `json:"created"`
	// ClaudeHooks is what --claude-hooks did to the Claude Code settings: added or present.
	ClaudeHooks string `json:"claude_hooks,omitempty"`
	// CodexHooks is what --codex-hooks did to the Codex hooks file: added or present.
	CodexHooks string `json:"codex_hooks,omitempty"`
	// MergeDriver is what --merge-driver did to .gitattributes: added or present.
	MergeDriver string `json:"merge_driver,omitempty"`
	// GitConfig lists the git config commands --merge-driver leaves for the user to run.
	GitConfig []string `json:"git_config,omitempty"`
}

// Flags of init: add the capcom prime hooks to the Claude Code settings or to the Codex hooks, or
// hand issue files to the capcom-log merge driver in .gitattributes.
const (
	flagClaudeHooks = "claude-hooks"
	flagCodexHooks  = "codex-hooks"
	flagMergeDriver = "merge-driver"
)

// codexFeatureNote is what init --codex-hooks prints in text mode: Codex ignores its hooks file
// unless the feature is on.
const codexFeatureNote = "Codex runs hooks only with [features] hooks = true in its config.toml\n"

// ClaudeHooks, CodexHooks and MergeDriver values of initData.
const (
	claudeHooksAdded   = "added"
	claudeHooksPresent = "present"
)

// newInitCommand creates the store in the current git working tree, or does nothing when it
// already exists. With --claude-hooks it also merges the hooks running capcom prime into
// .claude/settings.json; without it, text mode prints those hooks for the user to add. With
// --codex-hooks it merges the same hooks into .codex/hooks.json. With --merge-driver it adds the
// capcom-log merge driver line to .gitattributes and prints the git config commands that define the
// driver. The three flags are independent.
func (a *app) newInitCommand() *cobra.Command {
	isClaudeHooks := false
	isCodexHooks := false
	isMergeDriver := false
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
				return output.StoreFailure(err)
			}
			if created == nil {
				created = []string{}
			}
			data := initData{Root: opened.Root, Created: created}
			text := initText(opened.Root, created)
			snippet := "\n" + claudeHooksSnippet()
			if isClaudeHooks {
				var line string
				if data.ClaudeHooks, line, err = installHooks(opened.WorktreeRoot, claudeHooks); err != nil {
					return err
				}
				text += line
				snippet = ""
			}
			if isCodexHooks {
				var line string
				if data.CodexHooks, line, err = installHooks(opened.WorktreeRoot, codexHooks); err != nil {
					return err
				}
				text += line + codexFeatureNote
			}
			if isMergeDriver {
				var line string
				if data.MergeDriver, line, err = installMergeDriver(opened.WorktreeRoot); err != nil {
					return err
				}
				data.GitConfig = mergeDriverConfig
				text += line + mergeDriverConfigText()
			}
			return a.emit(cmd, output.Result{Kind: "init", Data: data, Text: text + snippet})
		}),
	}
	cmd.Flags().BoolVar(&isClaudeHooks, flagClaudeHooks, false,
		"also add SessionStart and PreCompact hooks running capcom prime to "+claudeSettingsPath)
	cmd.Flags().BoolVar(&isCodexHooks, flagCodexHooks, false,
		"also add SessionStart and PreCompact hooks running capcom prime to "+codexHooksPath)
	cmd.Flags().BoolVar(&isMergeDriver, flagMergeDriver, false,
		"also hand issue files to the capcom-log merge driver in "+gitattributesPath)
	return cmd
}

// installHooks merges the capcom prime hooks into target's settings file and returns the initData
// value and the text line saying what it did.
func installHooks(worktreeRoot string, target hookSettings) (status string, line string, err error) {
	isAdded, err := mergeHooks(worktreeRoot, target)
	if err != nil {
		return "", "", err
	}
	if isAdded {
		return claudeHooksAdded, "added the " + primeCommand + " hooks to " + target.path + "\n", nil
	}
	return claudeHooksPresent, target.path + " already runs " + primeCommand + "\n", nil
}

// The .gitattributes file init --merge-driver adds to, relative to the working tree root.
const gitattributesPath = ".gitattributes"

// mergeDriverAttribute is the .gitattributes line that hands issue files to the capcom-log merge
// driver.
const mergeDriverAttribute = ".capcom/issues/**/*.md merge=capcom-log"

// mergeDriverConfig are the git config commands that define the capcom-log merge driver. capcom
// never writes .git/config, so init --merge-driver prints them for the user to run.
var mergeDriverConfig = []string{
	`git config merge.capcom-log.name "capcom Log merge"`,
	`git config merge.capcom-log.driver "capcom merge-driver %O %A %B %P %S %X %Y"`,
}

// Line endings of a .gitattributes file.
const (
	lineEndingLF   = "\n"
	lineEndingCRLF = "\r\n"
)

// installMergeDriver adds the capcom-log merge driver line to .gitattributes in the working tree at
// worktreeRoot and returns the initData value and the text line saying what it did.
func installMergeDriver(worktreeRoot string) (status string, line string, err error) {
	isAdded, err := addMergeAttribute(worktreeRoot)
	if err != nil {
		return "", "", err
	}
	if isAdded {
		return claudeHooksAdded, "added the capcom-log merge driver to " + gitattributesPath + "\n", nil
	}
	return claudeHooksPresent, gitattributesPath + " already sets the capcom-log merge driver\n", nil
}

// mergeDriverConfigText is what init --merge-driver prints in text mode: git runs the driver only
// once the repository's config defines it.
func mergeDriverConfigText() string {
	return "git uses the capcom-log merge driver only once it is configured; run:\n  " +
		strings.Join(mergeDriverConfig, "\n  ") + "\n"
}

// addMergeAttribute appends the merge driver line to the .gitattributes file at worktreeRoot,
// creating the file when missing; isAdded reports whether it was written. A file that already holds
// the line is left as it is, so a re-run changes nothing. An existing file keeps its bytes, its
// permissions and its line ending: it is replaced byte for byte, never through nodefile.WriteFileAtomic,
// which would turn CRLF into LF and reset the mode.
func addMergeAttribute(worktreeRoot string) (isAdded bool, err error) {
	path := filepath.Join(worktreeRoot, gitattributesPath)
	content, err := os.ReadFile(path)
	isMissing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !isMissing {
		return false, &output.Error{Code: output.CodeInvalid, Path: gitattributesPath, Message: err.Error()}
	}
	if hasMergeAttribute(content) {
		return false, nil
	}
	if isMissing {
		// A new file has no bytes, mode or line ending to keep.
		if err := nodefile.WriteFileAtomic(path, strings.NewReader(mergeDriverAttribute+lineEndingLF)); err != nil {
			return false, output.AsError(err)
		}
		return true, nil
	}
	if err := nodefile.ReplaceFileVerbatim(path, bytes.NewReader(appendAttributeLine(content))); err != nil {
		return false, &output.Error{Code: output.CodeInvalid, Path: gitattributesPath, Message: err.Error()}
	}
	return true, nil
}

// hasMergeAttribute reports whether the .gitattributes content holds the merge driver line, however
// its fields are spaced.
func hasMergeAttribute(content []byte) bool {
	want := strings.Fields(mergeDriverAttribute)
	for line := range strings.Lines(string(content)) {
		if slices.Equal(strings.Fields(line), want) {
			return true
		}
	}
	return false
}

// appendAttributeLine returns the .gitattributes content with the merge driver line appended, ended
// as the file's lines are: CRLF when the file holds a CRLF, LF otherwise. A last line without a line
// ending gets one first.
func appendAttributeLine(content []byte) []byte {
	ending := lineEndingLF
	if bytes.Contains(content, []byte(lineEndingCRLF)) {
		ending = lineEndingCRLF
	}
	if len(content) > 0 && !bytes.HasSuffix(content, []byte(lineEndingLF)) {
		content = append(content, ending...)
	}
	content = append(content, mergeDriverAttribute...)
	return append(content, ending...)
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

// The Claude Code settings file init --claude-hooks merges into, relative to the working tree root.
const claudeSettingsPath = ".claude/settings.json"

// The Codex hooks file init --codex-hooks merges into, relative to the working tree root.
const codexHooksPath = ".codex/hooks.json"

// primeCommand is the command the Claude Code and Codex hooks run.
const primeCommand = "capcom prime"

// Keys of the Claude Code settings and Codex hooks capcom writes; both share one hooks shape.
const (
	settingsHooksKey  = "hooks"
	hookTypeCommand   = "command"
	settingsIndent    = "  "
	settingsSeparator = ","
)

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

// hookSettings is a settings file init merges the capcom prime hooks into: its path relative to
// the working tree root, and the events whose hooks run capcom prime.
type hookSettings struct {
	path   string
	events []hookEvent
}

// claudeHookEvents are the Claude Code events whose hooks run capcom prime: every session start,
// including a resume, a /clear and the restart after compaction, and the moment before compaction.
var claudeHookEvents = []hookEvent{
	{name: "SessionStart", group: hookGroup{
		Matcher: "startup|resume|clear|compact",
		Hooks:   []settingsHook{{Type: hookTypeCommand, Command: primeCommand}},
	}},
	{name: "PreCompact", group: hookGroup{
		Hooks: []settingsHook{{Type: hookTypeCommand, Command: primeCommand}},
	}},
}

// codexHookEvents are the Codex events whose hooks run capcom prime: every session start, including
// a resume and a /clear, and the moment before a manual or automatic compaction.
var codexHookEvents = []hookEvent{
	{name: "SessionStart", group: hookGroup{
		Matcher: "startup|resume|clear",
		Hooks:   []settingsHook{{Type: hookTypeCommand, Command: primeCommand}},
	}},
	{name: "PreCompact", group: hookGroup{
		Matcher: "manual|auto",
		Hooks:   []settingsHook{{Type: hookTypeCommand, Command: primeCommand}},
	}},
}

// The settings files init merges the capcom prime hooks into.
var (
	claudeHooks = hookSettings{path: claudeSettingsPath, events: claudeHookEvents}
	codexHooks  = hookSettings{path: codexHooksPath, events: codexHookEvents}
)

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

// mergeHooks adds the capcom prime hook group to each event of target in target's settings file in
// the working tree at worktreeRoot, creating the file when missing. An event that already runs
// capcom prime is left as it is, so a re-run changes nothing; isAdded reports whether the file was
// written. Only the hooks member is rewritten: every other member keeps its key order and its
// values their bytes, re-indented two spaces. It fails, naming the file, on a settings file that is
// not a JSON object or whose hooks are not shaped as Claude Code and Codex read them.
func mergeHooks(worktreeRoot string, target hookSettings) (isAdded bool, err error) {
	path := filepath.Join(worktreeRoot, filepath.FromSlash(target.path))
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, &output.Error{Code: output.CodeInvalid, Path: target.path, Message: err.Error()}
	}
	settings, err := decodeObject(content)
	if err != nil {
		return false, settingsFailure(target.path, content, "", err)
	}
	position := slices.IndexFunc(settings, func(member jsonMember) bool { return member.key == settingsHooksKey })
	var hooks []jsonMember
	if position >= 0 {
		if hooks, err = decodeObject(settings[position].value); err != nil {
			return false, settingsFailure(target.path, nil, settingsHooksKey, err)
		}
	}

	for _, event := range target.events {
		isEventAdded, err := addHookGroup(target.path, &hooks, event)
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
	if err := nodefile.WriteFileAtomic(path, bytes.NewReader(indentJSON(encodeObject(settings)))); err != nil {
		return false, output.AsError(err)
	}
	return true, nil
}

// addHookGroup appends event's group to the event's array in hooks, read from the settings file at
// path, unless one of its groups already runs capcom prime. Existing groups are kept byte for byte.
func addHookGroup(path string, hooks *[]jsonMember, event hookEvent) (bool, error) {
	position := slices.IndexFunc(*hooks, func(member jsonMember) bool { return member.key == event.name })
	var groups []json.RawMessage
	if position >= 0 {
		if err := json.Unmarshal((*hooks)[position].value, &groups); err != nil {
			return false, &output.Error{
				Code:    output.CodeInvalid,
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
		if slices.ContainsFunc(group.Hooks, func(hook settingsHook) bool { return hook.Command == primeCommand }) {
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

// settingsFailure types a settings file that cannot be read as an object: invalid at the file at
// path, and at the line of a syntax error in content. member names the member at fault, if any.
func settingsFailure(path string, content []byte, member string, err error) *output.Error {
	failure := &output.Error{Code: output.CodeInvalid, Path: path, Message: "invalid settings: " + err.Error()}
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
