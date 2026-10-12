package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
)

// reindexData is the --json data of capcom reindex.
type reindexData struct {
	// Path is the absolute index file.
	Path string `json:"path"`
	// Issues is the number of issues indexed; files left out are reported as warnings.
	Issues int `json:"issues"`
}

// newReindexCommand rebuilds the index from the issue and Note files. The rebuild itself runs when
// prepareIndex opens the index in index.ModeRebuild, as the command's annotation asks.
func (a *app) newReindexCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "reindex",
		Short:       "Rebuild the index from the issue and Note files",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRebuild},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			path := index.Path(a.store)
			indexed := a.indexReport.Indexed
			return a.emit(cmd, Result{
				Kind: "reindex",
				Data: reindexData{Path: path, Issues: indexed},
				Text: fmt.Sprintf("reindexed %d issues into %s\n", indexed, path),
			})
		}),
	}
}
