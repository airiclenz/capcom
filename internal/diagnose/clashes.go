package diagnose

import (
	"fmt"

	"github.com/airiclenz/capcom/internal/tree"
)

// NameClashes renders the repeated ids of a membership verdict (ADR 0002: names are unique across
// a tree): conflicts are tree.DecideMembership's IDConflicts, one per file of a repeated id after
// the first, naming the first in Other. It returns a Duplicate finding for every file of each such
// id: one per file, at the file's key, naming the first other file of the id and how many more
// there are, so k files yield k findings. conflicts hold the verdict of one tree only, since an
// issue and a Note never clash. Findings follow the order of conflicts: by id as each first
// appears, then the id's first file, then the rest in order.
func NameClashes(conflicts []tree.Conflict) []Finding {
	var ids []string
	filesOf := map[string][]string{}
	for _, conflict := range conflicts {
		if _, isNamed := filesOf[conflict.Name]; !isNamed {
			ids = append(ids, conflict.Name)
			filesOf[conflict.Name] = []string{conflict.Other}
		}
		filesOf[conflict.Name] = append(filesOf[conflict.Name], conflict.Key)
	}

	var findings []Finding
	for _, id := range ids {
		files := filesOf[id]
		for position, file := range files {
			other := files[0]
			if position == 0 {
				other = files[1]
			}
			findings = append(findings, Finding{
				Code:    Duplicate,
				Path:    file,
				Message: clashMessage(id, other, len(files)-2),
			})
		}
	}
	return findings
}

// clashMessage words the finding of a file of the repeated id: it names other, another file of the
// id, and the count of the files beyond those two when there are any.
func clashMessage(id, other string, moreCount int) string {
	if moreCount == 0 {
		return fmt.Sprintf("%v id %q: also the id of %s; both are left out", tree.ErrDuplicate, id, other)
	}
	return fmt.Sprintf("%v id %q: also the id of %s and %d more; all are left out",
		tree.ErrDuplicate, id, other, moreCount)
}
