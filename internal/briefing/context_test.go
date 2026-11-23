package briefing

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/index"
)

// contextBody is the body of the Issue the ApplyingNotes tests select for: it mentions one file
// and one directory that refs of the contextNotes name.
const contextBody = "Change internal/auth/token.go and the files under internal/store/ here.\n"

// contextNotes are two Notes whose refs name paths contextBody mentions, one whose ref only shares
// a prefix, a retired one whose ref matches, and one a search on "Rotation cadence" finds.
var contextNotes = map[string]string{
	"token-rules.md": "---\ntitle: Token rules\nrefs: [internal/auth/token.go#Sign]\n---\n\nSign with the shared key.\n",
	"store-rules.md": "---\ntitle: Store rules\nrefs: [internal/store]\n---\n\nWrites go through ops.\n",
	"prefix-only.md": "---\ntitle: Prefix only\nrefs: [internal/auth/token.go.bak]\n---\n\nUnrelated.\n",
	"old-token.md":   "---\ntitle: Old token\nretired: true\nrefs: [internal/auth/token.go]\n---\n\nGone.\n",
	"cadence.md":     "---\ntitle: Rotation cadence notes\n---\n\nEvery ninety days.\n",
}

// indexNoteFiles writes files, Note files by their path under the Knowledge tree, to a temp store
// with no Issues and opens a refreshed index on it.
func indexNoteFiles(t *testing.T, files map[string]string) *index.Index {
	t.Helper()
	x, _, root := indexIssueFiles(t, nil)
	knowledge := filepath.Join(root, ".capcom", "knowledge")
	for name, content := range files {
		path := filepath.Join(knowledge, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	if _, err := x.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return x
}

// applyingPairs flattens applying to one `<id> <ref>` string per Note, the ref empty for a Note
// the title search found.
func applyingPairs(applying []ApplyingNote) []string {
	pairs := make([]string, 0, len(applying))
	for _, note := range applying {
		pairs = append(pairs, note.Entry.ID+" "+note.Ref)
	}
	return pairs
}

func TestApplyingNotes_RefsThenTitleHits_InPathOrderWithoutRetiredOrPrefixOnly(t *testing.T) {
	t.Parallel()
	x := indexNoteFiles(t, contextNotes)
	want := []string{"store-rules internal/store", "token-rules internal/auth/token.go#Sign", "cadence "}

	applying, err := ApplyingNotes(x, "Rotation cadence", contextBody, 8)

	if err != nil || !slices.Equal(applyingPairs(applying), want) {
		t.Errorf("ApplyingNotes = %q, %v; want %q", applyingPairs(applying), err, want)
	}
}

func TestApplyingNotes_Limit_KeepsTheFirstNotes(t *testing.T) {
	t.Parallel()
	x := indexNoteFiles(t, contextNotes)
	want := []string{"store-rules internal/store", "token-rules internal/auth/token.go#Sign"}

	applying, err := ApplyingNotes(x, "Rotation cadence", contextBody, 2)

	if err != nil || !slices.Equal(applyingPairs(applying), want) {
		t.Errorf("ApplyingNotes = %q, %v; want %q", applyingPairs(applying), err, want)
	}
}

func TestApplyingNotes_NoteFoundByRefAndTitle_ListedOnceWithItsRef(t *testing.T) {
	t.Parallel()
	x := indexNoteFiles(t, contextNotes)
	want := []string{"store-rules internal/store", "token-rules internal/auth/token.go#Sign"}

	applying, err := ApplyingNotes(x, "Token rules", contextBody, 8)

	if err != nil || !slices.Equal(applyingPairs(applying), want) {
		t.Errorf("ApplyingNotes = %q, %v; want %q", applyingPairs(applying), err, want)
	}
}

func TestApplyingNotes_NoNoteHoldsEveryTitleWord_ListsTheAnyWordHits(t *testing.T) {
	t.Parallel()
	x := indexNoteFiles(t, contextNotes)
	want := []string{"cadence "}

	applying, err := ApplyingNotes(x, "cadence submarine", "No path here.\n", 8)

	if err != nil || !slices.Equal(applyingPairs(applying), want) {
		t.Errorf("ApplyingNotes = %q, %v; want %q", applyingPairs(applying), err, want)
	}
}

func TestMentionsPath_WholePathOrDirectory_Counts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		path string
		want bool
	}{
		{"edit internal/auth/token.go now", "internal/auth/token.go", true},
		{"edit internal/auth/token.go.", "internal/auth/token.go", true},
		{"(internal/auth/token.go)", "internal/auth/token.go", true},
		{"under internal/store/ops.go", "internal/store", true},
		{"under internal/store/", "internal/store/", true},
		{"edit internal/auth/token.go.bak", "internal/auth/token.go", false},
		{"edit vendor/internal/store/x.go", "internal/store", false},
		{"edit internal/storefront", "internal/store", false},
		{"anything", "", false},
	}

	for _, current := range cases {
		got := mentionsPath(current.text, current.path)

		if got != current.want {
			t.Errorf("mentionsPath(%q, %q) = %v, want %v", current.text, current.path, got, current.want)
		}
	}
}
