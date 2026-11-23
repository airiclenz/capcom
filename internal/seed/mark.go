package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/knowledge"
)

const (
	// seedLabelPrefix starts the Label that marks a Note as seeded; the id of the set's root
	// Note follows it.
	seedLabelPrefix = "seed:"
	// seedTextLabelPrefix starts the Label that records the text a Note was seeded with; the
	// capcom version that seeded it, labelFieldSeparator and the Hash of that text follow it.
	seedTextLabelPrefix = "seed-text:"
	// labelFieldSeparator parts the version and the hash of a seed-text Label.
	labelFieldSeparator = ":"
	// hashLength is the number of hex digits of a Hash.
	hashLength = 12
	// hashDigits are the digits a Hash is written in.
	hashDigits = "0123456789abcdef"
	// lineBreak ends a line of a Note's text, and parts the title from the body under a Hash.
	lineBreak = "\n"
)

// State is how the store's Note of an id stands against the Note of that id in a set.
type State string

const (
	// Same is a Note whose title and body are the set's.
	Same State = "same"
	// Outdated is a Note whose text is not the set's but is still, unedited, the text it was
	// seeded with: the set has changed since.
	Outdated State = "outdated"
	// Differs is a Note whose text is neither the set's nor provably what it was seeded with: the
	// repository's own Note, an edited one, or one that lost its seed-text Label.
	Differs State = "differs"
)

// Hash returns the hash of a Note's text: the first 12 hex digits, in lower case, of the SHA-256
// of the title, a line break and the body. CRLF line endings count as LF and line breaks at the
// end do not count, so a Note hashes the same however its file ends its lines.
func Hash(title, body string) string {
	sum := sha256.Sum256([]byte(comparableText(title, body)))
	return hex.EncodeToString(sum[:])[:hashLength]
}

// Labels returns the two Labels a Note carries when capcom of the given version seeds it with the
// text of note: `seed:<root id>`, which marks it as seeded from this set, and
// `seed-text:<version>:<hash>`, which records the Hash of that text.
func (s Set) Labels(note Note, version string) []string {
	return []string{s.seedLabel(), textLabel(note, version)}
}

// textLabel returns the seed-text Label of the text of note, seeded by capcom of the given
// version.
func textLabel(note Note, version string) string {
	return seedTextLabelPrefix + version + labelFieldSeparator + Hash(note.Title, note.Body)
}

// Relabel returns labels, the Labels of a Note seeded with an earlier text, as they stand once
// capcom of the given version has replaced that text with the text of note: the first seed-text
// Label gives its place to the seed-text Label of note's text and every further one is dropped,
// so the Note records one text. Every other Label keeps its place. Without a seed-text Label
// among labels the new one is added at the end. labels itself is left as it is.
func (s Set) Relabel(labels []string, note Note, version string) []string {
	newLabel := textLabel(note, version)
	relabelled := make([]string, 0, len(labels)+1)
	isPlaced := false
	for _, label := range labels {
		if !strings.HasPrefix(label, seedTextLabelPrefix) {
			relabelled = append(relabelled, label)
			continue
		}
		if !isPlaced {
			relabelled = append(relabelled, newLabel)
			isPlaced = true
		}
	}
	if !isPlaced {
		relabelled = append(relabelled, newLabel)
	}
	return relabelled
}

// IsSeeded reports whether labels, a Note's Labels, include the `seed:<root id>` Label of this
// set.
func (s Set) IsSeeded(labels []string) bool {
	return slices.Contains(labels, s.seedLabel())
}

// State returns how existing, the store's Note of the given id, stands against the set's Note of
// that id. It compares title and body only, as Hash reads them: Labels, Refs and line endings
// make no difference.
//
// The state is Same when the text is the set's, Outdated when it is not but a seed-text Label of
// existing carries the Hash of existing's own text, and Differs otherwise. So a Note without a
// readable seed-text Label is never Outdated, and neither is an id the set holds no Note for.
func (s Set) State(id string, existing knowledge.Note) State {
	note, isInSet := s.Note(id)
	if !isInSet {
		return Differs
	}
	if comparableText(existing.Title, existing.Body) == comparableText(note.Title, note.Body) {
		return Same
	}
	if slices.Contains(seededHashes(existing.Labels), Hash(existing.Title, existing.Body)) {
		return Outdated
	}
	return Differs
}

// seedLabel returns the Label that marks a Note as seeded from this set.
func (s Set) seedLabel() string {
	return seedLabelPrefix + s.Root().ID
}

// comparableText returns the text of a Note that Hash and State compare: the title, a line break
// and the body, with CRLF line endings as LF and without the line breaks at the end.
func comparableText(title, body string) string {
	return strings.TrimRight(edit.NormalizeNewlines(title+lineBreak+body), lineBreak)
}

// seededHashes returns the hash of every readable seed-text Label among labels.
func seededHashes(labels []string) []string {
	var hashes []string
	for _, label := range labels {
		if hash, isReadable := seededHash(label); isReadable {
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

// seededHash returns the hash that label carries, and whether label is a readable seed-text
// Label: `seed-text:<version>:<hash>` with a version and a hash as Hash writes one. The hash is
// what follows the last separator, so a version may hold one.
func seededHash(label string) (string, bool) {
	fields, hasPrefix := strings.CutPrefix(label, seedTextLabelPrefix)
	if !hasPrefix {
		return "", false
	}
	separatorAt := strings.LastIndex(fields, labelFieldSeparator)
	if separatorAt <= 0 {
		return "", false
	}
	hash := fields[separatorAt+len(labelFieldSeparator):]
	if len(hash) != hashLength || strings.Trim(hash, hashDigits) != "" {
		return "", false
	}
	return hash, true
}
