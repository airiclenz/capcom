package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// Store format of capcom related add.
const (
	// relatedFormatVersion is the first store format whose issue files may carry related links.
	relatedFormatVersion = 2
	// formatRaisedWarning is the warning of the write that raises the store format: the config
	// file, the new format, and the format older releases read.
	formatRaisedWarning = "%s: store format raised to %d; capcom releases that read only format %d can no longer read this store"
)

// newRelatedCommand groups the commands that add and remove Related links. A link is stored on
// one issue only, under the canonical id of the other; show derives the reverse.
func (a *app) newRelatedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "related",
		Short: "Manage related links between issues",
		Args:  cobra.NoArgs,
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
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			holder, other, err := a.resolveEdge(args)
			if err != nil {
				return err
			}
			if holder.ID == other.ID {
				return &Error{
					Code:    CodeInvalid,
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
			warning := ""
			written, err := a.editIssue(holder.ID, ifMatch, func(target issue.Issue) (issue.Issue, error) {
				if isLinked {
					return target, nil
				}
				// Raised only once the etag check has passed, and before the link is written, so a
				// store never holds a related link while it still declares format 1.
				raiseWarning, raiseErr := a.raiseStoreFormat(relatedFormatVersion)
				if raiseErr != nil {
					return issue.Issue{}, raiseErr
				}
				warning = raiseWarning
				target.Related = append(slices.Clone(target.Related), other.ID)
				return target, nil
			})
			if err != nil {
				return err
			}
			result := writeResult(written.ID, written.Path, written.ETag)
			if warning != "" {
				result.Warnings = append(result.Warnings, warning)
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().StringVar(&ifMatch, flagIfMatch, "", "only write when the first issue's etag is this one (see show)")
	return a.withKV(cmd)
}

// newRelatedRmCommand removes the link between two issues from the issue that holds it: every
// related entry of the first naming the second, or failing that, every entry of the second naming
// the first. An entry naming no issue at all is removed from the first by its exact text.
func (a *app) newRelatedRmCommand() *cobra.Command {
	var ifMatch string
	cmd := &cobra.Command{
		Use:         "rm <id|alias|prefix> <related-id|alias|prefix>",
		Short:       "Remove the related link between two issues",
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
					return issue.Issue{}, Errorf(CodeNotFound, "%s is not related to %s", first.ID, args[1])
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
// When arg names no issue, first holds it and the entry is matched by its exact text.
func (a *app) relatedRemoval(first index.Entry, arg string) (index.Entry, func(string) bool, error) {
	other, err := a.issueIndex.Resolve(arg)
	if errors.Is(err, tree.ErrNotFound) {
		return first, func(related string) bool { return related == arg }, nil
	}
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return index.Entry{}, nil, failure
	}
	graph, err := a.loadDepGraph()
	if err != nil {
		return index.Entry{}, nil, err
	}
	if namesOther := graph.names(other.ID); slices.ContainsFunc(first.Related, namesOther) {
		return first, namesOther, nil
	}
	if namesFirst := graph.names(first.ID); slices.ContainsFunc(other.Related, namesFirst) {
		return other, namesFirst, nil
	}
	return index.Entry{}, nil, &Error{
		Code:    CodeNotFound,
		Path:    first.Path,
		Message: fmt.Sprintf("%s and %s are not related", first.ID, other.ID),
	}
}

// raiseStoreFormat raises the store format to at least version (store.RaiseFormat) and returns the
// warning to give when it changed config.yaml, or "" when the store already declared version or
// above. Every failure is invalid at config.yaml.
func (a *app) raiseStoreFormat(version int) (string, error) {
	displayPath := a.worktreeRelative(a.store.ConfigPath())
	isRaised, err := a.store.RaiseFormat(version)
	if err != nil {
		return "", &Error{Code: CodeInvalid, Path: displayPath, Message: err.Error()}
	}
	if !isRaised {
		return "", nil
	}
	return fmt.Sprintf(formatRaisedWarning, displayPath, version, version-1), nil
}
