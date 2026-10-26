package cli

import (
	"cmp"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/ui"
)

// completionDirective is the directive of every completion: the shell keeps the candidates in the
// order given and never falls back to file names, not even when there are no candidates.
const completionDirective = cobra.ShellCompDirectiveKeepOrder | cobra.ShellCompDirectiveNoFileComp

// everyArgument is the argument count of completeIDs for a command whose every positional
// argument is an id, such as show and close, and for a flag's value, which no positional argument
// limits.
const everyArgument = -1

// valueSeparator separates the values of a StringSlice flag given as one argument.
const valueSeparator = ","

// uiTabs are the tabs capcom ui takes, in the order of its tab bar.
var uiTabs = []ui.Tab{ui.TabIssues, ui.TabReady, ui.TabKnowledge, ui.TabClosed}

// isCompletionRequest reports whether args, the program name excluded, ask cobra's hidden
// __complete or __completeNoDesc command for the shell's candidates.
func isCompletionRequest(args []string) bool {
	return len(args) > 0 && (args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd)
}

// completeFlag registers complete as the completion of cmd's flag name, on a completion request
// only: cobra keeps flag completions in a package-global map that never forgets a flag, so
// registering them for every command tree built, such as each Run and each cli.Command(), would
// grow it without bound.
func (a *app) completeFlag(cmd *cobra.Command, name string, complete cobra.CompletionFunc) {
	if !a.isCompleting {
		return
	}
	_ = cmd.RegisterFlagCompletionFunc(name, complete) // fails only for an undefined or twice-registered flag
}

// completeIssueIDs returns the completion function of a command whose first count positional
// arguments are Issue ids, or every one with everyArgument; see completeIDs.
func completeIssueIDs(count int) cobra.CompletionFunc {
	return completeIDs(issueIDs, count)
}

// completeNoteIDs returns the completion function of a command whose first count positional
// arguments are Note ids, or every one with everyArgument; see completeIDs.
func completeNoteIDs(count int) cobra.CompletionFunc {
	return completeIDs(noteIDs, count)
}

// completeIDValue returns the completion function of a flag whose value is an id of kind.
func completeIDValue(kind idKind) cobra.CompletionFunc {
	return completeIDs(kind, everyArgument)
}

// completeIDs returns the completion function of a command whose first count positional arguments
// are ids of kind, or every one with everyArgument: each completes the ids and aliases of kind it
// starts, described by their titles, as idCandidates lists them. A later argument has no
// candidates.
func completeIDs(kind idKind, count int) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if count != everyArgument && len(args) >= count {
			return nil, completionDirective
		}
		return withCompletionIndex(func(x *index.Index) ([]cobra.Completion, error) {
			return idCandidates(x, kind, toComplete)
		})
	}
}

// idCandidates returns the ids and aliases of kind that start with prefix: every usable Issue's, or
// the Notes' that are not retired.
func idCandidates(x *index.Index, kind idKind, prefix string) ([]cobra.Completion, error) {
	if kind == noteIDs {
		return noteCandidates(x, prefix, false)
	}
	return issueCandidates(x, prefix)
}

// completeStatuses returns the completion function of a --status: the statuses config.yaml declares
// whose category isOffered accepts, in file order, each described by its category.
func completeStatuses(isOffered func(category config.Category) bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return withCompletionStore(func(opened *store.Store) ([]cobra.Completion, error) {
			var completions []cobra.Completion
			for _, status := range opened.Config.Statuses {
				if isOffered(status.Category) && strings.HasPrefix(status.Name, toComplete) {
					completions = append(completions, cobra.CompletionWithDesc(status.Name, string(status.Category)))
				}
			}
			return completions, nil
		})
	}
}

// isOpenCategory reports whether category is not closed: what --status of create, update and
// reopen accepts.
func isOpenCategory(category config.Category) bool {
	return !category.IsClosed()
}

// isAnyCategory accepts every category: what --status of list accepts.
func isAnyCategory(config.Category) bool {
	return true
}

// completeTypes completes a --type: the types config.yaml declares, in file order.
func completeTypes(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return withCompletionStore(func(opened *store.Store) ([]cobra.Completion, error) {
		var completions []cobra.Completion
		for _, issueType := range opened.Config.Types {
			if strings.HasPrefix(issueType, toComplete) {
				completions = append(completions, issueType)
			}
		}
		return completions, nil
	})
}

// completeCategories completes a --category: the status categories, in workflow order. They are
// fixed, so they complete outside a store too.
func completeCategories(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	var completions []cobra.Completion
	for _, category := range statusCategories {
		if strings.HasPrefix(string(category), toComplete) {
			completions = append(completions, string(category))
		}
	}
	return completions, completionDirective
}

// commaSeparated returns complete for the value of a StringSlice flag, which takes several values
// separated by commas in one argument: it completes the part after the last comma and keeps what
// comes before it, that comma included, at the front of every candidate.
func commaSeparated(complete cobra.CompletionFunc) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		separator := strings.LastIndex(toComplete, valueSeparator)
		if separator < 0 {
			return complete(cmd, args, toComplete)
		}
		completed := toComplete[:separator+len(valueSeparator)]
		completions, directive := complete(cmd, args, toComplete[len(completed):])
		for position, completion := range completions {
			completions[position] = completed + completion
		}
		return completions, directive
	}
}

// completeUITarget completes the arguments of capcom ui: a tab name first, then an id of that tab's
// kind, a Note's for the Knowledge tab and an Issue's for the others.
func completeUITarget(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		completions := make([]cobra.Completion, 0, len(uiTabs))
		for _, tab := range uiTabs {
			if strings.HasPrefix(tab.String(), toComplete) {
				completions = append(completions, tab.String())
			}
		}
		return completions, completionDirective
	case 1:
		tab, err := ui.ParseTab(args[0])
		if err != nil {
			return nil, completionDirective
		}
		return withCompletionIndex(func(x *index.Index) ([]cobra.Completion, error) {
			if tab == ui.TabKnowledge {
				return noteCandidates(x, toComplete, true)
			}
			return issueCandidates(x, toComplete)
		})
	default:
		return nil, completionDirective
	}
}

// withCompletionIndex opens the store of the working directory and its refreshed index, returns
// what candidates reads from it and closes the index again, as withCompletionStore does for the
// store alone.
func withCompletionIndex(
	candidates func(x *index.Index) ([]cobra.Completion, error),
) ([]cobra.Completion, cobra.ShellCompDirective) {
	return withCompletionStore(func(opened *store.Store) ([]cobra.Completion, error) {
		return readCompletionIndex(opened, candidates)
	})
}

// withCompletionStore opens the store of the working directory and returns what candidates reads
// from it. cobra's __complete calls a completion function without running its command, so prepare
// opens nothing. Any failure, such as a working directory outside a git repository or a store,
// leaves no candidates; it is written to cobra's completion debug log, as the shell shows no
// errors.
func withCompletionStore(
	candidates func(opened *store.Store) ([]cobra.Completion, error),
) ([]cobra.Completion, cobra.ShellCompDirective) {
	completions, err := readCompletionStore(candidates)
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, completionDirective
	}
	return completions, completionDirective
}

// readCompletionStore opens the store for withCompletionStore and returns what candidates reads
// from it, or the first failure.
func readCompletionStore(
	candidates func(opened *store.Store) ([]cobra.Completion, error),
) ([]cobra.Completion, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	opened, err := store.Open(cwd)
	if err != nil {
		return nil, err
	}
	return candidates(opened)
}

// readCompletionIndex opens the refreshed index of opened for withCompletionIndex and returns what
// candidates reads from it, or the first failure; it closes the index again.
func readCompletionIndex(
	opened *store.Store,
	candidates func(x *index.Index) ([]cobra.Completion, error),
) (completions []cobra.Completion, err error) {
	issueIndex, err := index.Open(opened, index.ModeRefresh)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := issueIndex.Close(); closeErr != nil && err == nil {
			completions, err = nil, closeErr
		}
	}()
	return candidates(issueIndex)
}

// issueCandidates returns the ids and aliases of every usable Issue that start with prefix, each
// described by its title: the open Issues first, then the closed ones, each group by id and every
// Issue's id before its aliases.
func issueCandidates(x *index.Index, prefix string) ([]cobra.Completion, error) {
	entries, err := x.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(entries, func(left, right index.Entry) int {
		if isLeftClosed, isRightClosed := left.Category.IsClosed(), right.Category.IsClosed(); isLeftClosed != isRightClosed {
			if isLeftClosed {
				return 1
			}
			return -1
		}
		return cmp.Compare(left.ID, right.ID)
	})
	var completions []cobra.Completion
	for _, entry := range entries {
		completions = appendCandidates(completions, prefix, entry.Title, entry.ID, entry.Aliases)
	}
	return completions, nil
}

// noteCandidates returns the ids and aliases of every usable Note that start with prefix, each
// described by its title: by id, every Note's id before its aliases. Retired Notes are among them
// only with isRetiredIncluded.
func noteCandidates(x *index.Index, prefix string, isRetiredIncluded bool) ([]cobra.Completion, error) {
	entries, err := x.Notes(isRetiredIncluded)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(entries, func(left, right index.NoteEntry) int {
		return cmp.Compare(left.ID, right.ID)
	})
	var completions []cobra.Completion
	for _, entry := range entries {
		completions = appendCandidates(completions, prefix, entry.Title, entry.ID, entry.Aliases)
	}
	return completions, nil
}

// appendCandidates appends to completions id and then each of aliases that starts with prefix,
// each described by title.
func appendCandidates(
	completions []cobra.Completion,
	prefix string,
	title string,
	id string,
	aliases []string,
) []cobra.Completion {
	for _, name := range append([]string{id}, aliases...) {
		if strings.HasPrefix(name, prefix) {
			completions = append(completions, cobra.CompletionWithDesc(name, title))
		}
	}
	return completions
}
