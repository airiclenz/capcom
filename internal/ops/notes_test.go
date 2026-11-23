package ops

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/reflist"
	"github.com/airiclenz/capcom/internal/tree"
)

// successorNotes are the Notes the superseded-by tests resolve against: git-fixtures claims the
// alias fixtures alone, and two Notes claim shared.
var successorNotes = []index.NoteEntry{
	{ID: "release-flow"},
	{ID: "git-fixtures", Aliases: []string{"fixtures", "shared"}},
	{ID: "cli-tests", Aliases: []string{"shared"}},
}

// successorList returns the superseded-by list of release-flow over successorNotes, resolving an
// argument by exact id or by an Alias exactly one Note claims.
func successorList() reflist.List {
	names := NoteNames(successorNotes)
	return reflist.List{
		Holder: "release-flow",
		Resolve: func(arg string) (string, error) {
			if id, isFound := names(arg); isFound {
				return id, nil
			}
			return "", fmt.Errorf("%s: %w", arg, tree.ErrNotFound)
		},
		Names: names,
	}
}

func TestNoteNames_Entry_NamesAnIDOrAnAliasOneNoteClaims(t *testing.T) {
	cases := []struct {
		name        string
		entry       string
		wantID      string
		wantIsFound bool
	}{
		{name: "an id", entry: "cli-tests", wantID: "cli-tests", wantIsFound: true},
		{name: "an alias one note claims", entry: "fixtures", wantID: "git-fixtures", wantIsFound: true},
		{name: "an alias two notes claim", entry: "shared", wantIsFound: false},
		{name: "no note's name", entry: "absent", wantIsFound: false},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			id, isFound := NoteNames(successorNotes)(current.entry)

			if id != current.wantID || isFound != current.wantIsFound {
				t.Errorf("NoteNames(%q) = %q, %v; want %q, %v",
					current.entry, id, isFound, current.wantID, current.wantIsFound)
			}
		})
	}
}

func TestEditSuccessors_AddThenRemoveTheSameSuccessor_RemovesIt(t *testing.T) {
	cases := []struct {
		name    string
		entries []string
	}{
		{name: "not stored", entries: []string{"cli-tests"}},
		{name: "already stored", entries: []string{"git-fixtures", "cli-tests"}},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			stored := slices.Clone(current.entries)

			got, err := EditSuccessors(successorList(), []string{"git-fixtures"}, []string{"git-fixtures"}, current.entries)

			if err != nil || !slices.Equal(got, []string{"cli-tests"}) {
				t.Errorf("EditSuccessors = %q, %v; want [cli-tests]", got, err)
			}
			if !slices.Equal(current.entries, stored) {
				t.Errorf("entries changed to %q; want them left at %q", current.entries, stored)
			}
		})
	}
}

func TestEditSuccessors_AddBesideItsAliasSpelling_AddsNothing(t *testing.T) {
	entries := []string{"fixtures"}

	got, err := EditSuccessors(successorList(), []string{"git-fixtures"}, nil, entries)

	if err != nil || !slices.Equal(got, entries) {
		t.Errorf("EditSuccessors = %q, %v; want %q unchanged", got, err, entries)
	}
}

func TestEditSuccessors_AliasArgument_AppendsTheID(t *testing.T) {
	got, err := EditSuccessors(successorList(), []string{"fixtures"}, nil, []string{"cli-tests"})

	if err != nil || !slices.Equal(got, []string{"cli-tests", "git-fixtures"}) {
		t.Errorf("EditSuccessors = %q, %v; want [cli-tests git-fixtures]", got, err)
	}
}

func TestEditSuccessors_RemovalMatchingNothing_ChangesNothing(t *testing.T) {
	entries := []string{"cli-tests"}

	got, err := EditSuccessors(successorList(), nil, []string{"git-fixtures"}, entries)

	if err != nil || !slices.Equal(got, entries) {
		t.Errorf("EditSuccessors = %q, %v; want %q unchanged", got, err, entries)
	}
}

func TestEditSuccessors_RefusedArgument_ReturnsTheRefusalUnchanged(t *testing.T) {
	cases := []struct {
		name       string
		addArgs    []string
		wantReason reflist.Reason
	}{
		{name: "the holder itself", addArgs: []string{"release-flow"}, wantReason: reflist.ReasonSelf},
		{name: "no note's name", addArgs: []string{"cli-tests", "absent"}, wantReason: reflist.ReasonNotFound},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			got, err := EditSuccessors(successorList(), current.addArgs, nil, nil)

			var refusal *reflist.Refusal
			if got != nil || !errors.As(err, &refusal) || refusal.Reason != current.wantReason {
				t.Errorf("EditSuccessors = %q, %v; want no entries and a refusal of reason %v",
					got, err, current.wantReason)
			}
		})
	}
}
