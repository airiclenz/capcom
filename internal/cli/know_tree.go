package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// Flags of capcom know tree.
const (
	flagBackground = "background"
)

// backgroundFlagUsage is the help of the --background flag of know tree.
const backgroundFlagUsage = "include notes labelled background and everything below them"

// Help text of capcom know tree.
const (
	knowTreeLong = "Print the outline of the Knowledge tree, or of the subtree of the Note named: one line per " +
		"Note with its id and title, indented two spaces per level. --depth limits the levels printed " +
		"below the top, or below the named Note, and a \"... <n> more\" line marks each Note whose " +
		"children it cut. Retired subtrees are left out unless --retired is given, and Background " +
		"subtrees, labelled background, unless --background is given. A Note named as the start is " +
		"printed even when retired or background."
	knowTreeExample = "  capcom know tree                          # the whole Knowledge tree\n" +
		"  capcom know tree --depth 2                # the top level and its direct children\n" +
		"  capcom know tree release-steps            # one branch\n" +
		"  capcom know tree --retired --background   # retired and background Notes included"
)

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
		Long:        knowTreeLong,
		Example:     knowTreeExample,
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if depth < unlimitedDepth {
				return output.UsageErrorf("--%s must be %d (every level) or more", flagDepth, unlimitedDepth)
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
			return a.emit(cmd, output.Result{Kind: "know-tree", Data: data, Text: renderOutline(rows)})
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

// noteID returns the id of entry, the key its children are grouped under.
func noteID(entry index.NoteEntry) string {
	return entry.ID
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
		// Dropping a background Note drops its subtree too: walkOutline reaches a Note only
		// through its parent's group. The filter stays here, not in childrenByParent, which know
		// show --with-children also uses.
		notes = slices.DeleteFunc(notes, func(note index.NoteEntry) bool {
			return knowledge.IsBackground(note.Labels)
		})
	}
	children := childrenByParent(notes)
	var lines []outlineLine
	walk := func(parentID string, level int, indent int) {
		walkOutline(
			func(parentID string) []index.NoteEntry { return children[parentID] },
			noteID,
			parentID,
			level,
			indent,
			maxDepth,
			func(entry index.NoteEntry, indent int, more int, moreDepth int) {
				line := noteOutlineLine(entry, indent)
				line.more, line.moreDepth = more, moreDepth
				lines = append(lines, line)
			},
		)
	}
	if len(args) == 0 {
		walk("", 1, 0)
		return lines, nil
	}
	start, err := a.resolveNote(args[0])
	if err != nil {
		return nil, err
	}
	lines = append(lines, noteOutlineLine(start, 0))
	walk(start.ID, 1, 1)
	return lines, nil
}
