package export

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/store"
)

const (
	// knowledgePage is the page of the Knowledge tree.
	knowledgePage = "knowledge.html"
	// notePagesDir is the directory below the export directory that holds one page per Note.
	notePagesDir = "knowledge"
	// noteRoot leads a link from a Note page to a page at the top of the export.
	noteRoot = "../"
)

// noteRow is one Note as the Knowledge tree prints it: Href leads to its page from knowledge.html,
// and Children are its rows below it in the tree.
type noteRow struct {
	ID       string
	Title    string
	Href     string
	Children []*noteRow
}

// knowledgeData is the content of knowledge.html: the top-level rows of the Knowledge tree.
type knowledgeData struct {
	frame
	Roots []*noteRow
}

// noteData is the content of one Note page.
type noteData struct {
	frame
	ID     string
	Title  string
	Labels []string
	Refs   []string
	Body   string
}

// notePages renders the Note pages of the export from ix: knowledge.html and one page per Note it
// shows, in tree order. The Notes are those `capcom know tree` prints by default: ix.Notes without
// retired subtrees, shown through outline.Notes without background subtrees. Each body is read
// from its file. It fails when the index or a file cannot be read.
func notePages(st *store.Store, ix *index.Index) ([]page, error) {
	notes, err := ix.Notes(false)
	if err != nil {
		return nil, err
	}
	rows := outline.Notes(notes, false).Rows(nil, outline.UnlimitedDepth)
	treeContent, err := render(knowledgePage, knowledgeData{
		frame: frame{PageTitle: "Knowledge", Root: topRoot},
		Roots: nestNoteRows(rows),
	})
	if err != nil {
		return nil, err
	}

	pages := make([]page, 0, len(rows)+1)
	pages = append(pages, page{path: knowledgePage, content: treeContent})
	reader := nodestore.NoteReader()
	for _, row := range rows {
		body, err := readNoteBody(st, reader, row.Entry)
		if err != nil {
			return nil, err
		}
		content, err := render("note.html", newNoteData(row.Entry, body))
		if err != nil {
			return nil, err
		}
		pages = append(pages, page{path: notePagePath(row.ID), content: content})
	}
	return pages, nil
}

// notePagePath returns the path of the page of the Note id below the export directory.
func notePagePath(id string) string {
	return path.Join(notePagesDir, id+pageExtension)
}

// nestNoteRows turns the outline rows, in walk order with each row's depth below the top, into the
// top-level rows of the Knowledge tree with every row's children below it.
func nestNoteRows(rows []outline.Row[index.NoteEntry]) []*noteRow {
	var roots []*noteRow
	var ancestors []*noteRow
	for _, row := range rows {
		current := &noteRow{ID: row.ID, Title: row.Entry.Title, Href: notePagePath(row.ID)}
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

// newNoteData returns the content of the page of entry, whose file holds body.
func newNoteData(entry index.NoteEntry, body string) noteData {
	return noteData{
		frame:  frame{PageTitle: entry.ID + ": " + entry.Title, Root: noteRoot},
		ID:     entry.ID,
		Title:  entry.Title,
		Labels: entry.Labels,
		Refs:   entry.Refs,
		Body:   strings.Trim(body, "\n"),
	}
}

// readNoteBody returns the body of the Note file of entry as reader reads it; a file that no longer
// parses gives its findings in place of the body. It fails when the file cannot be read.
func readNoteBody(st *store.Store, reader nodestore.Reader[knowledge.Note], entry index.NoteEntry) (string, error) {
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
