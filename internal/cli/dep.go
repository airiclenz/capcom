package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/issue"
)

// cycleArrow joins the issues of a refused cycle in dep add.
const cycleArrow = " -> "

// The refusals of dep add: an issue depending on itself, and an edge that would close a cycle, with
// the dependent, the dependency and the cycle.
const (
	depSelfFormat  = "%s cannot depend on itself"
	depCycleFormat = "%s cannot depend on %s: that would create the cycle %s"
)

// The help text of capcom dep, dep add and dep rm.
const (
	depLong = "Manage the Dependencies between Issues. A Dependency is a directed edge: the dependent Issue " +
		"is not Ready until every Issue it depends on is closed. Each edge is stored in the dependent's " +
		"file, under the canonical id of the Issue it depends on. dep add and dep rm change an existing " +
		"Issue's deps, create --dep sets a new Issue's, and dep list and dep graph show them."
	depExample = "  capcom dep add login-timeout-race token-store-rework   # login-timeout-race waits for token-store-rework\n" +
		"  capcom dep list login-timeout-race                     # its deps and dependents\n" +
		"  capcom dep graph                                       # the open Issues' dependencies as a Mermaid flowchart"
	depAddLong = "Make the first Issue depend on the second: the first is not Ready until the second is " +
		"closed. Either argument may be an id, an Alias or a unique id prefix. A dependency of an Issue " +
		"on itself is refused, and so is one that would close a cycle, with the cycle named; an edge " +
		"that is already there is left as it is. It prints the id, path and etag of the dependent Issue."
	depAddExample = "  capcom dep add login-timeout-race token-store-rework   # login-timeout-race waits for token-store-rework\n" +
		"  capcom dep add refresh-retry login-timeout-race --kv    # KEY=VALUE lines ending in STATUS="
	depRmLong = "Remove the dependency of the first Issue on the second, whether the file names the second by " +
		"its id or by an Alias. A dep that names no Issue, such as one a hand edit or a merge left behind, " +
		"is removed by its text as the file writes it. It prints the id, path and etag of the dependent " +
		"Issue, and fails not_found, exit code 3, when the first Issue does not depend on the second."
	depRmExample = "  capcom dep rm login-timeout-race token-store-rework   # login-timeout-race no longer waits for it\n" +
		"  capcom dep rm refresh-retry old-cache-issue           # drop a dep that names no Issue any more"
)

// newDepCommand groups the commands that add, remove, list and graph Dependency edges. dep add and dep
// rm change an existing issue's deps and create --dep writes a new issue's; every edge is stored
// on the dependent, under the canonical id of the issue it depends on.
func (a *app) newDepCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dep",
		Short:   "Manage dependencies between issues",
		Long:    depLong,
		Example: depExample,
		Args:    cobra.NoArgs,
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
		Long:        depAddLong,
		Example:     depAddExample,
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
			deps := a.referenceList(graph, dependent.ID, dependent.Deps)
			deps.FindCycle = graph.graph.FindCycle
			addition, err := deps.Add(args[1])
			if err != nil {
				return refusalFailure(err, dependent.Path, refusalWords{self: depSelfFormat, cycle: depCycleFormat})
			}
			written, err := a.editIssue(dependent.ID, "", func(target issue.Issue) (issue.Issue, error) {
				if !addition.IsPresent {
					target.Deps = append(slices.Clone(target.Deps), addition.ID)
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
		Long:        depRmLong,
		Example:     depRmExample,
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
			removal, err := a.referenceList(graph, dependent.ID, dependent.Deps).Remove(args[1])
			if err != nil {
				return refusalFailure(err, "", refusalWords{})
			}
			notDep := refusalWords{notFound: fmt.Sprintf("%s does not depend on %s", dependent.ID, args[1])}
			written, err := a.editIssue(dependent.ID, "", func(target issue.Issue) (issue.Issue, error) {
				dropped, err := removal.Drop(target.Deps)
				if err != nil {
					return issue.Issue{}, refusalFailure(err, "", notDep)
				}
				target.Deps = withoutEntries(target.Deps, dropped)
				return target, nil
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}

// withoutEntries returns entries without the ones dropped, or nil when none is left, so the file
// leaves the list's key out.
func withoutEntries(entries []string, dropped []string) []string {
	kept := slices.DeleteFunc(slices.Clone(entries), func(entry string) bool {
		return slices.Contains(dropped, entry)
	})
	if len(kept) == 0 {
		return nil
	}
	return kept
}
