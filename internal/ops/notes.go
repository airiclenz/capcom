package ops

import (
	"slices"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/reflist"
)

// NoteNames returns the reflist.NamesFunc over notes, a superseded-by entry as the file writes it
// naming the Note whose id it is, else the one Note of notes listing it as an Alias. An alias two
// Notes list names neither.
func NoteNames(notes []index.NoteEntry) reflist.NamesFunc {
	byID := make(map[string]bool, len(notes))
	for _, note := range notes {
		byID[note.ID] = true
	}
	byAlias := map[string]string{}
	for _, note := range notes {
		for _, alias := range note.Aliases {
			if _, isClaimed := byAlias[alias]; isClaimed {
				byAlias[alias] = ""
				continue
			}
			byAlias[alias] = note.ID
		}
	}
	return func(entry string) (string, bool) {
		if byID[entry] {
			return entry, true
		}
		id := byAlias[entry]
		return id, id != ""
	}
}

// EditSuccessors applies the addArgs, then the removeArgs, to entries, a superseded-by list as the
// file writes it, each decided by list against entries as the edits before it left them; entries
// itself is left as it is. An addition appends the id its argument resolves to unless an entry
// already names that Note, by id or by any Alias; a removal drops the entries it matches, and one
// matching none changes nothing. It fails on the first argument list.Add or list.Remove refuses,
// with that error unchanged.
func EditSuccessors(
	list reflist.List,
	addArgs []string,
	removeArgs []string,
	entries []string,
) ([]string, error) {
	entries = slices.Clone(entries)
	for _, arg := range addArgs {
		list.Entries = entries
		addition, err := list.Add(arg)
		if err != nil {
			return nil, err
		}
		if !addition.IsPresent {
			entries = append(entries, addition.ID)
		}
	}
	for _, arg := range removeArgs {
		list.Entries = entries
		removal, err := list.Remove(arg)
		if err != nil {
			return nil, err
		}
		// Drop's only failure is a removal that drops no entry: a silent no-op here.
		if dropped, err := removal.Drop(entries); err == nil {
			entries = WithoutEntries(entries, dropped)
		}
	}
	return entries, nil
}
