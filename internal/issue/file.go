package issue

import (
	"strconv"
	"time"

	"github.com/airiclenz/capcom/internal/frontmatter"
)

// The two values the standing key accepts; Serialize writes only standingTrue.
const (
	standingTrue  = "true"
	standingFalse = "false"
)

// issueSchema is the issue field table: every frontmatter key, how Parse reads it into an Issue
// and how Serialize writes it back, in the order Serialize writes them.
var issueSchema = frontmatter.NewSchema(
	"an issue",
	frontmatter.Text(keyTitle, func(issue *Issue) *string { return &issue.Title }),
	frontmatter.Text(keyStatus, func(issue *Issue) *string { return &issue.Status }),
	frontmatter.Text(keyType, func(issue *Issue) *string { return &issue.Type }),
	frontmatter.Raw(
		keyPriority,
		func(value frontmatter.Field, issue *Issue) (err error) {
			issue.Priority, err = priority(value)
			return err
		},
		func(issue Issue, out *frontmatter.Encoder) {
			out.Raw(keyPriority, strconv.Itoa(issue.Priority))
		},
	),
	frontmatter.List(keyLabels, func(issue *Issue) *[]string { return &issue.Labels }),
	frontmatter.List(keyDeps, func(issue *Issue) *[]string { return &issue.Deps }),
	frontmatter.List(keyRelated, func(issue *Issue) *[]string { return &issue.Related }),
	frontmatter.Text(keyPlan, func(issue *Issue) *string { return &issue.Plan }),
	frontmatter.Raw(
		keyStanding,
		func(value frontmatter.Field, issue *Issue) (err error) {
			issue.IsStanding, err = standing(value)
			return err
		},
		func(issue Issue, out *frontmatter.Encoder) {
			if issue.IsStanding {
				out.Raw(keyStanding, standingTrue)
			}
		},
	),
	frontmatter.List(keyAliases, func(issue *Issue) *[]string { return &issue.Aliases }),
	timeField(keyCreated, func(issue *Issue) *time.Time { return &issue.Created }),
	timeField(keyClosed, func(issue *Issue) *time.Time { return &issue.Closed }),
	frontmatter.Text(keyReason, func(issue *Issue) *string { return &issue.Reason }),
)

// timeField is a timestamp key held in the Issue field at returns, written in RFC3339 UTC and left
// out when zero.
func timeField(key string, at func(issue *Issue) *time.Time) frontmatter.SchemaField[Issue] {
	return frontmatter.Raw(
		key,
		func(value frontmatter.Field, issue *Issue) (err error) {
			*at(issue), err = timestamp(value)
			return err
		},
		func(issue Issue, out *frontmatter.Encoder) {
			if at(&issue).IsZero() {
				return
			}
			out.Raw(key, at(&issue).UTC().Format(time.RFC3339))
		},
	)
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
	document, err := issueSchema.Decode(path, data, &issue)
	if err != nil {
		return Issue{}, err
	}
	issue.Body = document.Body
	issue.lines = document.Lines
	return issue, nil
}

// Serialize writes the issue in the canonical form: the frontmatter keys in the fixed order title,
// status, type, priority, labels, deps, related, plan, standing, aliases, created, closed, reason;
// empty fields and a false standing left out, except priority, which is always written; lists in
// flow style; timestamps in RFC3339 UTC; text plain where YAML reads it back unchanged and
// double-quoted otherwise; then the body verbatim.
func (i Issue) Serialize() []byte {
	return issueSchema.Encode(i, i.Body)
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

// standing reads the standing key, which takes true or false only; an empty value reads as false.
func standing(value frontmatter.Field) (bool, error) {
	text, isScalar := value.Scalar()
	switch {
	case isScalar && text == standingTrue:
		return true, nil
	case isScalar && (text == standingFalse || text == ""):
		return false, nil
	}
	return false, value.Errorf("%s: expected %s or %s", keyStanding, standingTrue, standingFalse)
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
