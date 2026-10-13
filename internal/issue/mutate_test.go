package issue_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
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
			name: "fenced example of a Log above the real one",
			body: "\nExample:\n\n```markdown\n## Log\n\n- 2026-10-01 Bob: quoted\n```\n\n" +
				"## Log\n\n- 2026-10-10 Bob: seen\n",
			text: "fixed",
			want: "\nExample:\n\n```markdown\n## Log\n\n- 2026-10-01 Bob: quoted\n```\n\n" +
				"## Log\n\n- 2026-10-10 Bob: seen\n- 2026-10-11 Alice: fixed\n",
		},
		{
			name: "unclosed fence above the Log appends to it",
			body: "\nIntro.\n\n```\nnever closed\n\n## Log\n\n- 2026-10-10 Bob: seen\n",
			text: "fixed",
			want: "\nIntro.\n\n```\nnever closed\n\n## Log\n\n- 2026-10-10 Bob: seen\n" +
				"- 2026-10-11 Alice: fixed\n",
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

// closedAt is the closed time of every closed Issue in these tests.
var closedAt = time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)

// lifecycleIssue returns an Issue at status; a closed one carries a closed time and reason.
func lifecycleIssue(status string) issue.Issue {
	subject := issue.Issue{Title: "Login flow", Status: status, Body: "\nIntro.\n"}
	if category, _ := config.Default().CategoryOf(status); category.IsClosed() {
		subject.Closed = closedAt
		subject.Reason = "shipped"
	}
	return subject
}

// authorAlice is an author func that names Alice.
func authorAlice() (string, error) {
	return "Alice", nil
}

func TestSetStatus_Transitions_FollowTheClosedCategories(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		from    string
		to      string
		want    string
		wantErr error
	}{
		{name: "same status", from: "done", to: "done", want: "done"},
		{name: "open to open", from: "open", to: "active", want: "active"},
		{name: "into done", from: "open", to: "done", want: "open", wantErr: issue.ErrEntersClosed},
		{name: "out of done", from: "done", to: "open", want: "done", wantErr: issue.ErrLeavesClosed},
		{name: "done to dropped", from: "done", to: "dropped", want: "done", wantErr: issue.ErrLeavesClosed},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			subject := lifecycleIssue(current.from)

			got, err := issue.SetStatus(subject, config.Default(), current.to)

			if !errors.Is(err, current.wantErr) {
				t.Fatalf("SetStatus(%q -> %q) error = %v, want %v", current.from, current.to, err, current.wantErr)
			}
			if got.Status != current.want {
				t.Errorf("SetStatus(%q -> %q) status = %q, want %q", current.from, current.to, got.Status, current.want)
			}
		})
	}
}

func TestSetStatus_Refusal_NamesTheJudgedStatusAndCategory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		from string
		to   string
		want issue.StatusError
	}{
		{
			name: "into a closed category names the target",
			from: "open",
			to:   "dropped",
			want: issue.StatusError{Status: "dropped", Category: config.CategoryDropped, Err: issue.ErrEntersClosed},
		},
		{
			name: "out of a closed category names the current status",
			from: "done",
			to:   "open",
			want: issue.StatusError{Status: "done", Category: config.CategoryDone, Err: issue.ErrLeavesClosed},
		},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()

			_, err := issue.SetStatus(lifecycleIssue(current.from), config.Default(), current.to)

			var refusal *issue.StatusError
			if !errors.As(err, &refusal) || *refusal != current.want {
				t.Errorf("SetStatus(%q -> %q) error = %#v, want %#v", current.from, current.to, err, current.want)
			}
		})
	}
}

func TestClose_OpenIssue_SetsStatusClosedTimeAndReason(t *testing.T) {
	t.Parallel()
	subject := lifecycleIssue("active")

	got, err := issue.Close(subject, config.Default(), "dropped", "superseded", closedAt)

	if err != nil {
		t.Fatalf("Close error = %v", err)
	}
	want := subject
	want.Status, want.Closed, want.Reason = "dropped", closedAt, "superseded"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Close = %#v, want %#v", got, want)
	}
}

func TestClose_Refused_ReturnsTheIssueUnchanged(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		from    string
		status  string
		wantErr error
	}{
		{name: "already closed", from: "done", status: "dropped", wantErr: issue.ErrAlreadyClosed},
		{name: "open status", from: "open", status: "active", wantErr: issue.ErrNotClosed},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			subject := lifecycleIssue(current.from)

			got, err := issue.Close(subject, config.Default(), current.status, "again", closedAt.Add(time.Hour))

			if !errors.Is(err, current.wantErr) {
				t.Fatalf("Close error = %v, want %v", err, current.wantErr)
			}
			if !reflect.DeepEqual(got, subject) {
				t.Errorf("Close changed the issue: %#v, want %#v", got, subject)
			}
		})
	}
}

func TestReopen_ClosedIssue_ClearsClosingFieldsAndLogsTheReason(t *testing.T) {
	t.Parallel()
	subject := lifecycleIssue("dropped")

	got, err := issue.Reopen(subject, config.Default(), "parked", "needed after all", authorAlice, logDate)

	if err != nil {
		t.Fatalf("Reopen error = %v", err)
	}
	want := subject
	want.Status, want.Closed, want.Reason = "parked", time.Time{}, ""
	want.Body = "\nIntro.\n\n## Log\n\n- 2026-10-11 Alice: reopened: needed after all\n"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Reopen = %#v, want %#v", got, want)
	}
}

func TestReopen_Refused_NeverCallsAuthor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		from    string
		status  string
		wantErr error
	}{
		{name: "open issue", from: "active", status: "open", wantErr: issue.ErrNotClosed},
		{name: "closed target status", from: "done", status: "dropped", wantErr: issue.ErrEntersClosed},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			subject := lifecycleIssue(current.from)
			isAuthorCalled := false
			author := func() (string, error) {
				isAuthorCalled = true
				return "Alice", nil
			}

			got, err := issue.Reopen(subject, config.Default(), current.status, "why", author, logDate)

			if !errors.Is(err, current.wantErr) {
				t.Fatalf("Reopen error = %v, want %v", err, current.wantErr)
			}
			if isAuthorCalled {
				t.Error("Reopen called author before refusing")
			}
			if !reflect.DeepEqual(got, subject) {
				t.Errorf("Reopen changed the issue: %#v, want %#v", got, subject)
			}
		})
	}
}

func TestReopen_OpenIssue_StatusErrorCarriesItsStatusAndCategory(t *testing.T) {
	t.Parallel()

	_, err := issue.Reopen(lifecycleIssue("parked"), config.Default(), "open", "why", authorAlice, logDate)

	var refusal *issue.StatusError
	want := issue.StatusError{Status: "parked", Category: config.CategoryWaiting, Err: issue.ErrNotClosed}
	if !errors.As(err, &refusal) || *refusal != want {
		t.Errorf("Reopen error = %#v, want %#v", err, want)
	}
}

func TestReopen_AuthorFails_ReturnsItsErrorUnwrapped(t *testing.T) {
	t.Parallel()
	failure := errors.New("no git author")
	author := func() (string, error) { return "", failure }

	_, err := issue.Reopen(lifecycleIssue("done"), config.Default(), "open", "why", author, logDate)

	if err != failure {
		t.Errorf("Reopen error = %v, want the author's error %v", err, failure)
	}
}
