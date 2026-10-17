package issue_test

import (
	"errors"
	"reflect"
	"strings"
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

func TestIssue_AppendLog_FencedCodeAroundTheLog_LandsInTheLogBounds(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "nested fences hide a Log above the real one",
			body: "\n````markdown\n```\n## Log\n```\n````\n\n## Log\n\n- 2026-10-10 Bob: seen\n",
			want: "\n````markdown\n```\n## Log\n```\n````\n\n## Log\n\n- 2026-10-10 Bob: seen\n" +
				"- 2026-10-11 Alice: fixed\n",
		},
		{
			name: "unclosed longer opener above a closed fence",
			body: "\n````\n```\n## Log\n```\n\n## Log\n\n- 2026-10-10 Bob: seen\n",
			want: "\n````\n```\n## Log\n```\n\n## Log\n\n- 2026-10-10 Bob: seen\n" +
				"- 2026-10-11 Alice: fixed\n",
		},
		{
			name: "longer closer closes the fence",
			body: "\n```\n## Log\n`````\n\n## Log\n\n- 2026-10-10 Bob: seen\n",
			want: "\n```\n## Log\n`````\n\n## Log\n\n- 2026-10-10 Bob: seen\n" +
				"- 2026-10-11 Alice: fixed\n",
		},
		{
			name: "backtick line with a backtick in its info string opens no fence",
			body: "\n```inline``` quoted\n```\n## Log\n```\n\n## Log\n\n- 2026-10-10 Bob: seen\n",
			want: "\n```inline``` quoted\n```\n## Log\n```\n\n## Log\n\n- 2026-10-10 Bob: seen\n" +
				"- 2026-10-11 Alice: fixed\n",
		},
		{
			name: "tilde fence holds a backtick line",
			body: "\n~~~\n## Log\n```\n~~~\n\n## Log\n\n- 2026-10-10 Bob: seen\n",
			want: "\n~~~\n## Log\n```\n~~~\n\n## Log\n\n- 2026-10-10 Bob: seen\n" +
				"- 2026-10-11 Alice: fixed\n",
		},
		{
			name: "unclosed tilde opener above a backtick fence",
			body: "\n~~~\n```\n## Log\n```\n\n## Log\n\n- 2026-10-10 Bob: seen\n",
			want: "\n~~~\n```\n## Log\n```\n\n## Log\n\n- 2026-10-10 Bob: seen\n" +
				"- 2026-10-11 Alice: fixed\n",
		},
		{
			name: "unclosed opener inside the Log hides no section end",
			body: "\n## Log\n\n- 2026-10-10 Bob: seen\n\n```x\n\n## Notes\n\nKeep.\n",
			want: "\n## Log\n\n- 2026-10-10 Bob: seen\n\n```x\n- 2026-10-11 Alice: fixed\n" +
				"\n## Notes\n\nKeep.\n",
		},
		{
			name: "fence spanning the next heading keeps the Log open",
			body: "\n## Log\n\n- 2026-10-10 Bob: seen\n\n```\n## Notes\n```\n\n## Notes\n",
			want: "\n## Log\n\n- 2026-10-10 Bob: seen\n\n```\n## Notes\n```\n" +
				"- 2026-10-11 Alice: fixed\n\n## Notes\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := issue.Issue{Body: tc.body}.AppendLog(logDate, "Alice", "fixed")

			if changed.Body != tc.want {
				t.Errorf("body = %q\nwant   %q", changed.Body, tc.want)
			}
		})
	}
}

// unclosedOpenerCount is how many unclosed fence openers precede the Log in the linear-time test:
// enough that a pass per opener takes seconds.
const unclosedOpenerCount = 50_000

// linearTimeBound is the time the linear-time test allows AppendLog, far above a single pass.
const linearTimeBound = time.Second

func TestIssue_AppendLog_ManyUnclosedOpeners_FindsTheLogInLinearTime(t *testing.T) {
	openers := strings.Repeat("```x\n", unclosedOpenerCount)
	body := openers + "\n## Log\n\n- 2026-10-10 Bob: seen\n"
	want := body + "- 2026-10-11 Alice: fixed\n"

	began := time.Now()
	changed := issue.Issue{Body: body}.AppendLog(logDate, "Alice", "fixed")
	elapsed := time.Since(began)

	if changed.Body != want {
		t.Errorf("body ends %q\nwant      %q", changed.Body[len(openers):], want[len(openers):])
	}
	if elapsed > linearTimeBound {
		t.Errorf("AppendLog took %v, want at most %v", elapsed, linearTimeBound)
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

// standingIssue returns a standing Issue at status, as lifecycleIssue builds it.
func standingIssue(status string) issue.Issue {
	subject := lifecycleIssue(status)
	subject.IsStanding = true
	return subject
}

func TestSetStatus_StandingIssue_RefusesOnlyTheDoingCategory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		from    string
		to      string
		want    string
		wantErr error
	}{
		{name: "into doing", from: "open", to: "active", want: "open", wantErr: issue.ErrStanding},
		{name: "waiting into doing", from: "parked", to: "active", want: "parked", wantErr: issue.ErrStanding},
		{name: "into waiting", from: "open", to: "parked", want: "parked"},
		{name: "same status", from: "active", to: "active", want: "active"},
		{name: "into done", from: "open", to: "done", want: "open", wantErr: issue.ErrEntersClosed},
		{name: "out of done", from: "done", to: "active", want: "done", wantErr: issue.ErrLeavesClosed},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			subject := standingIssue(current.from)

			got, err := issue.SetStatus(subject, config.Default(), current.to)

			if !errors.Is(err, current.wantErr) {
				t.Fatalf("SetStatus(%q -> %q) error = %v, want %v", current.from, current.to, err, current.wantErr)
			}
			if got.Status != current.want || !got.IsStanding {
				t.Errorf("SetStatus(%q -> %q) = %q standing %v, want %q standing", current.from, current.to,
					got.Status, got.IsStanding, current.want)
			}
		})
	}
}

func TestSetStatus_StandingIntoDoing_NamesTheTargetStatus(t *testing.T) {
	t.Parallel()

	_, err := issue.SetStatus(standingIssue("open"), config.Default(), "active")

	want := issue.StatusError{Status: "active", Category: config.CategoryDoing, Err: issue.ErrStanding}
	var refusal *issue.StatusError
	if !errors.As(err, &refusal) || *refusal != want {
		t.Errorf("SetStatus error = %#v, want %#v", err, want)
	}
}

func TestSetStatus_StandingBetweenTodoStatuses_Moves(t *testing.T) {
	t.Parallel()
	cfg, err := config.Parse("config.yaml", []byte("statuses:\n  triage: todo\n  open: todo\n  active: doing\n  done: done\n"))
	if err != nil {
		t.Fatalf("config.Parse: %v", err)
	}

	got, err := issue.SetStatus(standingIssue("triage"), cfg, "open")

	if err != nil || got.Status != "open" || !got.IsStanding {
		t.Errorf("SetStatus(triage -> open) = %q standing %v, %v; want open standing", got.Status, got.IsStanding, err)
	}
}

func TestClose_StandingIssue_IsRefusedAfterTheClosedRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		from    string
		status  string
		wantErr error
	}{
		{name: "open", from: "open", status: "done", wantErr: issue.ErrStanding},
		{name: "already done", from: "done", status: "dropped", wantErr: issue.ErrAlreadyClosed},
		{name: "open status", from: "open", status: "active", wantErr: issue.ErrNotClosed},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			subject := standingIssue(current.from)

			got, err := issue.Close(subject, config.Default(), current.status, "shipped", closedAt)

			if !errors.Is(err, current.wantErr) {
				t.Fatalf("Close error = %v, want %v", err, current.wantErr)
			}
			if !reflect.DeepEqual(got, subject) {
				t.Errorf("Close changed the issue: %#v, want %#v", got, subject)
			}
		})
	}
}

func TestSetStanding_Mark_FollowsTheCategory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		from       string
		isStanding bool
		want       bool
		wantErr    error
	}{
		{name: "mark todo", from: "open", isStanding: true, want: true},
		{name: "mark waiting", from: "parked", isStanding: true, want: true},
		{name: "mark doing", from: "active", isStanding: true, wantErr: issue.ErrStandingNotOpen},
		{name: "mark done", from: "done", isStanding: true, wantErr: issue.ErrStandingNotOpen},
		{name: "mark dropped", from: "dropped", isStanding: true, wantErr: issue.ErrStandingNotOpen},
		{name: "clear todo", from: "open"},
		{name: "clear doing", from: "active"},
		{name: "clear done", from: "done"},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			subject := lifecycleIssue(current.from)
			subject.IsStanding = !current.isStanding

			got, err := issue.SetStanding(subject, config.Default(), current.isStanding)

			if !errors.Is(err, current.wantErr) {
				t.Fatalf("SetStanding(%q, %v) error = %v, want %v", current.from, current.isStanding, err, current.wantErr)
			}
			if got.IsStanding != current.want || got.Status != current.from {
				t.Errorf("SetStanding(%q, %v) = %q standing %v, want %q standing %v", current.from, current.isStanding,
					got.Status, got.IsStanding, current.from, current.want)
			}
		})
	}
}

func TestSetStanding_Refusal_NamesTheCurrentStatus(t *testing.T) {
	t.Parallel()

	_, err := issue.SetStanding(lifecycleIssue("done"), config.Default(), true)

	want := issue.StatusError{Status: "done", Category: config.CategoryDone, Err: issue.ErrStandingNotOpen}
	var refusal *issue.StatusError
	if !errors.As(err, &refusal) || *refusal != want {
		t.Errorf("SetStanding error = %#v, want %#v", err, want)
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

func TestIssue_SplitLog_CutsTheLogSectionOut(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantRest string
		wantLog  string
	}{
		{
			name:     "no log",
			body:     "Body.\n",
			wantRest: "Body.\n",
			wantLog:  "",
		},
		{
			name:     "log at the end",
			body:     "Body.\n\n## Log\n\n- 2026-10-11 bob: one\n",
			wantRest: "Body.\n\n",
			wantLog:  "## Log\n\n- 2026-10-11 bob: one\n",
		},
		{
			name:     "section after the log",
			body:     "Body.\n## Log\n- 2026-10-11 bob: one\n## Notes\nLater.\n",
			wantRest: "Body.\n## Notes\nLater.\n",
			wantLog:  "## Log\n- 2026-10-11 bob: one\n",
		},
		{
			name:     "log heading inside a fence",
			body:     "```\n## Log\n```\n",
			wantRest: "```\n## Log\n```\n",
			wantLog:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rest, log := issue.Issue{Body: tc.body}.SplitLog()

			if rest != tc.wantRest || log != tc.wantLog {
				t.Errorf("SplitLog() = %q, %q, want %q, %q", rest, log, tc.wantRest, tc.wantLog)
			}
		})
	}
}
