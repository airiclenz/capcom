package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// cycleArrow joins the issues of a refused cycle in dep add.
const cycleArrow = " -> "

// newDepCommand groups the commands that add, remove, list and graph Dependency edges. dep add and dep
// rm change an existing issue's deps and create --dep writes a new issue's; every edge is stored
// on the dependent, under the canonical id of the issue it depends on.
func (a *app) newDepCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage dependencies between issues",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.newDepAddCommand())
	cmd.AddCommand(a.newDepGraphCommand())
	cmd.AddCommand(a.newDepListCommand())
	cmd.AddCommand(a.newDepRmCommand())
	return cmd
}

// newDepAddCommand makes one issue depend on another. A self-dependency and an edge that would
// close a cycle are refused; an edge that is already there is left as it is.
func (a *app) newDepAddCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:         "add <id|alias|prefix> <dep-id|alias|prefix>",
		Short:       "Make an issue depend on another",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			dependent, dependency, err := a.resolveEdge(args)
			if err != nil {
				return err
			}
			if dependent.ID == dependency.ID {
				return &output.Error{
					Code:    output.CodeInvalid,
					Path:    dependent.Path,
					Message: fmt.Sprintf("%s cannot depend on itself", dependent.ID),
				}
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			isPresent := slices.ContainsFunc(dependent.Deps, graph.names(dependency.ID))
			if cycle := graph.graph.FindCycle(dependent.ID, dependency.ID); !isPresent && cycle != nil {
				return &output.Error{
					Code: output.CodeInvalid,
					Path: dependent.Path,
					Message: fmt.Sprintf(
						"%s cannot depend on %s: that would create the cycle %s",
						dependent.ID,
						dependency.ID,
						strings.Join(cycle, cycleArrow),
					),
				}
			}
			written, err := a.editIssue(dependent.ID, "", func(target issue.Issue) (issue.Issue, error) {
				if !isPresent {
					target.Deps = append(slices.Clone(target.Deps), dependency.ID)
				}
				return target, nil
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}

// newDepRmCommand removes every dep of an issue that names the given one, whether the file writes
// it as the id or as an Alias. An argument that is not exactly an issue's id or Alias removes the
// dep that is its text, before it is matched as a prefix; one naming no issue at all is matched
// only by its text.
func (a *app) newDepRmCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:         "rm <id|alias|prefix> <dep-id|alias|prefix>",
		Short:       "Remove a dependency of an issue",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			dependent, err := a.resolveIssue(args[0])
			if err != nil {
				return err
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			isRemoved, err := a.depRemoval(graph, dependent.Deps, args[1])
			if err != nil {
				return err
			}
			written, err := a.editIssue(dependent.ID, "", func(target issue.Issue) (issue.Issue, error) {
				kept := slices.DeleteFunc(slices.Clone(target.Deps), isRemoved)
				if len(kept) == len(target.Deps) {
					return issue.Issue{}, output.Errorf(output.CodeNotFound, "%s does not depend on %s", dependent.ID, args[1])
				}
				if len(kept) == 0 {
					kept = nil
				}
				target.Deps = kept
				return target, nil
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}

// depRemoval returns the test a dep of the dependent must pass to be removed by dep rm: it names
// the issue arg resolves to, or it is arg's text (isEntryText) when isRemovedByText says so.
func (a *app) depRemoval(graph depGraph, deps []string, arg string) (func(string) bool, error) {
	dependency, err := a.issueIndex.Resolve(arg)
	if graph.isRemovedByText(deps, arg, err) {
		return isEntryText(arg), nil
	}
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return nil, failure
	}
	return graph.names(dependency.ID), nil
}

// resolveEdge resolves the two arguments of dep add: the dependent issue, then the one it is to
// depend on.
func (a *app) resolveEdge(args []string) (dependent index.Entry, dependency index.Entry, err error) {
	if dependent, err = a.resolveIssue(args[0]); err != nil {
		return index.Entry{}, index.Entry{}, err
	}
	if dependency, err = a.resolveIssue(args[1]); err != nil {
		return index.Entry{}, index.Entry{}, err
	}
	return dependent, dependency, nil
}

// isEntryText returns a test of whether an entry of a deps or related list is arg's text: arg as
// typed or as tree.NormalizeArg spells it.
func isEntryText(arg string) func(string) bool {
	name := tree.NormalizeArg(arg)
	return func(entry string) bool { return entry == arg || entry == name }
}

// isRemovedByText reports whether dep rm or related rm removes the entries of a holder's list that
// are arg's text (isEntryText), given resolveErr, what issueIndex.Resolve made of arg. It does when
// arg names no issue. When arg is a prefix, of one issue's id or of several, it does when one of
// entries is arg's text, so a dangling entry is never mistaken for the issue it prefixes. An arg
// that is exactly a usable issue's id or Alias, and a name a duplicate leaves out, never are.
func (g depGraph) isRemovedByText(entries []string, arg string, resolveErr error) bool {
	switch {
	case errors.Is(resolveErr, tree.ErrNotFound):
		return true
	case errors.Is(resolveErr, tree.ErrAmbiguous):
		return slices.ContainsFunc(entries, isEntryText(arg))
	case resolveErr != nil:
		return false
	}
	if _, isExact := g.graph.Resolve(tree.NormalizeArg(arg)); isExact {
		return false
	}
	return slices.ContainsFunc(entries, isEntryText(arg))
}
