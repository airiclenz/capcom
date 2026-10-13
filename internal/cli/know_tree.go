package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// Flags of capcom know tree.
const (
	flagBackground = "background"
)

// backgroundFlagUsage is the help of the --background flag of know tree.
const backgroundFlagUsage = "include notes labelled background and everything below them"

// outlineData is one line of know tree in --json: a Note, its indentation depth in the outline and
// how many of its direct children --depth cut (more, left out when none).
type outlineData struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Parent string `json:"parent,omitempty"`
	Depth  int    `json:"depth"`
	More   int    `json:"more,omitempty"`
}

// newKnowTreeCommand prints the outline of the Knowledge tree, or of the subtree of one Note: one
// line per Note, `id  title`, indented two spaces per level, and a `... <n> more (--depth <d>)`
// line under each Note whose children --depth cut. Retired subtrees are left out unless --retired
// is given, and background subtrees unless --background is given; neither is counted as cut. A
// Note named as the start is printed even when retired or background.
func (a *app) newKnowTreeCommand() *cobra.Command {
	depth := unlimitedDepth
	isRetiredIncluded := false
	isBackgroundIncluded := false
	cmd := &cobra.Command{
		Use:         "tree [id|alias|prefix]",
		Short:       "Print the outline of Note ids and titles",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if depth < unlimitedDepth {
				return UsageErrorf("--%s must be %d (every level) or more", flagDepth, unlimitedDepth)
			}
			lines, err := a.outline(args, depth, isRetiredIncluded, isBackgroundIncluded)
			if err != nil {
				return err
			}
			data := make([]outlineData, 0, len(lines))
			rows := make([]outlineRow, 0, len(lines))
			for _, line := range lines {
				data = append(data, outlineData{
					line.entry.ID, line.entry.Title, line.entry.ParentID, line.depth, line.more,
				})
				rows = append(rows, line.outlineRow)
			}
			return a.emit(cmd, Result{Kind: "know-tree", Data: data, Text: renderOutline(rows)})
		}),
	}
	cmd.Flags().IntVar(&depth, flagDepth, unlimitedDepth,
		"levels below the top (or below the named note) to print; 0 prints every level")
	cmd.Flags().BoolVar(&isRetiredIncluded, flagRetired, false, retiredFlagUsage)
	cmd.Flags().BoolVar(&isBackgroundIncluded, flagBackground, false, backgroundFlagUsage)
	return cmd
}

// outlineLine is one Note of an outline and its row: `id  title` at its indentation depth, with
// the count of direct children maxDepth cut.
type outlineLine struct {
	entry index.NoteEntry
	outlineRow
}

// noteOutlineLine returns the outline line of a Note at an indentation depth, cutting nothing.
func noteOutlineLine(entry index.NoteEntry, depth int) outlineLine {
	return outlineLine{entry, outlineRow{depth: depth, text: entry.ID + "  " + flatten(entry.Title)}}
}

// outline lists the Notes know tree prints: the whole tree, or the Note args names followed by its
// subtree, each Note followed by its children in id order, at most maxDepth levels below the
// start (unlimitedDepth for all). A Note on the last level counts the direct children it cut.
// Unless isRetiredIncluded, retired subtrees are left out below the start, and unless
// isBackgroundIncluded, so are background subtrees; neither is ever counted. The named start
// itself always resolves.
func (a *app) outline(
	args []string,
	maxDepth int,
	isRetiredIncluded bool,
	isBackgroundIncluded bool,
) ([]outlineLine, error) {
	notes, err := a.issueIndex.Notes(isRetiredIncluded)
	if err != nil {
		return nil, err
	}
	if !isBackgroundIncluded {
		// Dropping a background Note drops its subtree too: visit reaches a Note only through its
		// parent's group. The filter stays here, not in childrenByParent, which know show
		// --with-children also uses.
		notes = slices.DeleteFunc(notes, func(note index.NoteEntry) bool {
			return knowledge.IsBackground(note.Labels)
		})
	}
	children := childrenByParent(notes)
	var lines []outlineLine
	var visit func(parentID string, level int, indent int)
	visit = func(parentID string, level int, indent int) {
		isLastLevel := maxDepth != unlimitedDepth && level >= maxDepth
		for _, child := range children[parentID] {
			line := noteOutlineLine(child, indent)
			if isLastLevel {
				line.more, line.moreDepth = len(children[child.ID]), level+1
			}
			lines = append(lines, line)
			if !isLastLevel {
				visit(child.ID, level+1, indent+1)
			}
		}
	}
	if len(args) == 0 {
		visit("", 1, 0)
		return lines, nil
	}
	start, err := a.resolveNote(args[0])
	if err != nil {
		return nil, err
	}
	lines = append(lines, noteOutlineLine(start, 0))
	visit(start.ID, 1, 1)
	return lines, nil
}
