package rawfm_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/rawfm"
)

// issueTestdata holds the issue files the issue package parses, the realistic seeds.
const issueTestdata = "../issue/testdata"

// conflictSeeds are the hand-written inputs FuzzConflictMarkers starts from besides the issue
// testdata: conflict markers and aliases lists where a merge or a hand edit leaves them.
var conflictSeeds = []string{
	"---\ntitle: A Note\naliases: [a, 'b']\n---\n\nBody.\n",
	"---\ntitle: T\naliases:\n",
	"---\r\n<<<<<<< ours\r\naliases: [a]\r\n=======\r\naliases:\r\n  - b\r\n>>>>>>> theirs\r\n---\r\n",
	"---\ntitle: a\x00b\x1b[31m\n---\n\x00\x1b\n=======\n",
	"---\naliases: [a]\naliases: [a, b\n---\n",
	"---\ntitle: T\n---\n```\n---\n<<<<<<< ours\n```\n",
	"---\ntitle: T\n---\n<<<<<<< ours\na\n||||||| base\n=======\nb\n=======\n>>>>>>> theirs\n",
	"---\ntitle: T\n---\n```\n<<<<<<< ours\na\n=======\nb\n>>>>>>> theirs\n",
	"=======\n>>>>>>> theirs\r\r\n<<<<<<< ours",
	"---\r\r\n---\r",
}

// FuzzConflictMarkers scans arbitrary content for conflict markers and aliases: neither scan
// panics, every marker cites a line of the content by its text without one trailing "\r", the
// markers come in line order, and every alias is non-empty and listed once.
func FuzzConflictMarkers(f *testing.F) {
	entries, err := os.ReadDir(issueTestdata)
	if err != nil {
		f.Fatalf("read the seed directory: %v", err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(issueTestdata, entry.Name()))
		if err != nil {
			f.Fatalf("read a seed: %v", err)
		}
		f.Add(data)
	}
	for _, seed := range conflictSeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		lines := strings.Split(string(data), "\n")
		previousLine := 0
		for _, marker := range rawfm.ConflictMarkers(data) {
			if marker.Line < 1 || marker.Line > len(lines) {
				t.Fatalf("marker %+v cites no line of the %d lines", marker, len(lines))
			}
			if want := strings.TrimSuffix(lines[marker.Line-1], "\r"); marker.Text != want {
				t.Errorf("marker %+v: text want %q", marker, want)
			}
			if marker.Line <= previousLine {
				t.Errorf("marker %+v follows a marker at line %d", marker, previousLine)
			}
			previousLine = marker.Line
		}
		aliases := rawfm.Aliases(data)
		for index, alias := range aliases {
			if alias == "" || slices.Contains(aliases[:index], alias) {
				t.Errorf("Aliases = %q: alias %d is empty or listed twice", aliases, index)
			}
		}
	})
}
