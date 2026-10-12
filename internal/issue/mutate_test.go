package issue_test

import (
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/issue"
)

// logDate is the date every Log entry in these tests is written on.
var logDate = time.Date(2026, 10, 11, 15, 0, 0, 0, time.UTC)

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
