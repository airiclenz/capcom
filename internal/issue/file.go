package issue

import (
	"strconv"
	"time"

	"github.com/airiclenz/capcom/internal/frontmatter"
)

// issueKind is the issue kind of frontmatter file: the noun its failures use and every key it
// knows, in the order Serialize writes them.
var issueKind = frontmatter.Kind{Noun: "an issue", Keys: fieldKeys()}

// field is one frontmatter key of an issue: how Parse reads it into an Issue and how Serialize
// writes it back.
type field struct {
	key    string
	decode func(value frontmatter.Field, issue *Issue) error
	encode func(issue Issue, out *frontmatter.Encoder)
}

// fields is the issue field table: every frontmatter key, in the order Serialize writes them.
var fields = []field{
	textField(keyTitle, func(issue *Issue) *string { return &issue.Title }),
	textField(keyStatus, func(issue *Issue) *string { return &issue.Status }),
	textField(keyType, func(issue *Issue) *string { return &issue.Type }),
	{
		key: keyPriority,
		decode: func(value frontmatter.Field, issue *Issue) (err error) {
			issue.Priority, err = priority(value)
			return err
		},
		encode: func(issue Issue, out *frontmatter.Encoder) {
			out.Raw(keyPriority, strconv.Itoa(issue.Priority))
		},
	},
	listField(keyLabels, func(issue *Issue) *[]string { return &issue.Labels }),
	listField(keyDeps, func(issue *Issue) *[]string { return &issue.Deps }),
	listField(keyRelated, func(issue *Issue) *[]string { return &issue.Related }),
	textField(keyPlan, func(issue *Issue) *string { return &issue.Plan }),
	listField(keyAliases, func(issue *Issue) *[]string { return &issue.Aliases }),
	timeField(keyCreated, func(issue *Issue) *time.Time { return &issue.Created }),
	timeField(keyClosed, func(issue *Issue) *time.Time { return &issue.Closed }),
	textField(keyReason, func(issue *Issue) *string { return &issue.Reason }),
}

// textField is a text key held in the Issue field at returns.
func textField(key string, at func(issue *Issue) *string) field {
	return field{
		key: key,
		decode: func(value frontmatter.Field, issue *Issue) (err error) {
			*at(issue), err = value.Text()
			return err
		},
		encode: func(issue Issue, out *frontmatter.Encoder) { out.Text(key, *at(&issue)) },
	}
}

// listField is a list key held in the Issue field at returns; an issue keeps no entry lines.
func listField(key string, at func(issue *Issue) *[]string) field {
	return field{
		key: key,
		decode: func(value frontmatter.Field, issue *Issue) (err error) {
			*at(issue), _, err = value.List()
			return err
		},
		encode: func(issue Issue, out *frontmatter.Encoder) { out.List(key, *at(&issue)) },
	}
}

// timeField is a timestamp key held in the Issue field at returns, written in RFC3339 UTC and left
// out when zero.
func timeField(key string, at func(issue *Issue) *time.Time) field {
	return field{
		key: key,
		decode: func(value frontmatter.Field, issue *Issue) (err error) {
			*at(issue), err = timestamp(value)
			return err
		},
		encode: func(issue Issue, out *frontmatter.Encoder) {
			if at(&issue).IsZero() {
				return
			}
			out.Raw(key, at(&issue).UTC().Format(time.RFC3339))
		},
	}
}

// fieldKeys lists the keys of the field table, in its order.
func fieldKeys() []string {
	keys := make([]string, 0, len(fields))
	for _, entry := range fields {
		keys = append(keys, entry.key)
	}
	return keys
}

// Parse decodes the issue file content data. path names the file in every error. CRLF line
// endings are read as LF. A missing priority reads as DefaultPriority.
//
// Parse fails, with an *Error citing the file line, when the content does not open with a `---`
// line, when the frontmatter is never closed, on invalid YAML, on an unknown or repeated key and on
// a value of the wrong shape. It does not check values against the config; that is Validate.
// When the frontmatter holds a git conflict marker, such a failure cites the first marker
// instead, in the words `capcom check` reports it with.
func Parse(path string, data []byte) (Issue, error) {
	issue := Issue{Priority: DefaultPriority}
	document, err := frontmatter.Decode(path, data, issueKind, func(value frontmatter.Field) error {
		for _, entry := range fields {
			if entry.key == value.Key {
				return entry.decode(value, &issue)
			}
		}
		return nil // unreachable: Decode hands over only the keys of issueKind
	})
	if err != nil {
		return Issue{}, err
	}
	issue.Body = document.Body
	issue.lines = document.Lines
	return issue, nil
}

// Serialize writes the issue in the canonical form: the frontmatter keys in the fixed order title,
// status, type, priority, labels, deps, related, plan, aliases, created, closed, reason; empty
// fields left out except priority, which is always written; lists in flow style; timestamps in
// RFC3339 UTC; text plain where YAML reads it back unchanged and double-quoted otherwise; then the
// body verbatim.
func (i Issue) Serialize() []byte {
	var out frontmatter.Encoder
	for _, entry := range fields {
		entry.encode(i, &out)
	}
	return out.File(i.Body)
}

// priority reads a whole number; its range is Validate's to check. An empty value reads as
// DefaultPriority.
func priority(value frontmatter.Field) (int, error) {
	if value.IsEmpty() {
		return DefaultPriority, nil
	}
	text, isScalar := value.Scalar()
	priority, err := strconv.Atoi(text)
	if !isScalar || err != nil {
		return 0, value.Errorf(
			"%s: expected a whole number from %d to %d",
			keyPriority,
			MinPriority,
			MaxPriority,
		)
	}
	return priority, nil
}

// timestamp reads an RFC3339 timestamp as UTC to the second; an empty value reads as the zero time.
func timestamp(value frontmatter.Field) (time.Time, error) {
	if value.IsEmpty() {
		return time.Time{}, nil
	}
	text, isScalar := value.Scalar()
	parsed, err := time.Parse(time.RFC3339, text)
	if !isScalar || err != nil {
		return time.Time{}, value.Errorf(
			"%s: expected an RFC3339 timestamp such as 2026-10-11T09:30:00Z",
			value.Key,
		)
	}
	return parsed.UTC().Truncate(time.Second), nil
}
