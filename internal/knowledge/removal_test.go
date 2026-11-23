package knowledge

import (
	"reflect"
	"testing"
)

func TestRefsBrokenByRemoval_RefsToTheFileAndItsDirectory_KeepsOnlyThoseRefs(t *testing.T) {
	t.Parallel()
	const (
		removed = ".capcom/knowledge/beta.md"
		sibling = ".capcom/knowledge/beta-new.md"
	)
	notes := []NoteFile{
		{ID: "beta", Path: removed, Note: Note{Refs: []string{removed, ".capcom/knowledge/beta/child.md"}}},
		{ID: "alpha", Path: ".capcom/knowledge/alpha.md", Note: Note{Labels: []string{"cli"}, Refs: []string{"internal/cli/rm.go", removed + "#Acceptance"}}},
		{ID: "gamma", Path: ".capcom/knowledge/gamma.md", Note: Note{Refs: []string{sibling}}},
		{ID: "delta", Path: ".capcom/knowledge/delta.md", Note: Note{Refs: []string{sibling, "./.capcom/knowledge/beta", ".capcom/knowledge/beta/child.md"}}},
		{ID: "epsilon", Path: ".capcom/knowledge/epsilon.md"},
	}

	got := RefsBrokenByRemoval(notes, removed)

	want := []NoteFile{
		{ID: "alpha", Path: ".capcom/knowledge/alpha.md", Note: Note{Refs: []string{removed + "#Acceptance"}}},
		{ID: "delta", Path: ".capcom/knowledge/delta.md", Note: Note{Refs: []string{"./.capcom/knowledge/beta", ".capcom/knowledge/beta/child.md"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RefsBrokenByRemoval(%q) = %+v, want %+v", removed, got, want)
	}
}

func TestRefsBrokenByRemoval_NoRefNamesTheRemoval_ReturnsNil(t *testing.T) {
	t.Parallel()
	const removed = ".capcom/issues/beta.md"
	notes := []NoteFile{
		{ID: "beta", Path: removed, Note: Note{Refs: []string{removed}}},
		{ID: "alpha", Path: ".capcom/knowledge/alpha.md", Note: Note{Refs: []string{".capcom/issues/beta-new.md"}}},
	}

	got := RefsBrokenByRemoval(notes, removed)

	if got != nil {
		t.Errorf("RefsBrokenByRemoval(%q) = %+v, want nil", removed, got)
	}
}
