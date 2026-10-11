package issue

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"time"
)

// TextMerge merges three versions of a text line by line, as git merge-file does, returning the
// merged bytes and whether they hold conflict markers.
type TextMerge func(base []byte, ours []byte, theirs []byte) ([]byte, bool, error)

// logPlaceholder stands in for an issue's Log section while the rest of the file goes through the
// text merge. It is a single run of letters and digits so that git, which folds conflict hunks
// separated only by lines without letters or digits (ZEALOUS_ALNUM), treats it as real content.
const logPlaceholder = "capcomLogMergePlaceholder9f4c2e7a1b"

// logEntryPattern matches the first line of a Log entry, `- YYYY-MM-DD <author>: <text>`.
var logEntryPattern = regexp.MustCompile(`^- \d{4}-\d{2}-\d{2} \S[^:]*:( |$)`)

// logEntryDateStart is the offset of the date in a Log entry's first line, after the `- `.
const logEntryDateStart = len("- ")

// logEntryDateEnd is the offset just past the date in a Log entry's first line.
const logEntryDateEnd = logEntryDateStart + len(time.DateOnly)

// logSection is the Log section of one version of an issue file.
type logSection struct {
	isPresent bool
	// start and end bound the section's bytes in the file: its heading line through its last
	// non-blank line, without that line's terminator. Trailing blank lines stay outside.
	start int
	end   int
	// heading, lead and entries hold the section's lines without their terminators: the heading
	// line, the blank lines before the first entry, and each entry with its continuation lines
	// joined by "\n".
	heading string
	lead    []string
	entries []string
}

// MergeFile merges the base, ours and theirs versions of an issue file into one, returning the
// merged bytes and whether they hold conflict markers.
//
// The Log section merges by entry and never conflicts: an entry is a `- YYYY-MM-DD <author>: `
// line plus its continuation lines, those indented by two spaces, blank lines followed by an
// indented line included. When one side's Log equals base's, the other side's is taken verbatim;
// otherwise the merged Log holds ours's entries in ours's order, less those in base that theirs
// deleted, with theirs's new entries merged in by date, ours first on equal dates. Existing entries
// are never reordered. Everything outside the Log goes through textMerge with the Log swapped for a
// placeholder line, which is then replaced, every time it occurs, by the merged Log; the merged Log
// uses CRLF line endings when ours does.
//
// The whole file goes to textMerge unchanged when any version's Log holds a line that is neither
// part of an entry nor blank before the first entry or after the last, when any version holds the
// placeholder, and when the text merge drops the Log that the entry merge keeps, or the reverse.
// An error from textMerge is returned as is.
func MergeFile(base []byte, ours []byte, theirs []byte, textMerge TextMerge) ([]byte, bool, error) {
	versions := [][]byte{base, ours, theirs}
	sections := make([]logSection, 0, len(versions))
	for _, version := range versions {
		section, isEntryOnly := parseLog(version)
		if !isEntryOnly || bytes.Contains(version, []byte(logPlaceholder)) {
			return textMerge(base, ours, theirs)
		}
		sections = append(sections, section)
	}

	merged, hasConflict, err := textMerge(
		sections[0].withPlaceholder(base),
		sections[1].withPlaceholder(ours),
		sections[2].withPlaceholder(theirs),
	)
	if err != nil {
		return nil, false, err
	}

	log := mergeLogs(sections[0], sections[1], sections[2])
	if log.isPresent != bytes.Contains(merged, []byte(logPlaceholder)) {
		return textMerge(base, ours, theirs)
	}
	newline := "\n"
	if bytes.Contains(ours, []byte("\r\n")) {
		newline = "\r\n"
	}
	return bytes.ReplaceAll(merged, []byte(logPlaceholder), []byte(log.render(newline))), hasConflict, nil
}

// parseLog reads the Log section of an issue file: the section that opens with the first
// LogHeading line and runs to the next level-one or level-two heading. A file without one gives a
// section that is not present. isEntryOnly is false when the section holds a line that is neither
// part of an entry nor blank before the first entry or after the last.
func parseLog(data []byte) (section logSection, isEntryOnly bool) {
	lines := strings.SplitAfter(string(data), "\n")
	start := slices.IndexFunc(lines, func(line string) bool {
		return strings.TrimRight(line, " \t\r\n") == LogHeading
	})
	if start < 0 {
		return logSection{}, true
	}

	end := start + 1
	for end < len(lines) && !isSectionBoundary(trimLineEnd(lines[end])) {
		end++
	}
	last := end - 1
	for last > start && strings.TrimSpace(lines[last]) == "" {
		last--
	}

	content := make([]string, 0, last-start)
	for _, line := range lines[start+1 : last+1] {
		content = append(content, trimLineEnd(line))
	}
	lead, entries, isEntryOnly := parseLogContent(content)
	if !isEntryOnly {
		return logSection{}, false
	}

	offset := len(strings.Join(lines[:start], ""))
	return logSection{
		isPresent: true,
		start:     offset,
		end:       offset + len(strings.Join(lines[start:last], "")) + len(trimLineEnd(lines[last])),
		heading:   trimLineEnd(lines[start]),
		lead:      lead,
		entries:   entries,
	}, true
}

// parseLogContent splits the lines of a Log section between its heading and its last non-blank
// line into the leading blank lines and the entries. isEntryOnly is false when a line after the
// leading blanks is neither an entry's first line nor one of its continuation lines.
func parseLogContent(lines []string) (lead []string, entries []string, isEntryOnly bool) {
	index := 0
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		lead = append(lead, lines[index])
		index++
	}
	for index < len(lines) {
		if !logEntryPattern.MatchString(lines[index]) {
			return nil, nil, false
		}
		next := index + 1
		for next < len(lines) && isLogContinuation(lines[next:]) {
			next++
		}
		entries = append(entries, strings.Join(lines[index:next], "\n"))
		index = next
	}
	return lead, entries, true
}

// isLogContinuation reports whether the first of lines continues the entry above it: it is
// indented by logContinuationIndent, or it is blank and the next non-blank line is.
func isLogContinuation(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return strings.HasPrefix(line, logContinuationIndent)
		}
	}
	return false
}

// trimLineEnd returns line without its LF or CRLF terminator.
func trimLineEnd(line string) string {
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
}

// withPlaceholder returns data with the section's bytes replaced by logPlaceholder, or data itself
// when the section is not present.
func (s logSection) withPlaceholder(data []byte) []byte {
	if !s.isPresent {
		return data
	}
	swapped := make([]byte, 0, len(data)-(s.end-s.start)+len(logPlaceholder))
	swapped = append(swapped, data[:s.start]...)
	swapped = append(swapped, logPlaceholder...)
	return append(swapped, data[s.end:]...)
}

// render returns the section's lines joined by newline, without a final one.
func (s logSection) render(newline string) string {
	lines := make([]string, 0, 1+len(s.lead)+len(s.entries))
	lines = append(lines, s.heading)
	lines = append(lines, s.lead...)
	lines = append(lines, s.entries...)
	return strings.ReplaceAll(strings.Join(lines, "\n"), "\n", newline)
}

// equalsLog reports whether two sections are both absent, or both present with the same entries.
func (s logSection) equalsLog(other logSection) bool {
	return s.isPresent == other.isPresent && slices.Equal(s.entries, other.entries)
}

// mergeLogs merges the Log sections of base, ours and theirs. A side whose Log equals base's
// yields the other side's verbatim. Otherwise the result takes its heading and leading blank lines
// from ours, or from theirs when ours is absent or has no entries while theirs has some.
func mergeLogs(base logSection, ours logSection, theirs logSection) logSection {
	if base.equalsLog(ours) {
		return theirs
	}
	if base.equalsLog(theirs) {
		return ours
	}

	merged := ours
	if !ours.isPresent || len(ours.entries) == 0 && len(theirs.entries) > 0 {
		merged = theirs
	}
	merged.entries = mergeEntries(base.entries, ours.entries, theirs.entries)
	return merged
}

// mergeEntries returns ours's entries less those in base that theirs deleted, with theirs's entries
// that neither base nor ours holds merged in by date. Neither side's entries change their order;
// on equal dates ours's come first.
func mergeEntries(base []string, ours []string, theirs []string) []string {
	kept := make([]string, 0, len(ours))
	for _, entry := range ours {
		if !slices.Contains(base, entry) || slices.Contains(theirs, entry) {
			kept = append(kept, entry)
		}
	}
	var added []string
	for _, entry := range theirs {
		if !slices.Contains(base, entry) && !slices.Contains(ours, entry) {
			added = append(added, entry)
		}
	}

	merged := make([]string, 0, len(kept)+len(added))
	for len(added) > 0 {
		if len(kept) > 0 && entryDate(kept[0]) <= entryDate(added[0]) {
			merged = append(merged, kept[0])
			kept = kept[1:]
			continue
		}
		merged = append(merged, added[0])
		added = added[1:]
	}
	return append(merged, kept...)
}

// entryDate returns the YYYY-MM-DD date of a Log entry that logEntryPattern matched.
func entryDate(entry string) string {
	return entry[logEntryDateStart:logEntryDateEnd]
}
