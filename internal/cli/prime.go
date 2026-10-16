package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/briefing"
	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/lock"
)

const kindPrime = "prime"

// The advice the Briefing gives on this worktree's Tree lock, one line per state.
const (
	worktreeLockFree = "this worktree: free; before a multi-file change run " +
		"capcom lock acquire --purpose <text> and keep the token"
	worktreeLockHeld = "this worktree: held; if the token is yours resume with " +
		"capcom lock acquire --purpose <text> --token <token>, otherwise do not edit it"
	worktreeLockStale = "this worktree: stale; resume with your --token, " +
		"otherwise ask the user before taking it over with --take-stale"
	worktreeLockInvalid = "this worktree: invalid; its lock record does not parse, so acquire fails; " +
		"ask the user before capcom lock break frees it"
	worktreeLockUnreadable = "this worktree: unreadable; its lock record cannot be read; " +
		"ask the user before editing it"
)

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

// The help text of capcom prime.
const (
	primeLong = "Print the Briefing an agent session starts with: the Ready Issues, split into those with a " +
		"plan and those without, each list capped at 15; the Issues in progress; the Tree locks held " +
		"in every worktree; changes to Issues on other branches that are not merged yet; Note refs that no longer " +
		"resolve; and the number of problems capcom check finds. It never fails: a part that cannot be " +
		"read becomes a warning line, and the command exits 0. init --claude-hooks and " +
		"init --codex-hooks run it at session start and before compaction."
	primeExample = "  capcom prime          # the session-start Briefing\n" +
		"  capcom prime --json   # the same Briefing as a JSON envelope"
)

// newPrimeCommand prints the Briefing an agent session starts with: Ready work split planned and
// unplanned, the issues in progress, every worktree's Tree lock, issue changes on other branches,
// broken Note refs and the count of check problems. It never fails: a source that cannot be read
// becomes one warning line, and the command exits 0.
func (a *app) newPrimeCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "prime",
		Short:   "Print the session Briefing: ready work, locks, other branches and store health",
		Long:    primeLong,
		Example: primeExample,
		Args:    cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			return a.emit(cmd, primeResult())
		}),
	}
}

// primeResult composes the Briefing of the working directory and renders it. It opens the store
// and the index through briefing.Compose, rather than through the shared annotationIndex, so that
// their failures are warnings too.
func primeResult() output.Result {
	now := time.Now()
	cwd, err := os.Getwd()
	if err != nil {
		failure := briefing.Failure{Section: briefing.SectionStore, Err: err}
		return renderBriefing(briefing.Briefing{Failures: []briefing.Failure{failure}}, now)
	}
	return renderBriefing(briefing.Compose(cwd, now), now)
}

// renderBriefing renders composed as prime's result, with lock ages measured back from now. A
// section composed left out is left out of the text and the data, and each failure is one warning.
func renderBriefing(composed briefing.Briefing, now time.Time) (result output.Result) {
	warnings := primeWarnings(composed.Failures)
	defer func() {
		// A Briefing must never cost the session its start: even a defect becomes a warning.
		if recovered := recover(); recovered != nil {
			warnings = append(warnings, fmt.Sprintf("prime: internal error: %v", recovered))
			result = output.Result{Kind: kindPrime, Data: primeData{}, Warnings: warnings}
		}
	}()

	var data primeData
	var text strings.Builder
	if work := composed.Work; work != nil {
		data.ReadyPlanned = addCappedList(&text, "ready, planned", work.ReadyPlanned)
		data.ReadyUnplanned = addCappedList(&text, "ready, unplanned", work.ReadyUnplanned)
		data.Doing = issueListData(work.Doing)
		fmt.Fprintf(&text, "doing: %d\n", len(work.Doing))
		text.WriteString(indentLines(issueLines(work.Doing, "")))
	}
	if overview := composed.LockOverview; overview != nil {
		data.Locks = addLocks(&text, *overview, now)
	}
	if health := composed.Health; health != nil {
		data.BrokenRefs = append([]check.Problem{}, health.BrokenRefs...)
		data.CheckProblems = &health.ProblemCount
		addHealth(&text, *health)
	}
	return output.Result{
		Kind:      kindPrime,
		Data:      data,
		Text:      text.String(),
		Warnings:  warnings,
		Elsewhere: composed.Elsewhere,
	}
}

// primeWarnings renders each failure as one warning line: `<section>: <error>` on one line, the
// store's typed as any store failure is, and an Elsewhere scan warning as the scan worded it.
func primeWarnings(failures []briefing.Failure) []string {
	warnings := make([]string, 0, len(failures))
	for _, failure := range failures {
		err := failure.Err
		switch failure.Section {
		case briefing.SectionElsewhere:
			warnings = append(warnings, err.Error())
			continue
		case briefing.SectionStore:
			err = output.StoreFailure(err)
		}
		warnings = append(warnings, string(failure.Section)+": "+flatten(err.Error()))
	}
	return warnings
}

// addCappedList writes the heading and the first primeListCap issues of one Ready list.
func addCappedList(text *strings.Builder, heading string, entries []index.Entry) *primeListData {
	shown := entries[:min(len(entries), primeListCap)]
	fmt.Fprintf(text, "%s: %d", heading, len(entries))
	if len(shown) < len(entries) {
		fmt.Fprintf(text, ", first %d shown", len(shown))
	}
	text.WriteString("\n" + indentLines(issueLines(shown, "")))
	return &primeListData{Count: len(entries), Issues: issueListData(shown)}
}

// addLocks writes the Tree lock of every worktree that has one, with its age and stale flag, then
// the advice on the lock of the overview's own worktree.
func addLocks(text *strings.Builder, overview lock.Overview, now time.Time) []lockData {
	held := overview.Locks
	data := make([]lockData, 0, len(held))
	fmt.Fprintf(text, "locks: %d\n", len(held))
	for _, status := range held {
		data = append(data, newLockData(lockStateName(status.State()), status, ""))
		record := status.Record
		fmt.Fprintf(text, "  %s: held by %s for %s, heartbeat %s ago: %s",
			status.Key,
			record.Holder,
			lock.AgeText(now.Sub(record.Since)),
			lock.AgeText(now.Sub(record.Heartbeat)),
			flatten(record.Purpose),
		)
		if status.IsStale {
			text.WriteString(" (stale)")
		}
		text.WriteString("\n")
	}
	text.WriteString(worktreeLockAdvice(overview.State) + "\n")
	return data
}

// worktreeLockAdvice returns the advice on a worktree's lock in state, one line per state.
func worktreeLockAdvice(state lock.State) string {
	switch state {
	case lock.StateHeld:
		return worktreeLockHeld
	case lock.StateStale:
		return worktreeLockStale
	case lock.StateInvalid:
		return worktreeLockInvalid
	case lock.StateUnreadable:
		return worktreeLockUnreadable
	default:
		return worktreeLockFree
	}
}

// addHealth writes the broken Note refs and the count of every check problem.
func addHealth(text *strings.Builder, health briefing.Health) {
	fmt.Fprintf(text, "broken Note refs: %d\n", len(health.BrokenRefs))
	for _, problem := range health.BrokenRefs {
		text.WriteString("  " + problem.String() + "\n")
	}
	if health.ProblemCount == 0 {
		text.WriteString("check: no problems\n")
		return
	}
	noun := "problems"
	if health.ProblemCount == 1 {
		noun = "problem"
	}
	fmt.Fprintf(text, "check: %d %s; run capcom check\n", health.ProblemCount, noun)
}

// indentLines indents every line of text by textIndent.
func indentLines(text string) string {
	if text == "" {
		return ""
	}
	return textIndent + strings.ReplaceAll(strings.TrimSuffix(text, "\n"), "\n", "\n"+textIndent) + "\n"
}
