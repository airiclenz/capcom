package frontmatter

import (
	"fmt"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// tagString is the YAML tag the parser resolves plain text to.
const tagString = "!!str"

// Encoder writes one file in the canonical form of docs/site/reference/file-format.md: the opening
// fence, one line per key in the order the caller writes them, the closing fence, then the body
// byte for byte. Text and list entries are written plain where YAML reads them back as the same
// text, and double-quoted on one line otherwise, so reading an encoded file and encoding it again
// gives the same bytes. Every write leaves out an empty value; a value the kind always writes is
// never empty. The zero Encoder is ready to use.
type Encoder struct {
	fields strings.Builder
}

// Text writes key with the text value, plain or double-quoted. An empty value writes nothing.
func (e *Encoder) Text(key string, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(&e.fields, "%s: %s\n", key, scalar(value, false))
}

// List writes key with the entries as a flow list on one line (`labels: [a, b]`), each entry plain
// or double-quoted. No entries write nothing.
func (e *Encoder) List(key string, entries []string) {
	if len(entries) == 0 {
		return
	}
	quoted := make([]string, 0, len(entries))
	for _, entry := range entries {
		quoted = append(quoted, scalar(entry, true))
	}
	fmt.Fprintf(&e.fields, "%s: [%s]\n", key, strings.Join(quoted, ", "))
}

// Raw writes key with value exactly as given, for a value the caller has already rendered as YAML
// that reads back as itself: a whole number, an RFC3339 timestamp. An empty value writes nothing.
func (e *Encoder) Raw(key string, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(&e.fields, "%s: %s\n", key, value)
}

// File returns the whole file: the frontmatter fenced by `---` lines, then body byte for byte.
func (e *Encoder) File(body string) []byte {
	var out strings.Builder
	out.WriteString(fenceLine)
	out.WriteString(e.fields.String())
	out.WriteString(fenceLine)
	out.WriteString(body)
	return []byte(out.String())
}

// scalar renders value as a YAML scalar: plain when YAML reads it back as the same text, in the
// context it is written in (a mapping value or a flow-list entry), and double-quoted otherwise.
func scalar(value string, isFlowEntry bool) string {
	if isPlainSafe(value, isFlowEntry) {
		return value
	}
	return doubleQuoted(value)
}

// isPlainSafe reports whether value, written unquoted, parses back as exactly the same string.
// It asks the YAML parser itself, so every indicator, comment, number and keyword is covered.
func isPlainSafe(value string, isFlowEntry bool) bool {
	if value == "" || strings.IndexFunc(value, needsEscape) >= 0 {
		return false
	}
	probe := "k: " + value
	if isFlowEntry {
		probe = "k: [" + value + "]"
	}
	var document yaml.Node
	if yaml.Unmarshal([]byte(probe), &document) != nil || len(document.Content) != 1 {
		return false
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode || len(mapping.Content) != 2 {
		return false
	}
	parsed := mapping.Content[1]
	if isFlowEntry {
		if parsed.Kind != yaml.SequenceNode || len(parsed.Content) != 1 {
			return false
		}
		parsed = parsed.Content[0]
	}
	return parsed.Kind == yaml.ScalarNode && parsed.Style == 0 && parsed.Tag == tagString &&
		parsed.Value == value
}

// needsEscape reports a rune a YAML double-quoted scalar must escape: control characters, the
// Unicode line breaks YAML folds, the byte order mark, and every other rune outside YAML 1.2's
// printable set (the surrogates and the noncharacters U+FFFE and U+FFFF), which a YAML parser
// rejects when written raw.
func needsEscape(r rune) bool {
	switch {
	case unicode.IsControl(r), r == '\u2028', r == '\u2029', r == '\uFEFF':
		return true
	case r <= '\uD7FF', r >= '\uE000' && r <= '\uFFFD', r >= '\U00010000' && r <= unicode.MaxRune:
		return false
	default:
		return true
	}
}

// doubleQuoted renders value as a YAML double-quoted scalar on one line.
func doubleQuoted(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch {
		case r == '"' || r == '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case r == '\n':
			out.WriteString(`\n`)
		case r == '\t':
			out.WriteString(`\t`)
		case needsEscape(r):
			fmt.Fprintf(&out, `\u%04X`, r)
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}
