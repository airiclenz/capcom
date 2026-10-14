package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// nodeVerbs describes one Tree kind to the verbs built for both kinds by one generic constructor
// each, capcom mv and know mv, capcom rm and know rm, and to the write path createNode runs for
// capcom create and know add. It holds the kind's node store and everything the commands of the
// two kinds differ in: their wording, the kind's Descriptor, the write
// and remove results, how a failed removal is typed, what is rendered after a write, and whether
// --kv and --drop-deps are registered.
type nodeVerbs[T any] struct {
	// kind returns the node store of the kind's Tree.
	kind func(a *app) *nodestore.Kind[T]
	// commandPath is the mv command as typed after capcom ("mv", "know mv"), naming it in usage
	// errors.
	commandPath string
	// nounPhrase is the kind's noun with its article ("an issue", "a Note"), used in Short.
	nounPhrase string
	// flagNoun is the kind's noun in flag help ("issue", "note").
	flagNoun string
	// showCommand prints a node's etag ("show", "know show"); the --if-match help names it.
	showCommand string
	// descriptor describes the kind without a store, so the command tree builds before a store is
	// open; its ref keys name the reference fields in the warning for a file whose references
	// could not be checked. The issue Descriptor is built from a zero config.Config, so its
	// Validate is not the store's: never validate through it.
	descriptor nodestore.Descriptor[T]
	// writeResult is the result of a command that wrote a node: its id, path and new etag.
	writeResult func(id string, path string, etag string) output.Result
	// removeResult is the result of a command that removed a node.
	removeResult func(removed nodestore.Removed) output.Result
	// removeFailure types a failure of the removal of a node.
	removeFailure func(err error) error
	// afterWrite renders the Effects a write returned onto its result; nil when the kind has
	// nothing to render.
	afterWrite func(result *output.Result, effects nodestore.Effects)
	// hasKV reports whether the kind's verbs take --kv: only where results carry KV records.
	hasKV bool
	// hasDropDeps reports whether the kind's rm takes --drop-deps, which removes the deps entries
	// of the nodes that depend on the removed one instead of refusing the removal.
	hasDropDeps bool
	// mvHelp and rmHelp are the Long and Example help text of the kind's mv and rm.
	mvHelp commandHelp
	rmHelp commandHelp
}

// commandHelp is the Long and Example help text of a command built from a nodeVerbs.
type commandHelp struct {
	long    string
	example string
}

// issueVerbs describes the issue Tree: capcom create, mv and rm.
var issueVerbs = nodeVerbs[issue.Issue]{
	kind: func(a *app) *nodestore.Kind[issue.Issue] {
		return nodestore.Issues(a.store, nil)
	},
	commandPath:   "mv",
	nounPhrase:    "an issue",
	flagNoun:      "issue",
	showCommand:   "show",
	descriptor:    nodestore.IssueDescriptor(config.Config{}),
	writeResult:   writeResult,
	removeResult:  issueRemoveResult,
	removeFailure: issueRemovalFailure,
	hasKV:         true,
	hasDropDeps:   true,
	mvHelp:        issueMvHelp,
	rmHelp:        issueRmHelp,
}

// noteVerbs describes the Knowledge tree: capcom know add, know mv and know rm. Its node store refreshes the
// capcom block of AGENTS.md after each write (see notes), and afterWrite renders that refresh.
var noteVerbs = nodeVerbs[knowledge.Note]{
	kind: func(a *app) *nodestore.Kind[knowledge.Note] {
		return a.notes(nil)
	},
	commandPath:  "know mv",
	nounPhrase:   "a Note",
	flagNoun:     "note",
	showCommand:  "know show",
	descriptor:   nodestore.NoteDescriptor(),
	writeResult:  noteWriteResult,
	removeResult: noteRemoveResult,
	removeFailure: func(err error) error {
		return kindFailure(err)
	},
	afterWrite:  addSyncEffects,
	hasKV:       false,
	hasDropDeps: false,
	mvHelp:      noteMvHelp,
	rmHelp:      noteRmHelp,
}

// newMvCommand builds capcom mv or know mv, as verbs says: it renames a node, moves it below
// another, or both. A rename keeps the old id as an Alias and rewrites every reference
// (verbs.descriptor's ref keys) that named the node; the node's subtree moves with it. capcom moves files with a
// plain file-system rename and never runs git.
func newMvCommand[T any](a *app, verbs nodeVerbs[T]) *cobra.Command {
	options := &mvOptions{}
	cmd := &cobra.Command{
		Use:         "mv <id|alias|prefix> [<new-id>] [--parent <id|alias|prefix|.>] [--if-match <etag>]",
		Short:       "Rename " + verbs.nounPhrase + " or move it below another",
		Long:        verbs.mvHelp.long,
		Example:     verbs.mvHelp.example,
		Args:        cobra.RangeArgs(1, 2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			options.isReparented = cmd.Flags().Changed(flagParent)
			if len(args) == 1 && !options.isReparented {
				return output.UsageErrorf("%s needs a new id, --parent, or both", verbs.commandPath)
			}
			result, err := move(a, verbs, args, options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(
		&options.parent,
		flagParent,
		"",
		"move the "+verbs.flagNoun+" below this one; . moves it to the top of the tree",
	)
	flags.StringVar(
		&options.ifMatch,
		flagIfMatch,
		"",
		"only move when the file's etag is this one (see "+verbs.showCommand+")",
	)
	if verbs.hasKV {
		return a.withKV(cmd)
	}
	return cmd
}

// move moves the node args[0] names to the new id args[1], when given, and below options.parent,
// when reparented. It refuses with conflict, before anything moves, when --if-match is set and is
// not the node file's etag, and, taking the move back, when a node file it rewrites changed while
// it ran. The result names the node's new id, path and etag, with a warning for every tree problem
// that concerns neither the node, its new path nor the new parent, for every node file whose
// references could not be checked, and for an emptied old parent directory it could not remove;
// verbs.afterWrite then renders the write's Effects.
func move[T any](
	a *app,
	verbs nodeVerbs[T],
	args []string,
	options *mvOptions,
) (output.Result, error) {
	destination, err := moveDestination(args, options.parent, options.isReparented)
	if err != nil {
		return output.Result{}, err
	}
	moved, problems, err := verbs.kind(a).Move(args[0], destination, options.ifMatch)
	if err != nil {
		return output.Result{}, kindFailure(err)
	}
	result := verbs.writeResult(moved.ID, moved.Path, moved.ETag)
	indexWarnings, warnings := a.movedWarnings(moved, problems)
	a.indexWarnings = indexWarnings
	result.Warnings = append(result.Warnings, warnings...)
	unread := unreadWarnings(moved.Unread, verbs.descriptor.RefKeys(), moved.OldID)
	result.Warnings = append(result.Warnings, unread...)
	result.Warnings = append(result.Warnings, cleanupWarnings(moved.LeftInPlace)...)
	if verbs.afterWrite != nil {
		verbs.afterWrite(&result, moved.Effects)
	}
	return result, nil
}

// newRmCommand builds capcom rm or know rm, as verbs says: it deletes one node file. A node with
// children, or whose children directory holds anything, is refused, so removal never leaves an
// orphan directory; with verbs.hasDropDeps, so is a node other nodes depend on, and --drop-deps
// removes those deps entries instead. Every other reference to the node is dropped, and its
// Aliases go with the file. capcom deletes the file itself and never runs git.
func newRmCommand[T any](a *app, verbs nodeVerbs[T]) *cobra.Command {
	options := &rmOptions{}
	use := "rm <id|alias|prefix> [--if-match <etag>]"
	if verbs.hasDropDeps {
		use += " [--" + flagDropDeps + "]"
	}
	cmd := &cobra.Command{
		Use:         use,
		Short:       "Remove " + verbs.nounPhrase + " that has no children",
		Long:        verbs.rmHelp.long,
		Example:     verbs.rmHelp.example,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			result, err := remove(a, verbs, args[0], options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(
		&options.ifMatch,
		flagIfMatch,
		"",
		"only remove when the file's etag is this one (see "+verbs.showCommand+")",
	)
	if verbs.hasDropDeps {
		flags.BoolVar(
			&options.shouldDropDeps,
			flagDropDeps,
			false,
			"remove the deps entries of the issues that depend on it instead of refusing",
		)
	}
	if verbs.hasKV {
		return a.withKV(cmd)
	}
	return cmd
}

// remove removes the node arg names through the kind's node store, which deletes its file together
// with the node's own children directory when that is empty, then the parent's children directory
// when the removal emptied it, and drops every other node's references naming it. It refuses
// before anything changes, see nodestore.Kind.Remove, and, taking the removal back, when a file it
// rewrites changed while it ran. The result names the removed file and the nodes whose references
// were dropped, with a warning for every tree problem that concerns neither the node nor its
// parent, for every node file whose references could not be checked and for an emptied parent
// directory left in place; verbs.afterWrite then renders the write's Effects.
func remove[T any](
	a *app,
	verbs nodeVerbs[T],
	arg string,
	options *rmOptions,
) (output.Result, error) {
	removed, problems, err := verbs.kind(a).Remove(arg, options.ifMatch, options.shouldDropDeps)
	if err != nil {
		return output.Result{}, verbs.removeFailure(err)
	}
	result := verbs.removeResult(removed)
	result.Warnings = append(result.Warnings, a.unwarnedTreeProblems(problems)...)
	unread := unreadWarnings(removed.Unread, verbs.descriptor.RefKeys(), removed.ID)
	result.Warnings = append(result.Warnings, unread...)
	result.Warnings = append(result.Warnings, cleanupWarnings(removed.LeftInPlace)...)
	if verbs.afterWrite != nil {
		verbs.afterWrite(&result, removed.Effects)
	}
	return result, nil
}

// unreadWarnings renders one warning per node file in paths, which does not parse, so its entries
// naming the node id under every one of refKeys were not checked.
func unreadWarnings(paths []string, refKeys []string, id string) []string {
	keys := joinKeys(refKeys)
	warnings := make([]string, 0, len(paths))
	for _, path := range paths {
		warnings = append(warnings, fmt.Sprintf(
			"%s: %s not checked for %s: the file does not parse",
			path,
			keys,
			id,
		))
	}
	return warnings
}

// joinKeys joins keys as a message lists them: "a", "a and b", "a, b and c".
func joinKeys(keys []string) string {
	if len(keys) < 2 {
		return strings.Join(keys, "")
	}
	return strings.Join(keys[:len(keys)-1], ", ") + " and " + keys[len(keys)-1]
}

// createNode writes the new node build returns under id, below parent when it is set, through the
// kind's node store, which refuses before build runs when id is invalid or already named, see
// nodestore.Kind.Create. The result names the new node's id, path and etag, with a warning for
// every tree problem that concerns neither the node's path nor its parent, unless the index
// refresh already warned about it; verbs.afterWrite then renders the write's Effects.
func createNode[T any](
	a *app,
	verbs nodeVerbs[T],
	id string,
	parent string,
	build func() (T, error),
) (output.Result, error) {
	written, problems, err := verbs.kind(a).Create(id, parent, build)
	if err != nil {
		return output.Result{}, kindFailure(err)
	}
	result := verbs.writeResult(written.ID, written.Path, written.ETag)
	result.Warnings = append(result.Warnings, a.unwarnedTreeProblems(problems)...)
	if verbs.afterWrite != nil {
		verbs.afterWrite(&result, written.Effects)
	}
	return result, nil
}
