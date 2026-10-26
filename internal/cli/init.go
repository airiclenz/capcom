package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/hooks"
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
	// ApogeeHooks is what --apogee-hooks did to the apogee project config: added or present.
	ApogeeHooks string `json:"apogee_hooks,omitempty"`
	// MergeDriver is what --merge-driver did to .gitattributes: added or present.
	MergeDriver string `json:"merge_driver,omitempty"`
	// GitConfig lists the git config commands --merge-driver leaves for the user to run.
	GitConfig []string `json:"git_config,omitempty"`
}

// Flags of init: add the capcom prime hooks to the Claude Code settings or to the Codex hooks, add
// the capcom prime reaction to the apogee project config, or hand issue files to the capcom-log
// merge driver in .gitattributes.
const (
	flagClaudeHooks = "claude-hooks"
	flagCodexHooks  = "codex-hooks"
	flagApogeeHooks = "apogee-hooks"
	flagMergeDriver = "merge-driver"
)

// Help text of capcom init.
const (
	initLong = "Create the capcom store in the current git working tree: .capcom/ with its issues and " +
		"knowledge directories and config.yaml.\n\n" +
		"An existing store is left as it is.\n\n" +
		"Without --claude-hooks it prints the Claude Code hooks that run capcom prime at session " +
		"start and before compaction, for you to add; --claude-hooks merges those SessionStart and " +
		"PreCompact hooks into .claude/settings.json, and --codex-hooks into .codex/hooks.json.\n\n" +
		"--apogee-hooks adds a reaction running capcom prime on apogee's session-started and " +
		"compaction-finished to .apogee/config.yaml, which apogee asks each user to adopt once; it " +
		"needs apogee v0.28.0 or later.\n\n" +
		"--merge-driver hands issue files to the capcom-log merge driver in .gitattributes and " +
		"prints the two git config commands that define it.\n\n" +
		"The flags are independent and can be combined."
	initExample = "  capcom init                  # create .capcom/ in this git repository\n" +
		"  capcom init --claude-hooks   # ...and run capcom prime from Claude Code's session hooks\n" +
		"  capcom init --codex-hooks    # ...and from Codex's, which needs [features] hooks = true\n" +
		"  capcom init --apogee-hooks   # ...and from apogee's session-started and compaction-finished reaction\n" +
		"  capcom init --merge-driver   # ...and merge issue Logs by entry (prints two git config lines)"
)

// codexFeatureNote is what init --codex-hooks prints in text mode: Codex ignores its hooks file
// unless the feature is on.
const codexFeatureNote = "Codex runs hooks only with [features] hooks = true in its config.toml\n"

// apogeeAdoptNote and apogeeVersionNote are what init --apogee-hooks prints in text mode after its
// added or present line: apogee runs a project reaction only once the user adopts it, and an apogee
// before v0.28.0, the release that admits advise on session-started and compaction-finished skips the entry.
const (
	apogeeAdoptNote   = "apogee asks to adopt reaction capcom-prime at the next session start\n"
	apogeeVersionNote = "needs apogee v0.28.0 or later; an older apogee ignores the capcom-prime entry\n"
)

// ClaudeHooks, CodexHooks, ApogeeHooks and MergeDriver values of initData.
const (
	claudeHooksAdded   = string(hooks.Added)
	claudeHooksPresent = string(hooks.Present)
)

// newInitCommand creates the store in the current git working tree, or does nothing when it
// already exists. With --claude-hooks it also merges the hooks running capcom prime into
// .claude/settings.json; without it, text mode prints those hooks for the user to add. With
// --codex-hooks it merges the same hooks into .codex/hooks.json. With --apogee-hooks it adds the
// reaction running capcom prime on session-started and compaction-finished to .apogee/config.yaml.
// With --merge-driver it adds the capcom-log merge driver line to .gitattributes and prints the git
// config commands that define the driver. The four flags are independent.
func (a *app) newInitCommand() *cobra.Command {
	isClaudeHooks := false
	isCodexHooks := false
	isApogeeHooks := false
	isMergeDriver := false
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Create the capcom store (.capcom/) in this git repository",
		Long:    initLong,
		Example: initExample,
		Args:    cobra.NoArgs,
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
			if isApogeeHooks {
				var line string
				if data.ApogeeHooks, line, err = installApogeeHooks(opened.WorktreeRoot); err != nil {
					return err
				}
				text += line + apogeeAdoptNote + apogeeVersionNote
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
	cmd.Flags().BoolVar(&isApogeeHooks, flagApogeeHooks, false,
		"also add a reaction running capcom prime at session start and after compaction to "+hooks.ApogeePath)
	cmd.Flags().BoolVar(&isMergeDriver, flagMergeDriver, false,
		"also hand issue files to the capcom-log merge driver in "+hooks.GitattributesPath)
	return cmd
}

// installHooks merges the capcom prime hooks into target's settings file and returns the initData
// value and the text line saying what it did. A settings file hooks refuses is invalid at its path
// and line.
func installHooks(worktreeRoot string, target hooks.Target) (status string, line string, err error) {
	installed, err := hooks.Install(worktreeRoot, target)
	switch {
	case err != nil:
		return "", "", hooksFailure(err)
	case installed == hooks.Added:
		return claudeHooksAdded, "added the " + primeCommand + " hooks to " + target.Path + "\n", nil
	default:
		return claudeHooksPresent, target.Path + " already runs " + primeCommand + "\n", nil
	}
}

// installApogeeHooks adds the capcom prime reaction to the apogee project config and returns the
// initData value and the text line saying what it did. A config hooks refuses is invalid at its
// path and, where one is to blame, its line.
func installApogeeHooks(worktreeRoot string) (status string, line string, err error) {
	installed, err := hooks.InstallApogee(worktreeRoot)
	switch {
	case err != nil:
		return "", "", hooksFailure(err)
	case installed == hooks.Added:
		return claudeHooksAdded, "added the " + primeCommand + " reaction to " + hooks.ApogeePath + "\n", nil
	default:
		return claudeHooksPresent, hooks.ApogeePath + " already runs " + primeCommand + "\n", nil
	}
}

// hooksFailure types a failure of an internal/hooks editor: a *hooks.Error is invalid at its path
// and line, anything else goes through output.AsError.
func hooksFailure(err error) error {
	var failure *hooks.Error
	if errors.As(err, &failure) {
		return &output.Error{
			Code:    output.CodeInvalid,
			Path:    failure.Path,
			Line:    failure.Line,
			Message: failure.Message,
		}
	}
	return output.AsError(err)
}

// mergeDriverConfig are the git config commands that define the capcom-log merge driver. capcom
// never writes .git/config, so init --merge-driver prints them for the user to run.
var mergeDriverConfig = []string{
	`git config merge.capcom-log.name "capcom Log merge"`,
	`git config merge.capcom-log.driver "capcom merge-driver %O %A %B %P %S %X %Y"`,
}

// installMergeDriver adds the capcom-log merge driver line to .gitattributes in the working tree at
// worktreeRoot and returns the initData value and the text line saying what it did. A .gitattributes
// that cannot be read or replaced is invalid at its path.
func installMergeDriver(worktreeRoot string) (status string, line string, err error) {
	installed, err := hooks.InstallMergeAttribute(worktreeRoot)
	switch {
	case err != nil:
		return "", "", hooksFailure(err)
	case installed == hooks.Added:
		return claudeHooksAdded, "added the capcom-log merge driver to " + hooks.GitattributesPath + "\n", nil
	default:
		return claudeHooksPresent, hooks.GitattributesPath + " already sets the capcom-log merge driver\n", nil
	}
}

// mergeDriverConfigText is what init --merge-driver prints in text mode: git runs the driver only
// once the repository's config defines it.
func mergeDriverConfigText() string {
	return "git uses the capcom-log merge driver only once it is configured; run:\n  " +
		strings.Join(mergeDriverConfig, "\n  ") + "\n"
}

// agentsHint is the line init prints after creating a store: init writes no agent guide, so it
// names the command that does.
const agentsHint = "run capcom agents to write the capcom block into " + agents.FileName + "\n"

// initText reports the store init found at root, or created with the paths in created, followed,
// for a store it created, by agentsHint.
func initText(root string, created []string) string {
	if len(created) == 0 {
		return "capcom store already initialized at " + root + "\n"
	}
	var text strings.Builder
	text.WriteString("initialized capcom store at " + root + "\n")
	for _, path := range created {
		text.WriteString("created " + path + "\n")
	}
	text.WriteString(agentsHint)
	return text.String()
}

// codexHooksPath is the Codex hooks file init --codex-hooks merges into, relative to the working
// tree root.
var codexHooksPath = hooks.Codex.Path

// primeCommand is the command the Claude Code and Codex hooks and the apogee reaction run.
const primeCommand = hooks.PrimeCommand

// claudeHooksSnippet is what plain init prints: the settings that make Claude Code run capcom
// prime, for the user to add by hand or through init --claude-hooks.
func claudeHooksSnippet() string {
	return "To brief Claude Code at every session start and before compaction, add to " +
		hooks.Claude.Path + " (or run capcom init --" + flagClaudeHooks + "):\n" + string(hooks.Snippet(hooks.Claude))
}
