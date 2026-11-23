package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/nearest"
	"github.com/airiclenz/capcom/internal/tree"
)

// longFlagPrefix is how a long flag is written on the command line, and so in a flag suggestion.
const longFlagPrefix = "--"

// unknownFlagError returns err as a usage error naming the closest flag of cmd when err is pflag's
// unknown long flag, such as --statsu; its Suggestion is that flag written as --status, and it
// is empty when no flag is close. Any other error, an unknown shorthand among them, is returned
// unchanged.
func unknownFlagError(cmd *cobra.Command, err error) error {
	var notExist *pflag.NotExistError
	if !errors.As(err, &notExist) || notExist.GetSpecifiedShortnames() != "" {
		return err
	}
	failure := output.UsageErrorf("%s", err.Error())
	if closest := closestFlagName(cmd, notExist.GetSpecifiedName()); closest != "" {
		failure.Suggestion = longFlagPrefix + closest
	}
	return failure
}

// closestFlagName returns the name of the visible flag of cmd, its own or inherited, closest to
// typed: among the flags within nearest.SuggestionDistance of typed, or whose name typed starts, the one
// with the smallest edit distance, the first in name order on a tie. It is empty when none is
// that close or typed is empty.
func closestFlagName(cmd *cobra.Command, typed string) string {
	candidates := nearFlagNames(cmd, typed)
	if len(candidates) == 0 {
		return ""
	}
	return nearest.Closest(typed, candidates)
}

// nearFlagNames returns, in name order and each once, the names of the visible flags of cmd, its
// own and inherited, that nearest.IsNear accepts for typed; none for an empty typed.
func nearFlagNames(cmd *cobra.Command, typed string) []string {
	if typed == "" {
		return nil
	}
	var names []string
	collect := func(flag *pflag.Flag) {
		if flag.Hidden || slices.Contains(names, flag.Name) || !nearest.IsNear(typed, flag.Name) {
			return
		}
		names = append(names, flag.Name)
	}
	cmd.Flags().VisitAll(collect)
	cmd.InheritedFlags().VisitAll(collect)
	slices.Sort(names)
	return names
}

// commandSynonyms maps, for each group by its command path, a word another CLI uses for one of its
// subcommands to that subcommand's name, so that capcom get is answered with show. A synonym only
// feeds the suggestion of an unknown command: it never runs the subcommand it names.
var commandSynonyms = map[string]map[string]string{
	"capcom": {
		"get":  "show",
		"view": "show",
		"read": "show",
		"cat":  "show",
		"ls":   "list",
		"new":  "create",
		"add":  "create",
		"done": "close",
	},
	"capcom know": {
		"get":    "show",
		"view":   "show",
		"read":   "show",
		"cat":    "show",
		"ls":     "tree",
		"list":   "tree",
		"new":    "add",
		"create": "add",
	},
}

// synonymSubcommand returns the name of the subcommand of group that word is a synonym for in
// commandSynonyms, ignoring case; it is empty when word is none, or names no subcommand of group.
func synonymSubcommand(group *cobra.Command, word string) string {
	name := commandSynonyms[group.CommandPath()][strings.ToLower(word)]
	if name == "" || !namesSubcommand(group, name) {
		return ""
	}
	return name
}

// idKind is the kind of node an id argument names: an unknown one is matched against the ids and
// Aliases of its own kind only.
type idKind int

// The kinds of node an id argument names.
const (
	issueIDs idKind = iota
	noteIDs
)

// notFoundFailure types err, a failure to resolve an argument naming a node of kind that matches
// tree.ErrNotFound, as not_found. When the argument matched nothing at all (*tree.NotFoundError),
// its Suggestions are the closest ids or Aliases of kind; a name only an unusable file claims gets
// none. An unknown Note hints capcom know search, as capcom search finds issues only. isArgument
// is false only when the index fails to list the names of kind, which a multi-id command cannot go
// on from.
func (a *app) notFoundFailure(err error, kind idKind) (failure *output.Error, isArgument bool) {
	failure = output.Errorf(output.CodeNotFound, "%s", err.Error())
	if kind == noteIDs {
		failure.Hint = "capcom know search <text>"
	}
	var notFound *tree.NotFoundError
	if !errors.As(err, &notFound) {
		return failure, true
	}
	suggestions, err := a.closestIDs(kind, notFound.Arg)
	if err != nil {
		return output.AsError(err), false
	}
	failure.Suggestions = suggestions
	return failure, true
}

// closestIDs returns the ids and Aliases of the usable nodes of kind closest to arg, as
// nearest.Ranked picks them for arg's citation spelling normalised (tree.NormalizeArg); none when
// the command opened no index.
func (a *app) closestIDs(kind idKind, arg string) ([]string, error) {
	if a.issueIndex == nil {
		return nil, nil
	}
	listNames := a.issueIndex.IssueNames
	if kind == noteIDs {
		listNames = a.issueIndex.NoteNames
	}
	names, err := listNames()
	if err != nil {
		return nil, err
	}
	return nearest.Ranked(tree.NormalizeArg(arg), names), nil
}
