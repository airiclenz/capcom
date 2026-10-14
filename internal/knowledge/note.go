// Package knowledge reads and writes Note files and checks their Refs. A Note is markdown with a
// YAML frontmatter between two `---` lines holding title, labels, refs, retired, superseded-by and
// aliases; its id is the file name and its place in the Knowledge Tree is its directory (ADR 0001). Parse turns a file
// into a Note and Serialize writes one back, both through the shared frontmatter codec; Validate
// checks a Note, and Verify reports the Refs that no longer resolve in the working tree. Every
// error names the file, and the line when one is to blame.
package knowledge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/tree"
)

// Frontmatter keys, in the order Serialize writes them.
const (
	keyTitle        = "title"
	keyLabels       = "labels"
	keyRefs         = "refs"
	keyRetired      = "retired"
	keySupersededBy = "superseded-by"
	keyAliases      = "aliases"
)

// The two values the retired key accepts; Serialize writes only retiredTrue.
const (
	retiredTrue  = "true"
	retiredFalse = "false"
)

// LabelBackground marks a Note that records context rather than a rule to follow, such as an ADR's
// rejected options: know tree and the Managed block in AGENTS.md leave it out, with its whole
// subtree, unless asked for.
const LabelBackground = "background"

// noteSchema is the Note field table: every frontmatter key, how Parse reads it into a Note and
// how Serialize writes it back, in the order Serialize writes them.
var noteSchema = frontmatter.NewSchema(
	"a Note",
	frontmatter.Text(keyTitle, func(note *Note) *string { return &note.Title }),
	frontmatter.List(keyLabels, func(note *Note) *[]string { return &note.Labels }),
	frontmatter.Raw(
		keyRefs,
		func(value frontmatter.Field, note *Note) (err error) {
			note.Refs, note.refLines, err = value.List()
			return err
		},
		func(note Note, out *frontmatter.Encoder) { out.List(keyRefs, note.Refs) },
	),
	frontmatter.Raw(
		keyRetired,
		func(value frontmatter.Field, note *Note) (err error) {
			note.IsRetired, err = retired(value)
			return err
		},
		func(note Note, out *frontmatter.Encoder) {
			if note.IsRetired {
				out.Raw(keyRetired, retiredTrue)
			}
		},
	),
	frontmatter.Raw(
		keySupersededBy,
		func(value frontmatter.Field, note *Note) (err error) {
			note.SupersededBy, note.supersededByLines, err = value.List()
			return err
		},
		func(note Note, out *frontmatter.Encoder) { out.List(keySupersededBy, note.SupersededBy) },
	),
	frontmatter.List(keyAliases, func(note *Note) *[]string { return &note.Aliases }),
)

// retired reads the retired key, which takes true or false only; an empty value reads as false.
func retired(value frontmatter.Field) (bool, error) {
	text, isScalar := value.Scalar()
	switch {
	case isScalar && text == retiredTrue:
		return true, nil
	case isScalar && (text == retiredFalse || text == ""):
		return false, nil
	}
	return false, value.Errorf("%s: expected %s or %s", keyRetired, retiredTrue, retiredFalse)
}

// Note is one parsed Note file. Its id is the file name and is never stored in the file. Empty
// strings and nil lists are fields the file leaves out.
type Note struct {
	Title  string
	Labels []string
	// Refs are the files, directories and symbols the Note depends on: a path relative to the
	// working tree root with forward slashes, or `path#symbol` for a text the file must contain.
	Refs []string
	// IsRetired marks a Note that no longer holds: the Note and its whole subtree are left out of
	// browsing (ADR 0010). The file writes it only when true.
	IsRetired bool
	// SupersededBy are the ids of the Notes that replace this one, never its own id.
	SupersededBy []string
	// Aliases are the Note's former ids (ADR 0002).
	Aliases []string
	// Body is the markdown after the frontmatter, byte for byte (LF line endings).
	Body string

	// lines maps each frontmatter key Parse found to its 1-based line in the file; refLines and
	// supersededByLines hold the line of each entry of Refs and SupersededBy. All are nil for a
	// Note built in memory.
	lines             map[string]int
	refLines          []int
	supersededByLines []int
}

// IsBackground reports whether labels, a Note's Labels, include LabelBackground.
func IsBackground(labels []string) bool {
	return slices.Contains(labels, LabelBackground)
}

// Error is a Note file that cannot be used: the one frontmatter.Error both kinds report, with the
// file, the 1-based line of the offending key or value (0 when no single line is to blame) and the
// message.
type Error = frontmatter.Error

// AliasesLine returns the 1-based line of the aliases key in the file the Note was parsed from, or
// 0 when the file has none or the Note was built in memory.
func (n Note) AliasesLine() int {
	return n.lines[keyAliases]
}

// RetiredLine returns the 1-based line of the retired key in the file the Note was parsed from, or
// 0 when the file has none or the Note was built in memory.
func (n Note) RetiredLine() int {
	return n.lines[keyRetired]
}

// SupersededByKeyLine returns the 1-based line of the superseded-by key in the file the Note was
// parsed from, or 0 when the file has none or the Note was built in memory.
func (n Note) SupersededByKeyLine() int {
	return n.lines[keySupersededBy]
}

// RefLine returns the 1-based line of the i-th Ref in the file the Note was parsed from, or 0 for
// a Note built in memory.
func (n Note) RefLine(i int) int {
	if i < 0 || i >= len(n.refLines) {
		return 0
	}
	return n.refLines[i]
}

// SupersededByLine returns the 1-based line of the i-th entry of SupersededBy in the file the Note
// was parsed from, or 0 for a Note built in memory.
func (n Note) SupersededByLine(i int) int {
	if i < 0 || i >= len(n.supersededByLines) {
		return 0
	}
	return n.supersededByLines[i]
}

// Parse decodes the Note file content data. path names the file in every error. CRLF line endings
// are read as LF.
//
// Parse fails, with an *Error citing the file line, when the content does not open with a `---`
// line, when the frontmatter is never closed, on invalid YAML, on an unknown or repeated key and on
// a value of the wrong shape. Whether the values make a usable Note is Validate's to check.
// When the frontmatter holds a git conflict marker, such a failure cites the first marker
// instead, in the words `capcom check` reports it with.
func Parse(path string, data []byte) (Note, error) {
	var note Note
	document, err := noteSchema.Decode(path, data, &note)
	if err != nil {
		return Note{}, err
	}
	note.Body = document.Body
	note.lines = document.Lines
	return note, nil
}

// Validate checks note and returns every problem it finds, or nil when there is none. path names
// the file in each problem, and a problem cites the line of the key it concerns when the Note came
// from Parse. A Note needs a title, every label must be non-empty, and every Ref must name a path.
// Every superseded-by entry must be spelled as an id, appear once and name a Note other than this
// one, whose id is the base name of path without .md. Whether a Ref resolves is Verify's to check,
// and whether a superseded-by entry names an existing Note is `capcom check`'s, so a Note with a
// stale Ref or successor stays usable.
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
	return append(problems, validateSupersededBy(path, note)...)
}

// validateSupersededBy checks the superseded-by entries of note, the Note file path, citing the
// line of each entry it reports.
func validateSupersededBy(path string, note Note) []*Error {
	var problems []*Error
	ownID := tree.IDOf(path)
	for i, successor := range note.SupersededBy {
		var message string
		switch {
		case tree.ValidateAlias(successor) != nil:
			message = fmt.Sprintf("%s %q is not an id: use only a-z, 0-9, dots and hyphens", keySupersededBy, successor)
		case successor == ownID:
			message = fmt.Sprintf("%s %q names the Note itself", keySupersededBy, successor)
		case slices.Contains(note.SupersededBy[:i], successor):
			message = fmt.Sprintf("%s %q is listed more than once", keySupersededBy, successor)
		default:
			continue
		}
		problems = append(problems, &Error{Path: path, Line: note.SupersededByLine(i), Message: message})
	}
	return problems
}

// Serialize writes the Note in the canonical form: the frontmatter keys in the fixed order title,
// labels, refs, retired, superseded-by, aliases; empty fields and a false retired left out; lists in flow style; text plain where YAML reads it
// back unchanged and double-quoted otherwise; then the body verbatim.
func (n Note) Serialize() []byte {
	return noteSchema.Encode(n, n.Body)
}
