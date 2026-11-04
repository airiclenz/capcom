package issue_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
)

// fuzzPath is the path the fuzz targets hand Parse.
const fuzzPath = "x.md"

// parseSeeds are the hand-written inputs FuzzParseIssue starts from besides the testdata files:
// the shapes of issue file whose canonical form is easiest to get wrong.
var parseSeeds = []string{
	"---\ntitle: T\r\r\nstatus: open\r\n---\r\nbody\r\r\n",
	"---\ncreated: 0000-01-01T00:00:00Z\nclosed: 9999-12-31T23:59:59Z\n---\n",
	"---\ncreated: 0000-01-01T00:00:00+01:00\n---\n",
	"---\ncreated: 2026-10-11T09:30:00.999+02:00\n---\n",
	"---\ntitle: \"a: b # c\"\nlabels: ['x y', \"z\"]\nreason: '  padded  '\n---\n",
	"---\ntitle: a\x00b\x1b[31m\n---\n\x00\x1b\n",
	"---\nstanding: false\npriority:\n---\n",
	"---\n---\n",
}

// FuzzParseIssue parses arbitrary content as an issue: Parse never panics, and the canonical form
// of whatever it reads is a fixed point, read back by Parse and written again byte for byte.
func FuzzParseIssue(f *testing.F) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil {
		f.Fatalf("list the seed files: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatalf("read a seed: %v", err)
		}
		f.Add(data)
	}
	for _, seed := range parseSeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		parsed, err := issue.Parse(fuzzPath, data)
		if err != nil {
			return
		}
		canonical := parsed.Serialize()
		reparsed, err := issue.Parse(fuzzPath, canonical)
		if err != nil {
			t.Fatalf("Parse of the canonical form %q: %v", canonical, err)
		}
		if again := reparsed.Serialize(); !bytes.Equal(again, canonical) {
			t.Errorf("canonical form is not a fixed point:\n--- first ---\n%q\n--- second ---\n%q",
				canonical, again)
		}
	})
}

// FuzzMergeFile merges three arbitrary versions of an issue file: MergeFile never panics and,
// with a text merge that never fails, never returns an error.
func FuzzMergeFile(f *testing.F) {
	base := issueFile("T", entryOctFirst)
	both := issueFile("T", entryOctFirst, entryOctTenth)
	seeds := [][3]string{
		{base, both, issueFile("T", entryOctFirst, entryOctFifth)},
		{base, issueFile("Renamed", entryOctFirst), issueFile("T", entryOctFirst, entryOctLast)},
		{base, crlf(both), issueFile("T", entryOctFirst, entryOctLast)},
		{base, issueFileWithBody("Body names "+logMergePlaceholder+".", entryOctFirst), both},
		{fencedLogExampleFile(entryOctFirst), fencedLogExampleFile(entryOctFirst, entryOctTenth), base},
		{"", "", ""},
		{base, "---\ntitle: T\n---\n", both},
	}
	for _, seed := range seeds {
		f.Add([]byte(seed[0]), []byte(seed[1]), []byte(seed[2]))
	}

	f.Fuzz(func(t *testing.T, base []byte, ours []byte, theirs []byte) {
		if _, _, err := issue.MergeFile(base, ours, theirs, fakeTextMerge); err != nil {
			t.Errorf("MergeFile: %v", err)
		}
	})
}
