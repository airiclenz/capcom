package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// nodeVerbs describes one Tree kind to the verbs built for both kinds by one generic constructor:
// capcom mv and know mv. It holds the kind's node store and everything the two commands differ
// in: their wording, the reference field other nodes name the node in, the write result, what is
// rendered after a write, and whether --kv is registered.
type nodeVerbs[T any] struct {
	// kind returns the node store of the kind's Tree.
	kind func(a *app) *nodestore.Kind[T]
	// commandPath is the command as typed after capcom ("mv", "know mv"), naming it in usage errors.
	commandPath string
	// nounPhrase is the kind's noun with its article ("an issue", "a Note"), used in Short.
	nounPhrase string
	// flagNoun is the kind's noun in flag help ("issue", "note").
	flagNoun string
	// showCommand prints a node's etag ("show", "know show"); the --if-match help names it.
	showCommand string
	// linkKey is the reference field through which other nodes name a node ("deps",
	// "superseded-by"), named in the warning for a file whose references could not be checked.
	linkKey string
	// writeResult is the result of a command that wrote a node: its id, path and new etag.
	writeResult func(id string, path string, etag string) output.Result
	// afterWrite renders the Effects a write returned onto its result; nil when the kind has
	// nothing to render.
	afterWrite func(result *output.Result, effects nodestore.Effects)
	// hasKV reports whether the kind's verbs take --kv: only where results carry KV records.
	hasKV bool
}

// issueVerbs describes the issue Tree: capcom mv.
var issueVerbs = nodeVerbs[issue.Issue]{
	kind: func(a *app) *nodestore.Kind[issue.Issue] {
		return nodestore.Issues(a.store, nil)
	},
	commandPath: "mv",
	nounPhrase:  "an issue",
	flagNoun:    "issue",
	showCommand: "show",
	linkKey:     "deps",
	writeResult: writeResult,
	hasKV:       true,
}

// noteVerbs describes the Knowledge tree: capcom know mv. Its node store refreshes the capcom
// block of AGENTS.md after each write (see notes), and afterWrite renders that refresh.
var noteVerbs = nodeVerbs[knowledge.Note]{
	kind: func(a *app) *nodestore.Kind[knowledge.Note] {
		return a.notes(nil)
	},
	commandPath: "know mv",
	nounPhrase:  "a Note",
	flagNoun:    "note",
	showCommand: "know show",
	linkKey:     "superseded-by",
	writeResult: noteWriteResult,
	afterWrite:  addSyncEffects,
	hasKV:       false,
}

// newMvCommand builds capcom mv or know mv, as verbs says: it renames a node, moves it below
// another, or both. A rename keeps the old id as an Alias and rewrites every reference
// (verbs.linkKey) that named the node; the node's subtree moves with it. capcom moves files with a
// plain file-system rename and never runs git.
func newMvCommand[T any](a *app, verbs nodeVerbs[T]) *cobra.Command {
	options := &mvOptions{}
	cmd := &cobra.Command{
		Use:         "mv <id|alias|prefix> [<new-id>] [--parent <id|alias|prefix|.>] [--if-match <etag>]",
		Short:       "Rename " + verbs.nounPhrase + " or move it below another",
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
	unread := unreadWarnings(moved.Unread, verbs.linkKey, moved.OldID)
	result.Warnings = append(result.Warnings, unread...)
	result.Warnings = append(result.Warnings, cleanupWarnings(moved.LeftInPlace)...)
	if verbs.afterWrite != nil {
		verbs.afterWrite(&result, moved.Effects)
	}
	return result, nil
}

// unreadWarnings renders one warning per node file in paths, which does not parse, so its linkKey
// entries naming the node id were not checked.
func unreadWarnings(paths []string, linkKey string, id string) []string {
	warnings := make([]string, 0, len(paths))
	for _, path := range paths {
		warnings = append(warnings, fmt.Sprintf(
			"%s: %s not checked for %s: the file does not parse",
			path,
			linkKey,
			id,
		))
	}
	return warnings
}
