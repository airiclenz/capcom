package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/reflist"
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
	depLong = "Manage the Dependencies between Issues.\n\n" +
		"A Dependency is a directed edge: the dependent Issue is not Ready until every Issue it " +
		"depends on is closed.\n\n" +
		"Each edge is stored in the dependent's file, under the canonical id of the Issue it depends " +
		"on.\n\n" +
		"dep add and dep rm change an existing Issue's deps, create --dep sets a new Issue's, and dep " +
		"list and dep graph show them."
	depExample = "  capcom dep add login-timeout-race token-store-rework   # login-timeout-race waits for token-store-rework\n" +
		"  capcom dep list login-timeout-race                     # its deps and dependents\n" +
		"  capcom dep graph                                       # the open Issues' dependencies as a Mermaid flowchart"
	depAddLong = "Make the first Issue depend on the second: the first is not Ready until the second is " +
		"closed.\n\n" +
		"Either argument may be an id, an Alias or a unique id prefix.\n\n" +
		"A dependency of an Issue on itself is refused, and so is one that would close a cycle, with " +
		"the cycle named; an edge that is already there is left as it is.\n\n" +
		"It prints the id, path and etag of the dependent Issue."
	depAddExample = "  capcom dep add login-timeout-race token-store-rework   # login-timeout-race waits for token-store-rework\n" +
		"  capcom dep add refresh-retry login-timeout-race --kv    # KEY=VALUE lines ending in STATUS="
	depRmLong = "Remove the dependency of the first Issue on the second, whether the file names the second by " +
		"its id or by an Alias.\n\n" +
		"A dep that names no Issue, such as one a hand edit or a merge left behind, is removed by its " +
		"text as the file writes it.\n\n" +
		"It prints the id, path and etag of the dependent Issue, and fails not_found, exit code 3, " +
		"when the first Issue does not depend on the second."
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
	}
	cmd.AddCommand(a.newDepAddCommand())
	cmd.AddCommand(a.newDepGraphCommand())
	cmd.AddCommand(a.newDepListCommand())
	cmd.AddCommand(a.newDepRmCommand())
	return withSubcommandDispatch(cmd)
}

// newDepAddCommand makes one issue depend on another. A self-dependency and an edge that would
// close a cycle are refused; an edge that is already there is left as it is.
func (a *app) newDepAddCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:               "add <id|alias|prefix> <dep-id|alias|prefix>",
		Short:             "Make an issue depend on another",
		Long:              depAddLong,
		Example:           depAddExample,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeIssueIDs(2),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			written, err := a.ops().AddDep(args[0], "", args[1])
			written, err = a.depWritten(written, err, refusalWords{self: depSelfFormat, cycle: depCycleFormat})
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
		Use:               "rm <id|alias|prefix> <dep-id|alias|prefix>",
		Short:             "Remove a dependency of an issue",
		Long:              depRmLong,
		Example:           depRmExample,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeIssueIDs(2),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			written, err := a.ops().RemoveDep(args[0], "", args[1])
			notDep := refusalWords{
				notFound:     fmt.Sprintf("%s does not depend on %s", written.ID, args[1]),
				notFoundHint: "capcom dep list " + written.ID,
			}
			written, err = a.depWritten(written, err, notDep)
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}

// depWritten types the outcome of ops.AddDep or ops.RemoveDep for dep add or dep rm: a refusal of
// reflist's, whether of the argument or of the edit, is typed by refusalFailure with words at the
// dependent's file, and every other outcome as written types it.
func (a *app) depWritten(written nodestore.Written, err error, words refusalWords) (nodestore.Written, error) {
	var refusal *reflist.Refusal
	if errors.As(err, &refusal) {
		return nodestore.Written{}, a.refusalFailure(err, issueIDs, written.Path, words)
	}
	return a.written(issueIDs, written, err)
}
