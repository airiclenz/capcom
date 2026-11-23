// Package export writes capcom's static HTML export: a self-contained site of a store's Issues that
// opens from the file system, with no JavaScript and no external resource, every link relative.
// WriteHTML is the whole interface. The site holds index.html (the Ready list), issues.html (every
// Issue as a tree) and issues/<id>.html (one page per Issue); the same tree always writes the same
// bytes, since no page carries a generation time or an absolute path.
package export

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
)

// markerName is the file an export writes into its directory before any page: it marks the
// directory as one capcom export owns, whose every other entry the next export removes. It does
// not mean the export completed, so a failed export leaves a directory the next one accepts.
const markerName = ".capcom-export"

// markerContent is what the marker file says to a reader who finds it.
const markerContent = "This directory is written by capcom export --html.\n" +
	"Every export removes everything in it except this file, then writes the pages again.\n"

var (
	// ErrInsideStore is the failure of an export directory that is the store directory (.capcom)
	// or lies inside it.
	ErrInsideStore = errors.New("lies inside the store directory .capcom; choose a directory outside it")
	// ErrForeignDir is the failure of an export directory that exists, is not empty and holds no
	// marker file, so capcom does not own what is in it.
	ErrForeignDir = errors.New("is not empty and holds no " + markerName +
		" file, so it is not an export directory; choose an empty or new directory")
	// ErrHoldsStore is the failure of an export directory that holds the store directory, which
	// clearing it would delete.
	ErrHoldsStore = errors.New("holds the store directory .capcom; choose a directory outside the working tree")
)

//go:embed templates/*.html
var templateFiles embed.FS

// pageTemplates are the page templates, each named for its file, and the shared ones they define.
var pageTemplates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))

// page is one file of the export: its path below the export directory with forward slashes, and
// its content.
type page struct {
	path    string
	content []byte
}

// WriteHTML writes the HTML export of the Issues ix indexes for st into dir: index.html,
// issues.html and issues/<id>.html for every Issue, closed and dropped ones included. dir is made
// absolute and resolved through the symlinks of its nearest existing ancestor; it fails with
// ErrInsideStore when it is st.Root or lies inside it, with ErrHoldsStore when it holds st.Root,
// and with ErrForeignDir when it exists, is not empty and holds no marker file. A missing dir is
// created. Every page is rendered before dir is touched; then the marker file is written, every
// other entry of dir is removed and the pages are written, each atomically. A failure after the
// marker leaves a directory the next export accepts. Errors name the path they concern.
func WriteHTML(dir string, st *store.Store, ix *index.Index) error {
	pages, err := issuePages(st, ix)
	if err != nil {
		return err
	}
	target, err := prepareDir(dir, st.Root)
	if err != nil {
		return err
	}
	for _, written := range pages {
		path := filepath.Join(target, filepath.FromSlash(written.path))
		if err := nodefile.WriteFileAtomic(path, bytes.NewReader(written.content)); err != nil {
			return err
		}
	}
	return nil
}

// prepareDir resolves dir, checks it against the directory rules WriteHTML documents, writes the
// marker file into it and removes every other entry, and returns the resolved directory.
func prepareDir(dir string, storeRoot string) (string, error) {
	target, err := resolvePath(dir)
	if err != nil {
		return "", err
	}
	root, err := resolvePath(storeRoot)
	if err != nil {
		return "", err
	}
	if _, isInside := store.RelativePath(root, target); isInside {
		return "", fmt.Errorf("%s: %w", target, ErrInsideStore)
	}
	if _, isHeld := store.RelativePath(target, root); isHeld {
		return "", fmt.Errorf("%s: %w", target, ErrHoldsStore)
	}

	entries, err := os.ReadDir(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if len(entries) > 0 && !hasMarker(entries) {
		return "", fmt.Errorf("%s: %w", target, ErrForeignDir)
	}
	marker := filepath.Join(target, markerName)
	if err := nodefile.WriteFileAtomic(marker, strings.NewReader(markerContent)); err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Name() == markerName {
			continue
		}
		if err := os.RemoveAll(filepath.Join(target, entry.Name())); err != nil {
			return "", err
		}
	}
	return target, nil
}

// hasMarker reports whether entries hold the marker file.
func hasMarker(entries []fs.DirEntry) bool {
	for _, entry := range entries {
		if entry.Name() == markerName {
			return true
		}
	}
	return false
}

// resolvePath returns path made absolute with the symlinks of its nearest existing ancestor (path
// itself when it exists) resolved, and the components below that ancestor that do not exist yet
// joined on, so a directory reached through a symlink is judged where it really lies.
func resolvePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	existing, missing := absolute, ""
	for {
		resolved, err := filepath.EvalSymlinks(existing)
		if err == nil {
			return filepath.Join(resolved, missing), nil
		}
		parent := filepath.Dir(existing)
		if !errors.Is(err, fs.ErrNotExist) || parent == existing {
			return "", err
		}
		missing = filepath.Join(filepath.Base(existing), missing)
		existing = parent
	}
}

// render executes the page template name with data.
func render(name string, data any) ([]byte, error) {
	var content bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&content, name, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return content.Bytes(), nil
}
