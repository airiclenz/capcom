package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/hooks"
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
	claudeHooksAdded   = string(hooks.Added)
	claudeHooksPresent = string(hooks.Present)
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
				if data.ClaudeHooks, line, err = installHooks(opened.WorktreeRoot, hooks.Claude); err != nil {
					return err
				}
				text += line
				snippet = ""
			}
			if isCodexHooks {
				var line string
				if data.CodexHooks, line, err = installHooks(opened.WorktreeRoot, hooks.Codex); err != nil {
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
		"also add SessionStart and PreCompact hooks running capcom prime to "+hooks.Claude.Path)
	cmd.Flags().BoolVar(&isCodexHooks, flagCodexHooks, false,
		"also add SessionStart and PreCompact hooks running capcom prime to "+codexHooksPath)
	cmd.Flags().BoolVar(&isMergeDriver, flagMergeDriver, false,
		"also hand issue files to the capcom-log merge driver in "+gitattributesPath)
	return cmd
}

// installHooks merges the capcom prime hooks into target's settings file and returns the initData
// value and the text line saying what it did. A settings file hooks refuses is invalid at its path
// and line.
func installHooks(worktreeRoot string, target hooks.Target) (status string, line string, err error) {
	installed, err := hooks.Install(worktreeRoot, target)
	var settingsFailure *hooks.Error
	switch {
	case errors.As(err, &settingsFailure):
		return "", "", &output.Error{
			Code:    output.CodeInvalid,
			Path:    settingsFailure.Path,
			Line:    settingsFailure.Line,
			Message: settingsFailure.Message,
		}
	case err != nil:
		return "", "", output.AsError(err)
	case installed == hooks.Added:
		return claudeHooksAdded, "added the " + primeCommand + " hooks to " + target.Path + "\n", nil
	default:
		return claudeHooksPresent, target.Path + " already runs " + primeCommand + "\n", nil
	}
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

// codexHooksPath is the Codex hooks file init --codex-hooks merges into, relative to the working
// tree root.
var codexHooksPath = hooks.Codex.Path

// primeCommand is the command the Claude Code and Codex hooks run.
const primeCommand = hooks.PrimeCommand

// claudeHooksSnippet is what plain init prints: the settings that make Claude Code run capcom
// prime, for the user to add by hand or through init --claude-hooks.
func claudeHooksSnippet() string {
	return "To brief Claude Code at every session start and before compaction, add to " +
		hooks.Claude.Path + " (or run capcom init --" + flagClaudeHooks + "):\n" + string(hooks.Snippet(hooks.Claude))
}
