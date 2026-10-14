package nodestore

import (
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// relatedIssue is an issue file whose related key sits on line 6.
const relatedIssue = `---
title: Fix the parser
status: open
type: bug
priority: 1
related: [other-issue]
created: 2026-10-11T09:30:00Z
---

Body.
`

// formatRow is one row of a format table as a test expects it: the key, its format, and the line
// the row's accessor reports for the parsed test file.
type formatRow struct {
	key    string
	format int
	line   int
}

// descriptorWant is what a test expects of one kind's Descriptor.
type descriptorWant struct {
	noun       string
	nounPhrase string
	refKeys    []string
	formats    []formatRow
}

// assertDescriptor checks descriptor against want, reading each format row's line from content as
// parse reads it.
func assertDescriptor[T any](
	t *testing.T,
	descriptor Descriptor[T],
	parse func(path string, data []byte) (T, error),
	content string,
	want descriptorWant,
) {
	t.Helper()
	node, err := parse("node.md", []byte(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	rows := make([]formatRow, 0, len(descriptor.Formats))
	for _, entry := range descriptor.Formats {
		rows = append(rows, formatRow{key: entry.Key, format: entry.Format, line: entry.Line(node)})
	}

	if descriptor.Noun != want.noun || descriptor.NounPhrase != want.nounPhrase {
		t.Errorf("noun %q, noun phrase %q; want %q, %q", descriptor.Noun, descriptor.NounPhrase, want.noun, want.nounPhrase)
	}
	if got := descriptor.RefKeys(); !slices.Equal(got, want.refKeys) {
		t.Errorf("RefKeys = %v, want %v", got, want.refKeys)
	}
	if !slices.Equal(rows, want.formats) {
		t.Errorf("format table = %+v, want %+v", rows, want.formats)
	}
}

func TestDescriptor_PerKind_DescribesNounRefKeysAndFormats(t *testing.T) {
	t.Parallel()

	t.Run("issue", func(t *testing.T) {
		t.Parallel()
		descriptor := IssueDescriptor(config.Default())

		assertDescriptor(t, descriptor, issue.Parse, relatedIssue, descriptorWant{
			noun:       "issue",
			nounPhrase: "an issue",
			refKeys:    []string{RefDeps, RefRelated},
			formats:    []formatRow{{key: "related", format: config.FormatRelated, line: 6}},
		})
	})
	t.Run("note", func(t *testing.T) {
		t.Parallel()
		descriptor := NoteDescriptor()

		assertDescriptor(t, descriptor, knowledge.Parse, canonicalNote, descriptorWant{
			noun:       "Note",
			nounPhrase: "a Note",
			refKeys:    []string{RefSupersededBy},
			formats: []formatRow{
				{key: "retired", format: config.FormatSupersession, line: 5},
				{key: "superseded-by", format: config.FormatSupersession, line: 6},
			},
		})
	})
}
