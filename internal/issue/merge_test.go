package issue_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
)

// Log entries the merge tests combine, oldest first.
const (
	entryOctFirst = "- 2026-10-01 Bob: seen"
	entryOctFifth = "- 2026-10-05 Carol: triaged"
	entryOctTenth = "- 2026-10-10 Alice: started"
	entryOctLast  = "- 2026-10-11 Bob: fixed"
)

// issueFile returns an issue file titled title whose Log, its last section, holds entries.
func issueFile(title string, entries ...string) string {
	return "---\ntitle: " + title + "\nstatus: open\n---\n\nBody.\n\n## Log\n\n" +
		strings.Join(entries, "\n") + "\n"
}

// fencedLogExampleFile returns an issue file whose body quotes a Log section in a fenced code
// block above its real Log section, which holds entries.
func fencedLogExampleFile(entries ...string) string {
	return "---\ntitle: T\nstatus: open\n---\n\nExample:\n\n```markdown\n## Log\n\n" +
		"- 2026-09-01 Dave: quoted\n```\n\n## Log\n\n" + strings.Join(entries, "\n") + "\n"
}

// fakeTextMerge stands in for git merge-file at file granularity: a side that equals base yields
// the other side, equal sides yield either, and anything else is one conflict hunk spanning the
// whole file.
func fakeTextMerge(base []byte, ours []byte, theirs []byte) ([]byte, bool, error) {
	switch {
	case bytes.Equal(ours, base):
		return theirs, false, nil
	case bytes.Equal(theirs, base), bytes.Equal(ours, theirs):
		return ours, false, nil
	}
	return []byte(conflict(string(ours), string(theirs))), true, nil
}

// conflict returns the whole-file conflict hunk fakeTextMerge writes for ours and theirs.
func conflict(ours string, theirs string) string {
	return "<<<<<<< ours\n" + ours + "=======\n" + theirs + ">>>>>>> theirs\n"
}

func TestMergeFile_ThreeVersions_MergesLogByEntry(t *testing.T) {
	cases := []struct {
		name         string
		base         string
		ours         string
		theirs       string
		want         string
		wantConflict bool
	}{
		{
			name:   "both append, kept in date order",
			base:   issueFile("T", entryOctFirst),
			ours:   issueFile("T", entryOctFirst, entryOctLast),
			theirs: issueFile("T", entryOctFirst, entryOctTenth),
			want:   issueFile("T", entryOctFirst, entryOctTenth, entryOctLast),
		},
		{
			name:   "both append on the same date, ours first",
			base:   issueFile("T", entryOctFirst),
			ours:   issueFile("T", entryOctFirst, entryOctLast),
			theirs: issueFile("T", entryOctFirst, "- 2026-10-11 Carol: tested"),
			want: issueFile(
				"T", entryOctFirst, entryOctLast, "- 2026-10-11 Carol: tested",
			),
		},
		{
			name:   "one appends, one edits the title",
			base:   issueFile("T", entryOctFirst),
			ours:   issueFile("T", entryOctFirst, entryOctLast),
			theirs: issueFile("New", entryOctFirst),
			want:   issueFile("New", entryOctFirst, entryOctLast),
		},
		{
			name:   "both edit the title, Log merged inside the conflict",
			base:   issueFile("T", entryOctFirst),
			ours:   issueFile("Ours", entryOctFirst, entryOctLast),
			theirs: issueFile("Theirs", entryOctFirst, entryOctTenth),
			want: conflict(
				issueFile("Ours", entryOctFirst, entryOctTenth, entryOctLast),
				issueFile("Theirs", entryOctFirst, entryOctTenth, entryOctLast),
			),
			wantConflict: true,
		},
		{
			name:   "multi-line entries",
			base:   issueFile("T", entryOctFirst+"\n  more on that"),
			ours:   issueFile("T", entryOctFirst+"\n  more on that", entryOctLast+"\n  in two places"),
			theirs: issueFile("T", entryOctFirst+"\n  more on that", entryOctTenth+"\n  by hand"),
			want: issueFile(
				"T",
				entryOctFirst+"\n  more on that",
				entryOctTenth+"\n  by hand",
				entryOctLast+"\n  in two places",
			),
		},
		{
			name:   "an entry deleted on one side stays deleted",
			base:   issueFile("T", entryOctFirst, entryOctFifth),
			ours:   issueFile("T", entryOctFirst, entryOctFifth, entryOctLast),
			theirs: issueFile("T", entryOctFifth, entryOctTenth),
			want:   issueFile("T", entryOctFifth, entryOctTenth, entryOctLast),
		},
		{
			name:   "a Log not last",
			base:   "---\ntitle: T\n---\n\n## Log\n\n" + entryOctFirst + "\n\n## Notes\n\nKeep.\n",
			ours:   "---\ntitle: T\n---\n\n## Log\n\n" + entryOctFirst + "\n" + entryOctLast + "\n\n## Notes\n\nKeep.\n",
			theirs: "---\ntitle: T\n---\n\n## Log\n\n" + entryOctFirst + "\n" + entryOctTenth + "\n\n## Notes\n\nEdited.\n",
			want: "---\ntitle: T\n---\n\n## Log\n\n" + entryOctFirst + "\n" + entryOctTenth + "\n" +
				entryOctLast + "\n\n## Notes\n\nEdited.\n",
		},
		{
			name:   "CRLF keeps CRLF",
			base:   crlf(issueFile("T", entryOctFirst+"\n  more")),
			ours:   crlf(issueFile("T", entryOctFirst+"\n  more", entryOctLast)),
			theirs: crlf(issueFile("New", entryOctFirst+"\n  more", entryOctTenth)),
			want:   crlf(issueFile("New", entryOctFirst+"\n  more", entryOctTenth, entryOctLast)),
		},
		{
			name:   "out-of-order base entries kept when only ours appends",
			base:   issueFile("T", entryOctFifth, entryOctFirst),
			ours:   issueFile("T", entryOctFifth, entryOctFirst, entryOctLast),
			theirs: issueFile("New", entryOctFifth, entryOctFirst),
			want:   issueFile("New", entryOctFifth, entryOctFirst, entryOctLast),
		},
		{
			name:   "a blank line inside an entry",
			base:   issueFile("T", entryOctFirst+"\n\n  second paragraph"),
			ours:   issueFile("T", entryOctFirst+"\n\n  second paragraph", entryOctLast),
			theirs: issueFile("T", entryOctFirst+"\n\n  second paragraph", entryOctTenth),
			want: issueFile(
				"T", entryOctFirst+"\n\n  second paragraph", entryOctTenth, entryOctLast,
			),
		},
		{
			name:   "a non-entry line sends the whole file to the text merge",
			base:   issueFile("T", entryOctFirst),
			ours:   issueFile("T", entryOctFirst, entryOctLast),
			theirs: issueFile("T", entryOctFirst, "### Details", entryOctTenth),
			want: conflict(
				issueFile("T", entryOctFirst, entryOctLast),
				issueFile("T", entryOctFirst, "### Details", entryOctTenth),
			),
			wantConflict: true,
		},
		{
			name: "a hunk spanning the placeholder holds the merged Log twice",
			base: "---\ntitle: T\n---\n\n## Log\n\n" + entryOctFirst + "\n\n## Notes\n\nKeep.\n",
			ours: "---\ntitle: T\n---\n\n## Log\n\n" + entryOctFirst + "\n" + entryOctLast +
				"\n\n## Notes\n\nOurs.\n",
			theirs: "---\ntitle: T\n---\n\n## Log\n\n" + entryOctFirst + "\n" + entryOctTenth +
				"\n\n## Notes\n\nTheirs.\n",
			want: conflict(
				"---\ntitle: T\n---\n\n## Log\n\n"+entryOctFirst+"\n"+entryOctTenth+"\n"+entryOctLast+
					"\n\n## Notes\n\nOurs.\n",
				"---\ntitle: T\n---\n\n## Log\n\n"+entryOctFirst+"\n"+entryOctTenth+"\n"+entryOctLast+
					"\n\n## Notes\n\nTheirs.\n",
			),
			wantConflict: true,
		},
		{
			name:   "a fenced example of a Log above the real one stays out of the merge",
			base:   fencedLogExampleFile(entryOctFirst),
			ours:   fencedLogExampleFile(entryOctFirst, entryOctLast),
			theirs: fencedLogExampleFile(entryOctFirst, entryOctTenth),
			want:   fencedLogExampleFile(entryOctFirst, entryOctTenth, entryOctLast),
		},
		{
			name:   "only one side has a Log",
			base:   "---\ntitle: T\n---\n\nBody.\n",
			ours:   "---\ntitle: New\n---\n\nBody.\n\n## Log\n\n" + entryOctLast + "\n",
			theirs: "---\ntitle: T\n---\n\nBody.\n",
			want:   "---\ntitle: New\n---\n\nBody.\n\n## Log\n\n" + entryOctLast + "\n",
		},
		{
			name:   "a Log one side deleted while the other appended goes to the text merge",
			base:   issueFile("T", entryOctFirst),
			ours:   "---\ntitle: T\nstatus: open\n---\n\nBody.\n\n",
			theirs: issueFile("T", entryOctFirst, entryOctTenth),
			want: conflict(
				"---\ntitle: T\nstatus: open\n---\n\nBody.\n\n",
				issueFile("T", entryOctFirst, entryOctTenth),
			),
			wantConflict: true,
		},
		{
			name:         "no side has a Log",
			base:         "---\ntitle: T\n---\n\nBody.\n",
			ours:         "---\ntitle: T\n---\n\nEdited.\n",
			theirs:       "---\ntitle: New\n---\n\nBody.\n",
			want:         conflict("---\ntitle: T\n---\n\nEdited.\n", "---\ntitle: New\n---\n\nBody.\n"),
			wantConflict: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, hasConflict, err := issue.MergeFile(
				[]byte(tc.base), []byte(tc.ours), []byte(tc.theirs), fakeTextMerge,
			)

			if err != nil {
				t.Fatalf("MergeFile: %v", err)
			}
			if string(got) != tc.want || hasConflict != tc.wantConflict {
				t.Errorf(
					"MergeFile = %q, conflict %v\nwant        %q, conflict %v",
					got, hasConflict, tc.want, tc.wantConflict,
				)
			}
		})
	}
}

func TestMergeFile_TextMergeFails_ReturnsItsError(t *testing.T) {
	failure := errors.New("merge failed")
	failingMerge := func(base []byte, ours []byte, theirs []byte) ([]byte, bool, error) {
		return nil, false, failure
	}

	_, _, err := issue.MergeFile(
		[]byte(issueFile("T", entryOctFirst)),
		[]byte(issueFile("T", entryOctFirst, entryOctLast)),
		[]byte(issueFile("T", entryOctFirst, entryOctTenth)),
		failingMerge,
	)

	if !errors.Is(err, failure) {
		t.Errorf("err = %v, want %v", err, failure)
	}
}

// crlf returns text with every LF line ending turned into CRLF.
func crlf(text string) string {
	return strings.ReplaceAll(text, "\n", "\r\n")
}

// logMergePlaceholder spells out the unexported placeholder MergeFile swaps in for the Log.
const logMergePlaceholder = "capcomLogMergePlaceholder9f4c2e7a1b"

// issueFileWithBody returns an issue file whose body is body and whose Log, its last section,
// holds entries.
func issueFileWithBody(body string, entries ...string) string {
	return "---\ntitle: T\nstatus: open\n---\n\n" + body + "\n\n## Log\n\n" +
		strings.Join(entries, "\n") + "\n"
}

func TestMergeFile_VersionHoldsPlaceholder_FallsBackToTextMerge(t *testing.T) {
	bodyWithPlaceholder := "Body names " + logMergePlaceholder + "."
	entryWithPlaceholder := "- 2026-10-05 Carol: quoted " + logMergePlaceholder
	cases := []struct {
		name   string
		base   string
		ours   string
		theirs string
	}{
		{
			name:   "every body holds it",
			base:   issueFileWithBody(bodyWithPlaceholder, entryOctFirst),
			ours:   issueFileWithBody(bodyWithPlaceholder, entryOctFirst, entryOctTenth),
			theirs: issueFileWithBody(bodyWithPlaceholder, entryOctFirst, entryOctLast),
		},
		{
			name:   "only base's body holds it",
			base:   issueFileWithBody(bodyWithPlaceholder, entryOctFirst),
			ours:   issueFileWithBody("Body.", entryOctFirst, entryOctTenth),
			theirs: issueFileWithBody("Body.", entryOctFirst, entryOctLast),
		},
		{
			name:   "only ours's body holds it",
			base:   issueFileWithBody("Body.", entryOctFirst),
			ours:   issueFileWithBody(bodyWithPlaceholder, entryOctFirst, entryOctTenth),
			theirs: issueFileWithBody("Body.", entryOctFirst, entryOctLast),
		},
		{
			name:   "only theirs's body holds it",
			base:   issueFileWithBody("Body.", entryOctFirst),
			ours:   issueFileWithBody("Body.", entryOctFirst, entryOctTenth),
			theirs: issueFileWithBody(bodyWithPlaceholder, entryOctFirst, entryOctLast),
		},
		{
			name:   "an entry every Log shares holds it",
			base:   issueFileWithBody("Body.", entryWithPlaceholder),
			ours:   issueFileWithBody("Body.", entryWithPlaceholder, entryOctTenth),
			theirs: issueFileWithBody("Body.", entryWithPlaceholder, entryOctLast),
		},
		{
			name:   "an entry only ours added holds it",
			base:   issueFileWithBody("Body.", entryOctFirst),
			ours:   issueFileWithBody("Body.", entryOctFirst, entryWithPlaceholder),
			theirs: issueFileWithBody("Body.", entryOctFirst, entryOctLast),
		},
		{
			name:   "an entry only theirs added holds it",
			base:   issueFileWithBody("Body.", entryOctFirst),
			ours:   issueFileWithBody("Body.", entryOctFirst, entryOctTenth),
			theirs: issueFileWithBody("Body.", entryOctFirst, entryWithPlaceholder),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, wantConflict, _ := fakeTextMerge([]byte(tc.base), []byte(tc.ours), []byte(tc.theirs))

			got, hasConflict, err := issue.MergeFile(
				[]byte(tc.base), []byte(tc.ours), []byte(tc.theirs), fakeTextMerge,
			)

			if err != nil {
				t.Fatalf("MergeFile: %v", err)
			}
			if string(got) != string(want) || hasConflict != wantConflict {
				t.Errorf(
					"MergeFile = %q, conflict %v\nwant the text merge alone %q, conflict %v",
					got, hasConflict, want, wantConflict,
				)
			}
		})
	}
}

func TestLogEntries_IssueFiles(t *testing.T) {
	multiLine := entryOctFifth + "\n  second line"
	cases := []struct {
		name            string
		data            string
		wantEntries     []string
		wantIsEntryOnly bool
	}{
		{
			name:            "entries",
			data:            issueFile("T", entryOctFirst, multiLine),
			wantEntries:     []string{entryOctFirst, multiLine},
			wantIsEntryOnly: true,
		},
		{
			name:            "CRLF line endings",
			data:            crlf(issueFile("T", entryOctFirst, multiLine)),
			wantEntries:     []string{entryOctFirst, multiLine},
			wantIsEntryOnly: true,
		},
		{
			name:            "Log quoted in a fence above the real one",
			data:            fencedLogExampleFile(entryOctTenth),
			wantEntries:     []string{entryOctTenth},
			wantIsEntryOnly: true,
		},
		{
			name:            "no Log section",
			data:            "---\ntitle: T\nstatus: open\n---\n\nBody.\n",
			wantEntries:     nil,
			wantIsEntryOnly: true,
		},
		{
			name:            "free text in the Log",
			data:            issueFile("T", entryOctFirst, "not an entry"),
			wantEntries:     nil,
			wantIsEntryOnly: false,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			entries, isEntryOnly := issue.LogEntries([]byte(test.data))

			if !slices.Equal(entries, test.wantEntries) || isEntryOnly != test.wantIsEntryOnly {
				t.Errorf(
					"LogEntries = %q, %v; want %q, %v",
					entries, isEntryOnly, test.wantEntries, test.wantIsEntryOnly,
				)
			}
		})
	}
}
