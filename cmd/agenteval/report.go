package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// The report's fixed text: its title, the table's header and alignment rows, and the words in the
// Result column.
const (
	reportTitle       = "# capcom agent eval"
	reportTableHeader = "| Task | Result | Calls | Failed | Repeated failures | Time | Why |"
	reportTableAlign  = "| --- | --- | ---: | ---: | ---: | ---: | --- |"
	resultPassed      = "pass"
	resultFailed      = "fail"
)

// reportTimePrecision is what a task's time is rounded to in the report.
const reportTimePrecision = 100 * time.Millisecond

// runInfo is what the report's header says about the run as a whole.
type runInfo struct {
	Date time.Time
	// CapcomVersion is the output of the build under test's `capcom version`.
	CapcomVersion string
	// AgentArgv is the agent command that ran each task, as given after `--`.
	AgentArgv []string
}

// taskResult is one task's row of the report: whether the agent did the task, why the judge
// decided so, the task's call log score and how long the agent ran.
type taskResult struct {
	ID       string
	Passed   bool
	Why      string
	Score    taskScore
	Duration time.Duration
}

// renderReport writes the markdown report of a run to w: a header naming the date, the capcom
// build and the agent command, then one table with a row per task in the order given and a totals
// row counting the passed tasks and summing the counts and the time. It returns the first write
// error.
func renderReport(w io.Writer, info runInfo, results []taskResult) error {
	var report strings.Builder
	fmt.Fprintf(&report, "%s\n\n", reportTitle)
	fmt.Fprintf(&report, "- Date: %s\n", info.Date.Format(time.DateOnly))
	fmt.Fprintf(&report, "- capcom: %s\n", strings.TrimSpace(info.CapcomVersion))
	fmt.Fprintf(&report, "- Agent: `%s`\n\n", agentCommandLine(info.AgentArgv))
	fmt.Fprintf(&report, "%s\n%s\n", reportTableHeader, reportTableAlign)

	var total taskScore
	var totalDuration time.Duration
	passed := 0
	for _, result := range results {
		writeReportRow(&report, result.ID, resultWord(result.Passed), result.Score, result.Duration, result.Why)
		total = total.add(result.Score)
		totalDuration += result.Duration
		if result.Passed {
			passed++
		}
	}
	writeReportRow(&report, "**Total**", fmt.Sprintf("%d/%d passed", passed, len(results)), total, totalDuration, "")

	_, err := io.WriteString(w, report.String())
	return err
}

// writeReportRow writes one row of the report's table.
func writeReportRow(
	report *strings.Builder,
	task, result string,
	score taskScore,
	duration time.Duration,
	why string,
) {
	fmt.Fprintf(report, "| %s | %s | %d | %d | %d | %s | %s |\n",
		tableCell(task), result, score.Calls, score.Failed, score.RepeatedFailures,
		duration.Round(reportTimePrecision), tableCell(why))
}

// resultWord is the Result column's word for a task that passed or failed.
func resultWord(isPassed bool) string {
	if isPassed {
		return resultPassed
	}
	return resultFailed
}

// tableCell makes text safe inside a markdown table cell: a `|` is escaped, so it no longer ends
// the cell, and every run of white space, line breaks included, becomes one space, so the text
// stays on its row.
func tableCell(text string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(text, "|", `\|`)), " ")
}

// agentCommandLine renders argv the way a shell would read it back: arguments that are empty or
// hold a space, a quote or a backslash are quoted, the rest are written as they are.
func agentCommandLine(argv []string) string {
	words := make([]string, 0, len(argv))
	for _, argument := range argv {
		if argument == "" || strings.ContainsAny(argument, " \t\n\"'\\") {
			argument = strconv.Quote(argument)
		}
		words = append(words, argument)
	}
	return strings.Join(words, " ")
}
