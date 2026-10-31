// Package batch reads apply files: YAML lists of Issue operations that capcom apply runs all or
// none. Parse turns a file into typed operations, each with its position and line, and refuses an
// invalid file naming the operation's position and line; running the operations is the caller's.
package batch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/airiclenz/capcom/internal/ops"
)

// stdinArgument is the file name that reads the apply file from standard input; stdinName names
// that file in an error.
const (
	stdinArgument = "-"
	stdinName     = "stdin"
)

// firstLine is the line an error about a file with no operations at all names.
const firstLine = 1

// The tags yaml.v3 resolves a scalar to that Parse tells apart.
const (
	tagNull    = "!!null"
	tagInteger = "!!int"
	tagBoolean = "!!bool"
)

// yamlLine splits a yaml.v3 syntax error into its line and its message.
var yamlLine = regexp.MustCompile(`^yaml: line ([0-9]+): (.*)$`)

// Kind is the operation an entry of an apply file names with its op key.
type Kind string

// The operations an apply file can name.
const (
	KindCreate Kind = "create"
	KindUpdate Kind = "update"
	KindDepAdd Kind = "dep add"
	KindClose  Kind = "close"
	KindLog    Kind = "log"
)

// kinds lists every Kind, in the order a refusal names them.
var kinds = []Kind{KindCreate, KindUpdate, KindDepAdd, KindClose, KindLog}

// The keys of an operation: op names it, and the others are the flag names of its command without
// the leading --, plus id (the issue it writes), dep (dep add's dependency) and text (log's text).
const (
	keyOp             = "op"
	keyID             = "id"
	keyIfMatch        = "if-match"
	keyTitle          = "title"
	keyBody           = "body"
	keyWhy            = "why"
	keyAcceptance     = "acceptance"
	keyDep            = "dep"
	keyLabel          = "label"
	keyParent         = "parent"
	keyPlan           = "plan"
	keyPriority       = "priority"
	keyStanding       = "standing"
	keyNoStanding     = "no-standing"
	keyStatus         = "status"
	keyType           = "type"
	keyAddLabel       = "add-label"
	keyRemoveLabel    = "remove-label"
	keyBodyAppend     = "body-append"
	keyBodyReplaceOld = "body-replace-old"
	keyBodyReplaceNew = "body-replace-new"
	keyReason         = "reason"
	keyText           = "text"
	keyBodyFile       = "body-file"
	keyBodyAppendFile = "body-append-file"
)

// valueKind is the YAML shape the value of a key takes.
type valueKind int

const (
	// textValue is a string: any scalar, its text whatever its tag, and "" when it is null.
	textValue valueKind = iota
	// numberValue is an integer.
	numberValue
	// booleanValue is true or false.
	booleanValue
	// listValue is a string or a list of strings, each split on commas, as a repeatable flag that
	// separates its values with commas takes them.
	listValue
	// textsValue is a string or a list of strings, each kept whole, as a repeatable flag that takes
	// any text.
	textsValue
)

// keySpec is one key an operation takes and the shape of its value.
type keySpec struct {
	name string
	kind valueKind
}

// keysOf lists the keys each Kind takes besides op, in the order a refusal names them.
var keysOf = map[Kind][]keySpec{
	KindCreate: {
		{keyID, textValue}, {keyTitle, textValue}, {keyBody, textValue}, {keyWhy, textValue},
		{keyAcceptance, textsValue}, {keyDep, listValue}, {keyLabel, listValue}, {keyParent, textValue},
		{keyPlan, textValue}, {keyPriority, numberValue}, {keyStanding, booleanValue},
		{keyStatus, textValue}, {keyType, textValue},
	},
	KindUpdate: {
		{keyID, textValue}, {keyIfMatch, textValue}, {keyTitle, textValue}, {keyStatus, textValue},
		{keyType, textValue}, {keyPriority, numberValue}, {keyPlan, textValue},
		{keyStanding, booleanValue}, {keyNoStanding, booleanValue}, {keyAddLabel, listValue},
		{keyRemoveLabel, listValue}, {keyBodyAppend, textsValue}, {keyBodyReplaceOld, textsValue},
		{keyBodyReplaceNew, textsValue},
	},
	KindDepAdd: {{keyID, textValue}, {keyDep, textValue}, {keyIfMatch, textValue}},
	KindClose:  {{keyID, textValue}, {keyReason, textValue}, {keyStatus, textValue}, {keyIfMatch, textValue}},
	KindLog:    {{keyID, textValue}, {keyText, textValue}, {keyIfMatch, textValue}},
}

// requiredOf lists the keys each Kind cannot do without.
var requiredOf = map[Kind][]string{
	KindCreate: {keyID, keyTitle},
	KindUpdate: {keyID},
	KindDepAdd: {keyID, keyDep},
	KindClose:  {keyID, keyReason},
	KindLog:    {keyID, keyText},
}

// inlineOnlyKeys are the flags that read their text from a file, which an apply file refuses.
var inlineOnlyKeys = []string{keyBodyFile, keyBodyAppendFile}

// changingKeys are the keys of an update that change the issue, the flags capcom update requires
// at least one of.
var changingKeys = []string{
	keyTitle, keyStatus, keyType, keyPriority, keyPlan, keyAddLabel, keyRemoveLabel, keyBodyAppend,
	keyBodyReplaceOld, keyStanding, keyNoStanding,
}

// Op is one operation of an apply file.
type Op struct {
	// Kind is the operation.
	Kind Kind
	// Position is the operation's place in the file, counted from 1.
	Position int
	// Line is the file line the operation starts on.
	Line int
	// ID is the issue the operation writes: the new issue's id for a create, the reference to the
	// issue it changes for every other Kind.
	ID string
	// IfMatch is the etag the issue's file must have for update, dep add, close and log; "" checks
	// none. A create takes none.
	IfMatch string
	// Create is the issue a create writes. Created is left zero for the caller to set, and Body is
	// nil unless the operation gives a non-empty body.
	Create ops.NewIssue
	// Update is the change an update makes. It is empty when its only changing key changes nothing,
	// no-standing: false, which capcom update takes as rewriting the issue as it is.
	Update ops.IssueUpdate
	// Dep is the dependency a dep add adds.
	Dep string
	// Status is the status a close moves the issue to; "" takes the first done status.
	Status string
	// Reason is why a close closes the issue.
	Reason string
	// Text is the entry a log appends.
	Text string
}

// Error is an apply file Parse refuses: Name is the file ("stdin" for -), Line the line of the
// problem, and Position the place of the operation it is in, counted from 1, or 0 when the problem
// is the file's as a whole. Message starts "operation <n> (<op>): " for a problem in an operation,
// without the op when the operation names none Parse knows.
type Error struct {
	Name     string
	Line     int
	Position int
	Message  string
}

// Error renders "name:line: message", or "name: message" when the line is unknown.
func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Name, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Name, e.Message)
}

// Parse turns data, the apply file name names ("-" for stdin), into its operations in file order.
// The file is one YAML list of mappings, each with an op key naming its Kind and the keys that Kind
// takes. It refuses, as an *Error at the line of the problem, a file that is not YAML, holds more
// than one document, is no list or an empty one, and an operation that is no mapping, names no op or
// an unknown one, gives a key its Kind does not take, a file key (body-file, body-append-file), a
// key twice, standing with no-standing, a value of the wrong YAML shape, or lacks a required key;
// an update must give a key that changes the issue and its body-replace-old and body-replace-new
// in pairs. Values are not judged here: the ops checks own them.
func Parse(name string, data []byte) ([]Op, error) {
	if name == stdinArgument {
		name = stdinName
	}
	p := parser{name: name}
	list, err := p.list(data)
	if err != nil {
		return nil, err
	}
	operations := make([]Op, 0, len(list.Content))
	for index, entry := range list.Content {
		operation, err := p.operation(index+1, entry)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, nil
}

// parser reads the apply file name names.
type parser struct {
	name string
}

// fail returns the *Error of a problem of the file as a whole at line.
func (p parser) fail(line int, format string, args ...any) *Error {
	return &Error{Name: p.name, Line: line, Message: fmt.Sprintf(format, args...)}
}

// list returns the list of operations data holds, refusing a file that is not one non-empty YAML
// list.
func (p parser) list(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	err := decoder.Decode(&document)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, p.yamlError(err)
	}
	if err != nil || len(document.Content) == 0 || document.Content[0].ShortTag() == tagNull {
		return nil, p.fail(firstLine, "the file holds no operations; give a list of them")
	}
	var extra yaml.Node
	switch err := decoder.Decode(&extra); {
	case err == nil:
		return nil, p.fail(extra.Line, "a second YAML document; an apply file holds one list of operations")
	case !errors.Is(err, io.EOF):
		return nil, p.yamlError(err)
	}
	list := document.Content[0]
	if list.Kind != yaml.SequenceNode {
		return nil, p.fail(list.Line, "the file is %s; give a list of operations", describe(list))
	}
	if len(list.Content) == 0 {
		return nil, p.fail(list.Line, "the list holds no operations")
	}
	return list, nil
}

// yamlError turns a yaml.v3 syntax error into an *Error, lifting its line out of the message.
func (p parser) yamlError(err error) *Error {
	failure := p.fail(0, "invalid YAML: %s", strings.TrimPrefix(err.Error(), "yaml: "))
	if match := yamlLine.FindStringSubmatch(err.Error()); match != nil {
		line, convErr := strconv.Atoi(match[1])
		if convErr == nil {
			failure.Line = line
			failure.Message = "invalid YAML: " + match[2]
		}
	}
	return failure
}

// operation reads entry, the operation at position, into its Op.
func (p parser) operation(position int, entry *yaml.Node) (Op, error) {
	reader := operationReader{parser: p, position: position}
	if entry.Kind != yaml.MappingNode {
		return Op{}, reader.fail(entry.Line, "is %s; give a mapping of op and flag names", describe(entry))
	}
	kind, err := reader.readKind(entry)
	if err != nil {
		return Op{}, err
	}
	reader.kind = kind
	given, err := reader.values(entry)
	if err != nil {
		return Op{}, err
	}
	return reader.build(entry.Line, given)
}

// operationReader reads the operation at position, of kind once its op key is known.
type operationReader struct {
	parser
	position int
	kind     Kind
}

// fail returns the *Error of a problem at line in the operation.
func (r operationReader) fail(line int, format string, args ...any) *Error {
	label := fmt.Sprintf("operation %d", r.position)
	if r.kind != "" {
		label += fmt.Sprintf(" (%s)", r.kind)
	}
	return &Error{
		Name:     r.name,
		Line:     line,
		Position: r.position,
		Message:  label + ": " + fmt.Sprintf(format, args...),
	}
}

// readKind returns the Kind the op key of entry names, refusing an entry with no op key, or one whose
// op is no known Kind.
func (r operationReader) readKind(entry *yaml.Node) (Kind, error) {
	for index := 0; index+1 < len(entry.Content); index += 2 {
		key, value := entry.Content[index], entry.Content[index+1]
		if key.Kind != yaml.ScalarNode || key.Value != keyOp {
			continue
		}
		if value.Kind != yaml.ScalarNode {
			return "", r.fail(value.Line, "op is %s; give one of %s", describe(value), kindList())
		}
		if !slices.Contains(kinds, Kind(value.Value)) {
			return "", r.fail(value.Line, "unknown op %q; give one of %s", value.Value, kindList())
		}
		return Kind(value.Value), nil
	}
	return "", r.fail(entry.Line, "no op key; give one of %s", kindList())
}

// values reads every key of entry but op into its value, in file order, refusing the first key
// that is not a string, given twice, not taken by the Kind, or whose value has the wrong shape.
func (r operationReader) values(entry *yaml.Node) (values, error) {
	given := values{}
	isSeen := map[string]bool{}
	for index := 0; index+1 < len(entry.Content); index += 2 {
		key, node := entry.Content[index], entry.Content[index+1]
		if key.Kind != yaml.ScalarNode {
			return nil, r.fail(key.Line, "a key is %s; keys are flag names", describe(key))
		}
		if isSeen[key.Value] {
			return nil, r.fail(key.Line, "%s given twice; give it once", key.Value)
		}
		isSeen[key.Value] = true
		if key.Value == keyOp {
			continue
		}
		spec, err := r.spec(key, isSeen)
		if err != nil {
			return nil, err
		}
		value, err := r.value(spec, node)
		if err != nil {
			return nil, err
		}
		given[spec.name] = value
	}
	return given, nil
}

// spec returns the keySpec of key, refusing a file key, a key the Kind does not take, and standing
// or no-standing when the other is among the keys isSeen already holds.
func (r operationReader) spec(key *yaml.Node, isSeen map[string]bool) (keySpec, error) {
	if slices.Contains(inlineOnlyKeys, key.Value) {
		return keySpec{}, r.fail(key.Line, "%s is refused: apply files take text inline", key.Value)
	}
	specs := keysOf[r.kind]
	index := slices.IndexFunc(specs, func(spec keySpec) bool { return spec.name == key.Value })
	if index < 0 {
		return keySpec{}, r.fail(key.Line, "unknown key %q; %s takes %s", key.Value, r.kind, keyList(specs))
	}
	if (key.Value == keyStanding && isSeen[keyNoStanding]) || (key.Value == keyNoStanding && isSeen[keyStanding]) {
		return keySpec{}, r.fail(key.Line, "%s and %s cannot be combined", keyStanding, keyNoStanding)
	}
	return specs[index], nil
}

// value reads node, the value of the key spec names, refusing one of the wrong shape.
func (r operationReader) value(spec keySpec, node *yaml.Node) (value, error) {
	switch spec.kind {
	case numberValue:
		var number int
		if node.Kind != yaml.ScalarNode || node.ShortTag() != tagInteger || node.Decode(&number) != nil {
			return value{}, r.fail(node.Line, "%s is %s; give an integer", spec.name, describe(node))
		}
		return value{number: number}, nil
	case booleanValue:
		var isTrue bool
		if node.Kind != yaml.ScalarNode || node.ShortTag() != tagBoolean || node.Decode(&isTrue) != nil {
			return value{}, r.fail(node.Line, "%s is %s; give true or false", spec.name, describe(node))
		}
		return value{isTrue: isTrue}, nil
	case listValue, textsValue:
		return r.list(spec, node)
	}
	text, err := r.text(spec.name, node)
	return value{text: text}, err
}

// text returns the text of node, a value of the key name: any scalar's text whatever its tag, ""
// for a null one, refusing a node that is no scalar.
func (r operationReader) text(name string, node *yaml.Node) (string, error) {
	if node.Kind != yaml.ScalarNode {
		return "", r.fail(node.Line, "%s is %s; give a string", name, describe(node))
	}
	if node.ShortTag() == tagNull {
		return "", nil
	}
	return node.Value, nil
}

// list reads node, the value of the repeatable key spec names: one string or a list of them, each
// split on commas for a listValue, refusing any other node.
func (r operationReader) list(spec keySpec, node *yaml.Node) (value, error) {
	items := []*yaml.Node{node}
	if node.Kind == yaml.SequenceNode {
		items = node.Content
	} else if node.Kind != yaml.ScalarNode {
		return value{}, r.fail(node.Line, "%s is %s; give a string or a list of strings", spec.name, describe(node))
	}
	texts := make([]string, 0, len(items))
	for _, item := range items {
		text, err := r.text(spec.name, item)
		if err != nil {
			return value{}, err
		}
		if spec.kind == listValue {
			texts = append(texts, splitCommas(text)...)
			continue
		}
		texts = append(texts, text)
	}
	return value{texts: texts}, nil
}

// build returns the Op of the operation starting at line from its given values, refusing one that
// lacks a required key, and an update that changes nothing or gives uneven replacement pairs.
func (r operationReader) build(line int, given values) (Op, error) {
	for _, name := range requiredOf[r.kind] {
		if !given.has(name) {
			return Op{}, r.fail(line, "missing required key %s", name)
		}
	}
	operation := Op{
		Kind:     r.kind,
		Position: r.position,
		Line:     line,
		ID:       given[keyID].text,
		IfMatch:  given[keyIfMatch].text,
	}
	switch r.kind {
	case KindCreate:
		operation.Create = newIssue(given)
	case KindUpdate:
		update, err := r.update(line, given)
		if err != nil {
			return Op{}, err
		}
		operation.Update = update
	case KindDepAdd:
		operation.Dep = given[keyDep].text
	case KindClose:
		operation.Status = given[keyStatus].text
		operation.Reason = given[keyReason].text
	case KindLog:
		operation.Text = given[keyText].text
	}
	return operation, nil
}

// newIssue returns the issue the given values of a create describe, as capcom create's flags do.
func newIssue(given values) ops.NewIssue {
	created := ops.NewIssue{
		ID:         given[keyID].text,
		Parent:     given[keyParent].text,
		Title:      given[keyTitle].text,
		Status:     given[keyStatus].text,
		Type:       given[keyType].text,
		Priority:   given.number(keyPriority),
		Labels:     given[keyLabel].texts,
		Plan:       given[keyPlan].text,
		Deps:       given[keyDep].texts,
		IsStanding: given[keyStanding].isTrue,
		Why:        given[keyWhy].text,
		Acceptance: given[keyAcceptance].texts,
	}
	if body := given[keyBody].text; body != "" {
		created.Body = func() (string, error) { return body, nil }
	}
	return created
}

// update returns the change the given values of the update starting at line describe, as capcom
// update's flags do, refusing uneven replacement pairs and an update with no key that changes the
// issue.
func (r operationReader) update(line int, given values) (ops.IssueUpdate, error) {
	olds, news := given[keyBodyReplaceOld].texts, given[keyBodyReplaceNew].texts
	if len(olds) != len(news) {
		return ops.IssueUpdate{}, r.fail(
			line,
			"%d %s and %d %s given; give them in pairs",
			len(olds),
			keyBodyReplaceOld,
			len(news),
			keyBodyReplaceNew,
		)
	}
	if !slices.ContainsFunc(changingKeys, given.has) {
		return ops.IssueUpdate{}, r.fail(line, "update needs at least one flag that changes the issue")
	}
	change := ops.IssueUpdate{
		Title:        given.text(keyTitle),
		Status:       given.text(keyStatus),
		Type:         given.text(keyType),
		Priority:     given.number(keyPriority),
		Plan:         given.text(keyPlan),
		AddLabels:    given[keyAddLabel].texts,
		RemoveLabels: given[keyRemoveLabel].texts,
		Appends:      given[keyBodyAppend].texts,
	}
	for index, old := range olds {
		change.Replacements = append(change.Replacements, ops.Replacement{Old: old, New: news[index]})
	}
	switch {
	case given.has(keyStanding):
		isStanding := given[keyStanding].isTrue
		change.IsStanding = &isStanding
	case given[keyNoStanding].isTrue:
		isStanding := false
		change.IsStanding = &isStanding
	}
	return change, nil
}

// value is the value of one key, in the field its valueKind fills.
type value struct {
	text   string
	number int
	isTrue bool
	texts  []string
}

// values maps each key an operation gives, but op, to its value.
type values map[string]value

// has reports whether the operation gives the key name.
func (v values) has(name string) bool {
	_, isGiven := v[name]
	return isGiven
}

// text returns the text of the key name, nil when the operation does not give it.
func (v values) text(name string) *string {
	if !v.has(name) {
		return nil
	}
	text := v[name].text
	return &text
}

// number returns the integer of the key name, nil when the operation does not give it.
func (v values) number(name string) *int {
	if !v.has(name) {
		return nil
	}
	number := v[name].number
	return &number
}

// splitCommas splits text on commas into its values, each trimmed of white space; a text that is
// empty or only white space holds none.
func splitCommas(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	parts := strings.Split(text, ",")
	for index, part := range parts {
		parts[index] = strings.TrimSpace(part)
	}
	return parts
}

// describe words the shape of node in a refusal: a scalar's text quoted, else what it is.
func describe(node *yaml.Node) string {
	switch node.Kind {
	case yaml.ScalarNode:
		return strconv.Quote(node.Value)
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a list"
	case yaml.AliasNode:
		return "an alias"
	}
	return "a document"
}

// kindList words every Kind for a refusal: "create, update, dep add, close or log".
func kindList() string {
	names := make([]string, 0, len(kinds))
	for _, kind := range kinds[:len(kinds)-1] {
		names = append(names, string(kind))
	}
	return strings.Join(names, ", ") + " or " + string(kinds[len(kinds)-1])
}

// keyList words the keys specs names for a refusal, op first.
func keyList(specs []keySpec) string {
	names := []string{keyOp}
	for _, spec := range specs {
		names = append(names, spec.name)
	}
	return strings.Join(names, ", ")
}
