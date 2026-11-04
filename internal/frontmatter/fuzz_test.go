package frontmatter_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/frontmatter"
)

// fuzzPath is the path FuzzDecode hands Decode, which every error must name.
const fuzzPath = "x.md"

// issueTestdata holds the issue files the issue package parses, the realistic seeds.
const issueTestdata = "../issue/testdata"

// crlf is a CRLF line ending, which the text Split hands back never holds.
const crlf = "\r\n"

// decodeSeeds are the hand-written inputs FuzzDecode starts from besides the issue testdata: the
// shapes of file the parser has to survive.
var decodeSeeds = []string{
	"---\ntitle: A Note\nlabels: [a, b]\nrefs: [internal/x.go]\n---\n\nBody.\n",
	"---\ntitle: T\n",
	"---\r\ntitle: T\r\nlabels: [a]\r\n---\r\nbody\r\n",
	"---\r\ntitle: T\r\r\n---\r\nbody\r\r\nlone\r cr\r\r\r\n",
	"---\ntitle: a\x00b\x1b[31m\n---\n\x00\x1b\n",
	"---\ntitle: A\ntitle: B\n---\n",
	"---\ntitle: T\n---\n```\n---\n```\n",
	"---\n<<<<<<< ours\ntitle: A\n=======\ntitle: B\n>>>>>>> theirs\n---\n",
	"---\ntitle: T\n---\n<<<<<<< ours\na\n=======\nb\n>>>>>>> theirs\n",
	"---\ntitle: T\n---\n```\n<<<<<<< ours\na\n=======\nb\n>>>>>>> theirs\n",
	"---\nlabels:\n  - a\n  - [b]\npriority: x\n---\n",
	"---\ntitle: T\n---\n---\nextra: key\n",
}

// FuzzDecode decodes arbitrary content as an issue and as a Note: Decode never panics, every
// error names the file, and the text Split hands back holds no CRLF line ending.
func FuzzDecode(f *testing.F) {
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
	for _, seed := range decodeSeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, kind := range []frontmatter.Kind{issueKind, noteKind} {
			document, err := frontmatter.Decode(fuzzPath, data, kind, readField)
			if err != nil && !strings.Contains(err.Error(), fuzzPath) {
				t.Errorf("Decode as %s: error %q does not name %s", kind.Noun, err, fuzzPath)
			}
			if strings.Contains(document.Body, crlf) {
				t.Errorf("Decode as %s: body %q holds a CRLF line ending", kind.Noun, document.Body)
			}
		}
		front, body, err := frontmatter.Split(fuzzPath, data, noteKind.Noun)
		if err != nil {
			return
		}
		if strings.Contains(front, crlf) || strings.Contains(body, crlf) {
			t.Errorf("Split = %q, %q; want no CRLF line ending", front, body)
		}
	})
}

// readField reads a field the way a kind's callback does, priority as a whole number, a list key
// as a list and every other key as text, and calls Text and List on it whatever the key, so each
// helper, IsEmpty through them, meets every shape of value.
func readField(field frontmatter.Field) error {
	text, isScalar := field.Scalar()
	_, textErr := field.Text()
	_, _, listErr := field.List()
	switch {
	case field.Key == "priority":
		if _, err := strconv.Atoi(text); !isScalar || err != nil {
			return field.Errorf("priority: expected a whole number from 0 to 4")
		}
		return nil
	case slices.Contains(listKeys, field.Key):
		return listErr
	default:
		return textErr
	}
}
