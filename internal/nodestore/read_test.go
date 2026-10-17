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

func TestIssueReader_Decode_SortsItsFindings(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantTitle   string
		wantParse   []diagnose.Finding
		wantFields  []diagnose.Finding
		wantAliases []diagnose.Finding
	}{
		{
			name: "does not parse, with a line",
			data: "---\ntitle: a\nbogus: 1\n---\n",
			wantParse: []diagnose.Finding{{
				Code: diagnose.Frontmatter,
				Path: "alpha.md",
				Line: 3,
				Message: "unknown key \"bogus\"; an issue has title, status, type, priority, labels, deps, " +
					"related, plan, standing, aliases, created, closed and reason",
			}},
		},
		{
			name: "does not parse, without a line",
			data: "---\n\ttitle: a\n---\n",
			wantParse: []diagnose.Finding{{
				Code:    diagnose.Frontmatter,
				Path:    "alpha.md",
				Message: "invalid YAML frontmatter: yaml: found character that cannot start any token",
			}},
		},
		{
			name: "two bad fields",
			data: "---\ntitle: \"\"\nstatus: nope\ntype: task\ncreated: 2026-10-13T00:00:00Z\n---\n",
			wantFields: []diagnose.Finding{
				{Code: diagnose.Field, Path: "alpha.md", Line: 2, Message: "title is required"},
				{
					Code:    diagnose.Field,
					Path:    "alpha.md",
					Line:    3,
					Message: "status \"nope\" is not declared in config; use one of [open active parked done dropped]",
				},
			},
		},
		{
			name:      "an alias repeats the id",
			data:      "---\ntitle: Alpha\nstatus: open\ntype: task\naliases: [alpha]\ncreated: 2026-10-13T00:00:00Z\n---\n",
			wantTitle: "Alpha",
			wantAliases: []diagnose.Finding{{
				Code:    diagnose.RepeatedAlias,
				Path:    "alpha.md",
				Line:    5,
				Message: "alias \"alpha\" repeats the issue's own id",
			}},
		},
		{
			name:      "clean",
			data:      "---\ntitle: Alpha\nstatus: open\ntype: task\naliases: [first]\ncreated: 2026-10-13T00:00:00Z\n---\n",
			wantTitle: "Alpha",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reading := IssueReader(config.Default()).Decode("alpha.md", []byte(test.data))

			if reading.Node.Title != test.wantTitle {
				t.Errorf("parsed title = %q, want %q", reading.Node.Title, test.wantTitle)
			}
			if reading.IsParsed() != (test.wantParse == nil) {
				t.Errorf("IsParsed() = %v, want %v", reading.IsParsed(), test.wantParse == nil)
			}
			assertReadingFindings(t, reading.Parse, reading.Fields, reading.AliasFindings, reading.Findings(),
				test.wantParse, test.wantFields, test.wantAliases)
		})
	}
}

func TestNoteReader_Decode_SortsItsFindings(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantTitle   string
		wantParse   []diagnose.Finding
		wantFields  []diagnose.Finding
		wantAliases []diagnose.Finding
	}{
		{
			name: "does not parse",
			data: "---\ntitle: a\nbogus: 1\n---\n",
			wantParse: []diagnose.Finding{{
				Code:    diagnose.Frontmatter,
				Path:    "alpha.md",
				Line:    3,
				Message: "unknown key \"bogus\"; a Note has title, labels, refs, retired, superseded-by and aliases",
			}},
		},
		{
			name: "missing title",
			data: "---\nlabels: [x]\n---\n",
			wantFields: []diagnose.Finding{
				{Code: diagnose.Field, Path: "alpha.md", Line: 0, Message: "title is required"},
			},
		},
		{
			name:      "an alias is no valid name",
			data:      "---\ntitle: Alpha\naliases: [Not Valid]\n---\n",
			wantTitle: "Alpha",
			wantAliases: []diagnose.Finding{{
				Code:    diagnose.InvalidAlias,
				Path:    "alpha.md",
				Line:    3,
				Message: "invalid alias \"Not Valid\": must use only a-z, 0-9, dots and hyphens",
			}},
		},
		{
			name:      "clean",
			data:      "---\ntitle: Alpha\naliases: [first]\n---\n",
			wantTitle: "Alpha",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reading := NoteReader().Decode("alpha.md", []byte(test.data))

			if reading.Node.Title != test.wantTitle {
				t.Errorf("parsed title = %q, want %q", reading.Node.Title, test.wantTitle)
			}
			if reading.IsParsed() != (test.wantParse == nil) {
				t.Errorf("IsParsed() = %v, want %v", reading.IsParsed(), test.wantParse == nil)
			}
			assertReadingFindings(t, reading.Parse, reading.Fields, reading.AliasFindings, reading.Findings(),
				test.wantParse, test.wantFields, test.wantAliases)
		})
	}
}

// assertReadingFindings fails t unless each finding list of a read file's Reading is its want, and
// all is the three wants in order, nil when every list is.
func assertReadingFindings(
	t *testing.T,
	parse, fields, aliases, all []diagnose.Finding,
	wantParse, wantFields, wantAliases []diagnose.Finding,
) {
	t.Helper()
	if !slices.Equal(parse, wantParse) {
		t.Errorf("Parse = %#v, want %#v", parse, wantParse)
	}
	if !slices.Equal(fields, wantFields) {
		t.Errorf("Fields = %#v, want %#v", fields, wantFields)
	}
	if !slices.Equal(aliases, wantAliases) {
		t.Errorf("AliasFindings = %#v, want %#v", aliases, wantAliases)
	}
	wantAll := slices.Concat(wantParse, wantFields, wantAliases)
	if !slices.Equal(all, wantAll) || (all == nil) != (wantAll == nil) {
		t.Errorf("Findings() = %#v, want %#v", all, wantAll)
	}
}
