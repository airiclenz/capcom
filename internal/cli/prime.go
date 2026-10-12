package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/store"
)

const kindPrime = "prime"

// minutesPerHour splits a lock age into hours and minutes.
const minutesPerHour = 60

// primeListCap is how many issues each Ready list of the Briefing shows; the count covers all.
const primeListCap = 15

// primeData is the --json data of capcom prime. A section whose source failed is left out, and the
// failure is one of the envelope's warnings.
type primeData struct {
	ReadyPlanned   *primeListData  `json:"ready_planned,omitempty"`
	ReadyUnplanned *primeListData  `json:"ready_unplanned,omitempty"`
	Doing          []issueData     `json:"doing,omitzero"`
	Locks          []lockData      `json:"locks,omitzero"`
	BrokenRefs     []check.Problem `json:"broken_refs,omitzero"`
	CheckProblems  *int            `json:"check_problems,omitempty"`
}

// primeListData is one capped Ready list: how many issues there are, and the first primeListCap.
type primeListData struct {
	Count  int         `json:"count"`
	Issues []issueData `json:"issues"`
}

// briefing collects the Briefing's sections and the warnings of the sources that failed.
type briefing struct {
	data     primeData
	text     strings.Builder
	warnings []string
}

// warn records that section's source failed with err, as one line.
func (b *briefing) warn(section string, err error) {
	b.warnings = append(b.warnings, section+": "+flatten(err.Error()))
}

// newPrimeCommand prints the Briefing an agent session starts with: Ready work split planned and
// unplanned, the issues in progress, every worktree's Tree lock, issue changes on other branches,
// broken Note refs and the count of check problems. It never fails: a source that cannot be read
// becomes one warning line, and the command exits 0.
func (a *app) newPrimeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "prime",
		Short: "Print the session Briefing: ready work, locks, other branches and store health",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			return a.emit(cmd, a.brief())
		}),
	}
}

// brief composes the Briefing. It opens the store and refreshes the index itself, rather than
// through the shared annotationIndex, so that their failures are warnings too.
func (a *app) brief() (result Result) {
	var b briefing
	defer func() {
		// A Briefing must never cost the session its start: even a defect becomes a warning.
		if recovered := recover(); recovered != nil {
			b.warnings = append(b.warnings, fmt.Sprintf("prime: internal error: %v", recovered))
			result = Result{Kind: kindPrime, Data: primeData{}, Warnings: b.warnings}
		}
	}()

	cwd, err := os.Getwd()
	if err == nil {
		a.store, err = store.Open(cwd)
	}
	if err != nil {
		b.warn("store", storeFailure(err))
		return Result{Kind: kindPrime, Data: b.data, Warnings: b.warnings}
	}
	a.briefIssues(&b)
	a.briefLocks(&b)
	a.briefCheck(&b, cwd)
	result = Result{Kind: kindPrime, Data: b.data, Text: b.text.String(), Warnings: b.warnings}
	return a.addElsewhere(result, false, a.issuesPathspec())
}

// briefIssues adds the Ready lists and the issues in a doing status, read from the refreshed index.
func (a *app) briefIssues(b *briefing) {
	// The refresh report Open leaves on Report is never read on purpose: its warnings name files
	// the index left out, and briefCheck already counts each of them in "check: N problems".
	// Repeating them here would flood the session start with one line per broken file.
	opened, err := index.Open(a.store, index.ModeRefresh)
	if err != nil {
		b.warn("index", fmt.Errorf("%w; run capcom reindex", err))
		return
	}
	a.issueIndex = opened
	defer func() {
		a.issueIndex = nil
		if err := opened.Close(); err != nil {
			b.warn("index", err)
		}
	}()

	ready, err := a.readyEntries(&readyOptions{})
	if err != nil {
		b.warn("ready", err)
		return
	}
	doing, err := opened.List(index.Filter{Categories: []config.Category{config.CategoryDoing}})
	if err != nil {
		b.warn("doing", err)
		return
	}
	var planned, unplanned []index.Entry
	for _, current := range ready {
		if current.entry.Plan != "" {
			planned = append(planned, current.entry)
		} else {
			unplanned = append(unplanned, current.entry)
		}
	}
	b.data.ReadyPlanned = b.addCappedList("ready, planned", planned)
	b.data.ReadyUnplanned = b.addCappedList("ready, unplanned", unplanned)
	b.data.Doing = issueListData(doing)
	fmt.Fprintf(&b.text, "doing: %d\n", len(doing))
	b.text.WriteString(indentLines(issueLines(doing, "")))
}

// addCappedList writes the heading and the first primeListCap issues of one Ready list.
func (b *briefing) addCappedList(heading string, entries []index.Entry) *primeListData {
	shown := entries[:min(len(entries), primeListCap)]
	fmt.Fprintf(&b.text, "%s: %d", heading, len(entries))
	if len(shown) < len(entries) {
		fmt.Fprintf(&b.text, ", first %d shown", len(shown))
	}
	b.text.WriteString("\n" + indentLines(issueLines(shown, "")))
	return &primeListData{Count: len(entries), Issues: issueListData(shown)}
}

// briefLocks adds the Tree lock of every worktree that has one, with its age and stale flag.
func (a *app) briefLocks(b *briefing) {
	held, err := lock.Open(a.store.GitCommonDir, a.store.Config.Lock.TTL).List()
	if err != nil {
		b.warn("locks", err)
		return
	}
	now := time.Now()
	b.data.Locks = make([]lockData, 0, len(held))
	fmt.Fprintf(&b.text, "locks: %d\n", len(held))
	for _, status := range held {
		b.data.Locks = append(b.data.Locks, newLockData(foundState(status), status, ""))
		record := status.Record
		fmt.Fprintf(&b.text, "  %s: held by %s for %s, heartbeat %s ago: %s",
			status.Key,
			record.Holder,
			ageText(now.Sub(record.Since)),
			ageText(now.Sub(record.Heartbeat)),
			flatten(record.Purpose),
		)
		if status.IsStale {
			b.text.WriteString(" (stale)")
		}
		b.text.WriteString("\n")
	}
}

// briefCheck adds the broken Note refs and the count of every check problem.
func (a *app) briefCheck(b *briefing, cwd string) {
	problems, err := check.Run(cwd)
	if err != nil {
		b.warn("check", err)
		return
	}
	b.data.BrokenRefs = []check.Problem{}
	for _, problem := range problems {
		if problem.Code == check.CodeBrokenRef {
			b.data.BrokenRefs = append(b.data.BrokenRefs, problem)
		}
	}
	count := len(problems)
	b.data.CheckProblems = &count

	fmt.Fprintf(&b.text, "broken note refs: %d\n", len(b.data.BrokenRefs))
	for _, problem := range b.data.BrokenRefs {
		b.text.WriteString("  " + problem.String() + "\n")
	}
	if count == 0 {
		b.text.WriteString("check: no problems\n")
		return
	}
	noun := "problems"
	if count == 1 {
		noun = "problem"
	}
	fmt.Fprintf(&b.text, "check: %d %s; run capcom check\n", count, noun)
}

// indentLines indents every line of text by textIndent.
func indentLines(text string) string {
	if text == "" {
		return ""
	}
	return textIndent + strings.ReplaceAll(strings.TrimSuffix(text, "\n"), "\n", "\n"+textIndent) + "\n"
}

// ageText renders a lock age in whole minutes: `1h05m`, `12m`.
func ageText(age time.Duration) string {
	minutes := int(max(age, 0) / time.Minute)
	if minutes < minutesPerHour {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/minutesPerHour, minutes%minutesPerHour)
}
