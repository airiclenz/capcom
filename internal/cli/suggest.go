package cli

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/airiclenz/capcom/internal/cli/output"
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
// typed: among the flags within suggestionDistance of typed, or whose name typed starts, the one
// with the smallest edit distance, the first in name order on a tie. It is empty when none is
// that close or typed is empty.
func closestFlagName(cmd *cobra.Command, typed string) string {
	candidates := nearFlagNames(cmd, typed)
	if len(candidates) == 0 {
		return ""
	}
	return closestName(typed, candidates)
}

// nearFlagNames returns, in name order and each once, the names of the visible flags of cmd, its
// own and inherited, that isNearName accepts for typed; none for an empty typed.
func nearFlagNames(cmd *cobra.Command, typed string) []string {
	if typed == "" {
		return nil
	}
	var names []string
	collect := func(flag *pflag.Flag) {
		if flag.Hidden || slices.Contains(names, flag.Name) || !isNearName(typed, flag.Name) {
			return
		}
		names = append(names, flag.Name)
	}
	cmd.Flags().VisitAll(collect)
	cmd.InheritedFlags().VisitAll(collect)
	slices.Sort(names)
	return names
}

// isNearName reports whether candidate is close enough to typed to be suggested for it: within
// suggestionDistance edits, or starting with typed, ignoring case either way. It is the test cobra
// applies to command names.
func isNearName(typed, candidate string) bool {
	return editDistance(typed, candidate) <= suggestionDistance ||
		strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(typed))
}

// maxIDSuggestions is the most ids or Aliases an unknown id argument names as the closest.
const maxIDSuggestions = 3

// minSharedWordLetters is the fewest letters a hyphen-separated word of an unknown id must hold for
// a name sharing that word to count as close to it.
const minSharedWordLetters = 3

// idWordSeparator separates the words of an id or Alias.
const idWordSeparator = "-"

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
// closestNames picks them for arg's citation spelling normalised (tree.NormalizeArg); none when
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
	return closestNames(tree.NormalizeArg(arg), names), nil
}

// nameCandidate is a name close to an unknown id, with what ranks it: its edit distance to the id
// and the number of the id's words it shares.
type nameCandidate struct {
	name        string
	distance    int
	sharedWords int
}

// closestNames returns up to maxIDSuggestions of names close to typed: within suggestionDistance
// edits of it, or sharing with it a hyphen-separated word of at least minSharedWordLetters
// letters, ignoring case either way. They come by edit distance, then by shared words, most first,
// then by name; none when nothing is that close or typed is empty.
func closestNames(typed string, names []string) []string {
	if typed == "" {
		return nil
	}
	var candidates []nameCandidate
	for _, name := range names {
		candidate := nameCandidate{
			name:        name,
			distance:    editDistance(typed, name),
			sharedWords: sharedWordCount(typed, name),
		}
		if candidate.distance <= suggestionDistance || candidate.sharedWords > 0 {
			candidates = append(candidates, candidate)
		}
	}
	slices.SortFunc(candidates, func(left, right nameCandidate) int {
		return cmp.Or(
			cmp.Compare(left.distance, right.distance),
			cmp.Compare(right.sharedWords, left.sharedWords),
			cmp.Compare(left.name, right.name),
		)
	})
	var closest []string
	for _, candidate := range candidates[:min(len(candidates), maxIDSuggestions)] {
		closest = append(closest, candidate.name)
	}
	return closest
}

// sharedWordCount counts the distinct hyphen-separated words of typed holding at least
// minSharedWordLetters letters that are words of name too, ignoring case.
func sharedWordCount(typed, name string) int {
	typedWords := strings.Split(strings.ToLower(typed), idWordSeparator)
	slices.Sort(typedWords)
	nameWords := strings.Split(strings.ToLower(name), idWordSeparator)
	count := 0
	for _, word := range slices.Compact(typedWords) {
		if letterCount(word) >= minSharedWordLetters && slices.Contains(nameWords, word) {
			count++
		}
	}
	return count
}

// letterCount counts the letters of word.
func letterCount(word string) int {
	count := 0
	for _, character := range word {
		if unicode.IsLetter(character) {
			count++
		}
	}
	return count
}
