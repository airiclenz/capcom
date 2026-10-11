package issue

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// fence is the line that opens and closes the frontmatter.
const fence = "---"

// fenceLine is a fence with its line ending.
const fenceLine = fence + "\n"

// frontmatterLineOffset turns a line of the frontmatter YAML into a line of the file: the opening
// fence is line 1.
const frontmatterLineOffset = 1

// YAML tags the parser resolves plain scalars to.
const (
	tagString = "!!str"
	tagNull   = "!!null"
)

// yamlLineError matches the line a yaml.v3 syntax error names, relative to the frontmatter.
var yamlLineError = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

// Parse decodes the issue file content data. path names the file in every error. CRLF line
// endings are read as LF. A missing priority reads as DefaultPriority.
//
// Parse fails, with an *Error citing the file line, when the content does not open with a `---`
// line, when the frontmatter is never closed, on invalid YAML, on an unknown or repeated key and on
// a value of the wrong shape. It does not check values against the config; that is Validate.
func Parse(path string, data []byte) (Issue, error) {
	text := string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	if !strings.HasPrefix(text, fenceLine) {
		return Issue{}, &Error{Path: path, Line: 1, Message: "an issue file must start with a --- line"}
	}
	front, body, isClosed := splitFrontmatter(text[len(fenceLine):])
	if !isClosed {
		return Issue{}, &Error{
			Path:    path,
			Line:    1,
			Message: "the frontmatter opened here is never closed by a --- line",
		}
	}

	p := parser{path: path}
	root, err := p.document(front)
	if err != nil {
		return Issue{}, err
	}
	issue := Issue{Priority: DefaultPriority, Body: body, lines: map[string]int{}}
	if root == nil {
		return issue, nil
	}
	if err := p.fields(root, &issue); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

// splitFrontmatter splits rest, the content after the opening fence, at the first line that is
// exactly a fence. isClosed is false when there is none.
func splitFrontmatter(rest string) (front string, body string, isClosed bool) {
	offset := 0
	for {
		end := strings.IndexByte(rest[offset:], '\n')
		if end < 0 {
			if rest[offset:] == fence {
				return rest[:offset], "", true
			}
			return "", "", false
		}
		if rest[offset:offset+end] == fence {
			return rest[:offset], rest[offset+end+1:], true
		}
		offset += end + 1
	}
}

// Serialize writes the issue in the canonical form: the frontmatter keys in the fixed order title,
// status, type, priority, labels, deps, plan, aliases, created, closed, reason; empty fields left
// out except priority, which is always written; lists in flow style; timestamps in RFC3339 UTC;
// text plain where YAML reads it back unchanged and double-quoted otherwise; then the body
// verbatim.
func (i Issue) Serialize() []byte {
	var out strings.Builder
	out.WriteString(fenceLine)
	writeText(&out, keyTitle, i.Title)
	writeText(&out, keyStatus, i.Status)
	writeText(&out, keyType, i.Type)
	fmt.Fprintf(&out, "%s: %d\n", keyPriority, i.Priority)
	writeList(&out, keyLabels, i.Labels)
	writeList(&out, keyDeps, i.Deps)
	writeText(&out, keyPlan, i.Plan)
	writeList(&out, keyAliases, i.Aliases)
	writeTime(&out, keyCreated, i.Created)
	writeTime(&out, keyClosed, i.Closed)
	writeText(&out, keyReason, i.Reason)
	out.WriteString(fenceLine)
	out.WriteString(i.Body)
	return []byte(out.String())
}

// parser decodes the frontmatter of one file, attributing every failure to its path and line.
type parser struct {
	path string
}

func (p parser) errorAt(node *yaml.Node, format string, args ...any) *Error {
	return &Error{
		Path:    p.path,
		Line:    node.Line + frontmatterLineOffset,
		Message: fmt.Sprintf(format, args...),
	}
}

// document parses the frontmatter into its top-level mapping node, or nil when it holds no keys.
func (p parser) document(front string) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(front), &document); err != nil {
		return nil, p.yamlError(err)
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	root := document.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == tagNull {
		return nil, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, p.errorAt(root, "the frontmatter must be a mapping of keys")
	}
	return root, nil
}

// yamlError turns a yaml.v3 syntax error into an *Error citing the file line it names.
func (p parser) yamlError(err error) *Error {
	match := yamlLineError.FindStringSubmatch(err.Error())
	if match == nil {
		return &Error{Path: p.path, Message: fmt.Sprintf("invalid YAML frontmatter: %v", err)}
	}
	line, _ := strconv.Atoi(match[1]) // the pattern admits digits only
	return &Error{
		Path:    p.path,
		Line:    line + frontmatterLineOffset,
		Message: "invalid YAML frontmatter: " + match[2],
	}
}

// fields applies every key of root onto issue, rejecting unknown and repeated keys.
func (p parser) fields(root *yaml.Node, issue *Issue) error {
	for index := 0; index+1 < len(root.Content); index += 2 {
		key, value := root.Content[index], root.Content[index+1]
		if _, isSeen := issue.lines[key.Value]; isSeen {
			return p.errorAt(key, "duplicate key %q", key.Value)
		}
		if err := p.field(key, value, issue); err != nil {
			return err
		}
		issue.lines[key.Value] = key.Line + frontmatterLineOffset
	}
	return nil
}

// field decodes one key's value into issue.
func (p parser) field(key, value *yaml.Node, issue *Issue) error {
	var err error
	switch key.Value {
	case keyTitle:
		issue.Title, err = p.text(key.Value, value)
	case keyStatus:
		issue.Status, err = p.text(key.Value, value)
	case keyType:
		issue.Type, err = p.text(key.Value, value)
	case keyPriority:
		issue.Priority, err = p.priority(value)
	case keyLabels:
		issue.Labels, err = p.list(key.Value, value)
	case keyDeps:
		issue.Deps, err = p.list(key.Value, value)
	case keyPlan:
		issue.Plan, err = p.text(key.Value, value)
	case keyAliases:
		issue.Aliases, err = p.list(key.Value, value)
	case keyCreated:
		issue.Created, err = p.timestamp(key.Value, value)
	case keyClosed:
		issue.Closed, err = p.timestamp(key.Value, value)
	case keyReason:
		issue.Reason, err = p.text(key.Value, value)
	default:
		err = p.errorAt(key, "unknown key %q", key.Value)
	}
	return err
}

func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == tagNull
}

// text reads a scalar as text; an empty value reads as "".
func (p parser) text(key string, value *yaml.Node) (string, error) {
	if isNull(value) {
		return "", nil
	}
	if value.Kind != yaml.ScalarNode {
		return "", p.errorAt(value, "%s: expected text", key)
	}
	return value.Value, nil
}

// priority reads a whole number; its range is Validate's to check. An empty value reads as
// DefaultPriority.
func (p parser) priority(value *yaml.Node) (int, error) {
	if isNull(value) {
		return DefaultPriority, nil
	}
	priority, err := strconv.Atoi(value.Value)
	if value.Kind != yaml.ScalarNode || err != nil {
		return 0, p.errorAt(
			value,
			"%s: expected a whole number from %d to %d",
			keyPriority,
			MinPriority,
			MaxPriority,
		)
	}
	return priority, nil
}

// list reads a flow- or block-style list of non-empty text entries; an empty list reads as nil.
func (p parser) list(key string, value *yaml.Node) ([]string, error) {
	if isNull(value) {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, p.errorAt(value, "%s: expected a list such as [a, b]", key)
	}
	if len(value.Content) == 0 {
		return nil, nil
	}
	entries := make([]string, 0, len(value.Content))
	for _, item := range value.Content {
		if item.Kind != yaml.ScalarNode || isNull(item) || item.Value == "" {
			return nil, p.errorAt(item, "%s: every entry must be non-empty text", key)
		}
		entries = append(entries, item.Value)
	}
	return entries, nil
}

// timestamp reads an RFC3339 timestamp as UTC to the second; an empty value reads as the zero time.
func (p parser) timestamp(key string, value *yaml.Node) (time.Time, error) {
	if isNull(value) {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value.Value)
	if value.Kind != yaml.ScalarNode || err != nil {
		return time.Time{}, p.errorAt(
			value,
			"%s: expected an RFC3339 timestamp such as 2026-10-11T09:30:00Z",
			key,
		)
	}
	return parsed.UTC().Truncate(time.Second), nil
}

func writeText(out *strings.Builder, key string, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(out, "%s: %s\n", key, scalar(value, false))
}

func writeList(out *strings.Builder, key string, entries []string) {
	if len(entries) == 0 {
		return
	}
	quoted := make([]string, 0, len(entries))
	for _, entry := range entries {
		quoted = append(quoted, scalar(entry, true))
	}
	fmt.Fprintf(out, "%s: [%s]\n", key, strings.Join(quoted, ", "))
}

func writeTime(out *strings.Builder, key string, value time.Time) {
	if value.IsZero() {
		return
	}
	fmt.Fprintf(out, "%s: %s\n", key, value.UTC().Format(time.RFC3339))
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
