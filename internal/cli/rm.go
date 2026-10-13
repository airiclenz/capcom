package cli

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/cli/output"
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
// kindFailure types them.
func issueRemovalFailure(err error) error {
	var dependents *nodestore.DependentsError
	if errors.As(err, &dependents) {
		return &output.Error{
			Code:    output.CodeInvalid,
			Path:    dependents.Path,
			Message: fmt.Sprintf("%s; pass --%s to remove those deps entries", dependents.Message, flagDropDeps),
		}
	}
	return kindFailure(err)
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
