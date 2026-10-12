package nodestore

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
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

// canonicalNote is a Note file in the canonical form, recording one alias.
const canonicalNote = `---
title: "Quoting: the one policy"
labels: [conventions]
refs: [internal/cli/errors.go]
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

// aliasLookups are the two alias lookups of one kind, with a canonical file of that kind.
type aliasLookups struct {
	name    string
	dir     string
	content string
	aliases []string
	read    tree.AliasFunc
	recover tree.AliasFunc
}

// kindAliasLookups returns the alias lookups of both kinds of s.
func kindAliasLookups(s *store.Store) []aliasLookups {
	issues := Issues(s, nil)
	notes := Notes(s, nil)
	return []aliasLookups{
		{"issue", s.IssuesDir(), canonicalIssue, []string{"old-parser"}, issues.ReadAliases, issues.RecoverAliases},
		{"note", s.KnowledgeDir(), canonicalNote, []string{"old-rule"}, notes.ReadAliases, notes.RecoverAliases},
	}
}

func TestReadAliases_ParseableAndUnparseableFiles(t *testing.T) {
	t.Parallel()
	s := newStore(t)

	for _, kind := range kindAliasLookups(s) {
		t.Run(kind.name, func(t *testing.T) {
			parseable := writeNodeFile(t, kind.dir, "parseable.md", kind.content)
			broken := writeNodeFile(t, kind.dir, "broken.md", unparseable(kind.content))

			fromParseable, errParseable := kind.read(parseable)
			fromBroken, errBroken := kind.read(broken)

			if errParseable != nil || !slices.Equal(fromParseable, kind.aliases) {
				t.Errorf("parseable file: %v, %v; want %v", fromParseable, errParseable, kind.aliases)
			}
			if errBroken != nil || fromBroken != nil {
				t.Errorf("unparseable file: %v, %v; want none", fromBroken, errBroken)
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
		for name, lookup := range map[string]tree.AliasFunc{"read": kind.read, "recover": kind.recover} {
			_, err := lookup(missing)

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
