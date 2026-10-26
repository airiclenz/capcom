package cli

import (
	"cmp"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/ui"
)

// completionDirective is the directive of every id completion: the shell keeps the candidates in
// the order given and never falls back to file names, not even when there are no candidates.
const completionDirective = cobra.ShellCompDirectiveKeepOrder | cobra.ShellCompDirectiveNoFileComp

// everyArgument is the argument count of completeIssueIDs for a command whose every positional
// argument is an Issue id, such as show and close.
const everyArgument = -1

// uiTabs are the tabs capcom ui takes, in the order of its tab bar.
var uiTabs = []ui.Tab{ui.TabIssues, ui.TabReady, ui.TabKnowledge}

// completeIssueIDs returns the completion function of a command whose first count positional
// arguments are Issue ids, or every one with everyArgument: each completes the Issue ids and aliases
// it starts, described by the Issue's title. A later argument has no candidates.
func completeIssueIDs(count int) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if count != everyArgument && len(args) >= count {
			return nil, completionDirective
		}
		return withCompletionIndex(func(x *index.Index) ([]cobra.Completion, error) {
			return issueCandidates(x, toComplete)
		})
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
				return noteCandidates(x, toComplete)
			}
			return issueCandidates(x, toComplete)
		})
	default:
		return nil, completionDirective
	}
}

// withCompletionIndex opens the store of the working directory and its refreshed index, returns
// what candidates reads from it and closes the index again. cobra's __complete calls a completion
// function without running its command, so prepare opens neither. Any failure, such as a working directory outside a git repository or a
// store, leaves no candidates; it is written to cobra's completion debug log, as the shell shows
// no errors.
func withCompletionIndex(
	candidates func(x *index.Index) ([]cobra.Completion, error),
) ([]cobra.Completion, cobra.ShellCompDirective) {
	completions, err := readCompletionIndex(candidates)
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, completionDirective
	}
	return completions, completionDirective
}

// readCompletionIndex opens the store and index for withCompletionIndex and returns what candidates
// reads from the index, or the first failure.
func readCompletionIndex(
	candidates func(x *index.Index) ([]cobra.Completion, error),
) (completions []cobra.Completion, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	opened, err := store.Open(cwd)
	if err != nil {
		return nil, err
	}
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

// noteCandidates returns the ids and aliases of every usable Note, retired ones included, that start
// with prefix, each described by its title: by id, every Note's id before its aliases.
func noteCandidates(x *index.Index, prefix string) ([]cobra.Completion, error) {
	entries, err := x.Notes(true)
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
