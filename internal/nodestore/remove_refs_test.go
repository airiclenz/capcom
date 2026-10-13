package nodestore

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// unparseableIssueWithAlias is the content of an issue file that does not parse, from which the
// alias old-name can still be recovered.
const unparseableIssueWithAlias = "---\ntitle: [unclosed\naliases: [old-name]\n---\n"

// assertGone fails the test unless the slash path name below dir no longer exists.
func assertGone(t *testing.T, dir string, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stat %s = %v, want it removed", name, err)
	}
}

func TestRemove_DropDeps_DropsDepsAndRelatedNamingTheIdOrAnAlias(t *testing.T) {
	t.Parallel()
	issues, dir := newRefIssues(t, map[string]string{
		"gone-issue.md":   refIssue("aliases: [old-name]\n", ""),
		"by-id.md":        refIssue("deps: [gone-issue, kept-issue]\n", ""),
		"by-alias.md":     refIssue("deps: [old-name]\nrelated: [gone-issue]\n", ""),
		"related-only.md": refIssue("related: [old-name, kept-issue]\n", ""),
		"kept-issue.md":   refIssue("", ""),
	})

	removed, _, err := issues.Remove("gone-issue", "", true)

	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !slices.Equal(removed.Dropped[RefDeps], []string{"by-alias", "by-id"}) ||
		!slices.Equal(removed.Dropped[RefRelated], []string{"by-alias", "related-only"}) || len(removed.Dropped) != 2 {
		t.Errorf("Dropped = %v; want the deps and related holders in tree order", removed.Dropped)
	}
	wants := map[string]string{
		"by-id.md":        refIssue("deps: [kept-issue]\n", ""),
		"by-alias.md":     refIssue("", ""),
		"related-only.md": refIssue("related: [kept-issue]\n", ""),
	}
	for name, want := range wants {
		if got := readFile(t, filepath.Join(dir, name)); got != want {
			t.Errorf("%s =\n%s\nwant\n%s", name, got, want)
		}
	}
	assertGone(t, dir, "gone-issue.md")
}

func TestRemove_Dependents_RefusedInPathOrderAndNothingChanges(t *testing.T) {
	t.Parallel()
	issues, dir := newRefIssues(t, map[string]string{
		"gone-issue.md":                refIssue("deps: [gone-issue]\n", ""),
		"zeta-parent.md":               refIssue("", ""),
		"alpha-parent.md":              refIssue("", ""),
		"alpha-parent/zz-dependent.md": refIssue("deps: [gone-issue]\n", ""),
		"zeta-parent/aa-dependent.md":  refIssue("deps: [gone-issue]\n", ""),
		"related-holder.md":            refIssue("related: [gone-issue]\n", ""),
	})
	before := treeContent(t, dir)

	_, _, err := issues.Remove("gone-issue", "", false)

	var failure *DependentsError
	if !errors.As(err, &failure) || failure.Path != ".capcom/issues/gone-issue.md" || failure.ID != "gone-issue" ||
		!slices.Equal(failure.Dependents, []string{"zz-dependent", "aa-dependent"}) ||
		failure.Message != "issue gone-issue is a dependency of zz-dependent, aa-dependent" {
		t.Errorf("err = %#v, want a *DependentsError naming zz-dependent, aa-dependent in path order", err)
	}
	if after := treeContent(t, dir); !maps.Equal(after, before) {
		t.Errorf("tree changed by the refusal:\n%v\nwant\n%v", after, before)
	}
}

func TestRemove_StaleIfMatch_ConflictsBeforeTheDependentsRefusal(t *testing.T) {
	t.Parallel()
	gone := refIssue("", "")
	issues, dir := newRefIssues(t, map[string]string{
		"gone-issue.md":   gone,
		"blocked-work.md": refIssue("deps: [gone-issue]\n", ""),
	})
	before := treeContent(t, dir)

	_, _, err := issues.Remove("gone-issue", "0000000000000000", false)

	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != ".capcom/issues/gone-issue.md" ||
		conflict.ETag != nodefile.ETag([]byte(gone)) {
		t.Errorf("err = %v, want a *ConflictError at gone-issue.md carrying its etag", err)
	}
	if after := treeContent(t, dir); !maps.Equal(after, before) {
		t.Errorf("tree changed by the refusal:\n%v\nwant\n%v", after, before)
	}
}

func TestRemove_UnparseableIssueFiles_RemovesOwnAndListsOthersAsUnread(t *testing.T) {
	t.Parallel()
	brokenOther := "---\ntitle: [unclosed\ndeps: [gone-issue]\n---\n"
	issues, dir := newRefIssues(t, map[string]string{
		"gone-issue.md":   unparseableIssueWithAlias,
		"broken-other.md": brokenOther,
		"blocked-work.md": refIssue("deps: [old-name]\n", ""),
	})

	removed, _, err := issues.Remove("gone-issue", "", true)

	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if want := []string{".capcom/issues/broken-other.md"}; !slices.Equal(removed.Unread, want) {
		t.Errorf("Unread = %v, want %v", removed.Unread, want)
	}
	if !slices.Equal(removed.Dropped[RefDeps], []string{"blocked-work"}) {
		t.Errorf("Dropped = %v, want the dependent naming the recovered alias", removed.Dropped)
	}
	if got := readFile(t, filepath.Join(dir, "blocked-work.md")); got != refIssue("", "") {
		t.Errorf("blocked-work.md =\n%s\nwant its deps entry dropped", got)
	}
	if got := readFile(t, filepath.Join(dir, "broken-other.md")); got != brokenOther {
		t.Errorf("broken-other.md = %q, want it left as it was", got)
	}
	assertGone(t, dir, "gone-issue.md")
}

func TestRemove_Note_DropsOtherNotesSuccessorsNamingTheIdOrAnAlias(t *testing.T) {
	t.Parallel()
	bystander := refNote("Bystander", "superseded-by: [other]\n", "")
	notes, dir := newRefNotes(t, map[string]string{
		"old-rule.md":  refNote("Old rule", "aliases: [older-rule]\n", ""),
		"by-id.md":     refNote("By id", "superseded-by: [old-rule, other]\n", ""),
		"by-alias.md":  refNote("By alias", "superseded-by: [older-rule]\n", ""),
		"bystander.md": bystander,
	})

	removed, _, err := notes.Remove("old-rule", "", false)

	if err != nil || removed.ID != "old-rule" || removed.Path != ".capcom/knowledge/old-rule.md" {
		t.Fatalf("Remove = %+v, %v; want old-rule removed", removed, err)
	}
	if want := map[string][]string{RefSupersededBy: {"by-alias", "by-id"}}; !maps.EqualFunc(removed.Dropped, want, slices.Equal) {
		t.Errorf("Dropped = %v, want %v", removed.Dropped, want)
	}
	wants := map[string]string{
		"by-id.md":     refNote("By id", "superseded-by: [other]\n", ""),
		"by-alias.md":  refNote("By alias", "", ""),
		"bystander.md": bystander,
	}
	for name, want := range wants {
		if got := readFile(t, filepath.Join(dir, name)); got != want {
			t.Errorf("%s =\n%s\nwant\n%s", name, got, want)
		}
	}
	assertGone(t, dir, "old-rule.md")
}

func TestRemove_Note_UnparseableOtherNoteJoinsUnread(t *testing.T) {
	t.Parallel()
	broken := "no frontmatter\n"
	notes, dir := newRefNotes(t, map[string]string{
		"old-rule.md": refNote("Old rule", "", ""),
		"broken.md":   broken,
	})

	removed, _, err := notes.Remove("old-rule", "", false)

	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if want := []string{".capcom/knowledge/broken.md"}; !slices.Equal(removed.Unread, want) {
		t.Errorf("Unread = %v, want %v", removed.Unread, want)
	}
	if got := readFile(t, filepath.Join(dir, "broken.md")); got != broken {
		t.Errorf("broken.md = %q, want it left as it was", got)
	}
}

func TestRemove_UnparseableNote_RemovesItsFile(t *testing.T) {
	t.Parallel()
	notes, dir := newRefNotes(t, map[string]string{"broken-note.md": "no frontmatter\n"})

	removed, _, err := notes.Remove("broken-note", "", false)

	if err != nil || removed.Path != ".capcom/knowledge/broken-note.md" || len(removed.LeftInPlace) != 0 {
		t.Fatalf("Remove = %+v, %v; want broken-note removed with nothing left in place", removed, err)
	}
	assertGone(t, dir, "broken-note.md")
}
