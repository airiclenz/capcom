package hooks

import (
	"encoding/json"
	"path/filepath"
	"slices"
)

// CopilotPath is the GitHub Copilot hooks file InstallCopilot adds the capcom prime hook to,
// relative to the working tree root. Copilot CLI, VS Code and the Copilot cloud agent read it.
const CopilotPath = ".github/hooks/capcom.json"

// CopilotPrimeCommand is the command the Copilot sessionStart hook runs: capcom prime printing the
// Briefing as the JSON object whose additionalContext a sessionStart hook hands the model.
const CopilotPrimeCommand = PrimeCommand + " --additional-context"

// Keys and values of the Copilot hooks file.
const (
	copilotVersionKey      = "version"
	copilotVersion         = 1
	copilotSessionStartKey = "sessionStart"
)

// Refusals of a Copilot hooks file MergeCopilot cannot edit.
const (
	copilotVersionMessage      = "version must be 1"
	copilotSessionStartMessage = settingsHooksKey + "." + copilotSessionStartKey + " must be an array of hooks"
)

// copilotHook is one entry of a Copilot hook event, as capcom writes it.
type copilotHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// copilotHookCommands are the members of a Copilot hook entry that name the command it runs.
type copilotHookCommands struct {
	Command string `json:"command"`
	Bash    string `json:"bash"`
}

// InstallCopilot adds the capcom prime sessionStart hook to the Copilot hooks file in the working
// tree at worktreeRoot, creating the file and its directory when missing, and reports whether it
// wrote the file. The file is read through nodefile.ReadRegularFile, so a symlink is never followed
// and a named pipe never blocks. An existing file keeps its permission bits: it is replaced through
// nodefile.ReplaceFileVerbatim, never through nodefile.WriteFileAtomic, which would reset the mode.
// A file that is not a regular file, is larger than 1 MiB, cannot be read or replaced, or that
// MergeCopilot refuses fails as *Error naming CopilotPath and leaves the file untouched; a failed
// write of a new file returns nodefile.WriteFileAtomic's error as is.
func InstallCopilot(worktreeRoot string) (Status, error) {
	path := filepath.Join(worktreeRoot, filepath.FromSlash(CopilotPath))
	content, isMissing, err := readWorktreeFile(path, CopilotPath)
	if err != nil {
		return "", err
	}
	merged, status, err := MergeCopilot(content)
	if err != nil || status == Present {
		return status, err
	}
	if err := writeWorktreeFile(path, CopilotPath, merged, isMissing); err != nil {
		return "", err
	}
	return Added, nil
}

// MergeCopilot appends the hook {"type": "command", "command": CopilotPrimeCommand} to the
// sessionStart array of the Copilot hooks file content, which may be empty or null for a missing
// file; a file without a version member gains "version": 1 as its first member. When a sessionStart
// entry's command or bash already is CopilotPrimeCommand, merged is content unchanged and status is
// Present. Entries of another shape are kept as they are. The merged file keeps every other member
// in key order with its value's bytes, re-indented two spaces. It fails as *Error naming
// CopilotPath on content that is not a JSON object, a version other than the number 1, hooks that
// are not an object, or a sessionStart that is not an array; a syntax error carries its line.
func MergeCopilot(content []byte) (merged []byte, status Status, err error) {
	file, err := decodeObject(content)
	if err != nil {
		return nil, "", settingsFailure(CopilotPath, content, "", err)
	}
	versionPosition := slices.IndexFunc(file, func(member jsonMember) bool { return member.key == copilotVersionKey })
	if versionPosition >= 0 {
		var version float64
		if json.Unmarshal(file[versionPosition].value, &version) != nil || version != copilotVersion {
			return nil, "", &Error{Path: CopilotPath, Message: copilotVersionMessage}
		}
	}
	hooksPosition := slices.IndexFunc(file, func(member jsonMember) bool { return member.key == settingsHooksKey })
	var hooks []jsonMember
	if hooksPosition >= 0 {
		if hooks, err = decodeObject(file[hooksPosition].value); err != nil {
			return nil, "", settingsFailure(CopilotPath, nil, settingsHooksKey, err)
		}
	}
	eventPosition := slices.IndexFunc(hooks, func(member jsonMember) bool { return member.key == copilotSessionStartKey })
	var entries []json.RawMessage
	if eventPosition >= 0 {
		if json.Unmarshal(hooks[eventPosition].value, &entries) != nil {
			return nil, "", &Error{Path: CopilotPath, Message: copilotSessionStartMessage}
		}
	}
	if slices.ContainsFunc(entries, isCopilotPrimeHook) {
		return content, Present, nil
	}

	event := jsonArray(append(entries, encodeJSON(copilotHook{Type: hookTypeCommand, Command: CopilotPrimeCommand})))
	if eventPosition >= 0 {
		hooks[eventPosition].value = event
	} else {
		hooks = append(hooks, jsonMember{copilotSessionStartKey, event})
	}
	if hooksPosition >= 0 {
		file[hooksPosition].value = encodeObject(hooks)
	} else {
		file = append(file, jsonMember{settingsHooksKey, encodeObject(hooks)})
	}
	if versionPosition < 0 {
		file = slices.Insert(file, 0, jsonMember{copilotVersionKey, encodeJSON(copilotVersion)})
	}
	return indentJSON(encodeObject(file)), Added, nil
}

// isCopilotPrimeHook reports whether the sessionStart entry's command or bash is
// CopilotPrimeCommand; an entry of another shape is not capcom's.
func isCopilotPrimeHook(raw json.RawMessage) bool {
	var hook copilotHookCommands
	if json.Unmarshal(raw, &hook) != nil {
		return false
	}
	return hook.Command == CopilotPrimeCommand || hook.Bash == CopilotPrimeCommand
}
