package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
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
// showLinks lists them, whether it is Ready (todo category only) and what blocks it, the ids of
// the issues that depend on it, and how many of its direct children are Closed (left out when it has
// none). Related and Dependents are always present.
type showData struct {
	issueData
	Related    []string      `json:"related"`
	Ready      *bool         `json:"ready,omitempty"`
	Blocking   []blockerData `json:"blocking,omitempty"`
	Dependents []string      `json:"dependents"`
	Children   *childrenData `json:"children,omitempty"`
}

// childrenData counts an issue's direct children: all of them, and those that are Closed.
type childrenData struct {
	Total  int `json:"total"`
	Closed int `json:"closed"`
}

// childProgress counts the direct children of the issue with id, Closed ones included, and how
// many of them are Closed (category done or dropped). It returns nil for an issue without children.
func (a *app) childProgress(id string) (*childrenData, error) {
	children, err := a.issueIndex.ListIDs(index.Filter{ParentID: id, IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, nil
	}
	closed, err := a.issueIndex.ListIDs(index.Filter{
		ParentID:         id,
		Categories:       []config.Category{config.CategoryDone, config.CategoryDropped},
		IsClosedIncluded: true,
	})
	if err != nil {
		return nil, err
	}
	return &childrenData{Total: len(children), Closed: len(closed)}, nil
}

// showFacts is what show tells about an issue beyond its file: its Related links as showLinks
// lists them, the ready rules' verdict when the issue is in the todo category (nil otherwise), the
// ids of the issues that depend on it, in tree order, and its direct children's count (nil when it
// has none).
type showFacts struct {
	related    []string
	readiness  *issue.Readiness
	dependents []string
	children   *childrenData
}

// newShowData converts an indexed issue and what show tells about it to show's --json form.
func newShowData(entry index.Entry, facts showFacts) showData {
	shown := showData{
		issueData:  newIssueData(entry),
		Related:    facts.related,
		Dependents: facts.dependents,
		Children:   facts.children,
	}
	if facts.readiness != nil {
		shown.Ready = &facts.readiness.IsReady
		for _, blocker := range facts.readiness.Blocking {
			shown.Blocking = append(shown.Blocking, blockerData{Dep: blocker.Dep, Reason: blocker.Reason})
		}
	}
	return shown
}

// issueText renders one issue in full: its id, location, etag and frontmatter as `key: value`
// lines in the file's key order, a blank line, then the body byte for byte. After category comes
// ready, for an issue in the todo category: `yes`, or `no (<reasons>)` worded as ready --all words
// them; after deps come the issue's dependents, `<closed>/<total> closed` for its direct children,
// and its Related links as facts carries them. Empty fields are left out, except priority.
func issueText(entry index.Entry, etag string, facts showFacts, body string) string {
	fields := []KVPair{
		{"id", entry.ID},
		{"path", entry.Path},
		{"etag", etag},
		{"parent", entry.ParentID},
		{"title", entry.Title},
		{"status", entry.Status},
		{"category", string(entry.Category)},
		{"ready", readyText(facts.readiness)},
		{"type", entry.Type},
		{"priority", fmt.Sprint(entry.Priority)},
		{"labels", flowList(entry.Labels)},
		{"deps", flowList(entry.Deps)},
		{"dependents", flowList(facts.dependents)},
		{"children", childrenText(facts.children)},
		{"related", flowList(facts.related)},
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

// childrenText is the children line's value for a count: "" without one, `<closed>/<total> closed`
// otherwise.
func childrenText(children *childrenData) string {
	if children == nil {
		return ""
	}
	return fmt.Sprintf("%d/%d closed", children.Closed, children.Total)
}

// readyText is the ready line's value for a verdict: "" without one, `yes` when the issue is Ready,
// and otherwise `no (<reasons>)`.
func readyText(verdict *issue.Readiness) string {
	switch {
	case verdict == nil:
		return ""
	case verdict.IsReady:
		return "yes"
	default:
		return "no (" + notReadyReasons(*verdict) + ")"
	}
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
