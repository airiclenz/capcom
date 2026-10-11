// Package elsewhere finds the issue changes that exist on other local branches but not in HEAD
// (ADR 0004). It reads git only, never fetches and never merges: each local branch whose tip is
// recent and that holds commits HEAD lacks contributes one Entry per issue file it added, modified
// or removed since its merge base with HEAD. Branches checked out in other worktrees are local
// branches too and are scanned like any other; uncommitted changes in other worktrees are not.
package elsewhere

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
)

const (
	// head is the revision every branch is compared against.
	head = "HEAD"
	// issueExtension is the extension of an issue file; other files below the pathspecs are skipped.
	issueExtension = ".md"
	// hiddenPrefix marks file names the issue tree skips, temp files included.
	hiddenPrefix = "."
	day          = 24 * time.Hour
)

// Change is how a branch changed an issue file relative to its merge base with HEAD.
type Change string

// The changes an Entry reports.
const (
	Added    Change = "added"
	Modified Change = "modified"
	Removed  Change = "removed"
)

// Entry is one issue file changed on a branch that HEAD has not merged.
type Entry struct {
	// ID is the issue id: the file name without .md.
	ID string `json:"id"`
	// Branch is the branch name, without refs/heads/.
	Branch string `json:"branch"`
	// Change is how the branch changed the file.
	Change Change `json:"change"`
	// Status is the status in the branch copy of a Modified file; empty for Added and Removed, and
	// for a copy that does not parse.
	Status string `json:"status,omitempty"`
	// Path is the file, relative to the worktree root with forward slashes.
	Path string `json:"path"`
}

// String renders the entry as `<id>: <status> on <branch> (unmerged)`, with `added`, `removed`
// or, for a copy without a readable status, `modified` in place of the status.
func (e Entry) String() string {
	state := string(e.Change)
	if e.Change == Modified && e.Status != "" {
		state = e.Status
	}
	return fmt.Sprintf("%s: %s on %s (unmerged)", e.ID, state, e.Branch)
}

// Options configures one Scan.
type Options struct {
	// WorktreeRoot is the top directory of the worktree whose HEAD is the reference.
	WorktreeRoot string
	// Pathspecs limit the scan to these paths, relative to WorktreeRoot with forward slashes: the
	// issue directory, or single issue files.
	Pathspecs []string
	// Days is the window: a branch whose tip is older than this many days is ignored, so 0 ignores
	// every branch.
	Days int
	// Now is the time the window is measured back from.
	Now time.Time
}

// Report is the outcome of a Scan.
type Report struct {
	// Entries are ordered by branch name, then by path.
	Entries []Entry
	// Warnings describe every part of the scan that failed; the rest of the scan still ran.
	Warnings []string
}

// Scan reports the issue files changed on other local branches. It never fails: a git error, an
// unborn HEAD or a branch copy that does not parse becomes a warning, and an unparsable copy is
// still reported as Modified. The branch checked out in WorktreeRoot is never reported.
func Scan(options Options) Report {
	var report Report
	branches, err := gitx.Branches(options.WorktreeRoot)
	if err != nil {
		report.warn("list branches: %v", err)
		return report
	}

	cutoff := options.Now.Add(-time.Duration(options.Days) * day)
	var candidates []gitx.Branch
	for _, branch := range branches {
		if !branch.IsCurrent && branch.Tip.After(cutoff) {
			candidates = append(candidates, branch)
		}
	}
	if len(candidates) == 0 {
		return report
	}

	hasHead, err := gitx.HasCommit(options.WorktreeRoot, head)
	switch {
	case err != nil:
		report.warn("resolve HEAD: %v", err)
		return report
	case !hasHead:
		report.warn("HEAD has no commit yet; other branches not compared")
		return report
	}
	for _, branch := range candidates {
		report.scanBranch(options, branch)
	}
	return report
}

// scanBranch adds the entries of one branch, or a warning when git fails on it.
func (r *Report) scanBranch(options Options, branch gitx.Branch) {
	isMerged, err := gitx.IsAncestor(options.WorktreeRoot, branch.Ref, head)
	if err != nil {
		r.warn("branch %s: %v", branch.Name, err)
		return
	}
	if isMerged {
		return
	}

	changes, err := gitx.ChangedFiles(options.WorktreeRoot, head, branch.Ref, options.Pathspecs)
	if err != nil {
		r.warn("branch %s: %v", branch.Name, err)
		return
	}
	for _, change := range changes {
		id, isIssue := issueID(change.Path)
		if !isIssue {
			continue
		}
		// gitx names its change kinds exactly as Change does.
		entry := Entry{ID: id, Branch: branch.Name, Change: Change(change.Kind), Path: change.Path}
		if entry.Change == Modified {
			entry.Status = r.branchStatus(options.WorktreeRoot, branch, change.Path)
		}
		r.Entries = append(r.Entries, entry)
	}
}

// branchStatus reads the status of the branch copy of the issue file at filePath, or "" with a
// warning when the copy cannot be read or parsed.
func (r *Report) branchStatus(worktreeRoot string, branch gitx.Branch, filePath string) string {
	data, err := gitx.ShowFile(worktreeRoot, branch.Ref, filePath)
	if err != nil {
		r.warn("branch %s: %v", branch.Name, err)
		return ""
	}
	parsed, err := issue.Parse(branch.Name+":"+filePath, data)
	if err != nil {
		r.warn("branch %s: %v", branch.Name, err)
		return ""
	}
	return parsed.Status
}

// warn records one warning, prefixed so a reader knows it comes from the Elsewhere scan.
func (r *Report) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, "elsewhere: "+fmt.Sprintf(format, args...))
}

// issueID returns the id of the issue file at the slash path filePath, and false for a file the
// issue tree does not read (another extension, or a hidden or temp file).
func issueID(filePath string) (string, bool) {
	name := path.Base(filePath)
	if strings.HasPrefix(name, hiddenPrefix) {
		return "", false
	}
	return strings.CutSuffix(name, issueExtension)
}
