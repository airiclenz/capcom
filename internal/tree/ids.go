package tree

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	// maxIDLength is the longest id allowed, in bytes (ids are ASCII).
	maxIDLength = 40
	// maxIDWords is the most hyphen-joined words an id may have. Raising it needs a store format
	// raise (ADR 0009): an older binary would silently drop the longer ids.
	maxIDWords = 4
	// idWordSeparator joins the words of an id.
	idWordSeparator = "-"
)

var aliasPattern = regexp.MustCompile(`^[a-z0-9.-]+$`)

// ValidateID checks that id is a kebab slug of one to four lowercase words or numbers, at most 40
// characters long. The error wraps ErrInvalidID and reads `invalid id "<id>": <rule>`, naming the
// first rule id breaks, checked in this order: its length, an empty word (a leading, trailing or
// doubled hyphen, or no id at all), an upper-case letter, a character outside a-z, 0-9 and the
// hyphen, and the number of words.
func ValidateID(id string) error {
	if rule := brokenIDRule(id); rule != "" {
		return fmt.Errorf("%w %q: %s", ErrInvalidID, id, rule)
	}
	return nil
}

// brokenIDRule returns the first rule of ValidateID that id breaks, worded for the refusal, or ""
// when id breaks none.
func brokenIDRule(id string) string {
	if len(id) > maxIDLength {
		return fmt.Sprintf("longer than %d characters", maxIDLength)
	}
	words := strings.Split(id, idWordSeparator)
	if slices.Contains(words, "") {
		return "empty word; join words with single hyphens"
	}
	if index := strings.IndexFunc(id, isUpperCaseLetter); index >= 0 {
		return fmt.Sprintf("upper-case letter %q; ids are lower-case", firstRune(id[index:]))
	}
	if index := strings.IndexFunc(id, isForeignToIDs); index >= 0 {
		return fmt.Sprintf("character %q not allowed; use a-z, 0-9 and hyphens", firstRune(id[index:]))
	}
	if len(words) > maxIDWords {
		return fmt.Sprintf("%d words, at most %d", len(words), maxIDWords)
	}
	return ""
}

// isUpperCaseLetter reports whether r is an ASCII upper-case letter, the one kind of character an
// id refuses with the hint to lower-case it.
func isUpperCaseLetter(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

// isForeignToIDs reports whether r is neither a-z, 0-9 nor the hyphen.
func isForeignToIDs(r rune) bool {
	isLowerCaseLetter := r >= 'a' && r <= 'z'
	isDigit := r >= '0' && r <= '9'
	return !isLowerCaseLetter && !isDigit && r != '-'
}

// firstRune returns the first character of text as a string, whole even when it is not ASCII.
func firstRune(text string) string {
	r, _ := utf8.DecodeRuneInString(text)
	return string(r)
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
