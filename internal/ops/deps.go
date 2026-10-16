package ops

import (
	"slices"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/reflist"
)

// AddDep makes the issue ref names depend on the issue depRef names, writing the canonical id of
// the dependency into its deps, as EditIssue does; a dep already naming that issue leaves the list
// as it is and the file is written unchanged. reflist decides the edit against the deps and the
// Dependency graph the index holds: a self-dependency, an edge that would close a cycle and a
// depRef naming no issue are each its *reflist.Refusal, returned beside the dependent's located
// Written before anything is written.
func (o *Ops) AddDep(ref string, ifMatch string, depRef string) (nodestore.Written, error) {
	dependent, err := o.index.Resolve(ref)
	if err != nil {
		return nodestore.Written{}, err
	}
	located := nodestore.Written{ID: dependent.ID, Path: dependent.Path}
	graph, err := o.DepGraph()
	if err != nil {
		return located, err
	}
	deps := o.ReferenceList(graph, dependent.ID, dependent.Deps)
	deps.FindCycle = graph.FindCycle
	addition, err := deps.Add(depRef)
	if err != nil {
		return located, err
	}
	return o.EditIssue(dependent.ID, ifMatch, func(target issue.Issue) (issue.Issue, error) {
		if !addition.IsPresent {
			target.Deps = append(slices.Clone(target.Deps), addition.ID)
		}
		return target, nil
	})
}

// RemoveDep removes every dep of the issue ref names that names the issue depRef names, whether
// the file writes it as the id or as an Alias, as EditIssue does. A depRef that is not exactly an
// issue's id or Alias removes the dep that is its text, before it is matched as a prefix; one
// naming no issue at all is matched only by its text. A depRef reflist refuses is its
// *reflist.Refusal, returned beside the dependent's located Written before anything is written; a
// removal that drops no dep is a *ChangeError at the dependent's file wrapping the
// *reflist.Refusal of reason NotFound.
func (o *Ops) RemoveDep(ref string, ifMatch string, depRef string) (nodestore.Written, error) {
	dependent, err := o.index.Resolve(ref)
	if err != nil {
		return nodestore.Written{}, err
	}
	located := nodestore.Written{ID: dependent.ID, Path: dependent.Path}
	graph, err := o.DepGraph()
	if err != nil {
		return located, err
	}
	removal, err := o.ReferenceList(graph, dependent.ID, dependent.Deps).Remove(depRef)
	if err != nil {
		return located, err
	}
	return o.EditIssue(dependent.ID, ifMatch, func(target issue.Issue) (issue.Issue, error) {
		dropped, err := removal.Drop(target.Deps)
		if err != nil {
			return issue.Issue{}, err
		}
		target.Deps = WithoutEntries(target.Deps, dropped)
		return target, nil
	})
}

// DepGraph builds the Dependency graph of every usable issue in the index, Closed ones included.
func (o *Ops) DepGraph() (issue.DepGraph, error) {
	nodes, err := o.IssueNodes()
	if err != nil {
		return issue.DepGraph{}, err
	}
	return issue.NewDepGraph(nodes), nil
}

// ReferenceList is the reference list entries of the issue holder, the deps or related it writes,
// whose edits reflist decides: the index resolves an argument and graph names an entry by exact id
// or Alias.
func (o *Ops) ReferenceList(graph issue.DepGraph, holder string, entries []string) reflist.List {
	return reflist.List{Holder: holder, Entries: entries, Resolve: o.ResolveIssueID, Names: graph.Resolve}
}

// ResolveIssueID resolves ref to the id of the issue it names, failing as index.Index.Resolve does:
// the reflist.ResolveFunc of an issue's reference lists.
func (o *Ops) ResolveIssueID(ref string) (string, error) {
	entry, err := o.index.Resolve(ref)
	return entry.ID, err
}

// WithoutEntries returns entries without the ones dropped, or nil when none is left, so the file
// leaves the list's key out.
func WithoutEntries(entries []string, dropped []string) []string {
	kept := slices.DeleteFunc(slices.Clone(entries), func(entry string) bool {
		return slices.Contains(dropped, entry)
	})
	if len(kept) == 0 {
		return nil
	}
	return kept
}
