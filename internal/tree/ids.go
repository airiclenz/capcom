package tree

import (
	"fmt"
	"regexp"
	"slices"
)

// maxIDLength is the longest id allowed, in bytes (ids are ASCII).
const maxIDLength = 40

var (
	idPattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+){0,3}$`)
	aliasPattern = regexp.MustCompile(`^[a-z0-9.-]+$`)
)

// ValidateID checks that id is a kebab slug of one to four lowercase words or numbers, at most 40
// characters long. The error wraps ErrInvalidID.
func ValidateID(id string) error {
	if len(id) > maxIDLength {
		return fmt.Errorf("%w %q: longer than %d characters", ErrInvalidID, id, maxIDLength)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf(
			"%w %q: must be 1 to 4 words of a-z and 0-9 joined by single hyphens",
			ErrInvalidID,
			id,
		)
	}
	return nil
}

// ValidateAlias checks that alias is non-empty and uses only a-z, 0-9, dots and hyphens, the
// wider set that legacy ids (`apogee-kk0.4`) need. The error wraps ErrInvalidAlias.
func ValidateAlias(alias string) error {
	if !aliasPattern.MatchString(alias) {
		return fmt.Errorf("%w %q: must use only a-z, 0-9, dots and hyphens", ErrInvalidAlias, alias)
	}
	return nil
}

// CheckAliases checks the aliases of the one node file at path on their own: each must be a valid
// alias, must not repeat the node's own id and must not be listed twice. kind names the node in a
// message: alias "x" repeats the <kind>'s own id. It returns the aliases that pass, in order, and
// an *Error naming path and the alias for each one that does not; an invalid alias wraps
// ErrInvalidAlias, a repeated one ErrRepeatedAlias and ErrDuplicate. The first of an alias listed
// twice passes.
func CheckAliases(path, kind, id string, aliases []string) ([]string, Errors) {
	var kept []string
	var problems Errors
	seen := map[string]bool{}
	for _, alias := range aliases {
		err := checkAlias(kind, id, alias, seen[alias])
		seen[alias] = true
		if err != nil {
			problems = append(problems, &Error{Path: path, Alias: alias, Err: err})
			continue
		}
		kept = append(kept, alias)
	}
	return kept, problems
}

// checkAlias checks one alias of a node file, isSeen telling whether the file lists it earlier.
func checkAlias(kind, id, alias string, isSeen bool) error {
	if err := ValidateAlias(alias); err != nil {
		return err
	}
	if alias == id {
		return &repeatedAliasError{message: fmt.Sprintf("alias %q repeats the %s's own id", alias, kind)}
	}
	if isSeen {
		return &repeatedAliasError{message: fmt.Sprintf("alias %q is listed twice", alias)}
	}
	return nil
}

// RenameAliases returns the Aliases of a node renamed from oldID to newID: newID leaves them (it
// may have been a former id) and oldID joins them at the end. aliases is returned unchanged when
// the two ids agree.
func RenameAliases(aliases []string, oldID string, newID string) []string {
	if oldID == newID {
		return aliases
	}
	kept := slices.DeleteFunc(slices.Clone(aliases), func(alias string) bool {
		return alias == newID
	})
	return append(kept, oldID)
}
