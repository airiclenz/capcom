package rawfm

import (
	"slices"
	"strings"
)

// The parts of a frontmatter aliases list Aliases reads: the top-level key, the brackets of the
// flow form "[a, b]" and the item prefix of the block form "- a".
const (
	aliasesKey      = "aliases:"
	flowListOpen    = "["
	flowListClose   = "]"
	flowListComma   = ","
	blockItemPrefix = "-"
	yamlQuotes      = `"'`
)

// frontmatterFence is the line that opens and closes the frontmatter.
const frontmatterFence = "---"

// IsFrontmatterFence reports whether line, without its LF terminator, is a fence that opens or
// closes the frontmatter of an issue or Note file: exactly "---", with at most one trailing "\r",
// so a CRLF line and a closing fence that ends the file with a lone "\r" are fences too. Leading or
// trailing spaces, a byte order mark or any other text make it no fence.
func IsFrontmatterFence(line string) bool {
	return strings.TrimSuffix(line, "\r") == frontmatterFence
}

// Aliases returns the aliases the frontmatter of an issue or Note file lists under its top-level
// aliases key, read line by line from the raw text without parsing it, for a file whose frontmatter
// does not parse. It reads the flow form "aliases: [a, b]" and the block form, an "aliases:" line
// followed by "- a" lines; conflict marker lines and any other line it cannot read are passed over,
// so an aliases key on both sides of a conflict hunk contributes both lists. Each alias is returned
// once, in the order first listed, with surrounding quotes removed; it is not validated. A file
// without frontmatter or without the key gives none.
func Aliases(data []byte) []string {
	lines := strings.Split(string(data), "\n")
	frontmatterEnd := frontmatterEndIndex(lines)
	var aliases []string
	for index := 1; index < frontmatterEnd; index++ {
		value, isKey := strings.CutPrefix(strings.TrimSuffix(lines[index], "\r"), aliasesKey)
		if !isKey {
			continue
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, flowListOpen) {
			aliases = appendNew(aliases, flowListItems(value)...)
			continue
		}
		if value == "" {
			aliases = appendNew(aliases, blockListItems(lines[index+1:frontmatterEnd])...)
		}
	}
	return aliases
}

// frontmatterEndIndex returns the index of the "---" line that closes the frontmatter of a file
// split into lines, len(lines) for one whose frontmatter is never closed, and 0 for a file that
// opens with no frontmatter: its frontmatter lines lie between index 1 and the one returned.
func frontmatterEndIndex(lines []string) int {
	end := bodyStartIndex(lines)
	if end > 1 && IsFrontmatterFence(lines[end-1]) {
		return end - 1
	}
	return end
}

// flowListItems returns the items of a flow list "[a, b]"; a list whose closing bracket is missing
// runs to the end of the line.
func flowListItems(value string) []string {
	value = strings.TrimPrefix(value, flowListOpen)
	value, _, _ = strings.Cut(value, flowListClose)
	var items []string
	for item := range strings.SplitSeq(value, flowListComma) {
		if item = trimItem(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// blockListItems returns the items of the block list that starts at lines[0]: the "- a" lines up
// to the first line that is neither an item, a blank line nor a conflict marker.
func blockListItems(lines []string) []string {
	var items []string
	for _, line := range lines {
		text := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if text == "" || isConflictMarkerLine(text) {
			continue
		}
		item, isItem := strings.CutPrefix(text, blockItemPrefix)
		if !isItem {
			break
		}
		if item = trimItem(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// isConflictMarkerLine reports whether text, a line without its line ending, is a git conflict
// marker of any kind, a "=======" separator included.
func isConflictMarkerLine(text string) bool {
	return strings.HasPrefix(text, conflictOpen) ||
		strings.HasPrefix(text, conflictClose) ||
		strings.HasPrefix(text, conflictBase) ||
		text == conflictSeparator
}

// trimItem returns a list item without its surrounding spaces and quotes.
func trimItem(item string) string {
	return strings.Trim(strings.TrimSpace(item), yamlQuotes)
}

// appendNew appends to list each of items it does not hold yet.
func appendNew(list []string, items ...string) []string {
	for _, item := range items {
		if !slices.Contains(list, item) {
			list = append(list, item)
		}
	}
	return list
}
