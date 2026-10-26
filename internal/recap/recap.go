// Package recap computes what changed in a store's issue and Knowledge trees between a boundary
// commit and HEAD. It compares the two committed snapshots of `.capcom/`, keyed by id, and reads
// git only: uncommitted changes and the commits in between are not looked at, so an issue filed
// and closed inside the window is both Filed and Closed, and one closed and reopened inside it is
// neither.
package recap

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

const (
	// headRevision is the snapshot every recap ends at.
	headRevision = "HEAD"
	// nodeExtension is the extension of an issue or Note file; other files are skipped.
	nodeExtension = ".md"
	// hiddenPrefix marks names the trees skip, temp files included.
	hiddenPrefix = "."
	// maxFileSize is the largest file copy a recap reads, in bytes: 1 MiB, the size the trees read
	// a node file up to.
	maxFileSize int64 = 1 << 20
)

// ErrUnknownBoundary is the error Compute fails with when the boundary names no commit.
var ErrUnknownBoundary = errors.New("names no commit")

// Entry is one line of a recap group: an issue or a Note, by its id at HEAD.
type Entry struct {
	// ID is the id at HEAD; for a removed Note, the id it had at the boundary.
	ID string `json:"id"`
	// Title is the title at HEAD; for a removed Note, the title it had at the boundary.
	Title string `json:"title"`
	// Status is the status at HEAD, set in Closed and StatusChanged.
	Status string `json:"status,omitempty"`
	// Reason is why the issue was closed, set in Closed.
	Reason string `json:"reason,omitempty"`
	// LogEntry is one Log entry added, as issue.LogEntries returns it, set in Log.
	LogEntry string `json:"entry,omitempty"`
}

// Report is what changed between the boundary and HEAD. Every group is ordered by id; Log keeps
// the entries of one issue in file order.
type Report struct {
	// Closed are the issues whose status at HEAD is in a closed category and was not at the
	// boundary, those filed inside the window included.
	Closed []Entry `json:"closed"`
	// Filed are the issues HEAD holds and the boundary does not, under no former id either.
	Filed []Entry `json:"filed"`
	// StatusChanged are the issues whose status changed into a category that is not closed,
	// reopened ones included.
	StatusChanged []Entry `json:"status_changed"`
	// Log holds one Entry per Log entry HEAD holds that the boundary copy does not.
	Log []Entry `json:"log"`
	// NotesAdded are the Notes HEAD holds and the boundary does not, under no former id either.
	NotesAdded []Entry `json:"notes_added"`
	// NotesChanged are the Notes whose file differs, other than those added and retired.
	NotesChanged []Entry `json:"notes_changed"`
	// NotesRetired are the Notes whose own retired turned true, and those whose file was removed.
	NotesRetired []Entry `json:"notes_retired"`
	// Warnings describe each file that could not be read or parsed and was left out; the rest of
	// the recap still holds.
	Warnings []string `json:"-"`
}

// Compute reports what changed in the issue and Knowledge trees of s between the commit boundary
// names and HEAD. Each id counts once: a file moved to another parent keeps its id, and a renamed
// file is paired with its former id through the aliases its HEAD copy lists. An empty boundary
// compares HEAD with an empty store, so everything at HEAD is new. An unborn HEAD holds nothing.
// Only files whose content differs between the two snapshots are read, and a file larger than
// 1 MiB, unreadable or unparsable becomes a warning and is left out. Compute fails with an error
// wrapping ErrUnknownBoundary when boundary names no commit, and with git's error when git fails.
func Compute(s *store.Store, boundary string) (Report, error) {
	head, err := gitx.ResolveCommit(s.WorktreeRoot, headRevision)
	if err != nil {
		return Report{}, err
	}
	start := ""
	if boundary != "" {
		start, err = gitx.ResolveCommit(s.WorktreeRoot, boundary)
		if err != nil {
			return Report{}, err
		}
		if start == "" {
			return Report{}, fmt.Errorf("%s: %w", boundary, ErrUnknownBoundary)
		}
	}

	c := computation{
		root:   s.WorktreeRoot,
		config: s.Config,
		before: revision{label: boundary, commit: start},
		after:  revision{label: headRevision, commit: head},
	}
	issuesBefore, issuesAfter, err := c.snapshots(s.IssuesDir())
	if err != nil {
		return Report{}, err
	}
	notesBefore, notesAfter, err := c.snapshots(s.KnowledgeDir())
	if err != nil {
		return Report{}, err
	}
	c.compareIssues(issuesBefore, issuesAfter)
	c.compareNotes(notesBefore, notesAfter)
	c.report.NotesRetired = sortedByID(c.report.NotesRetired)
	return c.report, nil
}

// revision is one end of the recap: the commit's object name, "" for an empty snapshot, and the
// name warnings cite it by.
type revision struct {
	label  string
	commit string
}

// snapshot maps each id to its file in one revision.
type snapshot map[string]gitx.TreeFile

// parsed is one file copy read from git: the name warnings cite it by, its bytes and what it
// parses to.
type parsed[T any] struct {
	label string
	data  []byte
	value T
}

// computation holds the state of one Compute.
type computation struct {
	root   string
	config config.Config
	before revision
	after  revision
	report Report
}

// snapshots lists the files below dir in the boundary and at HEAD. A dir outside the worktree,
// which no pathspec can name, holds no file.
func (c *computation) snapshots(dir string) (before, after snapshot, err error) {
	pathspec, isInside := store.RelativePath(c.root, dir)
	if !isInside {
		return snapshot{}, snapshot{}, nil
	}
	before, err = c.list(c.before, pathspec)
	if err != nil {
		return nil, nil, err
	}
	after, err = c.list(c.after, pathspec)
	return before, after, err
}

// list maps the id of every node file below pathspec in end to its file; a second file with an id
// already seen gets a warning and is left out.
func (c *computation) list(end revision, pathspec string) (snapshot, error) {
	files := snapshot{}
	if end.commit == "" {
		return files, nil
	}
	listed, err := gitx.TreeFiles(c.root, end.commit, pathspec)
	if err != nil {
		return nil, err
	}
	for _, file := range listed {
		id, isNode := nodeID(pathspec, file.Path)
		if !isNode {
			continue
		}
		if first, isSeen := files[id]; isSeen {
			c.warn("%s: %s: id %s already used by %s", end.label, file.Path, id, first.Path)
			continue
		}
		files[id] = file
	}
	return files, nil
}

// nodeID returns the id of the file at filePath below pathspec, and false for a file the trees do
// not read: another extension, or a hidden name anywhere below pathspec.
func nodeID(pathspec, filePath string) (string, bool) {
	below := strings.TrimPrefix(filePath, pathspec+"/")
	for _, name := range strings.Split(below, "/") {
		if strings.HasPrefix(name, hiddenPrefix) {
			return "", false
		}
	}
	return strings.CutSuffix(path.Base(below), nodeExtension)
}

// compareIssues adds the Closed, Filed, Status changed and Log entries of the issue snapshots.
// before is changed: a renamed issue's former id is moved to its HEAD id.
func (c *computation) compareIssues(before, after snapshot) {
	copies := loadChanged(c, before, after, issue.Parse)
	pairRenames(before, after, copies, func(i issue.Issue) []string { return i.Aliases })
	for _, id := range sortedIDs(copies) {
		current := copies[id]
		file, isInBoundary := before[id]
		if !isInBoundary {
			c.fileIssue(id, current)
			continue
		}
		previous, isRead := load(c, c.before, file, issue.Parse)
		if isRead {
			c.changeIssue(id, previous, current)
		}
	}
}

// fileIssue adds an issue the boundary does not hold: Filed, Closed when it is, and its Log.
func (c *computation) fileIssue(id string, current parsed[issue.Issue]) {
	c.report.Filed = append(c.report.Filed, Entry{ID: id, Title: current.value.Title})
	if c.isClosed(current.value.Status) {
		c.report.Closed = append(c.report.Closed, closedEntry(id, current.value))
	}
	c.addLog(id, parsed[issue.Issue]{}, current)
}

// changeIssue adds what changed in an issue both snapshots hold: closing it, a status change
// that does not close it, and the Log entries added.
func (c *computation) changeIssue(id string, previous, current parsed[issue.Issue]) {
	wasClosed := c.isClosed(previous.value.Status)
	isClosed := c.isClosed(current.value.Status)
	switch {
	case isClosed && !wasClosed:
		c.report.Closed = append(c.report.Closed, closedEntry(id, current.value))
	case !isClosed && current.value.Status != previous.value.Status:
		c.report.StatusChanged = append(
			c.report.StatusChanged,
			Entry{ID: id, Title: current.value.Title, Status: current.value.Status},
		)
	}
	c.addLog(id, previous, current)
}

// closedEntry is the Closed entry of the issue i, whose id is id.
func closedEntry(id string, i issue.Issue) Entry {
	return Entry{ID: id, Title: i.Title, Status: i.Status, Reason: i.Reason}
}

// isClosed reports whether status is in a closed category; an unknown status is not.
func (c *computation) isClosed(status string) bool {
	category, _ := c.config.CategoryOf(status)
	return category.IsClosed()
}

// addLog adds one Log entry per entry of current that previous does not hold, counting repeats:
// an entry written twice is new once when previous holds it once. A zero previous holds none.
func (c *computation) addLog(id string, previous, current parsed[issue.Issue]) {
	previousEntries, isPreviousRead := c.logEntries(previous)
	currentEntries, isCurrentRead := c.logEntries(current)
	if !isPreviousRead || !isCurrentRead {
		return
	}
	unmatched := make(map[string]int, len(previousEntries))
	for _, entry := range previousEntries {
		unmatched[entry]++
	}
	for _, entry := range currentEntries {
		if unmatched[entry] > 0 {
			unmatched[entry]--
			continue
		}
		c.report.Log = append(
			c.report.Log,
			Entry{ID: id, Title: current.value.Title, LogEntry: entry},
		)
	}
}

// logEntries returns the Log entries of the issue copy, or false with a warning when its Log
// holds a line that is not part of an entry, where no entry can be told apart reliably.
func (c *computation) logEntries(issueCopy parsed[issue.Issue]) ([]string, bool) {
	entries, isEntryOnly := issue.LogEntries(issueCopy.data)
	if !isEntryOnly {
		c.warn("%s: Log not read: it holds a line that is not part of an entry", issueCopy.label)
	}
	return entries, isEntryOnly
}

// compareNotes adds the Notes added, changed and retired of the Note snapshots. before is
// changed: a renamed Note's former id is moved to its HEAD id.
func (c *computation) compareNotes(before, after snapshot) {
	copies := loadChanged(c, before, after, knowledge.Parse)
	pairRenames(before, after, copies, func(n knowledge.Note) []string { return n.Aliases })
	for _, id := range sortedIDs(copies) {
		current := copies[id].value
		entry := Entry{ID: id, Title: current.Title}
		file, isInBoundary := before[id]
		if !isInBoundary {
			c.report.NotesAdded = append(c.report.NotesAdded, entry)
			continue
		}
		previous, isRead := load(c, c.before, file, knowledge.Parse)
		if !isRead {
			continue
		}
		if current.IsRetired && !previous.value.IsRetired {
			c.report.NotesRetired = append(c.report.NotesRetired, entry)
		} else {
			c.report.NotesChanged = append(c.report.NotesChanged, entry)
		}
	}

	for _, id := range sortedIDs(before) {
		if _, isKept := after[id]; isKept {
			continue
		}
		previous, isRead := load(c, c.before, before[id], knowledge.Parse)
		if isRead {
			c.report.NotesRetired = append(
				c.report.NotesRetired,
				Entry{ID: id, Title: previous.value.Title},
			)
		}
	}
}

// loadChanged reads the HEAD copy of every id in after whose file is new or differs from its file
// in before; a copy that cannot be read or parsed gets a warning and is left out.
func loadChanged[T any](
	c *computation,
	before, after snapshot,
	parse func(string, []byte) (T, error),
) map[string]parsed[T] {
	copies := map[string]parsed[T]{}
	for _, id := range sortedIDs(after) {
		file := after[id]
		if previous, isInBoundary := before[id]; isInBoundary && previous.Blob == file.Blob {
			continue
		}
		if headCopy, isRead := load(c, c.after, file, parse); isRead {
			copies[id] = headCopy
		}
	}
	return copies
}

// pairRenames moves the boundary file of each id gone from HEAD to the id new at HEAD whose copy
// lists it among its aliases, so the pair counts as one id. The first alias that pairs wins, and
// new ids are tried in id order.
func pairRenames[T any](
	before, after snapshot,
	copies map[string]parsed[T],
	aliasesOf func(T) []string,
) {
	for _, id := range sortedIDs(copies) {
		if _, isInBoundary := before[id]; isInBoundary {
			continue
		}
		for _, alias := range aliasesOf(copies[id].value) {
			former, isFormer := before[alias]
			if _, isKept := after[alias]; !isFormer || isKept {
				continue
			}
			before[id] = former
			delete(before, alias)
			break
		}
	}
}

// load reads the copy of file in end and parses it, or returns false with a warning when it is
// larger than maxFileSize, cannot be read or does not parse.
func load[T any](
	c *computation,
	end revision,
	file gitx.TreeFile,
	parse func(string, []byte) (T, error),
) (parsed[T], bool) {
	label := end.label + ":" + file.Path
	data, err := gitx.ShowFile(c.root, end.commit, file.Path, maxFileSize)
	if errors.Is(err, gitx.ErrBlobTooLarge) {
		c.warn("%s: %v", label, tree.ErrTooLarge)
		return parsed[T]{}, false
	}
	if err != nil {
		c.warn("%s: %v", label, err)
		return parsed[T]{}, false
	}
	value, err := parse(label, data)
	if err != nil {
		c.warn("%v", err)
		return parsed[T]{}, false
	}
	return parsed[T]{label: label, data: data, value: value}, true
}

// warn records one warning, prefixed so a reader knows it comes from the recap.
func (c *computation) warn(format string, args ...any) {
	c.report.Warnings = append(c.report.Warnings, "recap: "+fmt.Sprintf(format, args...))
}

// sortedIDs returns the keys of byID in order.
func sortedIDs[V any](byID map[string]V) []string {
	return slices.Sorted(maps.Keys(byID))
}

// sortedByID returns entries ordered by id, keeping the order of entries with the same id.
func sortedByID(entries []Entry) []Entry {
	slices.SortStableFunc(entries, func(a, b Entry) int { return cmp.Compare(a.ID, b.ID) })
	return entries
}
