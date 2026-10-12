package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// Text of capcom dep list.
const (
	depsHeading       = "deps:"
	dependentsHeading = "dependents:"
	depMissingLabel   = "missing"
	cycleArrow        = " -> "
)

// depListData is the --json data of dep list: the issue, the issues its deps name, the deps that
// name none (as the file writes them), and the issues that depend on it.
type depListData struct {
	ID         string      `json:"id"`
	Deps       []issueData `json:"deps"`
	Missing    []string    `json:"missing"`
	Dependents []issueData `json:"dependents"`
}

// depGraph is the Dependency graph of every usable issue, with each issue's index entry by id.
type depGraph struct {
	graph   issue.DepGraph
	entries map[string]index.Entry
}

// newDepCommand groups the commands that add, remove and list Dependency edges. dep add and dep
// rm are the only writers of an issue's deps; every edge is stored on the dependent, under the
// canonical id of the issue it depends on.
func (a *app) newDepCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage dependencies between issues",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.newDepAddCommand())
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
				return &Error{
					Code:    CodeInvalid,
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
				return &Error{
					Code: CodeInvalid,
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
// it as the id or as an Alias. A dep naming no issue at all is removed by its exact text.
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
			isRemoved, err := a.depRemoval(graph, args[1])
			if err != nil {
				return err
			}
			written, err := a.editIssue(dependent.ID, "", func(target issue.Issue) (issue.Issue, error) {
				kept := slices.DeleteFunc(slices.Clone(target.Deps), isRemoved)
				if len(kept) == len(target.Deps) {
					return issue.Issue{}, Errorf(CodeNotFound, "%s does not depend on %s", dependent.ID, args[1])
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
// the issue arg resolves to, or, when arg names no issue, it is arg exactly.
func (a *app) depRemoval(graph depGraph, arg string) (func(string) bool, error) {
	dependency, err := a.issueIndex.Resolve(arg)
	if errors.Is(err, tree.ErrNotFound) {
		return func(dep string) bool { return dep == arg }, nil
	}
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return nil, failure
	}
	return graph.names(dependency.ID), nil
}

// newDepListCommand lists an issue's dependencies and the issues that depend on it.
func (a *app) newDepListCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "list <id|alias|prefix>",
		Short:       "List an issue's dependencies and dependents",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			entry, err := a.resolveIssue(args[0])
			if err != nil {
				return err
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			return a.emit(cmd, graph.listResult(entry))
		}),
	}
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

// loadDepGraph builds the Dependency graph of every usable issue in the index, Closed ones
// included.
func (a *app) loadDepGraph() (depGraph, error) {
	entries, err := a.issueIndex.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return depGraph{}, err
	}
	nodes := make([]issue.Node, 0, len(entries))
	byID := make(map[string]index.Entry, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, issue.Node{
			ID:       entry.ID,
			ParentID: entry.ParentID,
			Category: entry.Category,
			Deps:     entry.Deps,
			Aliases:  entry.Aliases,
		})
		byID[entry.ID] = entry
	}
	return depGraph{graph: issue.NewDepGraph(nodes), entries: byID}, nil
}

// names returns a test of whether a dep, as a file writes it, names the issue with id.
func (g depGraph) names(id string) func(string) bool {
	return func(dep string) bool {
		resolved, isFound := g.graph.Resolve(dep)
		return isFound && resolved == id
	}
}

// listResult is the result of dep list for entry: its deps in the file's order, a dep naming no
// issue marked missing, then its dependents in tree order. Each issue is an indented issue line.
func (g depGraph) listResult(entry index.Entry) Result {
	data := depListData{ID: entry.ID, Missing: []string{}}
	var deps, dependents []index.Entry
	var text strings.Builder

	text.WriteString(depsHeading + "\n")
	for _, dep := range entry.Deps {
		id, isFound := g.graph.Resolve(dep)
		if !isFound {
			data.Missing = append(data.Missing, dep)
			fmt.Fprintf(&text, "%s%s  %s\n", textIndent, flatten(dep), depMissingLabel)
			continue
		}
		deps = append(deps, g.entries[id])
		text.WriteString(textIndent + issueLines([]index.Entry{g.entries[id]}, ""))
	}

	text.WriteString(dependentsHeading + "\n")
	for _, id := range g.graph.Dependents(entry.ID) {
		dependents = append(dependents, g.entries[id])
		text.WriteString(textIndent + issueLines([]index.Entry{g.entries[id]}, ""))
	}

	data.Deps = issueListData(deps)
	data.Dependents = issueListData(dependents)
	return Result{Kind: "deps", Data: data, Text: text.String()}
}
