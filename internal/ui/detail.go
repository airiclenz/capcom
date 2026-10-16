package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/ui/markdown"
)

// The pieces of an issue's detail header.
const (
	headerSeparator = " · "
	listSeparator   = ", "
	codeMark        = "`"
	// headerTimeLayout is how the header shows the created and closed times, in UTC.
	headerTimeLayout = "2006-01-02 15:04"
)

// detailMsg carries one read of an issue or a Note for the detail pane: whether it is a Note,
// its id, the markdown source the pane renders (the frontmatter header, then the body or why
// the file could not be read), the file's ETag ("" when it did not parse) and the snapshot of
// every issue and Note id in the index that the read took. An issue's read also carries the
// parsed issue and its parent's id, which a write's conflict prompt compares against.
type detailMsg struct {
	isNote   bool
	id       string
	source   string
	etag     string
	ids      map[string]bool
	node     issue.Issue
	parentID string
}

// issueDetail is the issue or Note the detail pane shows: what its read carried, and its source rendered
// at renderedWidth. The rendering is cached per (source, width): it is redone only when either
// changes, and IsID reads the ids snapshot, never the index.
type issueDetail struct {
	id     string
	source string
	// etag is the ETag of the file as read, which a write checks it against; "" when the file did
	// not parse.
	etag string
	ids  map[string]bool
	// node is the issue as read and parentID its parent's id, the version on screen a write's
	// conflict prompt compares the file on disk with; both are zero for a Note.
	node     issue.Issue
	parentID string

	rendered       markdown.Rendered
	renderedSource string
	renderedWidth  int
}

// isID reports whether text is the id of an issue or Note in the snapshot the read took.
func (d issueDetail) isID(text string) bool {
	return d.ids[text]
}

// linkAt returns the target of the followable link drawn at column col of line line of the
// rendering, and false when no link covers that cell.
func (d issueDetail) linkAt(line, col int) (string, bool) {
	for _, link := range d.rendered.Links {
		if link.Line == line && col >= link.Col && col < link.Col+link.Width {
			return link.Target, true
		}
	}
	return "", false
}

// loadDetail returns a command that reads the issue at entry's path for the detail pane, through
// the session, as readIssueDetail does. Ids that cannot be read show `<path>: <error>` in place of
// the body, under a header from the index entry, and keep no ETag.
func loadDetail(s *session, entry index.Entry) tea.Cmd {
	return func() tea.Msg {
		msg := detailMsg{id: entry.ID}
		err := s.do(func(opened *store.Store, issueIndex *index.Index) error {
			var readErr error
			msg, readErr = readIssueDetail(opened, issueIndex, entry)
			return readErr
		})
		if err != nil {
			msg.source = issueHeader(entry) + fmt.Sprintf("%s: %v", entry.Path, err)
		}
		return msg
	}
}

// readIssueDetail reads the issue at entry's path for the detail pane: the file with
// nodestore.IssueReader and the ids issueIndex holds. A file that does not parse shows `<path>:
// <error>` in place of its body, under a header from the index entry, and keeps no ETag. It fails
// only when the ids cannot be read. The caller holds the session's mutex.
func readIssueDetail(opened *store.Store, issueIndex *index.Index, entry index.Entry) (detailMsg, error) {
	msg := detailMsg{id: entry.ID, parentID: entry.ParentID}
	ids, err := indexIDs(issueIndex)
	if err != nil {
		return msg, err
	}
	msg.ids = ids
	absolute := filepath.Join(opened.WorktreeRoot, filepath.FromSlash(entry.Path))
	reading, readErr := nodestore.IssueReader(opened.Config).Read(absolute, entry.Path)
	msg.source, msg.etag = issueSource(entry, reading, readErr)
	if msg.etag != "" {
		msg.node = reading.Node
	}
	return msg, nil
}

// indexIDs returns the set of every issue id, Closed ones included, and every Note id, retired
// ones included, that the index holds.
func indexIDs(issueIndex *index.Index) (map[string]bool, error) {
	issueIDs, err := issueIndex.ListIDs(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	notes, err := issueIndex.Notes(true)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(issueIDs)+len(notes))
	for _, id := range issueIDs {
		ids[id] = true
	}
	for _, note := range notes {
		ids[note.ID] = true
	}
	return ids, nil
}

// issueSource returns the markdown the detail pane renders for the issue file entry names, and
// the ETag to keep: the header and body from the file when it parses, or the header from entry
// and the read's failure, `<path>: <error>`, with no ETag when it does not.
func issueSource(entry index.Entry, reading nodestore.Reading[issue.Issue], err error) (string, string) {
	switch {
	case err != nil:
		return issueHeader(entry) + fmt.Sprintf("%s: %v", entry.Path, err), ""
	case !reading.IsRead():
		return issueHeader(entry) + reading.Refusal[0].String(), ""
	case !reading.IsParsed():
		return issueHeader(entry) + reading.Parse[0].String(), ""
	}
	parsed := reading.Node
	entry.Title = parsed.Title
	entry.Status = parsed.Status
	entry.Priority = parsed.Priority
	entry.Type = parsed.Type
	entry.Labels = parsed.Labels
	entry.Deps = parsed.Deps
	entry.Related = parsed.Related
	entry.Plan = parsed.Plan
	entry.Aliases = parsed.Aliases
	entry.Created = parsed.Created
	entry.Closed = parsed.Closed
	entry.Reason = parsed.Reason
	return issueHeader(entry) + parsed.Body, reading.ETag
}

// issueHeader returns the markdown header of an issue's detail: its title as a heading, its
// status, priority and type on one line, then its labels, parent, deps, related issues, plan,
// aliases, created and closed times and close reason, each line left out when empty, and a blank
// line before the body. The parent, deps and related issues are code spans, so the renderer makes
// the ones naming an id links; the times read in UTC to the minute.
func issueHeader(entry index.Entry) string {
	var text strings.Builder
	fmt.Fprintf(&text, "# %s\n\n", oneLine(entry.Title))
	fmt.Fprintf(&text, "status: %s%sP%d", entry.Status, headerSeparator, entry.Priority)
	if entry.Type != "" {
		text.WriteString(headerSeparator + entry.Type)
	}
	text.WriteString("\n")
	var parent []string
	if entry.ParentID != "" {
		parent = []string{entry.ParentID}
	}
	writeHeaderLine(&text, "labels", strings.Join(entry.Labels, listSeparator))
	writeHeaderLine(&text, "parent", idSpans(parent))
	writeHeaderLine(&text, "deps", idSpans(entry.Deps))
	writeHeaderLine(&text, "related", idSpans(entry.Related))
	writeHeaderLine(&text, "plan", entry.Plan)
	writeHeaderLine(&text, "aliases", strings.Join(entry.Aliases, listSeparator))
	writeHeaderLine(&text, "created", headerTime(entry.Created))
	writeHeaderLine(&text, "closed", headerTime(entry.Closed))
	writeHeaderLine(&text, "reason", oneLine(entry.Reason))
	text.WriteString("\n")
	return text.String()
}

// writeHeaderLine writes the header line `<key>: <value>` to text, or nothing when value is
// empty.
func writeHeaderLine(text *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(text, "%s: %s\n", key, value)
}

// idSpans returns ids as code spans joined by the list separator, so the renderer makes the ones
// naming an id links; "" when there are none.
func idSpans(ids []string) string {
	spans := make([]string, 0, len(ids))
	for _, id := range ids {
		spans = append(spans, codeSpan(id))
	}
	return strings.Join(spans, listSeparator)
}

// headerTime returns moment as a header shows it, in UTC to the minute; "" when it is zero.
func headerTime(moment time.Time) string {
	if moment.IsZero() {
		return ""
	}
	return moment.UTC().Format(headerTimeLayout)
}

// codeSpan wraps an id or a ref in a markdown code span.
func codeSpan(id string) string {
	return codeMark + id + codeMark
}

// loadNoteDetail returns a command that reads the Note at entry's path for the detail pane,
// through the session: the file with nodestore.NoteReader and the ids the index holds. A file
// that does not parse shows `<path>: <error>` in place of its body, under a header from the index
// entry, and keeps no ETag.
func loadNoteDetail(s *session, entry index.NoteEntry) tea.Cmd {
	return func() tea.Msg {
		msg := detailMsg{isNote: true, id: entry.ID}
		err := s.do(func(opened *store.Store, issueIndex *index.Index) error {
			ids, err := indexIDs(issueIndex)
			if err != nil {
				return err
			}
			msg.ids = ids
			absolute := filepath.Join(opened.WorktreeRoot, filepath.FromSlash(entry.Path))
			reading, readErr := nodestore.NoteReader().Read(absolute, entry.Path)
			msg.source, msg.etag = noteSource(entry, reading, readErr)
			return nil
		})
		if err != nil {
			msg.source = noteHeader(entry) + fmt.Sprintf("%s: %v", entry.Path, err)
		}
		return msg
	}
}

// noteSource returns the markdown the detail pane renders for the Note file entry names, and the
// ETag to keep: the header and body from the file when it parses, or the header from entry and
// the read's failure, `<path>: <error>`, with no ETag when it does not.
func noteSource(entry index.NoteEntry, reading nodestore.Reading[knowledge.Note], err error) (string, string) {
	switch {
	case err != nil:
		return noteHeader(entry) + fmt.Sprintf("%s: %v", entry.Path, err), ""
	case !reading.IsRead():
		return noteHeader(entry) + reading.Refusal[0].String(), ""
	case !reading.IsParsed():
		return noteHeader(entry) + reading.Parse[0].String(), ""
	}
	parsed := reading.Node
	entry.Title = parsed.Title
	entry.Labels = parsed.Labels
	entry.Refs = parsed.Refs
	entry.IsRetired = parsed.IsRetired
	entry.SupersededBy = parsed.SupersededBy
	entry.Aliases = parsed.Aliases
	return noteHeader(entry) + parsed.Body, reading.ETag
}

// noteHeader returns the markdown header of a Note's detail: its title as a heading, then its
// labels, refs, retired flag (only when true), superseded-by Notes and aliases, each line left out
// when empty, and a blank line before the body. The refs and superseded-by Notes are code spans,
// so the renderer makes the ones naming an issue or Note id links and leaves file refs inert.
func noteHeader(entry index.NoteEntry) string {
	var text strings.Builder
	fmt.Fprintf(&text, "# %s\n\n", oneLine(entry.Title))
	retired := ""
	if entry.IsRetired {
		retired = "true"
	}
	writeHeaderLine(&text, "labels", strings.Join(entry.Labels, listSeparator))
	writeHeaderLine(&text, "refs", idSpans(entry.Refs))
	writeHeaderLine(&text, "retired", retired)
	writeHeaderLine(&text, "superseded-by", idSpans(entry.SupersededBy))
	writeHeaderLine(&text, "aliases", strings.Join(entry.Aliases, listSeparator))
	text.WriteString("\n")
	return text.String()
}

// applyDetail takes in a read of an issue or a Note for the detail pane, unless the selection
// has moved on since it was asked for, and shows it from its top with the detail selection
// cleared; a read of the issue or Note shown already, as a live refresh takes, keeps the scroll.
// A Note's read applies only on the Knowledge tab and an issue's only on the others, so an issue
// and a Note sharing an id never take each other's place.
func (m model) applyDetail(msg detailMsg) model {
	selectedID, isSelected := m.sidebarSelection()
	if !isSelected || selectedID != msg.id || msg.isNote != (m.tab == TabKnowledge) {
		return m
	}
	isShown := m.issue.id == msg.id
	if !isShown {
		m.sel = detailSel{}
	}
	m.issue = issueDetail{
		id:       msg.id,
		source:   msg.source,
		etag:     msg.etag,
		ids:      msg.ids,
		node:     msg.node,
		parentID: msg.parentID,
	}
	if !isShown {
		m.detailOffset = 0
	}
	return m.renderDetail()
}

// renderDetail fills the detail pane with the shown issue or Note rendered at the pane's width, rendering
// again only when the source or the width changed since the last time. A re-render that rewrites a
// line the detail selection spans clears the selection.
func (m model) renderDetail() model {
	width := m.detailWidth()
	if m.issue.id == "" || width <= 0 {
		return m
	}
	if m.issue.renderedSource != m.issue.source || m.issue.renderedWidth != width {
		m.issue.rendered = markdown.Render(
			m.markdownTheme,
			m.issue.source,
			width,
			markdown.Options{IsID: m.issue.isID},
		)
		m.issue.renderedSource = m.issue.source
		m.issue.renderedWidth = width
	}
	if !m.sel.spanUnchanged(m.detail, m.issue.rendered.Lines) {
		m.sel = detailSel{}
	}
	m.detail = m.issue.rendered.Lines
	return m.scrollDetail(0)
}

// detailWidth returns the columns of the detail pane's text: the whole width on a narrow
// terminal, what the sidebar, the separator and its margins leave on a wide one, either less the
// scrollbar's gutter and bar at the pane's right edge.
func (m model) detailWidth() int {
	return max(m.width-m.detailLeft()-scrollbarColumns, 0)
}
