package knowledge

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// fuzzPath is the path FuzzParseNote hands Parse.
const fuzzPath = "x.md"

// committedKnowledgeDir is this repository's Knowledge tree, relative to this package's directory.
const committedKnowledgeDir = "../../.capcom/knowledge"

// noteSeeds are the hand-written inputs FuzzParseNote starts from besides the committed Notes:
// every frontmatter key, CRLF line endings and a git conflict marker.
var noteSeeds = []string{
	"---\ntitle: Old rule\nlabels: [rule, background]\nrefs: [internal/a.go, 'internal/b.go#func B']\n" +
		"retired: true\nsuperseded-by: [new-rule]\naliases: [older-rule]\n---\nBody text.\n",
	"---\r\ntitle: T\r\nrefs:\r\n  - a.go\r\n  - b.go\r\nretired: false\r\n---\r\nbody\r\r\n",
	"---\ntitle: T\n<<<<<<< ours\nlabels: [a]\n=======\nlabels: [b]\n>>>>>>> theirs\n---\nbody\n",
	"---\ntitle: T\n---\n<<<<<<< ours\nmine\n=======\ntheirs\n>>>>>>> theirs\n",
	"---\ntitle: \"a: b # c\"\nsuperseded-by: []\naliases: ['x y', \"z\"]\nretired:\n---\n",
	"---\ntitle: a\x00b\x1b[31m\n---\n\x00\x1b\n",
	"---\n---\n",
}

// FuzzParseNote parses arbitrary content as a Note: Parse never panics, and the canonical form of
// whatever it reads is a fixed point, read back by Parse and written again byte for byte.
func FuzzParseNote(f *testing.F) {
	for _, data := range committedNotes(f) {
		f.Add(data)
	}
	for _, seed := range noteSeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		parsed, err := Parse(fuzzPath, data)
		if err != nil {
			return
		}
		canonical := parsed.Serialize()
		reparsed, err := Parse(fuzzPath, canonical)
		if err != nil {
			t.Fatalf("Parse of the canonical form %q: %v", canonical, err)
		}
		if again := reparsed.Serialize(); !bytes.Equal(again, canonical) {
			t.Errorf("canonical form is not a fixed point:\n--- first ---\n%q\n--- second ---\n%q",
				canonical, again)
		}
	})
}

// committedNotes returns the content of every Note file in this repository's Knowledge tree, or
// none when the tree is absent.
func committedNotes(f *testing.F) [][]byte {
	f.Helper()
	var notes [][]byte
	err := filepath.WalkDir(committedKnowledgeDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		notes = append(notes, data)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		f.Fatalf("read the committed Notes: %v", err)
	}
	return notes
}
