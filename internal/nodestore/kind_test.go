package nodestore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// canonicalIssue is an issue file in the canonical form, recording one alias.
const canonicalIssue = `---
title: Fix the parser
status: open
type: bug
priority: 1
labels: [cli]
aliases: [old-parser]
created: 2026-10-11T09:30:00Z
---

Body.
`

// canonicalNote is a Note file in the canonical form with every key, recording one alias.
const canonicalNote = `---
title: "Quoting: the one policy"
labels: [conventions]
refs: [internal/cli/errors.go]
retired: true
superseded-by: [new-rule]
aliases: [old-rule]
---

Body.
`

// unknownKeyLine is a frontmatter line neither kind knows, which makes a file fail to parse while
// its aliases stay readable from the raw frontmatter.
const unknownKeyLine = "bogus: value\n"

// newStore returns a store in a fresh working tree, with the default config.
func newStore(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	return &store.Store{
		Root:         filepath.Join(root, ".capcom"),
		WorktreeRoot: root,
		Config:       config.Default(),
	}
}

// writeNodeFile writes content as the file name in dir and returns its path.
func writeNodeFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// unparseable returns content with a key the kinds do not know added to its frontmatter.
func unparseable(content string) string {
	return content[:len("---\n")] + unknownKeyLine + content[len("---\n"):]
}

// assertRoundTrip parses content with codec, checks it validates and serializes back to content
// byte for byte, and returns the parsed node.
func assertRoundTrip[T any](t *testing.T, codec Codec[T], content string) T {
	t.Helper()
	node, err := codec.Parse("node.md", []byte(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if problems := codec.Validate("node.md", node); len(problems) > 0 {
		t.Errorf("Validate = %v, want no problems", problems)
	}
	if got := string(codec.Serialize(node)); got != content {
		t.Errorf("Serialize =\n%s\nwant\n%s", got, content)
	}
	return node
}

func TestCodec_CanonicalFile_RoundTripsPerKind(t *testing.T) {
	t.Parallel()
	s := newStore(t)

	t.Run("issue", func(t *testing.T) {
		t.Parallel()
		codec := Issues(s, nil).codec

		node := assertRoundTrip(t, codec, canonicalIssue)

		if codec.Noun != "issue" || !slices.Equal(codec.Aliases(node), []string{"old-parser"}) {
			t.Errorf("noun %q, aliases %v", codec.Noun, codec.Aliases(node))
		}
	})
	t.Run("note", func(t *testing.T) {
		t.Parallel()
		codec := Notes(s, nil).codec

		node := assertRoundTrip(t, codec, canonicalNote)

		if codec.Noun != "Note" || !slices.Equal(codec.Aliases(node), []string{"old-rule"}) {
			t.Errorf("noun %q, aliases %v", codec.Noun, codec.Aliases(node))
		}
	})
}

func TestCodec_InvalidIssue_ValidateUsesTheStoreConfig(t *testing.T) {
	t.Parallel()
	codec := Issues(newStore(t), nil).codec
	node, err := codec.Parse("node.md", []byte(canonicalIssue))
	if err != nil {
		t.Fatal(err)
	}
	node.Status = "undeclared"

	problems := codec.Validate("node.md", node)

	if len(problems) != 1 {
		t.Errorf("Validate = %v, want the one undeclared status", problems)
	}
}

// aliasLookups are the content reader and the alias lookup of one kind, with a canonical file of
// that kind and the text of one that parses but fails validation.
type aliasLookups struct {
	name    string
	dir     string
	content string
	invalid string
	aliases []string
	read    tree.ContentFunc
	recover tree.AliasFunc
}

// kindAliasLookups returns the alias lookups of both kinds of s.
func kindAliasLookups(s *store.Store) []aliasLookups {
	issues := Issues(s, nil)
	notes := Notes(s, nil)
	return []aliasLookups{
		{
			"issue", s.IssuesDir(), canonicalIssue, strings.Replace(canonicalIssue, "status: open", "status: bogus", 1),
			[]string{"old-parser"}, issues.ReadContent, issues.RecoverAliases,
		},
		{
			"note", s.KnowledgeDir(), canonicalNote, strings.Replace(canonicalNote, `"Quoting: the one policy"`, `""`, 1),
			[]string{"old-rule"}, notes.ReadContent, notes.RecoverAliases,
		},
	}
}

func TestReadContent_ParseableUnparseableAndInvalidFiles(t *testing.T) {
	t.Parallel()
	s := newStore(t)

	for _, kind := range kindAliasLookups(s) {
		t.Run(kind.name, func(t *testing.T) {
			parseable := writeNodeFile(t, kind.dir, "parseable.md", kind.content)
			broken := writeNodeFile(t, kind.dir, "broken.md", unparseable(kind.content))
			invalid := writeNodeFile(t, kind.dir, "invalid.md", kind.invalid)
			large := writeNodeFile(t, kind.dir, "large.md", strings.Repeat("x", nodefile.MaxNodeFileSize+1))
			cases := []struct {
				path string
				want tree.Content
			}{
				{parseable, tree.Content{Aliases: kind.aliases, Parse: tree.ParsedFile, IsUsable: true}},
				{broken, tree.Content{Aliases: kind.aliases, Parse: tree.UnparsedFile}},
				{invalid, tree.Content{Aliases: kind.aliases, Parse: tree.ParsedFile}},
				{large, tree.Content{Parse: tree.UnreadFile}},
			}
			for _, testCase := range cases {
				got, err := kind.read(testCase.path)

				if err != nil || !reflect.DeepEqual(got, testCase.want) {
					t.Errorf("%s: %+v, %v; want %+v", filepath.Base(testCase.path), got, err, testCase.want)
				}
			}
			t.Run("named pipe", func(t *testing.T) {
				fifo := filepath.Join(kind.dir, "fifo.md")
				makeFIFO(t, fifo)

				got, err := kind.read(fifo)

				if want := (tree.Content{Parse: tree.UnreadFile}); err != nil || !reflect.DeepEqual(got, want) {
					t.Errorf("fifo.md: %+v, %v; want %+v", got, err, want)
				}
			})
		})
	}
}

func TestRecoverAliases_ParseableAndUnparseableFiles(t *testing.T) {
	t.Parallel()
	s := newStore(t)

	for _, kind := range kindAliasLookups(s) {
		t.Run(kind.name, func(t *testing.T) {
			parseable := writeNodeFile(t, kind.dir, "parseable.md", kind.content)
			broken := writeNodeFile(t, kind.dir, "broken.md", unparseable(kind.content))

			fromParseable, errParseable := kind.recover(parseable)
			fromBroken, errBroken := kind.recover(broken)

			if errParseable != nil || fromParseable != nil {
				t.Errorf("parseable file: %v, %v; want none", fromParseable, errParseable)
			}
			if errBroken != nil || !slices.Equal(fromBroken, kind.aliases) {
				t.Errorf("unparseable file: %v, %v; want %v", fromBroken, errBroken, kind.aliases)
			}
		})
	}
}

func TestAliasLookups_MissingFile_FailWithATreeError(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	missing := filepath.Join(s.IssuesDir(), "missing.md")

	for _, kind := range kindAliasLookups(s) {
		_, readErr := kind.read(missing)
		_, recoverErr := kind.recover(missing)

		for name, err := range map[string]error{"read": readErr, "recover": recoverErr} {
			var failure *tree.Error
			if !errors.As(err, &failure) || failure.Path != missing {
				t.Errorf("%s %s: err = %v, want a *tree.Error at %s", kind.name, name, err, missing)
			}
		}
	}
}

func TestErrors_NameFilesRelativeToTheWorktree(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	issues := Issues(s, nil)
	notes := Notes(s, nil)
	path := filepath.Join(s.IssuesDir(), "a.md")
	notePath := filepath.Join(s.KnowledgeDir(), "rule.md")
	// A message names a file with the OS's separators, as the error it came from did.
	inMessage := filepath.Join(".capcom", "issues", "a.md")

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "frontmatter failure keeps its line",
			err:  issues.fileError(path, &frontmatter.Error{Path: path, Line: 3, Message: "bad " + path}),
			want: ".capcom/issues/a.md:3: bad " + inMessage,
		},
		{
			name: "read failure",
			err:  issues.fileError(path, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}),
			want: ".capcom/issues/a.md: open " + inMessage + ": file does not exist",
		},
		{
			name: "conflict",
			err:  notes.conflictError(notePath, "aaaa", "bbbb"),
			want: ".capcom/knowledge/rule.md: etag aaaa does not match the file's etag bbbb; show the Note again",
		},
		{
			name: "id taken",
			err:  issues.existsError("a", tree.Node{ID: "a", Path: path}),
			want: `.capcom/issues/a.md: id "a" is already taken by issue a`,
		},
		{
			name: "alias taken",
			err:  notes.existsError("old-rule", tree.Node{ID: "rule", Path: notePath}),
			want: `.capcom/knowledge/rule.md: id "old-rule" is already an alias of Note rule`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.want {
				t.Errorf("Error() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRaise_UnplaceableFormatKey_FailsAtTheKeyLineNamingTheConfigOnce(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	const content = "types: [bug, feature, task, chore]\n? format\n: 1\n"
	configPath := writeStoreConfig(t, path, content)

	_, err := issues.Edit(editedIssueID, "", linkDBSchema("Fix the parser"))

	const want = ".capcom/config.yaml:2: format: cannot raise the store format; " +
		"write the key as `format: <number>` at the start of a line of its own"
	var failure *FileError
	if !errors.As(err, &failure) || failure.Path != ".capcom/config.yaml" || failure.Line != 2 ||
		err.Error() != want {
		t.Errorf("err = %v, want a *FileError %q", err, want)
	}
	if data := readFile(t, configPath); data != content {
		t.Errorf("config.yaml = %q, want it unchanged", data)
	}
	if data := readFile(t, path); data != canonicalIssue {
		t.Errorf("issue file =\n%s\nwant it unchanged", data)
	}
}

func TestRaise_WrittenFileCarryingAKeyLine_RaisesToItsFormatWhateverTheEditChanged(t *testing.T) {
	t.Parallel()
	retitleIssue := func(t *testing.T, s *store.Store, content string) (Written, error) {
		t.Helper()
		writeNodeFile(t, s.IssuesDir(), editedIssueID+".md", content)
		issues := Issues(s, fakeResolver{editedIssueID: {ID: editedIssueID, Path: editedIssuePath}})
		calls := 0
		return issues.Edit(editedIssueID, "", retitle("Fix the lexer", &calls))
	}
	retitleNote := func(t *testing.T, s *store.Store, content string) (Written, error) {
		t.Helper()
		writeNodeFile(t, s.KnowledgeDir(), "rule.md", content)
		notes := Notes(s, fakeResolver{"rule": {ID: "rule", Path: ".capcom/knowledge/rule.md"}})
		return notes.Edit("rule", "", func(note knowledge.Note) (knowledge.Note, error) {
			note.Title = "Quoting"
			return note, nil
		})
	}
	withLine := func(content string, line string) string {
		return strings.Replace(content, "aliases:", line+"\naliases:", 1)
	}
	tests := []struct {
		name       string
		edit       func(t *testing.T, s *store.Store, content string) (Written, error)
		content    string
		wantFormat int
	}{
		{
			name:       "issue holding a related entry",
			edit:       retitleIssue,
			content:    withLine(canonicalIssue, "related: [db-schema]"),
			wantFormat: config.FormatRelated,
		},
		{name: "issue holding no related key", edit: retitleIssue, content: canonicalIssue},
		{
			name:    "issue holding an empty related list, which the write leaves out",
			edit:    retitleIssue,
			content: withLine(canonicalIssue, "related: []"),
		},
		{
			name:       "retired note holding a superseded-by entry",
			edit:       retitleNote,
			content:    canonicalNote,
			wantFormat: config.FormatSupersession,
		},
		{
			name: "note holding retired: false and an empty superseded-by, which the write leaves out",
			edit: retitleNote,
			content: strings.Replace(canonicalNote, "retired: true\nsuperseded-by: [new-rule]",
				"retired: false\nsuperseded-by: []", 1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			configPath := writeNodeFile(t, s.Root, "config.yaml", keylessConfig)

			written, err := test.edit(t, s, test.content)

			if err != nil {
				t.Fatalf("Edit: %v", err)
			}
			wantConfig := keylessConfig
			if test.wantFormat != 0 {
				wantConfig = fmt.Sprintf("format: %d\n%s", test.wantFormat, keylessConfig)
			}
			if content := readFile(t, configPath); written.RaisedFormat != test.wantFormat ||
				content != wantConfig {
				t.Errorf("RaisedFormat = %d, config.yaml = %q; want %d and %q",
					written.RaisedFormat, content, test.wantFormat, wantConfig)
			}
		})
	}
}

// recordingFS is a nodefile.FS that reads through the disk and keeps every write in memory: it
// records the path of every file read, a file it wrote reads back from memory, and nothing it
// writes reaches the disk.
type recordingFS struct {
	nodefile.Disk
	reads  []string
	writes map[string][]byte
}

// newRecordingFS returns a recordingFS that has read and written nothing.
func newRecordingFS() *recordingFS {
	return &recordingFS{writes: map[string][]byte{}}
}

// ReadRegularFile records the read of path and returns what was written there, or the file on disk.
func (r *recordingFS) ReadRegularFile(path string) ([]byte, error) {
	r.reads = append(r.reads, path)
	if data, isWritten := r.writes[path]; isWritten {
		return data, nil
	}
	return r.Disk.ReadRegularFile(path)
}

// ReplaceFile keeps content as the file at path, failing as the disk would when no file is there.
func (r *recordingFS) ReplaceFile(path string, content io.Reader) error {
	if _, isWritten := r.writes[path]; !isWritten {
		if _, err := r.Lstat(path); err != nil {
			return err
		}
	}
	return r.keep(path, content)
}

// CreateFile keeps content as the file at path, failing with fs.ErrExist when a file is there.
func (r *recordingFS) CreateFile(path string, content io.Reader) error {
	_, isWritten := r.writes[path]
	if _, err := r.Lstat(path); isWritten || err == nil {
		return &fs.PathError{Op: "create", Path: path, Err: fs.ErrExist}
	}
	return r.keep(path, content)
}

// keep stores everything read from content as the file at path.
func (r *recordingFS) keep(path string, content io.Reader) error {
	var data bytes.Buffer
	if _, err := data.ReadFrom(content); err != nil {
		return err
	}
	r.writes[path] = data.Bytes()
	return nil
}

func TestWithFS_AliasLookups_ReadThroughTheFS(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	path := filepath.Join(s.IssuesDir(), "broken.md")
	recording := newRecordingFS()
	recording.writes[path] = []byte(unparseable(canonicalIssue))
	issues := Issues(s, nil).WithFS(recording)

	content, readErr := issues.ReadContent(path)
	aliases, recoverErr := issues.RecoverAliases(path)

	want := tree.Content{Aliases: []string{"old-parser"}, Parse: tree.UnparsedFile}
	if readErr != nil || !reflect.DeepEqual(content, want) {
		t.Errorf("ReadContent = %+v, %v; want %+v", content, readErr, want)
	}
	if recoverErr != nil || !slices.Equal(aliases, want.Aliases) {
		t.Errorf("RecoverAliases = %v, %v; want %v", aliases, recoverErr, want.Aliases)
	}
}

func TestWithFS_Edit_ReadsAndWritesThroughTheFS(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	recording := newRecordingFS()
	calls := 0
	routed := issues.WithFS(recording)

	written, err := routed.Edit(editedIssueID, "", retitle("Fix the lexer", &calls))

	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if want := []string{path, path}; !slices.Equal(recording.reads, want) {
		t.Errorf("reads = %v, want the read and the re-check %v", recording.reads, want)
	}
	data := recording.writes[path]
	if !strings.Contains(string(data), "title: Fix the lexer\n") || written.ETag != nodefile.ETag(data) {
		t.Errorf("written %q with ETag %q, want the retitled issue and its etag", data, written.ETag)
	}
	if content := readFile(t, path); content != canonicalIssue {
		t.Errorf("issue file on disk =\n%s\nwant it unchanged", content)
	}
}

func TestWithFS_Create_LoadsAndWritesThroughTheFS(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	other := writeNodeFile(t, s.IssuesDir(), "other-node.md", canonicalIssue)
	recording := newRecordingFS()

	written, _, err := Issues(s, nil).WithFS(recording).Create(newIssueID, "", buildIssue(t, "open"))

	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !slices.Contains(recording.reads, other) {
		t.Errorf("reads = %v, want the load to read %s", recording.reads, other)
	}
	path := filepath.Join(s.IssuesDir(), newIssueID+".md")
	if data, isWritten := recording.writes[path]; !isWritten || written.ETag != nodefile.ETag(data) {
		t.Errorf("writes = %v, ETag %q; want %s written with its etag", recording.writes, written.ETag, path)
	}
	if files := nodeFiles(t, s.IssuesDir()); !slices.Equal(files, []string{"other-node.md"}) {
		t.Errorf("issue files on disk = %v, want only other-node.md", files)
	}
}

func TestWithRaise_StandingIssueBelowTheStandingFormat_RaisesThroughTheFunc(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	declared := fmt.Sprintf("format: %d\n", config.FormatAgents)
	configPath := writeNodeFile(t, s.Root, "config.yaml", declared)
	node, err := issue.Parse("node.md", []byte(canonicalIssue))
	if err != nil {
		t.Fatal(err)
	}
	node.Aliases = nil
	node.IsStanding = true
	var raised []int
	raise := func(version int) (bool, error) {
		raised = append(raised, version)
		return true, nil
	}
	issues := Issues(s, nil).WithFS(newRecordingFS()).WithRaise(raise)

	written, _, err := issues.Create(newIssueID, "", func() (issue.Issue, error) { return node, nil })

	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := []int{config.FormatStanding}
	if !slices.Equal(raised, want) || written.RaisedFormat != config.FormatStanding {
		t.Errorf("raised %v, RaisedFormat %d; want %v", raised, written.RaisedFormat, want)
	}
	if content := readFile(t, configPath); content != declared {
		t.Errorf("config.yaml = %q, want it unchanged", content)
	}
}
