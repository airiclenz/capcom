package frontmatter

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/airiclenz/capcom/internal/rawfm"
)

// tagNull is the YAML tag the parser resolves an empty value to.
const tagNull = "!!null"

// yamlLineError matches the line a yaml.v3 syntax error names, relative to the frontmatter.
var yamlLineError = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

// Kind is one kind of file Decode reads: an issue or a Note.
type Kind struct {
	// Noun names one file of the kind with its article, "an issue" or "a note". It words the
	// missing-fence failure ("a note file must start with a --- line") and the unknown-key hint.
	Noun string
	// Keys lists every frontmatter key the kind knows, in the order the unknown-key hint names
	// them. Any other key is an error (ADR 0009).
	Keys []string
}

// Document is what Decode returns besides the fields it hands its callback.
type Document struct {
	// Body is the markdown after the frontmatter, byte for byte (LF line endings).
	Body string
	// Lines maps each frontmatter key the file holds to its 1-based line in the file. It is
	// never nil.
	Lines map[string]int
}

// Field is one frontmatter key and its value, as Decode hands it to the kind's callback. Its
// helpers read the value in the shape the kind expects and fail with an *Error citing the line of
// the value.
type Field struct {
	// Key is the frontmatter key, one of the kind's Keys.
	Key string
	// Line is the 1-based line of the key in the file.
	Line int

	path  string
	value *yaml.Node
}

// Decode reads the file content data of the given kind, as Split does, and calls apply once for
// each frontmatter key, in file order. path names the file in every error. A frontmatter with no
// keys calls apply never.
//
// Decode fails, with an *Error citing the file line, wherever Split does, on invalid YAML, on a
// frontmatter that is not a mapping, on a repeated key, on a key outside kind.Keys (its message
// ends with a hint generated from them: `; a note has title, labels, refs and aliases`), and with
// the first error apply returns. When the frontmatter holds a git conflict marker, a failure past
// Split cites the first marker instead, in the words `capcom check` reports it with, since the
// marker, not the YAML it breaks, is what to fix.
func Decode(path string, data []byte, kind Kind, apply func(Field) error) (Document, error) {
	front, body, err := Split(path, data, kind.Noun)
	if err != nil {
		return Document{}, err
	}

	d := decoder{path: path, kind: kind, apply: apply}
	lines, err := d.decode(front)
	if err != nil {
		return Document{}, d.conflictError(data, front, err)
	}
	return Document{Body: body, Lines: lines}, nil
}

// decoder decodes the frontmatter of one file, attributing every failure to its path and line.
type decoder struct {
	path  string
	kind  Kind
	apply func(Field) error
}

// decode applies every key of the frontmatter front and returns the file line of each.
func (d decoder) decode(front string) (map[string]int, error) {
	root, err := d.document(front)
	if err != nil {
		return nil, err
	}
	lines := map[string]int{}
	if root == nil {
		return lines, nil
	}
	for index := 0; index+1 < len(root.Content); index += 2 {
		key, value := root.Content[index], root.Content[index+1]
		if _, isSeen := lines[key.Value]; isSeen {
			return nil, errorAt(d.path, key, "duplicate key %q", key.Value)
		}
		if !slices.Contains(d.kind.Keys, key.Value) {
			return nil, errorAt(d.path, key, "unknown key %q%s", key.Value, d.keysHint())
		}
		line := key.Line + frontmatterLineOffset
		field := Field{Key: key.Value, Line: line, path: d.path, value: value}
		if err := d.apply(field); err != nil {
			return nil, err
		}
		lines[key.Value] = line
	}
	return lines, nil
}

// document parses the frontmatter into its top-level mapping node, or nil when it holds no keys.
func (d decoder) document(front string) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(front), &document); err != nil {
		return nil, d.yamlError(err)
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	root := document.Content[0]
	if isNull(root) {
		return nil, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, errorAt(d.path, root, "the frontmatter must be a mapping of keys")
	}
	return root, nil
}

// keysHint is the unknown-key hint naming the kind's keys, "; a note has title, labels, refs and
// aliases", or "" for a kind that names none.
func (d decoder) keysHint() string {
	keys := d.kind.Keys
	switch len(keys) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("; %s has %s", d.kind.Noun, keys[0])
	}
	last := len(keys) - 1
	return fmt.Sprintf("; %s has %s and %s", d.kind.Noun, strings.Join(keys[:last], ", "), keys[last])
}

// conflictError returns err, the failure to decode the frontmatter front of the file content data,
// unless the frontmatter holds a git conflict marker: then an *Error at the first such marker.
func (d decoder) conflictError(data []byte, front string, err error) error {
	lastFrontmatterLine := strings.Count(front, "\n") + frontmatterLineOffset
	markers := rawfm.ConflictMarkers(data)
	if len(markers) == 0 || markers[0].Line > lastFrontmatterLine {
		return err
	}
	return &Error{Path: d.path, Line: markers[0].Line, Message: rawfm.ConflictMessage(markers[0])}
}

// yamlError turns a yaml.v3 syntax error into an *Error citing the file line it names.
func (d decoder) yamlError(err error) *Error {
	match := yamlLineError.FindStringSubmatch(err.Error())
	if match == nil {
		return &Error{Path: d.path, Message: fmt.Sprintf("invalid YAML frontmatter: %v", err)}
	}
	line, _ := strconv.Atoi(match[1]) // the pattern admits digits only
	return &Error{
		Path:    d.path,
		Line:    line + frontmatterLineOffset,
		Message: "invalid YAML frontmatter: " + match[2],
	}
}

// IsEmpty reports whether the value is left empty (`key:` with nothing after it), which every
// helper reads as the zero value.
func (f Field) IsEmpty() bool {
	return isNull(f.value)
}

// Scalar returns the value's text exactly as the file gives it, and false when the value is a
// list or a mapping. An empty value gives "" and true.
func (f Field) Scalar() (string, bool) {
	if f.value.Kind != yaml.ScalarNode {
		return "", false
	}
	if f.IsEmpty() {
		return "", true
	}
	return f.value.Value, true
}

// Errorf returns an *Error citing the line of the value, with the message format and args give.
// A kind's callback reports a scalar it cannot read with it.
func (f Field) Errorf(format string, args ...any) *Error {
	return errorAt(f.path, f.value, format, args...)
}

// Text reads the value as text; an empty value reads as "". It fails on a list or a mapping.
func (f Field) Text() (string, error) {
	text, isScalar := f.Scalar()
	if !isScalar {
		return "", f.Errorf("%s: expected text", f.Key)
	}
	return text, nil
}

// List reads a flow- or block-style list of non-empty text entries, with the 1-based file line of
// each; an empty value or an empty list reads as nil. It fails on a value that is not a list and
// on an entry that is not non-empty text, citing that entry's line.
func (f Field) List() (entries []string, lines []int, err error) {
	if f.IsEmpty() {
		return nil, nil, nil
	}
	if f.value.Kind != yaml.SequenceNode {
		return nil, nil, f.Errorf("%s: expected a list such as [a, b]", f.Key)
	}
	if len(f.value.Content) == 0 {
		return nil, nil, nil
	}
	entries = make([]string, 0, len(f.value.Content))
	lines = make([]int, 0, len(f.value.Content))
	for _, item := range f.value.Content {
		if item.Kind != yaml.ScalarNode || isNull(item) || item.Value == "" {
			return nil, nil, errorAt(f.path, item, "%s: every entry must be non-empty text", f.Key)
		}
		entries = append(entries, item.Value)
		lines = append(lines, item.Line+frontmatterLineOffset)
	}
	return entries, lines, nil
}

// errorAt returns an *Error at the file line of node.
func errorAt(path string, node *yaml.Node, format string, args ...any) *Error {
	return &Error{
		Path:    path,
		Line:    node.Line + frontmatterLineOffset,
		Message: fmt.Sprintf(format, args...),
	}
}

// isNull reports whether node is an empty value.
func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == tagNull
}
