package issue

import (
	"slices"

	"github.com/airiclenz/capcom/internal/config"
)

// BlockMissing is the Blocker reason of a dependency that names no issue, by id or by alias.
const BlockMissing = "missing"

// Node is one issue as the ready rules see it: where it sits in the tree, what its status means,
// and what it names. Ready needs every issue of the tree, Closed ones included, since dependencies
// and children are judged by them.
type Node struct {
	ID string
	// ParentID is the id of the issue whose directory holds this one; "" at the top of the tree.
	ParentID string
	Category config.Category
	// IsStanding is the issue's own standing mark: a standing Parent that only groups its
	// children.
	IsStanding bool
	Labels     []string
	// Deps are the dependencies as the file writes them: each an id or an Alias.
	Deps    []string
	Aliases []string
}

// Blocker is one dependency that keeps an issue from being Ready.
type Blocker struct {
	// Dep is the dependency as the file writes it.
	Dep string
	// Reason is BlockMissing when Dep names no issue, and otherwise the category of the issue it
	// names (todo, doing or waiting).
	Reason string
}

// Readiness is the verdict of the ready rules on one issue in the todo category.
type Readiness struct {
	ID string
	// IsReady is true when nothing below keeps the issue back.
	IsReady bool
	// IsHidden is true when the issue carries a label the config's ready.hide lists; it says
	// nothing about readiness.
	IsHidden bool
	// Blocking are the dependencies that are not Closed, in the file's order.
	Blocking []Blocker
	// OpenChildren are the ids of the direct children that are not Closed, in node order.
	OpenChildren []string
	// HeldBy is the id of the nearest ancestor in the waiting or dropped category, or "".
	HeldBy string
	// DroppedDeps are the dependencies counted as satisfied only because they were dropped, in
	// the file's order; they deserve a warning.
	DroppedDeps []string
}

// Ready judges every node in the todo category against the ready rules and returns one Readiness
// per such node, in node order. An issue is Ready when every dependency is done or dropped, no
// direct child is open (not Closed) and no ancestor is waiting or dropped. A dependency resolves
// by exact id, then by Alias (ADR 0002), never by prefix; one that resolves to nothing, or to an
// alias several issues claim, blocks with reason BlockMissing. Only categories are read (ADR 0003).
func Ready(nodes []Node, cfg config.Config) []Readiness {
	tree := newReadyTree(nodes)
	var verdicts []Readiness
	for _, node := range nodes {
		if node.Category != config.CategoryTodo {
			continue
		}
		verdict := Readiness{
			ID:           node.ID,
			IsHidden:     hasAny(node.Labels, cfg.Ready.Hide),
			OpenChildren: tree.openChildren[node.ID],
			HeldBy:       tree.holdingAncestor(node),
		}
		verdict.Blocking, verdict.DroppedDeps = tree.judgeDeps(node.Deps)
		verdict.IsReady = len(verdict.Blocking) == 0 && len(verdict.OpenChildren) == 0 && verdict.HeldBy == ""
		verdicts = append(verdicts, verdict)
	}
	return verdicts
}

// readyTree indexes the nodes for the ready rules.
type readyTree struct {
	byID map[string]Node
	// byAlias maps each alias to the one node claiming it; an alias two nodes claim maps to nil.
	byAlias      map[string]*Node
	openChildren map[string][]string
}

// newReadyTree indexes nodes by id, by alias and by parent.
func newReadyTree(nodes []Node) readyTree {
	tree := readyTree{
		byID:         make(map[string]Node, len(nodes)),
		byAlias:      map[string]*Node{},
		openChildren: map[string][]string{},
	}
	for index := range nodes {
		node := &nodes[index]
		tree.byID[node.ID] = *node
		for _, alias := range node.Aliases {
			if _, isClaimed := tree.byAlias[alias]; isClaimed {
				tree.byAlias[alias] = nil
				continue
			}
			tree.byAlias[alias] = node
		}
		if node.ParentID != "" && !node.Category.IsClosed() {
			tree.openChildren[node.ParentID] = append(tree.openChildren[node.ParentID], node.ID)
		}
	}
	return tree
}

// resolve finds the node dep names: an exact id first, then an alias exactly one node claims.
func (t readyTree) resolve(dep string) (Node, bool) {
	if node, isFound := t.byID[dep]; isFound {
		return node, true
	}
	if node := t.byAlias[dep]; node != nil {
		return *node, true
	}
	return Node{}, false
}

// judgeDeps splits deps into the ones that block and the ones satisfied only by being dropped.
func (t readyTree) judgeDeps(deps []string) (blocking []Blocker, dropped []string) {
	for _, dep := range deps {
		target, isFound := t.resolve(dep)
		switch {
		case !isFound:
			blocking = append(blocking, Blocker{Dep: dep, Reason: BlockMissing})
		case !target.Category.IsClosed():
			blocking = append(blocking, Blocker{Dep: dep, Reason: string(target.Category)})
		case target.Category == config.CategoryDropped:
			dropped = append(dropped, dep)
		}
	}
	return blocking, dropped
}

// holdingAncestor returns the id of node's nearest ancestor in the waiting or dropped category,
// or "" when there is none. It stops at a parent missing from the tree, and at a cycle, which a
// directory tree cannot form but a hand-built node list could.
func (t readyTree) holdingAncestor(node Node) string {
	visited := map[string]bool{node.ID: true}
	for parentID := node.ParentID; parentID != "" && !visited[parentID]; {
		parent, isFound := t.byID[parentID]
		if !isFound {
			return ""
		}
		if parent.Category == config.CategoryWaiting || parent.Category == config.CategoryDropped {
			return parent.ID
		}
		visited[parentID] = true
		parentID = parent.ParentID
	}
	return ""
}

// hasAny reports whether values holds at least one of wanted.
func hasAny(values, wanted []string) bool {
	for _, value := range values {
		if slices.Contains(wanted, value) {
			return true
		}
	}
	return false
}
