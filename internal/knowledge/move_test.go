package knowledge

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// loadNotes writes files, keyed by slash path, to a new Knowledge directory and loads it as a
// tree whose Aliases come from each file's frontmatter.
func loadNotes(t *testing.T, files map[string]string) *tree.Tree {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	notes, err := tree.Load(dir, func(path string) ([]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		note, err := Parse(path, data)
		if err != nil {
			return nil, nil
		}
		return note.Aliases, nil
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return notes
}

// readFile returns the content of the file at path, failing the test when it cannot be read.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

// makeSymlink creates a symbolic link at link pointing to target, skipping the test on Windows and
// as root.
func makeSymlink(t *testing.T, target string, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%s): %v", link, err)
	}
}

// unreadSwap is one way of swapping a Note file after Load for one nodefile.ReadNodeFile refuses
// unread: swap replaces the regular Note file at path, whose content is content.
type unreadSwap struct {
	name string
	swap func(t *testing.T, path string, content string)
}

// unreadSwaps are the swaps nodefile.ReadNodeFile refuses: a symlink to a regular file holding the
// same content, a named pipe, and a regular file over 1 MiB written in place.
func unreadSwaps() []unreadSwap {
	return []unreadSwap{
		{name: "symlink", swap: func(t *testing.T, path string, content string) {
			target := filepath.Join(t.TempDir(), filepath.Base(path))
			if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			removeSwapped(t, path)
			makeSymlink(t, target, path)
		}},
		{name: "fifo", swap: func(t *testing.T, path string, _ string) {
			removeSwapped(t, path)
			makeFIFO(t, path)
		}},
		{name: "over 1 MiB", swap: func(t *testing.T, path string, content string) {
			oversized := content + strings.Repeat("x", 1<<20)
			if err := os.WriteFile(path, []byte(oversized), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}},
	}
}

// removeSwapped removes the Note file at path, so a symlink or named pipe can take its place.
func removeSwapped(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

// lstatSwapped returns the os.Lstat of the swapped file at path.
func lstatSwapped(t *testing.T, path string) fs.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	return info
}

// nodePath returns the file path of the Note with id id among the nodes of notes, failing the test
// when notes has no such Note.
func nodePath(t *testing.T, notes *tree.Tree, id string) string {
	t.Helper()
	for _, node := range notes.Nodes() {
		if node.ID == id {
			return node.Path
		}
	}
	t.Fatalf("no Note %s in the tree", id)
	return ""
}

func TestPlanMove_Rename_RecordsTheOldIDAsAnAlias(t *testing.T) {
	cases := []struct {
		name    string
		aliases string
		newID   string
		want    string
	}{
		{"new id", "", "release-steps", "aliases: [release-flow]\n"},
		{"former alias", "aliases: [release-steps, shipping]\n", "release-steps", "aliases: [shipping, release-flow]\n"},
	}

	for _, current := range cases {
		original := "---\ntitle: Release flow\n" + current.aliases + "---\n\nSee release-flow.\n"
		notes := loadNotes(t, map[string]string{"release-flow.md": original})

		move, err := PlanMove(notes, "release-flow", tree.Destination{ID: current.newID})
		if err != nil {
			t.Fatalf("%s: PlanMove: %v", current.name, err)
		}
		if err := move.Apply(); err != nil {
			t.Fatalf("%s: Apply: %v", current.name, err)
		}

		content := readFile(t, move.NewPath)
		if !strings.Contains(content, current.want) || !strings.HasSuffix(content, "\nSee release-flow.\n") {
			t.Errorf("%s: moved note = %q; want %q and the body unchanged", current.name, content, current.want)
		}
		if string(move.Original) != original || string(move.Content) != content {
			t.Errorf("%s: Original = %q, Content = %q", current.name, move.Original, move.Content)
		}
	}
}

func TestPlanMove_ReparentOnly_LeavesTheFileBytes(t *testing.T) {
	original := "---\ntitle:   Release flow\n---\n"
	notes := loadNotes(t, map[string]string{
		"release-flow.md":  original,
		"process-notes.md": "---\ntitle: Process\n---\n",
	})

	move, err := PlanMove(notes, "release-flow", tree.Destination{ID: "release-flow", ParentID: "process-notes"})
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if err := move.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if content := readFile(t, move.NewPath); content != original || string(move.Content) != original {
		t.Errorf("moved note = %q, Content = %q; want the original bytes", content, move.Content)
	}
	if want := filepath.Join("process-notes", "release-flow.md"); !strings.HasSuffix(move.NewPath, want) {
		t.Errorf("NewPath = %q; want it to end in %q", move.NewPath, want)
	}
}

func TestPlanMove_TakenID_FailsBeforeReadingTheNote(t *testing.T) {
	notes := loadNotes(t, map[string]string{
		"broken-note.md": "no frontmatter\n",
		"other-one.md":   "---\ntitle: Other\n---\n",
	})

	_, err := PlanMove(notes, "broken-note", tree.Destination{ID: "other-one"})

	var taken *tree.TakenError
	if !errors.As(err, &taken) || !strings.Contains(err.Error(), "already taken by note other-one") {
		t.Errorf("PlanMove onto a taken id = %v; want a *tree.TakenError naming the note", err)
	}
}

func TestPlanMove_UnparseableNote_FailsWithItsError(t *testing.T) {
	notes := loadNotes(t, map[string]string{"broken-note.md": "no frontmatter\n"})

	_, err := PlanMove(notes, "broken-note", tree.Destination{ID: "mended-note"})

	var failure *Error
	if !errors.As(err, &failure) || !strings.HasSuffix(failure.Path, "broken-note.md") {
		t.Errorf("PlanMove of an unparseable note = %v; want its *Error", err)
	}
}

func TestPlanMove_BelowItsOwnChild_NamesANote(t *testing.T) {
	notes := loadNotes(t, map[string]string{
		"process-notes.md":              "---\ntitle: Process\n---\n",
		"process-notes/release-flow.md": "---\ntitle: Release flow\n---\n",
	})

	_, err := PlanMove(notes, "process-notes", tree.Destination{ID: "process-notes", ParentID: "release-flow"})

	if !errors.Is(err, tree.ErrMoveIntoSubtree) || !strings.Contains(err.Error(), "cannot move a note below") {
		t.Errorf("PlanMove below its own child = %v", err)
	}
}

func TestPlanMove_Rename_NodeFileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt(t *testing.T) {
	for _, tc := range unreadSwaps() {
		t.Run(tc.name, func(t *testing.T) {
			successor := "---\ntitle: Swapped\nsuperseded-by: [old-rule]\n---\n"
			notes := loadNotes(t, map[string]string{
				"old-rule.md": "---\ntitle: Old rule\n---\n",
				"swapped.md":  successor,
			})
			swappedPath := nodePath(t, notes, "swapped")
			tc.swap(t, swappedPath, successor)
			swapped := lstatSwapped(t, swappedPath)

			move, err := PlanMove(notes, "old-rule", tree.Destination{ID: "new-rule"})
			if err != nil {
				t.Fatalf("PlanMove: %v", err)
			}
			if err := move.Apply(); err != nil {
				t.Fatalf("Apply: %v", err)
			}

			if want := []string{swappedPath}; !slices.Equal(move.Unread, want) {
				t.Errorf("Unread = %v, want %v", move.Unread, want)
			}
			if got := lstatSwapped(t, swappedPath); got.Mode() != swapped.Mode() || got.Size() != swapped.Size() {
				t.Errorf("swapped file = %v %d bytes, want it as swapped: %v %d bytes",
					got.Mode(), got.Size(), swapped.Mode(), swapped.Size())
			}
		})
	}
}
