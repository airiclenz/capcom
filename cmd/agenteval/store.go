package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/store"
)

// markdownExtension is the extension of every issue and Note file; the file's name without it is
// the id.
const markdownExtension = ".md"

// storedIssue is one issue file of a seeded repo as the agent left it: its id, the parsed file,
// and whether its status falls in a Closed category of the repo's config.
type storedIssue struct {
	ID       string
	Issue    issue.Issue
	IsClosed bool
}

// loadIssues returns every issue file under the store of repo, keyed by id, Parents and children
// alike. A file that does not parse fails the load with an error naming it.
func loadIssues(repo string) (map[string]storedIssue, error) {
	opened, err := store.Open(repo)
	if err != nil {
		return nil, err
	}
	issues := map[string]storedIssue{}
	err = walkMarkdown(opened.IssuesDir(), func(id, path string, data []byte) error {
		parsed, err := issue.Parse(path, data)
		if err != nil {
			return err
		}
		category, _ := opened.Config.CategoryOf(parsed.Status)
		issues[id] = storedIssue{ID: id, Issue: parsed, IsClosed: category.IsClosed()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// loadNotes returns every Note file under the store of repo, keyed by id, retired ones included. A
// file that does not parse fails the load with an error naming it.
func loadNotes(repo string) (map[string]knowledge.Note, error) {
	opened, err := store.Open(repo)
	if err != nil {
		return nil, err
	}
	notes := map[string]knowledge.Note{}
	err = walkMarkdown(opened.KnowledgeDir(), func(id, path string, data []byte) error {
		note, err := knowledge.Parse(path, data)
		if err != nil {
			return err
		}
		notes[id] = note
		return nil
	})
	if err != nil {
		return nil, err
	}
	return notes, nil
}

// logEntries returns the entries of the Log section of stored, in file order; an issue without a
// Log section has none.
func logEntries(stored storedIssue) []string {
	entries, _ := issue.LogEntries([]byte(stored.Issue.Body))
	return entries
}

// lockState returns the state of the Tree lock of repo's worktree, as `capcom lock check` reads it.
func lockState(repo string) (lock.Status, error) {
	opened, err := store.Open(repo)
	if err != nil {
		return lock.Status{}, err
	}
	return lock.ForWorktree(opened).Read()
}

// walkMarkdown calls visit with the id, path and content of every markdown file below dir, at any
// depth. A tree that does not exist yet holds no files.
func walkMarkdown(dir string, visit func(id, path string, data []byte) error) error {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), markdownExtension) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return visit(strings.TrimSuffix(entry.Name(), markdownExtension), path, data)
	})
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	return nil
}
