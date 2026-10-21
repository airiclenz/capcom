package hooks

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// ApogeePath is the project apogee config InstallApogee adds the capcom prime reaction to, relative
// to the working tree root.
const ApogeePath = ".apogee/config.yaml"

// Keys and values of the capcom prime reaction in the apogee config.
const (
	apogeeReactionsKey = "reactions"
	apogeeIDKey        = "id"
	apogeeAdviseKey    = "advise"
	apogeeReactionID   = "capcom-prime"
	apogeeIndent       = "  "
)

// apogeeReactionLines are the lines of the capcom prime reaction as an item of a block list, without
// indentation or line endings. advise is an argv list: apogee refuses a bare-string advise.
var apogeeReactionLines = []string{
	"- " + apogeeIDKey + ": " + apogeeReactionID,
	"  on: [session-started, compaction-finished]",
	"  " + apogeeAdviseKey + `: ["capcom", "prime"]`,
}

// Refusals of an apogee config MergeApogee cannot edit.
const (
	apogeeNotMappingMessage  = "the config must be a mapping of keys"
	apogeeFlowRootMessage    = "the config must be a block mapping; add the capcom-prime entry by hand"
	apogeeSecondDocMessage   = "only one YAML document is allowed"
	apogeeDocumentEndMessage = "a `...` document end line is not allowed; remove it"
	apogeeReactionsMessage   = "reactions must be a block list; add the capcom-prime entry by hand"
	apogeeInvalidPrefix      = "invalid config: "
)

// documentEndLine matches a `...` document end marker, trailing blanks or a comment allowed;
// yamlLinePrefix matches the `yaml: line N: ` prefix of a yaml.v3 syntax error.
var (
	documentEndLine = regexp.MustCompile(`^\.\.\.(?:[ \t]+(?:#.*)?)?$`)
	yamlLinePrefix  = regexp.MustCompile(`^yaml: line ([0-9]+): `)
)

// InstallApogee adds the capcom prime reaction to the apogee config in the working tree at
// worktreeRoot, creating the file and its directory when missing, and reports whether it wrote the
// file. The file is read through nodefile.ReadRegularFile, so a symlink is never followed and a
// named pipe never blocks. An existing file keeps every byte MergeApogee does not add, its line
// ending and its permission bits: it is replaced through nodefile.ReplaceFileVerbatim, never
// through nodefile.WriteFileAtomic, which would reset the mode. A file that is not a regular file,
// is larger than 1 MiB, cannot be read or replaced, or that MergeApogee refuses fails as *Error
// naming ApogeePath and leaves the file untouched; a failed write of a new file returns
// nodefile.WriteFileAtomic's error as is.
func InstallApogee(worktreeRoot string) (Status, error) {
	path := filepath.Join(worktreeRoot, filepath.FromSlash(ApogeePath))
	content, isMissing, err := readWorktreeFile(path, ApogeePath)
	if err != nil {
		return "", err
	}
	merged, status, err := MergeApogee(content)
	if err != nil || status == Present {
		return status, err
	}
	if isMissing {
		// A new file has no bytes, mode or line ending to keep.
		if err := nodefile.WriteFileAtomic(path, bytes.NewReader(merged)); err != nil {
			return "", err
		}
		return Added, nil
	}
	if err := nodefile.ReplaceFileVerbatim(path, bytes.NewReader(merged)); err != nil {
		return "", &Error{Path: ApogeePath, Message: err.Error()}
	}
	return Added, nil
}

// MergeApogee adds the capcom prime reaction, firing on session-started and compaction-finished,
// to the apogee config content, which is empty for a missing file. Without a reactions key it
// appends `reactions:` and the entry at the end of the file; with a block-list reactions it inserts
// the entry after the list's last item, its `- ` at the list's own indentation. Every other byte is
// kept: the content is edited as lines, never re-encoded, and new lines end CRLF when the content
// holds a CRLF, LF otherwise; a last line without an ending gains one. When a reactions item already
// has the id capcom-prime or advises `capcom prime`, merged is content unchanged and status is
// Present. It fails as *Error naming ApogeePath, at the offending line when one is to blame, on
// YAML that does not parse, a second YAML document or a `...` end line, a document that is not a
// block mapping, and a reactions key whose value is not a block list.
func MergeApogee(content []byte) (merged []byte, status Status, err error) {
	lines := bytes.SplitAfter(content, []byte("\n"))
	if index := slices.IndexFunc(lines, isDocumentEnd); index >= 0 {
		return nil, "", &Error{Path: ApogeePath, Line: index + 1, Message: apogeeDocumentEndMessage}
	}
	root, err := apogeeRoot(content)
	if err != nil {
		return nil, "", err
	}
	ending := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		ending = "\r\n"
	}

	key, reactions := mappingEntry(root, apogeeReactionsKey)
	if reactions == nil {
		indent := ""
		if root != nil {
			indent = strings.Repeat(" ", root.Column-1)
		}
		block := indent + apogeeReactionsKey + ":" + ending + reactionBlock(indent+apogeeIndent, ending)
		return insertBlock(lines, len(lines), block, ending), Added, nil
	}
	if reactions.Kind == yaml.SequenceNode && slices.ContainsFunc(reactions.Content, isPrimeReaction) {
		return content, Present, nil
	}
	if reactions.Kind != yaml.SequenceNode || reactions.Style&yaml.FlowStyle != 0 {
		return nil, "", &Error{Path: ApogeePath, Line: key.Line, Message: apogeeReactionsMessage}
	}
	last := reactions.Content[len(reactions.Content)-1]
	dashIndent := itemDashIndent(lines, last)
	after := lastItemLine(lines, last.Line-1, dashIndent)
	return insertBlock(lines, after+1, reactionBlock(strings.Repeat(" ", dashIndent), ending), ending), Added, nil
}

// itemDashIndent returns the number of spaces before the `- ` of the block-list item. It is read
// from the item's own lines, never from the list node's Column: yaml.v3 puts that at the list's
// anchor or tag when it has one (`reactions: &r`), not at its first dash. The dash is the first
// line, from the item's line upwards, that starts with `-` left of the item's own column; it sits
// on a line above when the dash stands alone (`-` with the item on the next line).
func itemDashIndent(lines [][]byte, item *yaml.Node) int {
	indent := 0
	for index := item.Line - 1; index >= 0; index-- {
		content := bytes.TrimLeft(lines[index], " ")
		indent = len(lines[index]) - len(content)
		if bytes.HasPrefix(content, []byte("-")) && indent < item.Column-1 {
			break
		}
	}
	return indent
}

// isDocumentEnd reports whether line is a `...` document end marker. yaml.v3 stops reading at one
// and reports no second document after it, so a key behind it would escape MergeApogee's checks.
func isDocumentEnd(line []byte) bool {
	return documentEndLine.Match(bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r")))
}

// apogeeRoot parses content into its top-level block mapping, or nil for a document without one
// (empty, only comments, or only a `---` line). It refuses as MergeApogee describes.
func apogeeRoot(content []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, apogeeYAMLError(err)
	}
	var second yaml.Node
	if err := decoder.Decode(&second); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, apogeeYAMLError(err)
		}
		return nil, &Error{Path: ApogeePath, Line: second.Line, Message: apogeeSecondDocMessage}
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	root := document.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" && root.Value == "" {
		return nil, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, &Error{Path: ApogeePath, Line: root.Line, Message: apogeeNotMappingMessage}
	}
	if root.Style&yaml.FlowStyle != 0 {
		return nil, &Error{Path: ApogeePath, Line: root.Line, Message: apogeeFlowRootMessage}
	}
	return root, nil
}

// apogeeYAMLError turns a yaml.v3 syntax error into an *Error at the line its `yaml: line N: `
// prefix names, the prefix stripped from the message; an error without one keeps Line 0.
func apogeeYAMLError(err error) *Error {
	message := err.Error()
	failure := &Error{Path: ApogeePath, Message: apogeeInvalidPrefix + message}
	match := yamlLinePrefix.FindStringSubmatch(message)
	if match == nil {
		return failure
	}
	// The prefix holds digits only, so only a line too large for an int fails; it keeps Line 0.
	if line, err := strconv.Atoi(match[1]); err == nil {
		failure.Line = line
	}
	failure.Message = apogeeInvalidPrefix + message[len(match[0]):]
	return failure
}

// mappingEntry returns the key and value nodes of the entry named name in mapping, or nils when
// mapping is nil, is not a mapping or has no such entry.
func mappingEntry(mapping *yaml.Node, name string) (key, value *yaml.Node) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if candidate := mapping.Content[index]; candidate.Kind == yaml.ScalarNode && candidate.Value == name {
			return candidate, mapping.Content[index+1]
		}
	}
	return nil, nil
}

// isPrimeReaction reports whether the reactions item has the id capcom-prime or advises the argv
// of PrimeCommand.
func isPrimeReaction(item *yaml.Node) bool {
	if _, id := mappingEntry(item, apogeeIDKey); id != nil && id.Kind == yaml.ScalarNode && id.Value == apogeeReactionID {
		return true
	}
	_, advise := mappingEntry(item, apogeeAdviseKey)
	if advise == nil || advise.Kind != yaml.SequenceNode {
		return false
	}
	argv := make([]string, 0, len(advise.Content))
	for _, argument := range advise.Content {
		if argument.Kind != yaml.ScalarNode {
			return false
		}
		argv = append(argv, argument.Value)
	}
	return slices.Equal(argv, strings.Fields(PrimeCommand))
}

// reactionBlock returns the capcom prime reaction as block-list lines, each indented by indent and
// ended with ending.
func reactionBlock(indent, ending string) string {
	var block strings.Builder
	for _, line := range apogeeReactionLines {
		block.WriteString(indent + line + ending)
	}
	return block.String()
}

// lastItemLine returns the index of the last line of the block-list item starting at lines[start],
// the list's `- ` at dashIndent spaces: the item runs on while lines are blank or indented further
// than its dash, and ends at the last of them that is not blank.
func lastItemLine(lines [][]byte, start, dashIndent int) int {
	last := start
	for index := start + 1; index < len(lines); index++ {
		content := bytes.TrimRight(lines[index], " \t\r\n")
		if len(content) == 0 {
			continue
		}
		if len(content)-len(bytes.TrimLeft(content, " ")) <= dashIndent {
			break
		}
		last = index
	}
	return last
}

// insertBlock returns lines joined with block inserted before lines[at]. A line above the block
// without a line ending gains ending first, so the block starts a line of its own.
func insertBlock(lines [][]byte, at int, block string, ending string) []byte {
	var joined bytes.Buffer
	for _, line := range lines[:at] {
		joined.Write(line)
	}
	if at > 0 && len(lines[at-1]) > 0 && !bytes.HasSuffix(lines[at-1], []byte("\n")) {
		joined.WriteString(ending)
	}
	joined.WriteString(block)
	for _, line := range lines[at:] {
		joined.Write(line)
	}
	return joined.Bytes()
}
