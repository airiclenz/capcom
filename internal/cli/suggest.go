package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/airiclenz/capcom/internal/cli/output"
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
