package knowledge

import (
	"slices"
	"testing"
)

func TestRewriteRefs_MovedFileAndDirectory_RewritesTheRefsNamingThem(t *testing.T) {
	t.Parallel()
	const (
		oldFile = ".capcom/issues/beta-new.md"
		newFile = ".capcom/issues/epic/beta-final.md"
		oldDir  = ".capcom/issues/beta-new"
		newDir  = ".capcom/issues/epic/beta-final"
	)

	tests := []struct {
		name          string
		refs          []string
		want          []string
		isRewritten   bool
		isSameBacking bool
	}{
		{
			name:        "file",
			refs:        []string{oldFile},
			want:        []string{newFile},
			isRewritten: true,
		},
		{
			name:        "file with a symbol",
			refs:        []string{oldFile + "### Acceptance"},
			want:        []string{newFile + "### Acceptance"},
			isRewritten: true,
		},
		{
			name:        "file written uncleaned",
			refs:        []string{"./.capcom/issues/beta-new.md"},
			want:        []string{newFile},
			isRewritten: true,
		},
		{
			name:        "children directory",
			refs:        []string{oldDir, oldDir + "/"},
			want:        []string{newDir, newDir},
			isRewritten: true,
		},
		{
			name:        "below the children directory",
			refs:        []string{oldDir + "/child.md#Body text", oldDir + "/child/grandchild.md"},
			want:        []string{newDir + "/child.md#Body text", newDir + "/child/grandchild.md"},
			isRewritten: true,
		},
		{
			name:        "unrelated beside rewritten",
			refs:        []string{"internal/cli/mv.go#movedPath", oldFile},
			want:        []string{"internal/cli/mv.go#movedPath", newFile},
			isRewritten: true,
		},
		{
			name:          "unrelated",
			refs:          []string{"internal/cli/mv.go", ".capcom/issues/beta-newer.md", oldDir + "-old/child.md"},
			want:          []string{"internal/cli/mv.go", ".capcom/issues/beta-newer.md", oldDir + "-old/child.md"},
			isSameBacking: true,
		},
		{
			name:          "malformed",
			refs:          []string{"#symbol", "../" + oldFile},
			want:          []string{"#symbol", "../" + oldFile},
			isSameBacking: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			original := slices.Clone(test.refs)

			got, isRewritten := RewriteRefs(test.refs, oldFile, newFile, oldDir, newDir)

			if !slices.Equal(got, test.want) || isRewritten != test.isRewritten {
				t.Errorf("RewriteRefs(%v) = %v, %t; want %v, %t", test.refs, got, isRewritten, test.want, test.isRewritten)
			}
			if !slices.Equal(test.refs, original) {
				t.Errorf("RewriteRefs changed its argument to %v, want %v", test.refs, original)
			}
			if test.isSameBacking && len(got) > 0 && &got[0] != &test.refs[0] {
				t.Errorf("RewriteRefs copied refs it left unchanged")
			}
		})
	}
}
