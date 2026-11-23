package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/export"
)

// flagHTML is the flag of capcom export that names the directory the HTML export goes to.
const flagHTML = "html"

// exportData is the --json data of capcom export.
type exportData struct {
	// Dir is the absolute directory the export was written to.
	Dir string `json:"dir"`
	// Pages is the number of HTML pages written, the marker file not counted.
	Pages int `json:"pages"`
}

// Help text of capcom export.
const (
	exportLong = "Write the Issues and Notes as a static HTML site into a directory.\n\n" +
		"The site opens from the file system: index.html lists the Ready Issues, issues.html every " +
		"Issue as a tree, closed and dropped ones included, and knowledge.html the Knowledge tree, " +
		"with one page per Issue and per Note. It holds no JavaScript and no external resource.\n\n" +
		"The directory is created when missing. An export writes a .capcom-export marker into it, " +
		"and the next export into that directory removes everything else in it first. A non-empty " +
		"directory without the marker is refused, and so is a directory inside .capcom or one " +
		"holding it.\n\n" +
		"The same Issues and Notes always give the same bytes. It prints the directory and the " +
		"number of pages written."
	exportExample = "  capcom export --html site          # write the site into ./site\n" +
		"  capcom export --html site --json   # {dir, pages} in a JSON envelope"
)

// newExportCommand writes the static HTML export of the store's Issues and Notes through
// export.WriteHTML.
func (a *app) newExportCommand() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:         "export --html <dir>",
		Short:       "Write the Issues and Notes as a static HTML site",
		Long:        exportLong,
		Example:     exportExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			return a.exportHTML(cmd, dir)
		}),
	}
	cmd.Flags().StringVar(&dir, flagHTML, "", "the directory to write the HTML site into (required)")
	_ = cmd.MarkFlagRequired(flagHTML) // fails only for an undefined flag
	return cmd
}

// exportHTML writes the HTML export into dir and reports the directory and the page count. A blank
// dir is a usage error; a directory export.WriteHTML refuses fails with its message, which names
// the directory.
func (a *app) exportHTML(cmd *cobra.Command, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return output.UsageErrorf("--%s is empty; name the directory to write into", flagHTML)
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	pages, err := export.WriteHTML(absolute, a.store, a.issueIndex)
	if err != nil {
		return err
	}
	return a.emit(cmd, output.Result{
		Kind: "export",
		Data: exportData{Dir: absolute, Pages: pages},
		Text: fmt.Sprintf("exported %d pages to %s\n", pages, absolute),
	})
}
