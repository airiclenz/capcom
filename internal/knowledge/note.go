// Package knowledge reads and writes Note files and checks their Refs. A Note is markdown with a
// YAML frontmatter between two `---` lines holding title, labels, refs and aliases; its id is the
// file name and its place in the Knowledge Tree is its directory (ADR 0001). Parse turns a file
// into a Note, Serialize writes one back, Validate checks it, and Verify reports the Refs that no
// longer resolve in the working tree. Every error names the file, and the line when one is to
// blame.
package knowledge

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter keys, in the order Serialize writes them.
const (
	keyTitle   = "title"
	keyLabels  = "labels"
	keyRefs    = "refs"
	keyAliases = "aliases"
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

// Note is one parsed Note file. Its id is the file name and is never stored in the file. Empty
// strings and nil lists are fields the file leaves out.
type Note struct {
	Title  string
	Labels []string
	// Refs are the files, directories and symbols the Note depends on: a path relative to the
	// working tree root with forward slashes, or `path#symbol` for a text the file must contain.
	Refs []string
	// Aliases are the Note's former ids (ADR 0002).
	Aliases []string
	// Body is the markdown after the frontmatter, byte for byte (LF line endings).
	Body string

	// lines maps each frontmatter key Parse found to its 1-based line in the file; refLines holds
	// the line of each entry of Refs. Both are nil for a Note built in memory.
	lines    map[string]int
	refLines []int
}

// Error is a Note file that cannot be used. Line is the 1-based line of the offending key or
// value, or 0 when no single line is to blame.
type Error struct {
	Path    string
	Line    int
	Message string
}

// Error renders "path:line: message", or "path: message" when the line is unknown.
func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// RefLine returns the 1-based line of the i-th Ref in the file the Note was parsed from, or 0 for
// a Note built in memory.
func (n Note) RefLine(i int) int {
	if i < 0 || i >= len(n.refLines) {
		return 0
	}
	return n.refLines[i]
}

// Parse decodes the Note file content data. path names the file in every error. CRLF line endings
// are read as LF.
//
// Parse fails, with an *Error citing the file line, when the content does not open with a `---`
// line, when the frontmatter is never closed, on invalid YAML, on an unknown or repeated key and on
// a value of the wrong shape. Whether the values make a usable Note is Validate's to check.
func Parse(path string, data []byte) (Note, error) {
	text := string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	if !strings.HasPrefix(text, fenceLine) {
		return Note{}, &Error{Path: path, Line: 1, Message: "a note file must start with a --- line"}
	}
	front, body, isClosed := splitFrontmatter(text[len(fenceLine):])
	if !isClosed {
		return Note{}, &Error{
			Path:    path,
			Line:    1,
			Message: "the frontmatter opened here is never closed by a --- line",
		}
	}

	p := parser{path: path}
	root, err := p.document(front)
	if err != nil {
		return Note{}, err
	}
	note := Note{Body: body, lines: map[string]int{}}
	if root == nil {
		return note, nil
	}
	if err := p.fields(root, &note); err != nil {
		return Note{}, err
	}
	return note, nil
}

// Validate checks note and returns every problem it finds, or nil when there is none. path names
// the file in each problem, and a problem cites the line of the key it concerns when the Note came
// from Parse. A Note needs a title, every label must be non-empty, and every Ref must name a path.
// Whether a Ref resolves is Verify's to check, so a Note with a stale Ref stays usable.
func Validate(path string, note Note) []*Error {
	var problems []*Error
	if strings.TrimSpace(note.Title) == "" {
		problems = append(problems, &Error{
			Path:    path,
			Line:    note.lines[keyTitle],
			Message: keyTitle + " is required",
		})
	}
	for _, label := range note.Labels {
		if label == "" {
			problems = append(problems, &Error{
				Path:    path,
				Line:    note.lines[keyLabels],
				Message: keyLabels + ": every entry must be non-empty text",
			})
			break
		}
	}
	for i, ref := range note.Refs {
		refPath, _, _ := strings.Cut(ref, symbolSeparator)
		if refPath == "" {
			problems = append(problems, &Error{
				Path:    path,
				Line:    note.RefLine(i),
				Message: fmt.Sprintf("ref %q %v", ref, errNoPath),
			})
		}
	}
	return problems
}

// Serialize writes the Note: the frontmatter keys in the fixed order title, labels, refs, aliases,
// empty fields left out, lists in flow style, text plain where YAML reads it back unchanged and
// quoted otherwise; then the body verbatim.
func (n Note) Serialize() []byte {
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	addText(mapping, keyTitle, n.Title)
	addList(mapping, keyLabels, n.Labels)
	addList(mapping, keyRefs, n.Refs)
	addList(mapping, keyAliases, n.Aliases)

	var out strings.Builder
	out.WriteString(fenceLine)
	if len(mapping.Content) > 0 {
		var encoded bytes.Buffer
		encoder := yaml.NewEncoder(&encoded)
		// Encoding a tree of string scalars cannot fail.
		_ = encoder.Encode(mapping)
		_ = encoder.Close()
		out.Write(encoded.Bytes())
	}
	out.WriteString(fenceLine)
	out.WriteString(n.Body)
	return []byte(out.String())
}

// addText appends key: value to mapping, unless value is empty.
func addText(mapping *yaml.Node, key string, value string) {
	if value == "" {
		return
	}
	mapping.Content = append(mapping.Content, stringNode(key), stringNode(value))
}

// addList appends key: [entries] in flow style to mapping, unless entries is empty.
func addList(mapping *yaml.Node, key string, entries []string) {
	if len(entries) == 0 {
		return
	}
	list := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, entry := range entries {
		list.Content = append(list.Content, stringNode(entry))
	}
	mapping.Content = append(mapping.Content, stringNode(key), list)
}

// stringNode is a text scalar; the encoder quotes it whenever YAML would read it as anything else.
func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagString, Value: value}
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
	if isNull(root) {
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

// fields applies every key of root onto note, rejecting unknown and repeated keys.
func (p parser) fields(root *yaml.Node, note *Note) error {
	for index := 0; index+1 < len(root.Content); index += 2 {
		key, value := root.Content[index], root.Content[index+1]
		if _, isSeen := note.lines[key.Value]; isSeen {
			return p.errorAt(key, "duplicate key %q", key.Value)
		}
		if err := p.field(key, value, note); err != nil {
			return err
		}
		note.lines[key.Value] = key.Line + frontmatterLineOffset
	}
	return nil
}

// field decodes one key's value into note.
func (p parser) field(key, value *yaml.Node, note *Note) error {
	var err error
	switch key.Value {
	case keyTitle:
		note.Title, err = p.text(key.Value, value)
	case keyLabels:
		note.Labels, _, err = p.list(key.Value, value)
	case keyRefs:
		note.Refs, note.refLines, err = p.list(key.Value, value)
	case keyAliases:
		note.Aliases, _, err = p.list(key.Value, value)
	default:
		err = p.errorAt(
			key,
			"unknown key %q; a note has %s, %s, %s and %s",
			key.Value,
			keyTitle,
			keyLabels,
			keyRefs,
			keyAliases,
		)
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

// list reads a flow- or block-style list of non-empty text entries, with the file line of each;
// an empty list reads as nil.
func (p parser) list(key string, value *yaml.Node) ([]string, []int, error) {
	if isNull(value) {
		return nil, nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, nil, p.errorAt(value, "%s: expected a list such as [a, b]", key)
	}
	if len(value.Content) == 0 {
		return nil, nil, nil
	}
	entries := make([]string, 0, len(value.Content))
	lines := make([]int, 0, len(value.Content))
	for _, item := range value.Content {
		if item.Kind != yaml.ScalarNode || isNull(item) || item.Value == "" {
			return nil, nil, p.errorAt(item, "%s: every entry must be non-empty text", key)
		}
		entries = append(entries, item.Value)
		lines = append(lines, item.Line+frontmatterLineOffset)
	}
	return entries, lines, nil
}
