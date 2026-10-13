package tree

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// mappedFilePattern matches a file name of the doc.go file map, written as `name.go`.
var mappedFilePattern = regexp.MustCompile("`([A-Za-z0-9_]+\\.go)`")

// TestDocFileMap_MatchesDirectory reads the package directory, so it relies on the working
// directory being the package's own and never runs in parallel with the tests that change it.
func TestDocFileMap_MatchesDirectory(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	docText, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("read doc.go: %v", err)
	}
	var mapped []string
	for _, match := range mappedFilePattern.FindAllStringSubmatch(string(docText), -1) {
		mapped = append(mapped, match[1])
	}

	for _, name := range files {
		if !slices.Contains(mapped, name) {
			t.Errorf("doc.go: file map is missing %s", name)
		}
	}
	for _, name := range mapped {
		if !slices.Contains(files, name) {
			t.Errorf("doc.go: file map names %s, which does not exist", name)
		}
	}
}
