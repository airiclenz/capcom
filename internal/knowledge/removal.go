package knowledge

import "github.com/airiclenz/capcom/internal/tree"

// RefsBrokenByRemoval returns the Notes of notes whose Refs name the removed node file, its
// children directory or a path below it, matched as RefsNaming matches them, ready for Verify: each
// keeps its ID and Path, and its Note carries those Refs alone (no other field and no Ref lines, so
// a finding's line is never one of a dropped Ref). The Note whose file is removed is left out, and
// so is every Note with no such Ref; the Notes keep their order, and nil means none is left.
// removed is relative to the working tree root with forward slashes.
func RefsBrokenByRemoval(notes []NoteFile, removed string) []NoteFile {
	childrenDir := tree.ChildrenDir(removed)
	var broken []NoteFile
	for _, file := range notes {
		if file.Path == removed {
			continue
		}
		if refs := RefsNaming(file.Note.Refs, removed, childrenDir); len(refs) > 0 {
			broken = append(broken, NoteFile{ID: file.ID, Path: file.Path, Note: Note{Refs: refs}})
		}
	}
	return broken
}
