package diagnose

import (
	"fmt"

	"github.com/airiclenz/capcom/internal/tree"
)

// Claim is one name a node file answers to: its id, or one of its Aliases. Kind is "id" or
// "alias", and Path is the file's, relative to the working tree root with forward slashes.
type Claim struct {
	Name string
	Path string
	Kind string
}

// NameClashes returns a Duplicate finding for every name two or more claims share (ADR 0002:
// names are unique across a tree): one per claimant, at the claimant's path, naming the first
// other claimant and how many more there are, so k claimants yield k findings. An identical
// claim repeated counts once. claims holds the names of one tree only, since an issue and a Note
// never clash. Findings follow the order of claims: by name as each first appears, then by
// claimant.
func NameClashes(claims []Claim) []Finding {
	byName := map[string][]Claim{}
	isSeen := map[Claim]bool{}
	var names []string
	for _, current := range claims {
		if isSeen[current] {
			continue
		}
		isSeen[current] = true
		if _, isNamed := byName[current.Name]; !isNamed {
			names = append(names, current.Name)
		}
		byName[current.Name] = append(byName[current.Name], current)
	}

	var findings []Finding
	for _, name := range names {
		claimants := byName[name]
		if len(claimants) < 2 {
			continue
		}
		for position, current := range claimants {
			other := claimants[0]
			if position == 0 {
				other = claimants[1]
			}
			findings = append(findings, Finding{
				Kind:    Duplicate,
				Path:    current.Path,
				Message: clashMessage(current, other, len(claimants)-2),
			})
		}
	}
	return findings
}

// clashMessage words the finding at current's path: it names other, and the count of the
// claimants beyond those two when there are any.
func clashMessage(current, other Claim, moreCount int) string {
	if moreCount == 0 {
		return fmt.Sprintf("%v %s %q: also the %s of %s; both are left out",
			tree.ErrDuplicate, current.Kind, current.Name, other.Kind, other.Path)
	}
	return fmt.Sprintf("%v %s %q: also the %s of %s and %d more; all are left out",
		tree.ErrDuplicate, current.Kind, current.Name, other.Kind, other.Path, moreCount)
}
