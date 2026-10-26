package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
)

// reindexData is the --json data of capcom reindex.
type reindexData struct {
	// Path is the absolute index file.
	Path string `json:"path"`
	// Issues is the number of issues indexed; files left out are reported as warnings.
	Issues int `json:"issues"`
}

// Help text of capcom reindex.
const (
	reindexLong = "Rebuild the index from the Issue and Note files.\n\n" +
		"The index is a disposable SQLite file under the git directory, never committed, that the " +
		"other commands bring up to date themselves; reindex recreates it from scratch.\n\n" +
		"It prints the number of Issues indexed and the index path, and reports each file it left out " +
		"as a warning."
	reindexExample = "  capcom reindex          # rebuild the index from scratch\n" +
		"  capcom reindex --json   # {path, issues} in a JSON envelope"
)

// newReindexCommand rebuilds the index from the issue and Note files. The rebuild itself runs when
// prepareIndex opens the index in index.ModeRebuild, as the command's annotation asks.
func (a *app) newReindexCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "reindex",
		Short:       "Rebuild the index from the issue and Note files",
		Long:        reindexLong,
		Example:     reindexExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRebuild},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			path := index.Path(a.store)
			indexed := a.indexReport.Indexed
			return a.emit(cmd, output.Result{
				Kind: "reindex",
				Data: reindexData{Path: path, Issues: indexed},
				Text: fmt.Sprintf("reindexed %d issues into %s\n", indexed, path),
			})
		}),
	}
}
