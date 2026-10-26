package index

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// The excerpt shape: how many matching lines an excerpt shows, the context lines either side of
// each, the most lines it holds, and the most runes of each line.
const (
	excerptMatchLimit   = 3
	excerptContextLines = 1
	excerptLineLimit    = 8
	excerptRuneLimit    = 120
)

// excerptGap is the line an excerpt puts between two windows the body keeps apart, and the mark
// that ends a line cut at the rune limit.
const excerptGap = "…"

// Excerpt returns the lines of body that show why it matches text: the first three lines holding
// a word of text, each with one line of context either side, windows that overlap or touch merged
// into one and a "…" line between windows that do not. It holds at most eight lines and never
// ends on a gap; a line longer than 120 runes is cut to its first 119 and ends with "…".
//
// A line holds a word when the word's letter and digit tokens occur, in order and side by side,
// among the line's, ignoring case and diacritics, so "login-timeout-race" matches as the full-text
// search matches it and "cafe" matches "café", composed or decomposed, either way round. The
// lines returned keep their original text. A body that holds no word of text, as when only the title matched, and blank text
// both give no lines.
func Excerpt(body, text string) []string {
	words := queryTokens(text)
	if len(words) == 0 {
		return nil
	}
	lines := bodyLines(body)
	var windows [][2]int
	matchCount := 0
	for index, line := range lines {
		if matchCount == excerptMatchLimit {
			break
		}
		if !holdsAnyWord(textTokens(line), words) {
			continue
		}
		matchCount++
		windows = addWindow(windows, index, len(lines))
	}
	return renderWindows(lines, windows)
}

// queryTokens returns the token sequence of each word of text, leaving out words with no token.
func queryTokens(text string) [][]string {
	var words [][]string
	for _, word := range strings.Fields(text) {
		if tokens := textTokens(word); len(tokens) > 0 {
			words = append(words, tokens)
		}
	}
	return words
}

// textTokens splits text into folded runs of letters, digits and marks, the tokens the
// full-text index reads: lower-cased and with their diacritics dropped, as the index drops them.
func textTokens(text string) []string {
	return strings.FieldsFunc(foldText(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r)
	})
}

// foldText returns text lower-cased and without diacritics: decomposed to NFD, its nonspacing
// marks dropped, so a composed "é", a decomposed one and a plain "e" all fold to "e".
func foldText(text string) string {
	return strings.ToLower(strings.Map(dropNonspacingMark, norm.NFD.String(text)))
}

// dropNonspacingMark returns r, or -1, which strings.Map drops, when r is a nonspacing mark.
func dropNonspacingMark(r rune) rune {
	if unicode.Is(unicode.Mn, r) {
		return -1
	}
	return r
}

// bodyLines splits body into lines without their line endings, dropping the empty line a final
// newline leaves.
func bodyLines(body string) []string {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// holdsAnyWord reports whether the token sequence of at least one word occurs side by side in
// line.
func holdsAnyWord(line []string, words [][]string) bool {
	for _, word := range words {
		for start := 0; start+len(word) <= len(line); start++ {
			if slices.Equal(line[start:start+len(word)], word) {
				return true
			}
		}
	}
	return false
}

// addWindow adds the window of context around the matching line at index, of a body of
// lineCount lines, merging it into the last window when the two overlap or touch.
func addWindow(windows [][2]int, index, lineCount int) [][2]int {
	window := [2]int{max(0, index-excerptContextLines), min(lineCount-1, index+excerptContextLines)}
	if len(windows) == 0 {
		return append(windows, window)
	}
	last := &windows[len(windows)-1]
	if window[0] > last[1]+1 {
		return append(windows, window)
	}
	last[1] = max(last[1], window[1])
	return windows
}

// renderWindows returns the lines of each window, cut to the rune limit, with a gap line between
// windows, stopping at the line limit. A gap goes in only when a line of the next window still
// fits after it, so the excerpt never ends on one.
func renderWindows(lines []string, windows [][2]int) []string {
	var excerpt []string
	for position, window := range windows {
		if position > 0 {
			if len(excerpt)+2 > excerptLineLimit {
				break
			}
			excerpt = append(excerpt, excerptGap)
		}
		for index := window[0]; index <= window[1] && len(excerpt) < excerptLineLimit; index++ {
			excerpt = append(excerpt, cutLine(lines[index]))
		}
	}
	return excerpt
}

// cutLine returns line, or its first runes and excerptGap when it is longer than the rune limit,
// so the result never passes the limit.
func cutLine(line string) string {
	if utf8.RuneCountInString(line) <= excerptRuneLimit {
		return line
	}
	return string([]rune(line)[:excerptRuneLimit-1]) + excerptGap
}
