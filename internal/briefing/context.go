package briefing

import (
	"strings"

	"github.com/airiclenz/capcom/internal/index"
)

// ApplyingNote is a Note that applies to an Issue, with the ref that made it apply; Ref is empty
// for a Note the title search found.
type ApplyingNote struct {
	Entry index.NoteEntry
	Ref   string
}

// ApplyingNotes lists the Notes in x that apply to an Issue with title and body, at most limit:
// first, in Note path order, each Note with a ref whose path part body mentions, then the Notes a
// search on title finds, every word first, else any word, best match first. Retired Notes and
// Notes under one are left out, and each Note appears once.
func ApplyingNotes(x *index.Index, title string, body string, limit int) ([]ApplyingNote, error) {
	notes, err := x.Notes(false)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]index.NoteEntry, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
	}
	refs, err := x.NoteRefs()
	if err != nil {
		return nil, err
	}
	hits, _, err := index.SearchEveryWordFirst(
		func() ([]index.NoteEntry, error) { return x.SearchNotes(title, nil, false) },
		func() ([]index.NoteEntry, error) { return x.SearchNotesAnyWord(title, nil, false) },
	)
	if err != nil {
		return nil, err
	}

	applying := make([]ApplyingNote, 0, limit)
	isListed := map[string]bool{}
	add := func(note ApplyingNote) {
		if len(applying) < limit && !isListed[note.Entry.ID] {
			isListed[note.Entry.ID] = true
			applying = append(applying, note)
		}
	}
	for _, ref := range refs {
		refPath, _, _ := strings.Cut(ref.Ref, "#")
		if note, isUsable := byID[ref.NoteID]; isUsable && mentionsPath(body, refPath) {
			add(ApplyingNote{Entry: note, Ref: ref.Ref})
		}
	}
	for _, hit := range hits {
		add(ApplyingNote{Entry: hit})
	}
	return applying, nil
}

// mentionsPath reports whether text mentions path, a slash path without a trailing slash
// counting: whole, between characters that cannot belong to a path, or as a directory, followed
// by a slash. A path ending a sentence before its full stop still counts. An empty path is never
// mentioned.
func mentionsPath(text string, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return false
	}
	for offset := 0; ; {
		at := strings.Index(text[offset:], path)
		if at < 0 {
			return false
		}
		start := offset + at
		end := start + len(path)
		if (start == 0 || !isPathByte(text[start-1])) && endsPath(text[end:]) {
			return true
		}
		offset = start + 1
	}
}

// endsPath reports whether rest, the text after a path mention, ends the path there: it is empty,
// starts with a slash (the path is a directory) or with a byte that cannot belong to a path, or
// with a full stop that ends the text or comes before such a byte.
func endsPath(rest string) bool {
	switch {
	case rest == "", rest[0] == '/', !isPathByte(rest[0]):
		return true
	case rest[0] == '.':
		return len(rest) == 1 || !isPathByte(rest[1])
	default:
		return false
	}
}

// isPathByte reports whether b can belong to a path as Note refs write them: an ASCII letter or
// digit, one of `_-./`, or a byte of a non-ASCII character.
func isPathByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '_', b == '-', b == '.', b == '/':
		return true
	default:
		return b >= 0x80
	}
}
