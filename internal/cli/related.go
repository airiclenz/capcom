package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// formatRaisedWarning is the warning of the write that raises the store format (capcom related add
// and know update): the config file, the new format, and the format older releases read.
const formatRaisedWarning = "%s: store format raised to %d; capcom releases that read only format %d can no longer read this store"

// The help text of capcom related, related add and related rm.
const (
	relatedLong = "Manage Related links: an undirected tie between two Issues that blocks neither of them. A " +
		"link is stored in one Issue's file, under the canonical id of the other, and show lists it on " +
		"both. Use a Dependency instead when one Issue has to wait for the other."
	relatedExample = "  capcom related add login-timeout-race session-refresh-bug   # shown on both; blocks neither\n" +
		"  capcom related rm login-timeout-race session-refresh-bug"
	relatedAddLong = "Link the first Issue to the second by writing the second's id into the first's related " +
		"list. Either argument may be an id, an Alias or a unique id prefix. Linking an Issue to itself " +
		"is refused, and a link already there, on either Issue, is left as it is. --if-match writes " +
		"only when the first Issue's etag still matches. It prints the id, path and etag of the first " +
		"Issue. The first link in a store below format 2 raises the store format to 2, with a warning " +
		"that older capcom releases can no longer read the store."
	relatedAddExample = "  capcom related add login-timeout-race session-refresh-bug   # shown on both; blocks neither\n" +
		"  capcom related add login-timeout-race session-refresh-bug --if-match <etag>   # etag from show"
	relatedRmLong = "Remove the Related link between two Issues from whichever of them holds it, so the " +
		"arguments may come in either order. An argument that names no Issue removes the first " +
		"Issue's entry with that text, so a link that names no Issue any more can still be removed. --if-match " +
		"writes only when the etag of the Issue holding the link still matches. It prints the id, path " +
		"and etag of that Issue, and fails not_found, exit code 3, when the two are not linked."
	relatedRmExample = "  capcom related rm login-timeout-race session-refresh-bug\n" +
		"  capcom related rm session-refresh-bug login-timeout-race   # either order removes the same link"
)

// newRelatedCommand groups the commands that add and remove Related links. A link is stored on
// one issue only, under the canonical id of the other; show derives the reverse.
func (a *app) newRelatedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "related",
		Short:   "Manage related links between issues",
		Long:    relatedLong,
		Example: relatedExample,
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(a.newRelatedAddCommand())
	cmd.AddCommand(a.newRelatedRmCommand())
	return cmd
}

// newRelatedAddCommand links one issue to another by writing the other's id into the first's
// related list. Linking an issue to itself is refused; a link already there, on either issue, is
// left as it is and the first issue's file is written unchanged. The first link written in a store
// below format 2 raises config.yaml to format 2, with a warning.
func (a *app) newRelatedAddCommand() *cobra.Command {
	var ifMatch string
	cmd := &cobra.Command{
		Use:         "add <id|alias|prefix> <related-id|alias|prefix>",
		Short:       "Link an issue to a related one",
		Long:        relatedAddLong,
		Example:     relatedAddExample,
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			holder, other, err := a.resolveEdge(args)
			if err != nil {
				return err
			}
			if holder.ID == other.ID {
				return &output.Error{
					Code:    output.CodeInvalid,
					Path:    holder.Path,
					Message: fmt.Sprintf("%s cannot be related to itself", holder.ID),
				}
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			isLinked := slices.ContainsFunc(holder.Related, graph.names(other.ID)) ||
				slices.ContainsFunc(other.Related, graph.names(holder.ID))
			written, err := a.editIssue(holder.ID, ifMatch, func(target issue.Issue) (issue.Issue, error) {
				if isLinked {
					return target, nil
				}
				target.Related = append(slices.Clone(target.Related), other.ID)
				return target, nil
			})
			if err != nil {
				return err
			}
			result := writeResult(written.ID, written.Path, written.ETag)
			a.warnRaisedFormat(&result, written.Effects)
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().StringVar(&ifMatch, flagIfMatch, "", "only write when the first issue's etag is this one (see show)")
	return a.withKV(cmd)
}

// newRelatedRmCommand removes the link between two issues from the issue that holds it: every
// related entry of the first naming the second, or failing that, every entry of the second naming
// the first. An argument that is not exactly an issue's id or Alias removes the entry of the first
// that is its text, before it is matched as a prefix; one naming no issue at all is matched only
// by its text.
func (a *app) newRelatedRmCommand() *cobra.Command {
	var ifMatch string
	cmd := &cobra.Command{
		Use:         "rm <id|alias|prefix> <related-id|alias|prefix>",
		Short:       "Remove the related link between two issues",
		Long:        relatedRmLong,
		Example:     relatedRmExample,
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			first, err := a.resolveIssue(args[0])
			if err != nil {
				return err
			}
			holder, isRemoved, err := a.relatedRemoval(first, args[1])
			if err != nil {
				return err
			}
			written, err := a.editIssue(holder.ID, ifMatch, func(target issue.Issue) (issue.Issue, error) {
				kept := slices.DeleteFunc(slices.Clone(target.Related), isRemoved)
				if len(kept) == len(target.Related) {
					return issue.Issue{}, output.Errorf(output.CodeNotFound, "%s is not related to %s", first.ID, args[1])
				}
				if len(kept) == 0 {
					kept = nil
				}
				target.Related = kept
				return target, nil
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	}
	cmd.Flags().StringVar(&ifMatch, flagIfMatch, "", "only write when the etag of the issue holding the link is this one (see show)")
	return a.withKV(cmd)
}

// relatedRemoval finds the issue holding the link between first and the issue arg names, and the
// test its related entries must pass to be removed. first holds it when one of its entries names
// that issue, else that issue holds it when one of its entries names first; neither is not_found.
// When isRemovedByText says so, first holds it and the entry is matched by its text (isEntryText).
func (a *app) relatedRemoval(first index.Entry, arg string) (index.Entry, func(string) bool, error) {
	graph, err := a.loadDepGraph()
	if err != nil {
		return index.Entry{}, nil, err
	}
	other, err := a.issueIndex.Resolve(arg)
	if graph.isRemovedByText(first.Related, arg, err) {
		return first, isEntryText(arg), nil
	}
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return index.Entry{}, nil, failure
	}
	if namesOther := graph.names(other.ID); slices.ContainsFunc(first.Related, namesOther) {
		return first, namesOther, nil
	}
	if namesFirst := graph.names(first.ID); slices.ContainsFunc(other.Related, namesFirst) {
		return other, namesFirst, nil
	}
	return index.Entry{}, nil, &output.Error{
		Code:    output.CodeNotFound,
		Path:    first.Path,
		Message: fmt.Sprintf("%s and %s are not related", first.ID, other.ID),
	}
}

// warnRaisedFormat adds to result the warning of a write whose effects raised the store format
// (formatRaisedWarning), and nothing when the write left config.yaml alone.
func (a *app) warnRaisedFormat(result *output.Result, effects nodestore.Effects) {
	if effects.RaisedFormat == 0 {
		return
	}
	displayPath := a.worktreeRelative(a.store.ConfigPath())
	result.Warnings = append(
		result.Warnings,
		fmt.Sprintf(formatRaisedWarning, displayPath, effects.RaisedFormat, effects.RaisedFormat-1),
	)
}
