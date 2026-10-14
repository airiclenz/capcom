package nodestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/knowledge"
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

		if codec.Noun != "note" || !slices.Equal(codec.Aliases(node), []string{"old-rule"}) {
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
			cases := []struct {
				path string
				want tree.Content
			}{
				{parseable, tree.Content{Aliases: kind.aliases, Parse: tree.ParsedFile, IsUsable: true}},
				{broken, tree.Content{Aliases: kind.aliases, Parse: tree.UnparsedFile}},
				{invalid, tree.Content{Aliases: kind.aliases, Parse: tree.ParsedFile}},
			}
			for _, testCase := range cases {
				got, err := kind.read(testCase.path)

				if err != nil || !reflect.DeepEqual(got, testCase.want) {
					t.Errorf("%s: %+v, %v; want %+v", filepath.Base(testCase.path), got, err, testCase.want)
				}
			}
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
			want: ".capcom/knowledge/rule.md: etag aaaa does not match the file's etag bbbb; show the note again",
		},
		{
			name: "id taken",
			err:  issues.existsError("a", tree.Node{ID: "a", Path: path}),
			want: `.capcom/issues/a.md: id "a" is already taken by issue a`,
		},
		{
			name: "alias taken",
			err:  notes.existsError("old-rule", tree.Node{ID: "rule", Path: notePath}),
			want: `.capcom/knowledge/rule.md: id "old-rule" is already an alias of note rule`,
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
