package issue_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
)

// removeFile is the content of a canonical issue file with the given deps, related and aliases
// (flow lists without brackets, "" for none).
func removeFile(deps string, related string, aliases string) string {
	var front strings.Builder
	front.WriteString("---\ntitle: Removed\nstatus: open\ntype: task\npriority: 2\n")
	if deps != "" {
		front.WriteString("deps: [" + deps + "]\n")
	}
	if related != "" {
		front.WriteString("related: [" + related + "]\n")
	}
	if aliases != "" {
		front.WriteString("aliases: [" + aliases + "]\n")
	}
	front.WriteString("created: 2026-10-11T09:00:00Z\n---\n")
	return front.String()
}

// unparseableFile is the content of an issue file that does not parse, with the alias old-name.
const unparseableFile = "---\ntitle: [unclosed\naliases: [old-name]\n---\n"

// readRemoveFile returns the content of the file at the slash path name below dir.
func readRemoveFile(t *testing.T, dir string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	return string(data)
}

// isRemovePathThere reports whether the slash path name below dir exists.
func isRemovePathThere(dir string, name string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil
}

func TestPlanRemove_DropDeps_DropsDepsAndRelatedNamingIdOrAlias(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"gone-issue.md":   removeFile("", "", "old-name"),
		"by-id.md":        removeFile("gone-issue, kept-issue", "", ""),
		"by-alias.md":     removeFile("old-name", "gone-issue", ""),
		"related-only.md": removeFile("", "old-name, kept-issue", ""),
		"kept-issue.md":   removeFile("", "", ""),
	})

	removal, err := issue.PlanRemove(loadMoveTree(t, dir), "gone-issue", issue.RemoveOptions{ShouldDropDeps: true})
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if _, err := removal.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !slices.Equal(removal.DepsDropped, []string{"by-alias", "by-id"}) ||
		!slices.Equal(removal.RelatedDropped, []string{"by-alias", "related-only"}) {
		t.Errorf("DepsDropped = %v, RelatedDropped = %v", removal.DepsDropped, removal.RelatedDropped)
	}
	wants := map[string]string{
		"by-id.md":        removeFile("kept-issue", "", ""),
		"by-alias.md":     removeFile("", "", ""),
		"related-only.md": removeFile("", "kept-issue", ""),
	}
	for name, want := range wants {
		if got := readRemoveFile(t, dir, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if isRemovePathThere(dir, "gone-issue.md") {
		t.Errorf("gone-issue.md was not removed")
	}
}

func TestPlanRemove_Dependents_RefusedInPathOrderAndNothingChanges(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"gone-issue.md":                removeFile("gone-issue", "", ""),
		"zeta-parent.md":               removeFile("", "", ""),
		"alpha-parent.md":              removeFile("", "", ""),
		"alpha-parent/zz-dependent.md": removeFile("gone-issue", "", ""),
		"zeta-parent/aa-dependent.md":  removeFile("gone-issue", "", ""),
		"related-holder.md":            removeFile("", "gone-issue", ""),
	})
	holder := readRemoveFile(t, dir, "related-holder.md")

	_, err := issue.PlanRemove(loadMoveTree(t, dir), "gone-issue", issue.RemoveOptions{})

	var dependents *issue.DependentsError
	if !errors.As(err, &dependents) || !slices.Equal(dependents.Dependents, []string{"zz-dependent", "aa-dependent"}) {
		t.Fatalf("PlanRemove = %v, want a DependentsError naming zz-dependent, aa-dependent", err)
	}
	if !isRemovePathThere(dir, "gone-issue.md") || readRemoveFile(t, dir, "related-holder.md") != holder {
		t.Errorf("a refused removal changed files")
	}
}

func TestPlanRemove_StaleIfMatch_ComesBeforeTheDependentsRefusal(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"gone-issue.md":   removeFile("", "", ""),
		"blocked-work.md": removeFile("gone-issue", "", ""),
	})

	_, err := issue.PlanRemove(loadMoveTree(t, dir), "gone-issue", issue.RemoveOptions{IfMatch: "0000000000000000"})

	var mismatch *issue.ETagMismatchError
	if !errors.As(err, &mismatch) || mismatch.ETag != edit.ETag([]byte(removeFile("", "", ""))) {
		t.Errorf("PlanRemove = %v, want an ETagMismatchError", err)
	}
}

func TestPlanRemove_UnparseableFiles_RemovesOwnAndListsOthersAsUnread(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"gone-issue.md":   unparseableFile,
		"broken-other.md": "---\ntitle: [unclosed\ndeps: [gone-issue]\n---\n",
		"blocked-work.md": removeFile("old-name", "", ""),
	})

	removal, err := issue.PlanRemove(loadMoveTree(t, dir), "gone-issue", issue.RemoveOptions{ShouldDropDeps: true})
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if _, err := removal.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if want := []string{filepath.Join(dir, "broken-other.md")}; !slices.Equal(removal.Unread, want) {
		t.Errorf("Unread = %v, want %v", removal.Unread, want)
	}
	if !slices.Equal(removal.DepsDropped, []string{"blocked-work"}) {
		t.Errorf("DepsDropped = %v, want the dependent naming the recovered alias", removal.DepsDropped)
	}
	if isRemovePathThere(dir, "gone-issue.md") {
		t.Errorf("gone-issue.md was not removed")
	}
}

func TestPlanRemove_ApplyFails_RestoresRewritesChildrenDirAndFile(t *testing.T) {
	original := strings.ReplaceAll(removeFile("gone-issue", "", ""), "\n", "\r\n")
	dir := newMoveTree(t, map[string]string{
		"gone-issue.md":   removeFile("", "", ""),
		"blocked-work.md": original,
	})
	if err := os.Mkdir(filepath.Join(dir, "gone-issue"), 0o750); err != nil {
		t.Fatal(err)
	}
	dependentPath := filepath.Join(dir, "blocked-work.md")
	if err := os.Chmod(dependentPath, 0o600); err != nil {
		t.Fatal(err)
	}
	removal, err := issue.PlanRemove(loadMoveTree(t, dir), "gone-issue", issue.RemoveOptions{ShouldDropDeps: true})
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	errInjected := errors.New("injected failure")
	removal.AddStep(func() error { return errInjected }, func() error { return nil })

	if _, err := removal.Apply(); !errors.Is(err, errInjected) {
		t.Fatalf("Apply = %v, want the injected failure", err)
	}

	if got := readRemoveFile(t, dir, "blocked-work.md"); got != original {
		t.Errorf("blocked-work.md = %q, want %q", got, original)
	}
	if info, err := os.Stat(dependentPath); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Errorf("blocked-work.md stat = %v, %v; want mode 0600 kept", info, err)
	}
	if got := readRemoveFile(t, dir, "gone-issue.md"); got != removeFile("", "", "") {
		t.Errorf("gone-issue.md = %q, want it back", got)
	}
	if info, err := os.Stat(filepath.Join(dir, "gone-issue")); err != nil || !info.IsDir() {
		t.Errorf("the children directory was not restored: %v", err)
	}
}
