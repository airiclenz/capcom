package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/recap"
)

// kindRecap is the --json kind of capcom recap.
const kindRecap = "recap"

// flagSince names the start of the window capcom recap reports on.
const flagSince = "since"

// The --since forms of capcom recap other than a git ref.
const (
	// defaultSince is the window recap reports on without --since.
	defaultSince = "7d"
	// sinceDateLayout is the layout of a --since date, read as local midnight.
	sinceDateLayout = "2006-01-02"
	// sinceOptionPrefix starts a value git would read as an option, never a ref.
	sinceOptionPrefix = "-"
)

// sinceDaysPattern matches a --since day count, <n>d.
var sinceDaysPattern = regexp.MustCompile(`^([0-9]+)d$`)

// The text of capcom recap.
const (
	recapClosedHeading        = "closed"
	recapFiledHeading         = "filed"
	recapStatusChangedHeading = "status changed"
	recapLogHeading           = "log"
	recapNotesAddedHeading    = "notes added"
	recapNotesChangedHeading  = "notes changed"
	recapNotesRetiredHeading  = "notes retired"
	// recapItemIndent indents an id line under its heading.
	recapItemIndent = "  "
	// recapEntryIndent indents the lines of a Log entry under its id line.
	recapEntryIndent = "    "
	// recapUnknownSinceHint is the command that lists the commits a --since ref can name.
	recapUnknownSinceHint = "git log --oneline -- .capcom"
)

// The help text of capcom recap.
var (
	recapLong = "List what changed in .capcom since a date or a commit: Issues closed (with the reason), " +
		"filed, whose status changed, and Log entries added; then Notes added, changed and retired.\n\n" +
		"--since takes a date YYYY-MM-DD (local midnight), a number of days <n>d, or a git ref; it " +
		"defaults to " + defaultSince + ". A date or day count starts at the last commit before it.\n\n" +
		"Only committed history counts: the snapshot of .capcom at the start is compared with HEAD, " +
		"by id. An Issue filed and closed in the window is both filed and closed.\n\n" +
		"An empty group is left out. --json gives kind recap with data {since, boundary, closed, " +
		"filed, status_changed, log, notes_added, notes_changed, notes_retired}."
	recapExample = "  capcom recap                       # what changed in the last 7 days\n" +
		"  capcom recap --since 2026-10-01    # since local midnight of that date\n" +
		"  capcom recap --since v1.2.0        # since a tag, branch or commit\n" +
		"  capcom recap --since 30d --json    # the versioned JSON envelope"
)

// recapData is the --json data of recap: the --since value as given, the commit the window starts
// after (empty when it starts before the first commit) and every group of the report.
type recapData struct {
	Since    string `json:"since"`
	Boundary string `json:"boundary"`
	recap.Report
}

// recapGroup is one group of the recap text: its heading, its entries and how one entry prints.
type recapGroup struct {
	heading string
	entries []recap.Entry
	detail  func(recap.Entry) string
}

// newRecapCommand lists what changed in .capcom between the commit --since names and HEAD, group
// by group, as recap.Compute reports it. An unknown --since is invalid, with a hint naming the git
// command that lists the commits it could name.
func (a *app) newRecapCommand() *cobra.Command {
	since := defaultSince
	cmd := &cobra.Command{
		Use:         "recap [--since <date|days|ref>]",
		Short:       "List what changed in .capcom since a date or a commit",
		Long:        recapLong,
		Example:     recapExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			boundary, err := a.recapBoundary(since)
			if err != nil {
				return err
			}
			report, err := recap.Compute(a.store, boundary)
			if errors.Is(err, recap.ErrUnknownBoundary) {
				return unknownSinceError(since)
			}
			if err != nil {
				return err
			}
			return a.emit(cmd, recapResult(since, boundary, report))
		}),
	}
	cmd.Flags().StringVar(&since, flagSince, defaultSince, "start of the window: YYYY-MM-DD, <n>d or a git ref")
	return cmd
}

// recapBoundary resolves since to the commit the recap starts after: for a date or a day count, the
// last commit on HEAD at or before that time, "" when there is none; else since names a commit
// itself. A value git would read as an option, or one naming no commit, is invalid.
func (a *app) recapBoundary(since string) (string, error) {
	start, isTime := sinceTime(since, time.Now())
	if isTime {
		return gitx.CommitBefore(a.store.WorktreeRoot, start)
	}
	if strings.HasPrefix(since, sinceOptionPrefix) {
		return "", unknownSinceError(since)
	}
	commit, err := gitx.ResolveCommit(a.store.WorktreeRoot, since)
	if err != nil {
		return "", err
	}
	if commit == "" {
		return "", unknownSinceError(since)
	}
	return commit, nil
}

// sinceTime reads since as a date, local midnight, or as <n>d, n days before now. isTime is false
// when since is neither, so it is a git ref.
func sinceTime(since string, now time.Time) (start time.Time, isTime bool) {
	if date, err := time.ParseInLocation(sinceDateLayout, since, time.Local); err == nil {
		return date, true
	}
	match := sinceDaysPattern.FindStringSubmatch(since)
	if match == nil {
		return time.Time{}, false
	}
	days, err := strconv.Atoi(match[1])
	if err != nil {
		return time.Time{}, false
	}
	return now.AddDate(0, 0, -days), true
}

// unknownSinceError is the failure of a --since that is no date, day count or commit.
func unknownSinceError(since string) *output.Error {
	failure := output.Errorf(
		output.CodeInvalid,
		"--%s %q: not a date (YYYY-MM-DD), a day count (<n>d) or a commit",
		flagSince,
		since,
	)
	failure.Hint = recapUnknownSinceHint
	return failure
}

// recapResult renders report: each group that holds an entry as its heading and count with one line
// per entry, or "nothing changed since <since>" when every group is empty. Each file the recap left
// out is a warning.
func recapResult(since, boundary string, report recap.Report) output.Result {
	groups := []recapGroup{
		{heading: recapClosedHeading, entries: report.Closed, detail: closedReason},
		{heading: recapFiledHeading, entries: report.Filed},
		{heading: recapStatusChangedHeading, entries: report.StatusChanged, detail: newStatus},
		{heading: recapLogHeading, entries: report.Log},
		{heading: recapNotesAddedHeading, entries: report.NotesAdded},
		{heading: recapNotesChangedHeading, entries: report.NotesChanged},
		{heading: recapNotesRetiredHeading, entries: report.NotesRetired},
	}
	var text strings.Builder
	for _, group := range groups {
		writeRecapGroup(&text, group)
	}
	if text.Len() == 0 {
		fmt.Fprintf(&text, "nothing changed since %s\n", since)
	}

	data := recapData{Since: since, Boundary: boundary, Report: withEmptyGroups(report)}
	return output.Result{Kind: kindRecap, Data: data, Text: text.String(), Warnings: report.Warnings}
}

// closedReason is the detail of a Closed entry: why the issue was closed.
func closedReason(entry recap.Entry) string {
	return entry.Reason
}

// newStatus is the detail of a Status changed entry: the status at HEAD.
func newStatus(entry recap.Entry) string {
	return entry.Status
}

// writeRecapGroup writes group to text, nothing when it is empty: its heading and count, then a
// line `  <id>  <title>` per entry, followed by ": <detail>" when the group's detail is not empty.
// A Log entry prints below its id line, indented, and consecutive entries of one id share that line.
func writeRecapGroup(text *strings.Builder, group recapGroup) {
	if len(group.entries) == 0 {
		return
	}
	fmt.Fprintf(text, "%s: %d\n", group.heading, len(group.entries))
	previousID := ""
	for _, entry := range group.entries {
		if entry.LogEntry != "" && entry.ID == previousID {
			text.WriteString(indentRecapEntry(entry.LogEntry))
			continue
		}
		previousID = entry.ID
		text.WriteString(recapItemIndent + entry.ID + "  " + entry.Title)
		if group.detail != nil {
			if detail := group.detail(entry); detail != "" {
				text.WriteString(": " + detail)
			}
		}
		text.WriteString("\n")
		if entry.LogEntry != "" {
			text.WriteString(indentRecapEntry(entry.LogEntry))
		}
	}
}

// indentRecapEntry indents each line of a Log entry by recapEntryIndent, ending each with "\n".
func indentRecapEntry(logEntry string) string {
	var text strings.Builder
	for line := range strings.SplitSeq(logEntry, "\n") {
		text.WriteString(recapEntryIndent + line + "\n")
	}
	return text.String()
}

// withEmptyGroups returns report with every nil group made empty, so --json gives [] for a group
// with nothing in it, never null.
func withEmptyGroups(report recap.Report) recap.Report {
	for _, group := range []*[]recap.Entry{
		&report.Closed,
		&report.Filed,
		&report.StatusChanged,
		&report.Log,
		&report.NotesAdded,
		&report.NotesChanged,
		&report.NotesRetired,
	} {
		if *group == nil {
			*group = []recap.Entry{}
		}
	}
	return report
}
