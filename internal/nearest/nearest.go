// Package nearest ranks the names closest to a mistyped one: the case-insensitive edit distance
// between two names, the single closest of a set of candidates, and the few closest ids or Aliases
// for an unknown id. It decides from the names alone; the wording of a suggestion is the caller's.
package nearest

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
)

// SuggestionDistance is the largest edit distance at which a mistyped name still has the closest
// valid one suggested.
const SuggestionDistance = 2

// maxIDSuggestions is the most ids or Aliases an unknown id argument names as the closest.
const maxIDSuggestions = 3

// minSharedWordLetters is the fewest letters a hyphen-separated word of an unknown id must hold for
// a name sharing that word to count as close to it.
const minSharedWordLetters = 3

// idWordSeparator separates the words of an id or Alias.
const idWordSeparator = "-"

// Closest returns the candidate with the smallest case-insensitive edit distance to word,
// the earliest one on a tie. cobra returns its suggestions in command order, not by distance, so
// know tre would otherwise suggest know rm before know tree. candidates must not be empty.
func Closest(word string, candidates []string) string {
	closest := candidates[0]
	closestDistance := EditDistance(word, closest)
	for _, candidate := range candidates[1:] {
		if distance := EditDistance(word, candidate); distance < closestDistance {
			closest, closestDistance = candidate, distance
		}
	}
	return closest
}

// EditDistance is the Levenshtein distance between left and right, ignoring case.
func EditDistance(left, right string) int {
	leftRunes := []rune(strings.ToLower(left))
	rightRunes := []rune(strings.ToLower(right))
	previousRow := make([]int, len(rightRunes)+1)
	for column := range previousRow {
		previousRow[column] = column
	}
	for row, leftRune := range leftRunes {
		currentRow := make([]int, len(rightRunes)+1)
		currentRow[0] = row + 1
		for column, rightRune := range rightRunes {
			substitution := previousRow[column]
			if leftRune != rightRune {
				substitution++
			}
			currentRow[column+1] = min(substitution, previousRow[column+1]+1, currentRow[column]+1)
		}
		previousRow = currentRow
	}
	return previousRow[len(rightRunes)]
}

// IsNear reports whether candidate is close enough to typed to be suggested for it: within
// SuggestionDistance edits, or starting with typed, ignoring case either way. It is the test cobra
// applies to command names.
func IsNear(typed, candidate string) bool {
	return EditDistance(typed, candidate) <= SuggestionDistance ||
		strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(typed))
}

// nameCandidate is a name close to an unknown id, with what ranks it: its edit distance to the id
// and the number of the id's words it shares.
type nameCandidate struct {
	name        string
	distance    int
	sharedWords int
}

// Ranked returns up to maxIDSuggestions of names close to typed: within SuggestionDistance
// edits of it, or sharing with it a hyphen-separated word of at least minSharedWordLetters
// letters, ignoring case either way. They come by edit distance, then by shared words, most first,
// then by name; none when nothing is that close or typed is empty.
func Ranked(typed string, names []string) []string {
	if typed == "" {
		return nil
	}
	var candidates []nameCandidate
	for _, name := range names {
		candidate := nameCandidate{
			name:        name,
			distance:    EditDistance(typed, name),
			sharedWords: sharedWordCount(typed, name),
		}
		if candidate.distance <= SuggestionDistance || candidate.sharedWords > 0 {
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
