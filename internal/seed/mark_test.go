package seed

import (
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/knowledge"
)

const (
	// seedingVersion is the capcom version the tests seed with.
	seedingVersion = "1.2.3"
	// earlierText is the body the made-up set held for a Note before its text changed.
	earlierText = "\nThe earlier text.\n"
)

// loadMadeUpSet returns the made-up set and a Note below its root.
func loadMadeUpSet(t *testing.T) (Set, Note) {
	t.Helper()
	set, err := LoadFS(madeUpFiles())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	notes := set.Notes()
	return set, notes[len(notes)-1]
}

// seededNote returns the store's Note after capcom seeded it with the text of note.
func seededNote(set Set, note Note) knowledge.Note {
	return knowledge.Note{Title: note.Title, Body: note.Body, Labels: set.Labels(note, seedingVersion)}
}

// earlierSeededNote returns the store's Note that capcom seeded when the set still held
// earlierText for note.
func earlierSeededNote(set Set, note Note) knowledge.Note {
	note.Body = earlierText
	return seededNote(set, note)
}

func TestHash_LineEndingsAndTrailingNewlines_HashTheSame(t *testing.T) {
	t.Parallel()
	_, note := loadMadeUpSet(t)
	want := Hash(note.Title, note.Body)

	tests := []struct {
		name string
		body string
	}{
		{"CRLF line endings", strings.ReplaceAll(note.Body, "\n", "\r\n")},
		{"one more newline at the end", note.Body + "\n"},
		{"no newline at the end", strings.TrimRight(note.Body, "\n")},
		{"CRLF and more newlines at the end", strings.ReplaceAll(note.Body, "\n", "\r\n") + "\r\n\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Hash(note.Title, tt.body)

			if got != want {
				t.Errorf("Hash(%q, %q) = %s, want %s", note.Title, tt.body, got, want)
			}
		})
	}
}

func TestHash_Text_IsTwelveLowerCaseHexDigits(t *testing.T) {
	t.Parallel()
	_, note := loadMadeUpSet(t)

	got := Hash(note.Title, note.Body)

	if len(got) != 12 || strings.Trim(got, "0123456789abcdef") != "" {
		t.Errorf("Hash = %q, want 12 lower-case hex digits", got)
	}
}

func TestHash_OtherTitleOrBody_HashesDifferently(t *testing.T) {
	t.Parallel()
	_, note := loadMadeUpSet(t)
	hash := Hash(note.Title, note.Body)

	tests := []struct {
		name  string
		title string
		body  string
	}{
		{"another title", note.Title + " changed", note.Body},
		{"another body", note.Title, note.Body + "One more line.\n"},
		{"a blank line more before the body", note.Title, "\n" + note.Body},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Hash(tt.title, tt.body)

			if got == hash {
				t.Errorf("Hash(%q, %q) = %s, the hash of the unchanged text", tt.title, tt.body, got)
			}
		})
	}
}

func TestSet_Labels_MarkTheSetAndTheSeededText(t *testing.T) {
	t.Parallel()
	set, note := loadMadeUpSet(t)

	got := set.Labels(note, seedingVersion)

	want := []string{
		"seed:" + set.Root().ID,
		"seed-text:" + seedingVersion + ":" + Hash(note.Title, note.Body),
	}
	if !slices.Equal(got, want) {
		t.Errorf("Labels = %q, want %q", got, want)
	}
}

func TestSet_Relabel_ReplacesTheSeedTextLabelAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	set, note := loadMadeUpSet(t)
	const laterVersion = "2.0.0"
	earlier := earlierSeededNote(set, note).Labels
	later := set.Labels(note, laterVersion)

	tests := []struct {
		name   string
		labels []string
		want   []string
	}{
		{"the Labels of a Note seeded with an earlier text", earlier, later},
		{
			"the user's own Labels around the seed-text Label",
			[]string{"conventions", earlier[0], earlier[1], "process"},
			[]string{"conventions", later[0], later[1], "process"},
		},
		{
			"a second seed-text Label",
			[]string{earlier[1], earlier[0], earlier[1] + "0"},
			[]string{later[1], later[0]},
		},
		{"no seed-text Label", []string{"conventions"}, []string{"conventions", later[1]}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			given := slices.Clone(tt.labels)

			got := set.Relabel(tt.labels, note, laterVersion)

			if !slices.Equal(got, tt.want) {
				t.Errorf("Relabel(%q) = %q, want %q", tt.labels, got, tt.want)
			}
			if !slices.Equal(tt.labels, given) {
				t.Errorf("Relabel changed its argument to %q, want %q", tt.labels, given)
			}
		})
	}
}

func TestSet_IsSeeded_ReportsTheSeedLabelOfThisSet(t *testing.T) {
	t.Parallel()
	set, note := loadMadeUpSet(t)
	labels := set.Labels(note, seedingVersion)

	tests := []struct {
		name   string
		labels []string
		want   bool
	}{
		{"the Labels of a seeded Note", labels, true},
		{"the seed Label among the user's own", []string{"conventions", labels[0]}, true},
		{"the seed-text Label alone", labels[1:], false},
		{"the seed Label of another set", []string{labels[0] + "-other"}, false},
		{"no Labels", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := set.IsSeeded(tt.labels)

			if got != tt.want {
				t.Errorf("IsSeeded(%q) = %t, want %t", tt.labels, got, tt.want)
			}
		})
	}
}

func TestSet_State_TellsSameOutdatedAndDiffersApart(t *testing.T) {
	t.Parallel()
	set, note := loadMadeUpSet(t)
	seeded := seededNote(set, note)
	earlier := earlierSeededNote(set, note)
	earlierHash := Hash(earlier.Title, earlier.Body)

	// withLabels returns existing carrying the given Labels instead of its own.
	withLabels := func(existing knowledge.Note, labels ...string) knowledge.Note {
		existing.Labels = labels
		return existing
	}
	// withBody returns existing with the given body and its Labels untouched.
	withBody := func(existing knowledge.Note, body string) knowledge.Note {
		existing.Body = body
		return existing
	}
	// withTitle returns existing with the given title and its Labels untouched.
	withTitle := func(existing knowledge.Note, title string) knowledge.Note {
		existing.Title = title
		return existing
	}

	tests := []struct {
		name     string
		existing knowledge.Note
		want     State
	}{
		{"a freshly seeded Note", seeded, Same},
		{"the set's text with CRLF line endings", withBody(seeded, strings.ReplaceAll(note.Body, "\n", "\r\n")), Same},
		{"the set's text with one more newline at the end", withBody(seeded, note.Body+"\n"), Same},
		{"the set's text without any Label", withLabels(seeded), Same},
		{"the set's text with the user's Labels and Refs", knowledge.Note{
			Title: note.Title, Body: note.Body, Labels: []string{"conventions"}, Refs: []string{"README.md"},
		}, Same},
		{"the set's text under the seed-text Label of an earlier text", withLabels(seeded, earlier.Labels...), Same},

		{"a Note seeded with an earlier text", earlier, Outdated},
		{"an earlier text with CRLF line endings", withBody(earlier, strings.ReplaceAll(earlierText, "\n", "\r\n")), Outdated},
		{"an earlier text seeded by another version", withLabels(earlier, "seed-text:0.9.0:"+earlierHash), Outdated},
		{"an earlier text beside the user's Labels", withLabels(earlier, append([]string{"conventions"}, earlier.Labels...)...), Outdated},

		{"a seeded Note with an edited body", withBody(seeded, note.Body+"A line the user added.\n"), Differs},
		{"a seeded Note with an edited title", withTitle(seeded, note.Title+" edited"), Differs},
		{"an earlier text, edited", withBody(earlier, earlierText+"A line the user added.\n"), Differs},
		{"an earlier text with a blank line more at the start", withBody(earlier, "\n"+earlierText), Differs},
		{"an earlier text whose Labels were removed", withLabels(earlier), Differs},
		{"an earlier text with the seed Label only", withLabels(earlier, earlier.Labels[0]), Differs},
		{"the repository's own Note of that id", knowledge.Note{Title: "Our own rule", Body: "\nOurs.\n"}, Differs},
		{"a seed-text Label without a hash", withLabels(earlier, "seed-text:"+seedingVersion), Differs},
		{"a seed-text Label without a version", withLabels(earlier, "seed-text:"+earlierHash), Differs},
		{"a seed-text Label with an empty version", withLabels(earlier, "seed-text::"+earlierHash), Differs},
		{"a seed-text Label with a shortened hash", withLabels(earlier, "seed-text:"+seedingVersion+":"+earlierHash[:8]), Differs},
		{"a seed-text Label with an upper-case hash", withLabels(earlier, "seed-text:"+seedingVersion+":"+strings.ToUpper(earlierHash)), Differs},
		{"a seed-text Label with text that is no hash", withLabels(earlier, "seed-text:"+seedingVersion+":not-a-hash!!"), Differs},
		{"a seed-text Label with text after the hash", withLabels(earlier, earlier.Labels[1]+":more"), Differs},
		{"the hash under another Label", withLabels(earlier, "text:"+seedingVersion+":"+earlierHash), Differs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := set.State(note.ID, tt.existing)

			if got != tt.want {
				t.Errorf("State(%s, %+v) = %s, want %s", note.ID, tt.existing, got, tt.want)
			}
		})
	}
}

func TestSet_State_IDOutsideTheSet_Differs(t *testing.T) {
	t.Parallel()
	set, note := loadMadeUpSet(t)

	got := set.State(note.ID+"-missing", seededNote(set, note))

	if got != Differs {
		t.Errorf("State of an id outside the set = %s, want %s", got, Differs)
	}
}

func TestSet_State_EmbeddedSet_EverySeededNoteIsSame(t *testing.T) {
	t.Parallel()
	set, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, note := range set.Notes() {
		existing := seededNote(set, note)

		if got := set.State(note.ID, existing); got != Same {
			t.Errorf("State(%s) of the Note as seeded = %s, want %s", note.ID, got, Same)
		}
		if !set.IsSeeded(existing.Labels) {
			t.Errorf("IsSeeded(%q) = false for the Note %s as seeded", existing.Labels, note.ID)
		}
	}
}
