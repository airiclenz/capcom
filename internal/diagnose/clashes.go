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
// names are unique across a tree): one per claimant per other claimant, at the claimant's path,
// naming the other one. claims holds the names of one tree only, since an issue and a Note never
// clash. Findings follow the order of claims: by name as each first appears, then by claimant.
func NameClashes(claims []Claim) []Finding {
	byName := map[string][]Claim{}
	var names []string
	for _, current := range claims {
		if _, isSeen := byName[current.Name]; !isSeen {
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
		for _, current := range claimants {
			for _, other := range claimants {
				if other == current {
					continue
				}
				findings = append(findings, Finding{
					Kind: Duplicate,
					Path: current.Path,
					Message: fmt.Sprintf("%v %s %q: also the %s of %s; both are left out",
						tree.ErrDuplicate, current.Kind, current.Name, other.Kind, other.Path),
				})
			}
		}
	}
	return findings
}
