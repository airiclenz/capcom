package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/index"
)

// textIndent is the indentation per tree depth in issue lines.
const textIndent = "  "

// issueData is one issue in the --json data of list, show and search. Lists are always present,
// empty when the file has none; ETag and Body are set by show only.
type issueData struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Parent   string   `json:"parent,omitempty"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Category string   `json:"category"`
	Type     string   `json:"type"`
	Priority int      `json:"priority"`
	Labels   []string `json:"labels"`
	Deps     []string `json:"deps"`
	Plan     string   `json:"plan,omitempty"`
	Aliases  []string `json:"aliases"`
	Created  string   `json:"created,omitempty"`
	Closed   string   `json:"closed,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	ETag     string   `json:"etag,omitempty"`
	Body     *string  `json:"body,omitempty"`
}

// newIssueData converts an indexed issue to its --json form.
func newIssueData(entry index.Entry) issueData {
	return issueData{
		ID:       entry.ID,
		Path:     entry.Path,
		Parent:   entry.ParentID,
		Title:    entry.Title,
		Status:   entry.Status,
		Category: string(entry.Category),
		Type:     entry.Type,
		Priority: entry.Priority,
		Labels:   nonNil(entry.Labels),
		Deps:     nonNil(entry.Deps),
		Plan:     entry.Plan,
		Aliases:  nonNil(entry.Aliases),
		Created:  formatTimestamp(entry.Created),
		Closed:   formatTimestamp(entry.Closed),
		Reason:   entry.Reason,
	}
}

// issueListData converts indexed issues to their --json form, an empty list when there are none.
func issueListData(entries []index.Entry) []issueData {
	data := make([]issueData, 0, len(entries))
	for _, entry := range entries {
		data = append(data, newIssueData(entry))
	}
	return data
}

// issueLines renders one line per issue, `id  status  P<n>  title`, never wrapped. With
// issuesPrefix set (the issue tree's directory, relative to the working tree root, with forward
// slashes) each id is indented two spaces per tree depth.
func issueLines(entries []index.Entry, issuesPrefix string) string {
	var text strings.Builder
	for _, entry := range entries {
		if issuesPrefix != "" {
			depth := strings.Count(strings.TrimPrefix(entry.Path, issuesPrefix+"/"), "/")
			text.WriteString(strings.Repeat(textIndent, depth))
		}
		fmt.Fprintf(&text, "%s  %s  P%d  %s\n", entry.ID, entry.Status, entry.Priority, flatten(entry.Title))
	}
	return text.String()
}

// showData is one issue in the --json data of show: the shared form, plus its Related links as
// showLinks lists them, always present.
type showData struct {
	issueData
	Related []string `json:"related"`
}

// issueText renders one issue in full: its id, location, etag and frontmatter as `key: value`
// lines in the file's key order, a blank line, then the body byte for byte. related is the issue's
// links as showLinks lists them. Empty fields are left out, except priority.
func issueText(entry index.Entry, etag string, related []string, body string) string {
	fields := []KVPair{
		{"id", entry.ID},
		{"path", entry.Path},
		{"etag", etag},
		{"parent", entry.ParentID},
		{"title", entry.Title},
		{"status", entry.Status},
		{"category", string(entry.Category)},
		{"type", entry.Type},
		{"priority", fmt.Sprint(entry.Priority)},
		{"labels", flowList(entry.Labels)},
		{"deps", flowList(entry.Deps)},
		{"related", flowList(related)},
		{"plan", entry.Plan},
		{"aliases", flowList(entry.Aliases)},
		{"created", formatTimestamp(entry.Created)},
		{"closed", formatTimestamp(entry.Closed)},
		{"reason", entry.Reason},
	}
	var text strings.Builder
	for _, field := range fields {
		if field.Value != "" {
			text.WriteString(field.Key + ": " + flatten(field.Value) + "\n")
		}
	}
	if body == "" {
		return text.String()
	}
	if !strings.HasPrefix(body, "\n") {
		text.WriteString("\n")
	}
	text.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		text.WriteString("\n")
	}
	return text.String()
}

// issueKV is the --kv record of one issue. STATUS is the outcome of looking it up; the issue's own
// status is ISSUE_STATUS. ETAG is the file's etag, for update --if-match.
func issueKV(entry index.Entry, etag string) KVRecord {
	return KVRecord{Pairs: []KVPair{
		{"ID", entry.ID},
		{"ETAG", etag},
		{"ISSUE_STATUS", entry.Status},
		{"CATEGORY", string(entry.Category)},
		{"TITLE", entry.Title},
	}}
}

// relativeSlashPath returns target relative to base with forward slashes, the form index paths
// take; it returns "" when target is not below base.
func relativeSlashPath(base, target string) string {
	relative, err := filepath.Rel(base, target)
	if err != nil || strings.HasPrefix(relative, "..") {
		return ""
	}
	return filepath.ToSlash(relative)
}

// flowList renders a list the way an issue file's frontmatter writes it: `[a, b]`, or "" when empty.
func flowList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return "[" + strings.Join(values, ", ") + "]"
}

// formatTimestamp renders a time as RFC3339 UTC, and a zero time as "".
func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// flatten keeps a value on one line.
func flatten(value string) string {
	return kvLineBreaks.Replace(value)
}

// nonNil returns values, or an empty list in place of nil so --json writes [].
func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
