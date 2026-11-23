package export

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/briefing"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/store"
)

const (
	// indexPage is the page of the Ready list, the export's start page.
	indexPage = "index.html"
	// issuesPage is the page of the Issue tree.
	issuesPage = "issues.html"
	// issuePagesDir is the directory below the export directory that holds one page per Issue.
	issuePagesDir = "issues"
	// pageExtension ends the name of every page.
	pageExtension = ".html"
	// topRoot leads a link from a page at the top of the export to another page.
	topRoot = ""
	// issueRoot leads a link from an Issue page to a page at the top of the export.
	issueRoot = "../"
)

// frame is what every page shows around its content: its title and the prefix that leads its
// links to the top of the export.
type frame struct {
	PageTitle string
	Root      string
}

// link is an anchor a page prints: Href is relative to the page, and an empty Href prints Text as
// plain text.
type link struct {
	Href string
	Text string
}

// issueRow is one Issue as the Ready list and the Issue tree print it: Href leads to its page from
// the page that prints it, and Children are its rows below it in the tree.
type issueRow struct {
	ID       string
	Title    string
	Status   string
	Type     string
	Priority int
	IsClosed bool
	Href     string
	Children []*issueRow
}

// readyData is the content of index.html: the Ready list, most urgent first.
type readyData struct {
	frame
	Ready []*issueRow
}

// treeData is the content of issues.html: the top-level rows of the Issue tree.
type treeData struct {
	frame
	Roots []*issueRow
}

// issueData is the content of one Issue page.
type issueData struct {
	frame
	issueRow
	Labels     []string
	Reason     string
	Deps       []link
	Dependents []link
	Body       string
}

// issuePages renders the Issue pages of the export from ix: index.html, issues.html and one page
// per Issue, closed and dropped ones included, in tree order. Readiness and order come from
// briefing.ReadyList, the tree from outline.Issues and the dependency links from the
// issue.DepGraph of every Issue; each body is read from its file. It fails when the index or a
// file cannot be read.
func issuePages(st *store.Store, ix *index.Index) ([]page, error) {
	entries, err := ix.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	ready, err := briefing.ReadyList(ix, st.Config, briefing.ReadyOptions{Root: st.WorktreeRoot})
	if err != nil {
		return nil, err
	}

	readyRows := make([]*issueRow, 0, len(ready))
	for _, item := range ready {
		readyRows = append(readyRows, newIssueRow(item.Entry, topRoot))
	}
	indexContent, err := render(indexPage, readyData{frame: frame{PageTitle: "Ready", Root: topRoot}, Ready: readyRows})
	if err != nil {
		return nil, err
	}
	rows := outline.Issues(entries, true).Rows(nil, outline.UnlimitedDepth)
	treeContent, err := render(issuesPage, treeData{frame: frame{PageTitle: "Issues", Root: topRoot}, Roots: nestRows(rows)})
	if err != nil {
		return nil, err
	}

	pages := make([]page, 0, len(entries)+2)
	pages = append(pages, page{path: indexPage, content: indexContent}, page{path: issuesPage, content: treeContent})
	graph := issue.NewDepGraph(index.Nodes(entries))
	reader := nodestore.IssueReader(st.Config)
	for _, entry := range entries {
		body, err := readBody(st, reader, entry)
		if err != nil {
			return nil, err
		}
		content, err := render("issue.html", newIssueData(entry, graph, body))
		if err != nil {
			return nil, err
		}
		pages = append(pages, page{path: issuePagePath(entry.ID, topRoot), content: content})
	}
	return pages, nil
}

// newIssueRow returns the row of entry as a page whose links start from root prints it.
func newIssueRow(entry index.Entry, root string) *issueRow {
	return &issueRow{
		ID:       entry.ID,
		Title:    entry.Title,
		Status:   entry.Status,
		Type:     entry.Type,
		Priority: entry.Priority,
		IsClosed: entry.Category.IsClosed(),
		Href:     issuePagePath(entry.ID, root),
	}
}

// issuePagePath returns the path of the page of the Issue id, as a page whose links start from root
// reaches it: below issuePagesDir from the top of the export, a bare name from another Issue page.
func issuePagePath(id string, root string) string {
	if root == issueRoot {
		return id + pageExtension
	}
	return path.Join(issuePagesDir, id+pageExtension)
}

// nestRows turns the outline rows, in walk order with each row's depth below the top, into the
// top-level rows of the Issue tree with every row's children below it.
func nestRows(rows []outline.Row[index.Entry]) []*issueRow {
	var roots []*issueRow
	var ancestors []*issueRow
	for _, row := range rows {
		current := newIssueRow(row.Entry, topRoot)
		ancestors = ancestors[:row.Depth]
		if row.Depth == 0 {
			roots = append(roots, current)
		} else {
			parent := ancestors[row.Depth-1]
			parent.Children = append(parent.Children, current)
		}
		ancestors = append(ancestors, current)
	}
	return roots
}

// newIssueData returns the content of the page of entry, whose file holds body: every dependency
// graph resolves links to its Issue's page, by exact id or Alias, and one that names no Issue
// prints as plain text; every dependent links to its page.
func newIssueData(entry index.Entry, graph issue.DepGraph, body string) issueData {
	data := issueData{
		frame:    frame{PageTitle: entry.ID + ": " + entry.Title, Root: issueRoot},
		issueRow: *newIssueRow(entry, issueRoot),
		Labels:   entry.Labels,
		Reason:   entry.Reason,
		Body:     strings.Trim(body, "\n"),
	}
	for _, dep := range entry.Deps {
		id, isFound := graph.Resolve(dep)
		switch {
		case !isFound:
			data.Deps = append(data.Deps, link{Text: dep + " (no such issue)"})
		case id != dep:
			data.Deps = append(data.Deps, link{Href: issuePagePath(id, issueRoot), Text: id + " (as " + dep + ")"})
		default:
			data.Deps = append(data.Deps, link{Href: issuePagePath(id, issueRoot), Text: id})
		}
	}
	for _, id := range graph.Dependents(entry.ID) {
		data.Dependents = append(data.Dependents, link{Href: issuePagePath(id, issueRoot), Text: id})
	}
	return data
}

// readBody returns the body of the Issue file of entry, Log included, as reader reads it; a file
// that no longer parses gives its findings in place of the body. It fails when the file cannot be
// read.
func readBody(st *store.Store, reader nodestore.Reader[issue.Issue], entry index.Entry) (string, error) {
	absolute := filepath.Join(st.WorktreeRoot, filepath.FromSlash(entry.Path))
	reading, err := reader.Read(absolute, entry.Path)
	if err != nil {
		return "", err
	}
	if !reading.IsParsed() {
		return findingsText(reading.Findings()), nil
	}
	return reading.Node.Body, nil
}

// findingsText renders findings one per line.
func findingsText(findings []diagnose.Finding) string {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, finding.String())
	}
	return strings.Join(lines, "\n")
}
