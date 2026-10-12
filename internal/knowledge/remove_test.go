package knowledge

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

func TestPlanRemove_UnparseableNote_RemovesItsFile(t *testing.T) {
	notes := loadNotes(t, map[string]string{"broken-note.md": "no frontmatter\n"})
	node, _ := notes.Lookup("broken-note")

	removal, err := PlanRemove(notes, "broken-note")
	if err != nil {
		t.Fatalf("PlanRemove of an unparseable note: %v", err)
	}
	warnings, err := removal.Apply()

	if _, statErr := os.Stat(node.Path); err != nil || len(warnings) > 0 || !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("Apply = %v, %v, stat %v; want the file removed", warnings, err, statErr)
	}
}

func TestPlanRemove_WithChildren_NamesANote(t *testing.T) {
	notes := loadNotes(t, map[string]string{
		"parent-note.md":            "---\ntitle: Parent\n---\n",
		"parent-note/child-note.md": "---\ntitle: Child\n---\n",
	})

	_, err := PlanRemove(notes, "parent-note")

	var refusal *tree.ChildrenError
	if !errors.As(err, &refusal) || err.Error() != "note parent-note has 1 child note; remove or move them first" {
		t.Errorf("PlanRemove of a parent = %v; want a *tree.ChildrenError naming a note", err)
	}
}
