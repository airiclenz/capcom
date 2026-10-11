package issue_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/issue"
)

// logDate is the date every Log entry in these tests is written on.
var logDate = time.Date(2026, 10, 11, 15, 0, 0, 0, time.UTC)

func TestIssue_WithLabels_AddsMissingThenRemoves(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		add    []string
		remove []string
		want   []string
	}{
		{name: "add to none", add: []string{"auth"}, want: []string{"auth"}},
		{name: "add keeps order and skips present", labels: []string{"auth"}, add: []string{"ui", "auth"}, want: []string{"auth", "ui"}},
		{name: "remove present", labels: []string{"auth", "ui"}, remove: []string{"auth"}, want: []string{"ui"}},
		{name: "remove absent", labels: []string{"ui"}, remove: []string{"auth"}, want: []string{"ui"}},
		{name: "remove last", labels: []string{"ui"}, remove: []string{"ui"}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := issue.Issue{Labels: slices.Clone(tc.labels)}

			changed := original.WithLabels(tc.add, tc.remove)

			if !slices.Equal(changed.Labels, tc.want) || (tc.want == nil && changed.Labels != nil) {
				t.Errorf("labels = %#v, want %#v", changed.Labels, tc.want)
			}
			if !slices.Equal(original.Labels, tc.labels) {
				t.Errorf("original labels changed to %#v", original.Labels)
			}
		})
	}
}

func TestIssue_AppendBody_AddsBlockAfterBlankLine(t *testing.T) {
	cases := []struct {
		name string
		body string
		text string
		want string
	}{
		{name: "empty body", body: "", text: "First.", want: "\nFirst.\n"},
		{name: "after paragraph", body: "\nIntro.\n", text: "More.\n", want: "\nIntro.\n\nMore.\n"},
		{name: "no final newline", body: "\nIntro.", text: "More.", want: "\nIntro.\n\nMore.\n"},
		{name: "literal at sign", body: "\nIntro.\n", text: "@alice", want: "\nIntro.\n\n@alice\n"},
		{name: "crlf text", body: "\nIntro.\n", text: "a\r\nb\r\n", want: "\nIntro.\n\na\nb\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := issue.Issue{Body: tc.body}.AppendBody(tc.text)

			if changed.Body != tc.want {
				t.Errorf("body = %q, want %q", changed.Body, tc.want)
			}
		})
	}
}

func TestIssue_ReplaceBody_OneMatch_Replaces(t *testing.T) {
	original := issue.Issue{Body: "\nThe old text stays old.\n"}

	changed, err := original.ReplaceBody("The old", "The new")

	if err != nil || changed.Body != "\nThe new text stays old.\n" {
		t.Errorf("body = %q, err = %v", changed.Body, err)
	}
}

func TestIssue_ReplaceBody_NotExactlyOnce_Fails(t *testing.T) {
	for _, old := range []string{"absent", "old", ""} {
		t.Run(old, func(t *testing.T) {
			original := issue.Issue{Body: "\nThe old text stays old.\n"}

			_, err := original.ReplaceBody(old, "new")

			if !errors.Is(err, issue.ErrReplaceMatch) {
				t.Errorf("err = %v, want ErrReplaceMatch", err)
			}
		})
	}
}

func TestIssue_AppendLog_AddsEntryToLogSection(t *testing.T) {
	cases := []struct {
		name string
		body string
		text string
		want string
	}{
		{
			name: "creates section in empty body",
			text: "started",
			want: "\n## Log\n\n- 2026-10-11 Alice: started\n",
		},
		{
			name: "creates section after body",
			body: "\nIntro.\n\n## Acceptance\n\n- works\n",
			text: "started",
			want: "\nIntro.\n\n## Acceptance\n\n- works\n\n## Log\n\n- 2026-10-11 Alice: started\n",
		},
		{
			name: "appends after last entry",
			body: "\n## Log\n\n- 2026-10-10 Bob: seen\n\n",
			text: "fixed",
			want: "\n## Log\n\n- 2026-10-10 Bob: seen\n- 2026-10-11 Alice: fixed\n\n",
		},
		{
			name: "section followed by another",
			body: "\n## Log\n\n- 2026-10-10 Bob: seen\n\n## Notes\n\nKeep.\n",
			text: "fixed",
			want: "\n## Log\n\n- 2026-10-10 Bob: seen\n- 2026-10-11 Alice: fixed\n\n## Notes\n\nKeep.\n",
		},
		{
			name: "heading only",
			body: "\n## Log\n",
			text: "first",
			want: "\n## Log\n\n- 2026-10-11 Alice: first\n",
		},
		{
			name: "multi-line text",
			body: "",
			text: "line one\r\nline two\n",
			want: "\n## Log\n\n- 2026-10-11 Alice: line one\n  line two\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := issue.Issue{Body: tc.body}.AppendLog(logDate, "Alice", tc.text)

			if changed.Body != tc.want {
				t.Errorf("body = %q\nwant   %q", changed.Body, tc.want)
			}
		})
	}
}
