package issue

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/rawfm"
)

// LogHeading is the line that opens an issue's Log section.
const LogHeading = "## Log"

// logContinuationIndent indents the second and later lines of a multi-line Log entry, keeping them
// inside the entry's list item.
const logContinuationIndent = "  "

// AppendLog returns a copy of the issue with the entry `- YYYY-MM-DD <author>: <text>` appended
// as the last item of its Log section, the section that opens with a LogHeading line and runs to
// the next level-one or level-two heading. A body without that section gets one at its end. date
// is written as its calendar date in its own location; the second and later lines of a multi-line
// text are indented to stay inside the entry. This is the single writer of Log entries: capcom log
// and reopen both append through it.
func (i Issue) AppendLog(date time.Time, author string, text string) Issue {
	lines := strings.Split(strings.TrimSpace(edit.NormalizeNewlines(text)), "\n")
	entry := fmt.Sprintf(
		"- %s %s: %s\n",
		date.Format(time.DateOnly),
		author,
		strings.Join(lines, "\n"+logContinuationIndent),
	)
	i.Body = appendLogEntry(i.Body, entry)
	return i
}

// appendLogEntry inserts entry after the last non-blank line of body's Log section, found by
// logBounds, or appends a new Log section holding it.
func appendLogEntry(body string, entry string) string {
	lines := strings.SplitAfter(body, "\n")
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, strings.TrimSuffix(line, "\n"))
	}
	start, end := logBounds(texts, 0)
	if start < 0 {
		return edit.AppendBlock(body, LogHeading+"\n\n"+entry)
	}

	last := end - 1
	for last > start && strings.TrimSpace(lines[last]) == "" {
		last--
	}

	before := edit.WithFinalNewline(strings.Join(lines[:last+1], ""))
	if last == start {
		// The section has only its heading; a blank line separates it from the first entry.
		before += "\n"
	}
	return before + entry + strings.Join(lines[last+1:], "")
}

// SplitLog cuts the issue's body in two: everything outside its Log section, and the section
// itself, from its LogHeading line up to the next level-one or level-two heading, as AppendLog
// finds it. log is "" when the body has no Log section; rest and log together hold every byte of
// the body.
func (i Issue) SplitLog() (rest string, log string) {
	lines := strings.SplitAfter(i.Body, "\n")
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, strings.TrimSuffix(line, "\n"))
	}
	start, end := logBounds(texts, 0)
	if start < 0 {
		return i.Body, ""
	}
	return strings.Join(lines[:start], "") + strings.Join(lines[end:], ""),
		strings.Join(lines[start:end], "")
}

// isSectionBoundary reports whether line is a level-one or level-two heading, which ends the Log
// section; deeper headings belong to it.
func isSectionBoundary(line string) bool {
	heading := strings.TrimRight(line, "\n")
	for _, prefix := range []string{"#", "##"} {
		if heading == prefix || strings.HasPrefix(heading, prefix+" ") {
			return true
		}
	}
	return false
}

// logBounds returns the index of the line that opens the Log section of lines, each without its
// line ending, and the index of the line that ends it: the first LogHeading line outside a fenced
// code block, and the first level-one or level-two heading after it outside one, or len(lines).
// start is -1 when lines hold no Log section. Fenced code blocks are followed from line bodyStart
// on, as rawfm.ClosedCodeLines finds them: a fence no later line closes hides nothing, so its
// opening line counts as plain text.
func logBounds(lines []string, bodyStart int) (start int, end int) {
	isCode := rawfm.ClosedCodeLines(lines, bodyStart)
	start = -1
	for index, line := range lines {
		if isCode[index] {
			continue
		}
		switch {
		case start < 0 && strings.TrimRight(line, " \t") == LogHeading:
			start = index
		case start >= 0 && isSectionBoundary(line):
			return start, index
		}
	}
	return start, len(lines)
}

// reopenLogPrefix opens the Log entry Reopen writes, ahead of the reason.
const reopenLogPrefix = "reopened: "

// The refusals of the status-category state machine (ADR 0003). Each reaches the caller wrapped in
// a *StatusError naming the status it is about.
var (
	// ErrAlreadyClosed refuses Close of an issue whose status is already in a Closed category.
	ErrAlreadyClosed = errors.New("the issue is already closed")
	// ErrNotClosed refuses Reopen of an issue that is not Closed, and Close to a status that is
	// not in a Closed category.
	ErrNotClosed = errors.New("the status is not in a closed category")
	// ErrEntersClosed refuses a status change into a Closed category, which only Close makes.
	ErrEntersClosed = errors.New("only close moves an issue into a closed category")
	// ErrLeavesClosed refuses a status change out of a Closed category, which only Reopen makes.
	ErrLeavesClosed = errors.New("only reopen moves an issue out of a closed category")
	// ErrStanding refuses Close of a standing issue, and a status change of one into the doing
	// category: a standing Parent only groups its children and is never worked on itself.
	ErrStanding = errors.New("a standing issue is never claimed or closed")
	// ErrStandingNotOpen refuses marking standing an issue that is Closed or in the doing category.
	ErrStandingNotOpen = errors.New("only an issue outside the doing and closed categories can be marked standing")
)

// StatusError is a refusal of SetStatus, SetStanding, Close or Reopen: Err is one of the sentinels above, and
// Status and Category name the status the rule was judged on and the category config gives it
// (empty for an undeclared status).
type StatusError struct {
	Status   string
	Category config.Category
	Err      error
}

// Error names the status, its category and the rule that refused it.
func (e *StatusError) Error() string {
	return fmt.Sprintf("status %q is in the %s category: %v", e.Status, e.Category, e.Err)
}

// Unwrap returns the sentinel, so errors.Is matches it.
func (e *StatusError) Unwrap() error {
	return e.Err
}

// SetStatus returns a copy of i moved to status to. The same status is a no-op. A move out of a
// Closed category is ErrLeavesClosed, judged on i's status; a move into one is ErrEntersClosed,
// judged on to: close and reopen own those moves, with the closed time, reason and Log entry they
// bring. A standing issue moved into the doing category is ErrStanding, judged on to; it may move
// between the other categories. On a refusal i is returned unchanged.
func SetStatus(i Issue, cfg config.Config, to string) (Issue, error) {
	if i.Status == to {
		return i, nil
	}
	from, _ := cfg.CategoryOf(i.Status)
	if from.IsClosed() {
		return i, &StatusError{Status: i.Status, Category: from, Err: ErrLeavesClosed}
	}
	target, _ := cfg.CategoryOf(to)
	if target.IsClosed() {
		return i, &StatusError{Status: to, Category: target, Err: ErrEntersClosed}
	}
	if i.IsStanding && target == config.CategoryDoing {
		return i, &StatusError{Status: to, Category: target, Err: ErrStanding}
	}
	i.Status = to
	return i, nil
}

// SetStanding returns a copy of i with its standing mark set to isStanding. Marking an issue that
// is Closed or in the doing category is ErrStandingNotOpen, judged on i's status; clearing the
// mark is always allowed. On a refusal i is returned unchanged.
func SetStanding(i Issue, cfg config.Config, isStanding bool) (Issue, error) {
	if isStanding {
		current, _ := cfg.CategoryOf(i.Status)
		if current.IsClosed() || current == config.CategoryDoing {
			return i, &StatusError{Status: i.Status, Category: current, Err: ErrStandingNotOpen}
		}
	}
	i.IsStanding = isStanding
	return i, nil
}

// Close returns a copy of i moved to status, closed at at for reason. An issue already Closed is
// ErrAlreadyClosed, judged on i's status; a status outside the Closed categories is ErrNotClosed;
// a standing issue is ErrStanding, judged on status, after both. at is stored as given, so the caller decides its precision and location. On a refusal i is
// returned unchanged.
func Close(i Issue, cfg config.Config, status string, reason string, at time.Time) (Issue, error) {
	current, _ := cfg.CategoryOf(i.Status)
	if current.IsClosed() {
		return i, &StatusError{Status: i.Status, Category: current, Err: ErrAlreadyClosed}
	}
	target, _ := cfg.CategoryOf(status)
	if !target.IsClosed() {
		return i, &StatusError{Status: status, Category: target, Err: ErrNotClosed}
	}
	if i.IsStanding {
		return i, &StatusError{Status: status, Category: target, Err: ErrStanding}
	}
	i.Status = status
	i.Closed = at
	i.Reason = reason
	return i, nil
}

// Reopen returns a copy of i moved out of its Closed category to status, with its closed time and
// reason cleared and the Log entry "reopened: <reason>" appended, dated at and attributed to the
// name author returns. An issue that is not Closed is ErrNotClosed, judged on i's status, and a
// status in a Closed category is ErrEntersClosed; both are checked before author runs. An error
// from author is returned as it is. On any failure i is returned unchanged.
func Reopen(
	i Issue,
	cfg config.Config,
	status string,
	reason string,
	author func() (string, error),
	at time.Time,
) (Issue, error) {
	current, _ := cfg.CategoryOf(i.Status)
	if !current.IsClosed() {
		return i, &StatusError{Status: i.Status, Category: current, Err: ErrNotClosed}
	}
	target, _ := cfg.CategoryOf(status)
	if target.IsClosed() {
		return i, &StatusError{Status: status, Category: target, Err: ErrEntersClosed}
	}
	name, err := author()
	if err != nil {
		return i, err
	}
	reopened := i
	reopened.Status = status
	reopened.Closed = time.Time{}
	reopened.Reason = ""
	return reopened.AppendLog(at, name, reopenLogPrefix+reason), nil
}
