package nodestore

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// readerFixture is one kind's node files for a Reader test: the directory of its tree relative to
// the working tree root, a clean file, the text of one that parses but fails validation, and the
// Aliases the clean file records.
type readerFixture struct {
	dir     string
	content string
	invalid string
	aliases []string
}

// readCase is one node file a Reader reads, and what the Reading must hold.
type readCase struct {
	name string
	// write creates the file at path.
	write func(t *testing.T, path string)
	// codes are the codes of the Reading's findings, in order.
	codes      []diagnose.Kind
	isRead     bool
	isParsed   bool
	hasAliases bool
}

// writeContent returns a readCase writer that writes content.
func writeContent(content string) func(t *testing.T, path string) {
	return func(t *testing.T, path string) {
		t.Helper()
		writeNodeFile(t, filepath.Dir(path), filepath.Base(path), content)
	}
}

// readCases returns the read cases of fixture's kind, keyed by the file's name.
func readCases(fixture readerFixture) []readCase {
	tooLarge := fixture.content + strings.Repeat("x", nodefile.MaxNodeFileSize)
	return []readCase{
		{
			name: "clean.md", write: writeContent(fixture.content),
			isRead: true, isParsed: true, hasAliases: true,
		},
		{
			name: "broken.md", write: writeContent(unparseable(fixture.content)),
			codes: []diagnose.Kind{diagnose.Frontmatter}, isRead: true, hasAliases: true,
		},
		{
			name: "invalid.md", write: writeContent(fixture.invalid),
			codes: []diagnose.Kind{diagnose.Field}, isRead: true, isParsed: true, hasAliases: true,
		},
		{
			// The file's id repeats its own alias.
			name: fixture.aliases[0] + ".md", write: writeContent(fixture.content),
			codes: []diagnose.Kind{diagnose.RepeatedAlias}, isRead: true, isParsed: true, hasAliases: true,
		},
		{
			name: "directory.md",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			codes: []diagnose.Kind{diagnose.NotRegular},
		},
		{
			name: "large.md", write: writeContent(tooLarge),
			codes: []diagnose.Kind{diagnose.NotRegular},
		},
	}
}

// assertReader reads every case of fixture through reader in a fresh working tree and checks its
// Reading: the codes of its findings, each naming the worktree-relative path; its verdicts; its
// Aliases; and the etag of the bytes read.
func assertReader[T any](t *testing.T, reader Reader[T], fixture readerFixture) {
	t.Helper()
	root := t.TempDir()
	for _, testCase := range readCases(fixture) {
		t.Run(testCase.name, func(t *testing.T) {
			relPath := fixture.dir + "/" + testCase.name
			path := filepath.Join(root, filepath.FromSlash(relPath))
			testCase.write(t, path)

			reading, err := reader.Read(path, relPath)

			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			var codes []diagnose.Kind
			for _, finding := range reading.Findings() {
				codes = append(codes, finding.Code)
				if finding.Path != relPath {
					t.Errorf("finding %v names %q, want %q", finding, finding.Path, relPath)
				}
			}
			if !slices.Equal(codes, testCase.codes) {
				t.Errorf("finding codes = %v, want %v", codes, testCase.codes)
			}
			if reading.IsRead() != testCase.isRead || reading.IsParsed() != testCase.isParsed ||
				reading.IsUsable() != (len(testCase.codes) == 0) {
				t.Errorf("read %t, parsed %t, usable %t; want %t, %t, %t", reading.IsRead(), reading.IsParsed(),
					reading.IsUsable(), testCase.isRead, testCase.isParsed, len(testCase.codes) == 0)
			}
			wantAliases := []string(nil)
			if testCase.hasAliases {
				wantAliases = fixture.aliases
			}
			if !slices.Equal(reading.Aliases, wantAliases) {
				t.Errorf("Aliases = %v, want %v", reading.Aliases, wantAliases)
			}
			wantETag := ""
			if testCase.isRead {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				wantETag = nodefile.ETag(data)
			}
			if reading.ETag != wantETag {
				t.Errorf("ETag = %q, want %q", reading.ETag, wantETag)
			}
		})
	}
}

func TestReader_ReadsEachFileOfItsKind(t *testing.T) {
	t.Parallel()

	t.Run("issue", func(t *testing.T) {
		t.Parallel()
		assertReader(t, IssueReader(config.Default()), readerFixture{
			dir:     ".capcom/issues",
			content: canonicalIssue,
			invalid: strings.Replace(canonicalIssue, "status: open", "status: bogus", 1),
			aliases: []string{"old-parser"},
		})
	})
	t.Run("note", func(t *testing.T) {
		t.Parallel()
		assertReader(t, NoteReader(), readerFixture{
			dir:     ".capcom/knowledge",
			content: canonicalNote,
			invalid: strings.Replace(canonicalNote, `"Quoting: the one policy"`, `""`, 1),
			aliases: []string{"old-rule"},
		})
	})
}

func TestReader_Decode_GivesTheParsedNodeAndAgreesWithRead(t *testing.T) {
	t.Parallel()
	path := writeNodeFile(t, t.TempDir(), "clean.md", canonicalIssue)
	const relPath = ".capcom/issues/clean.md"
	reader := IssueReader(config.Default())

	decoded := reader.Decode(relPath, []byte(canonicalIssue))
	read, err := reader.Read(path, relPath)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if decoded.Node.Title != "Fix the parser" || !slices.Equal(decoded.Node.Aliases, []string{"old-parser"}) {
		t.Errorf("Node = %+v, want the parsed issue", decoded.Node)
	}
	if decoded.ETag != read.ETag || !slices.Equal(decoded.Aliases, read.Aliases) || !decoded.IsUsable() {
		t.Errorf("Decode = %+v, want what Read gives: %+v", decoded, read)
	}
}

func TestReader_Read_MissingFile_FailsWithoutAFinding(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.md")

	reading, err := NoteReader().Read(path, ".capcom/knowledge/missing.md")

	if !os.IsNotExist(err) {
		t.Errorf("err = %v, want the file not to exist", err)
	}
	if findings := reading.Findings(); findings != nil {
		t.Errorf("Findings = %v, want none", findings)
	}
}

func TestReader_RefusedRead_WordsItsFindingAsTheWalkDoes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "directory.md")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	reading, err := NoteReader().Read(dir, "directory.md")

	want := []diagnose.Finding{{Path: "directory.md", Code: diagnose.NotRegular, Message: tree.ErrNotRegular.Error()}}
	if err != nil || !slices.Equal(reading.Findings(), want) {
		t.Errorf("Findings = %v, %v; want %v", reading.Findings(), err, want)
	}
}
