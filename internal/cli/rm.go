package cli

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// Flags and output of capcom rm and know rm.
const (
	kindIssueRemove = "issue-remove"
	// kindNoteRemove is the --json kind of know rm: the id and path of the removed Note file and
	// the ids of the Notes whose superseded-by entries naming it were dropped.
	kindNoteRemove = "note-remove"
	flagDropDeps   = "drop-deps"
)

// issueRmHelp is the help text of capcom rm.
var issueRmHelp = commandHelp{
	long: "Remove an Issue's file.\n\n" +
		"An Issue with children is refused, so no orphan directory is left behind, and so is an Issue " +
		"other Issues depend on, unless --drop-deps removes their deps entries naming it.\n\n" +
		"Every other reference to the Issue is dropped, and its Aliases go with the file.\n\n" +
		"A Note Ref naming the file is kept, with a warning, as capcom check fails on it.\n\n" +
		"capcom deletes the file itself and never runs git.\n\n" +
		"It prints the removed Issue's id and path, and the Issues whose deps entries were dropped.",
	example: "  capcom rm login-copy                       # refused while it has children or dependents\n" +
		"  capcom rm login-timeout-race --drop-deps   # also drops the deps entries that name it",
}

// noteRmHelp is the help text of capcom know rm.
var noteRmHelp = commandHelp{
	long: "Remove a Note's file.\n\n" +
		"A Note with children is refused, so no orphan directory is left behind.\n\n" +
		"Every superseded-by entry naming it is dropped, and its Aliases go with the file.\n\n" +
		"A Note Ref naming the file is kept, with a warning, as capcom check fails on it.\n\n" +
		"capcom deletes the file itself and never runs git.\n\n" +
		"It prints the removed Note's id and path, and the Notes whose superseded-by entries were " +
		"dropped.\n\n" +
		"To keep a Note that no longer holds on record, retire it with know update --retire instead.",
	example: "  capcom know rm release-tags                    # refused while it has children\n" +
		"  capcom know rm release-tags --if-match <etag>  # etag from know show",
}

// rmOptions holds the flags of one capcom rm or know rm; know rm has no --drop-deps, so
// shouldDropDeps stays false there.
type rmOptions struct {
	ifMatch        string
	shouldDropDeps bool
}

// issueRemoveData is the --json data of rm: the removed issue's id and path, and the ids of the
// issues whose deps on it and whose related entries naming it were dropped.
type issueRemoveData struct {
	ID             string   `json:"id"`
	Path           string   `json:"path"`
	DepsDropped    []string `json:"deps_dropped"`
	RelatedDropped []string `json:"related_dropped"`
}

// noteRemoveData is the --json data of know rm: the removed Note's id and path, the ids of the
// Notes whose superseded-by entries naming it were dropped, and whether the capcom block of
// AGENTS.md was refreshed.
type noteRemoveData struct {
	ID                  string   `json:"id"`
	Path                string   `json:"path"`
	SupersededByDropped []string `json:"superseded_by_dropped"`
	Agents              string   `json:"agents,omitempty"`
}

// issueRemovalFailure types err, a failure of the removal of an issue: dependents are invalid at
// the issue file, with the flag that drops their deps entries named, and the rest are typed as
// kindFailure types them for an issue.
func (a *app) issueRemovalFailure(err error) error {
	var dependents *nodestore.DependentsError
	if errors.As(err, &dependents) {
		return &output.Error{
			Code:    output.CodeInvalid,
			Path:    dependents.Path,
			Message: fmt.Sprintf("%s; pass --%s to remove those deps entries", dependents.Message, flagDropDeps),
		}
	}
	return a.kindFailure(err, issueIDs)
}

// issueRemoveResult is the result of rm: the id and path of the removed issue file and the ids of
// the issues whose deps on it and whose related entries naming it were dropped.
func issueRemoveResult(removed nodestore.Removed) output.Result {
	dropped := removed.Dropped[nodestore.RefDeps]
	relatedDropped := removed.Dropped[nodestore.RefRelated]
	data := issueRemoveData{
		ID:             removed.ID,
		Path:           removed.Path,
		DepsDropped:    append([]string{}, dropped...),
		RelatedDropped: append([]string{}, relatedDropped...),
	}
	text := fmt.Sprintf("id: %s\npath: %s\n", removed.ID, removed.Path)
	if len(dropped) > 0 {
		text += "deps_dropped: " + flowList(dropped) + "\n"
	}
	if len(relatedDropped) > 0 {
		text += "related_dropped: " + flowList(relatedDropped) + "\n"
	}
	return output.Result{
		Kind: kindIssueRemove,
		Data: data,
		Text: text,
		KV: []output.KVRecord{{Pairs: []output.KVPair{
			{Key: "ID", Value: removed.ID},
			{Key: "PATH", Value: removed.Path},
			{Key: "DEPS_DROPPED", Value: flowList(dropped)},
			{Key: "RELATED_DROPPED", Value: flowList(relatedDropped)},
		}}},
	}
}

// noteRemoveResult is the result of know rm: the id and path of the removed Note file and the ids
// of the Notes whose superseded-by entries naming it were dropped.
func noteRemoveResult(removed nodestore.Removed) output.Result {
	supersededByDropped := removed.Dropped[nodestore.RefSupersededBy]
	text := fmt.Sprintf("id: %s\npath: %s\n", removed.ID, removed.Path)
	if len(supersededByDropped) > 0 {
		text += "superseded_by_dropped: " + flowList(supersededByDropped) + "\n"
	}
	return output.Result{
		Kind: kindNoteRemove,
		Data: &noteRemoveData{
			ID:                  removed.ID,
			Path:                removed.Path,
			SupersededByDropped: append([]string{}, supersededByDropped...),
		},
		Text: text,
	}
}

// cleanupWarnings renders each of leftInPlace, an emptied directory the cleanup of a removal or a
// move left in place, as a warning naming the directory, why it was left, and that it was left.
func cleanupWarnings(leftInPlace []*nodestore.FileError) []string {
	warnings := make([]string, 0, len(leftInPlace))
	for _, failure := range leftInPlace {
		warnings = append(warnings, failure.Path+": "+flatten(failure.Message)+"; the empty directory was not removed")
	}
	return warnings
}

// brokenRefWarnings returns the warning capcom check gives for each Note Ref that named removed's
// file, its children directory or a path below it, and so no longer resolves, "path: ref "<ref>":
// message", as refWarnings words it. The Notes are those the index listed before the removal,
// outside a retired subtree as check reads them, less the removed Note's own file. When the Notes
// cannot be listed or their Refs checked, the one warning says the Refs were not checked.
func (a *app) brokenRefWarnings(removed nodestore.Removed) []string {
	notes, err := a.issueIndex.Notes(false)
	if err != nil {
		return a.refsUncheckedWarnings(removed.ID, err)
	}
	files := make([]knowledge.NoteFile, 0, len(notes))
	for _, entry := range notes {
		files = append(files, knowledge.NoteFile{ID: entry.ID, Path: entry.Path, Note: knowledge.Note{Refs: entry.Refs}})
	}
	files = knowledge.RefsBrokenByRemoval(files, removed.Path)
	if len(files) == 0 {
		return nil
	}
	findings, err := knowledge.Verify(nodestore.RefRepository(a.store), files)
	if err != nil {
		return a.refsUncheckedWarnings(removed.ID, err)
	}
	warnings := make([]string, 0, len(findings))
	for _, finding := range findings {
		warnings = append(warnings, finding.String())
	}
	return warnings
}

// refsUncheckedWarnings returns the one warning of a removal of the node id whose Note Refs could
// not be checked for err: the removal has landed, so it is a warning, not a failure.
func (a *app) refsUncheckedWarnings(id string, err error) []string {
	knowledgeDir := a.store.Relative(a.store.KnowledgeDir())
	return []string{unreadWarning(knowledgeDir, []string{refsKey}, id, a.worktreeRelativeText(err.Error()))}
}
