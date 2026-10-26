package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// newIgnoringRepo creates a repository that ignores *.log and holds one tracked file, tracked.log,
// that matches the pattern anyway, and returns its root.
func newIgnoringRepo(t *testing.T) string {
	t.Helper()
	root := newRepo(t)
	for name, content := range map[string]string{".gitignore": "*.log\n", "tracked.log": "kept\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	git(t, root, "add", "--force", ".gitignore", "tracked.log")
	git(t, root, "commit", "-q", "-m", "ignore logs")
	return root
}

func TestIgnoredPaths_Paths_ReturnsOnlyIgnoredOnes(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "ignored file", paths: []string{"build.log"}, want: []string{"build.log"}},
		{name: "unignored file", paths: []string{"notes.md"}, want: nil},
		{name: "tracked file matching a pattern", paths: []string{"tracked.log"}, want: nil},
		{name: "no paths", paths: nil, want: nil},
		{name: "none ignored", paths: []string{"notes.md", "tracked.log"}, want: nil},
		{
			name:  "mixed keeps caller order and spelling",
			paths: []string{"sub/b.log", "notes.md", filepath.Join("sub", "deeper", "a.log"), "tracked.log"},
			want:  []string{"sub/b.log", filepath.Join("sub", "deeper", "a.log")},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newIgnoringRepo(t)

			ignored, err := IgnoredPaths(root, testCase.paths)

			if err != nil {
				t.Fatalf("IgnoredPaths: %v", err)
			}
			if !slices.Equal(ignored, testCase.want) {
				t.Errorf("IgnoredPaths(%q) = %q, want %q", testCase.paths, ignored, testCase.want)
			}
		})
	}
}

func TestIgnoredPaths_AbsolutePathInsideTree_IsReported(t *testing.T) {
	root := newIgnoringRepo(t)
	path := filepath.Join(root, "build.log")

	ignored, err := IgnoredPaths(root, []string{path})

	if err != nil {
		t.Fatalf("IgnoredPaths: %v", err)
	}
	if !slices.Equal(ignored, []string{path}) {
		t.Errorf("IgnoredPaths = %q, want [%q]", ignored, path)
	}
}

func TestIgnoredPaths_PathOutsideTree_FailsWithGitError(t *testing.T) {
	root := newIgnoringRepo(t)

	ignored, err := IgnoredPaths(root, []string{"../outside.log"})

	var gitFailure *Error
	if !errors.As(err, &gitFailure) || ignored != nil {
		t.Fatalf("IgnoredPaths = %q, err %v; want a git *Error and no paths", ignored, err)
	}
}
