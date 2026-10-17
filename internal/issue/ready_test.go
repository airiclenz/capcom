package issue_test

import (
	"reflect"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
)

// readyNode returns a node at the top of the tree with the given category.
func readyNode(id string, category config.Category) issue.Node {
	return issue.Node{ID: id, Category: category}
}

// childNode returns a node below parentID with the given category.
func childNode(id, parentID string, category config.Category) issue.Node {
	return issue.Node{ID: id, ParentID: parentID, Category: category}
}

// standingNode returns a todo node at the top of the tree carrying the standing mark.
func standingNode(id string) issue.Node {
	return issue.Node{ID: id, Category: config.CategoryTodo, IsStanding: true}
}

// withDeps returns node depending on deps.
func withDeps(node issue.Node, deps ...string) issue.Node {
	node.Deps = deps
	return node
}

// verdictOf returns the Readiness Ready gives the node with id, failing when there is none.
func verdictOf(t *testing.T, verdicts []issue.Readiness, id string) issue.Readiness {
	t.Helper()
	for _, verdict := range verdicts {
		if verdict.ID == id {
			return verdict
		}
	}
	t.Fatalf("no readiness for %q in %+v", id, verdicts)
	return issue.Readiness{}
}

func TestReady_Rules_JudgeEachTodoIssue(t *testing.T) {
	cases := []struct {
		name  string
		nodes []issue.Node
		id    string
		want  issue.Readiness
	}{
		{
			name:  "plain todo is ready",
			nodes: []issue.Node{readyNode("login", config.CategoryTodo)},
			id:    "login",
			want:  issue.Readiness{ID: "login", IsReady: true},
		},
		{
			name: "dep done is satisfied",
			nodes: []issue.Node{
				withDeps(readyNode("login", config.CategoryTodo), "schema"),
				readyNode("schema", config.CategoryDone),
			},
			id:   "login",
			want: issue.Readiness{ID: "login", IsReady: true},
		},
		{
			name: "dep dropped is satisfied with a warning",
			nodes: []issue.Node{
				withDeps(readyNode("login", config.CategoryTodo), "schema"),
				readyNode("schema", config.CategoryDropped),
			},
			id:   "login",
			want: issue.Readiness{ID: "login", IsReady: true, DroppedDeps: []string{"schema"}},
		},
		{
			name: "dep still doing blocks with its category",
			nodes: []issue.Node{
				withDeps(readyNode("login", config.CategoryTodo), "schema"),
				readyNode("schema", config.CategoryDoing),
			},
			id: "login",
			want: issue.Readiness{
				ID:       "login",
				Blocking: []issue.Blocker{{Dep: "schema", Reason: string(config.CategoryDoing)}},
			},
		},
		{
			name:  "dep resolving to nothing blocks as missing",
			nodes: []issue.Node{withDeps(readyNode("login", config.CategoryTodo), "gone")},
			id:    "login",
			want: issue.Readiness{
				ID:       "login",
				Blocking: []issue.Blocker{{Dep: "gone", Reason: issue.BlockMissing}},
			},
		},
		{
			name: "dep named by alias is satisfied",
			nodes: []issue.Node{
				withDeps(readyNode("login", config.CategoryTodo), "apogee-kk0.4"),
				{ID: "schema", Category: config.CategoryDone, Aliases: []string{"apogee-kk0.4"}},
			},
			id:   "login",
			want: issue.Readiness{ID: "login", IsReady: true},
		},
		{
			name: "dep never resolves by prefix",
			nodes: []issue.Node{
				withDeps(readyNode("login", config.CategoryTodo), "sch"),
				readyNode("schema", config.CategoryDone),
			},
			id: "login",
			want: issue.Readiness{
				ID:       "login",
				Blocking: []issue.Blocker{{Dep: "sch", Reason: issue.BlockMissing}},
			},
		},
		{
			name: "parent with open child is not ready",
			nodes: []issue.Node{
				readyNode("epic", config.CategoryTodo),
				childNode("part", "epic", config.CategoryTodo),
				childNode("finished", "epic", config.CategoryDone),
			},
			id:   "epic",
			want: issue.Readiness{ID: "epic", OpenChildren: []string{"part"}},
		},
		{
			name: "child of a todo parent is ready",
			nodes: []issue.Node{
				readyNode("epic", config.CategoryTodo),
				childNode("part", "epic", config.CategoryTodo),
			},
			id:   "part",
			want: issue.Readiness{ID: "part", IsReady: true},
		},
		{
			name: "parked grandparent holds the subtree",
			nodes: []issue.Node{
				readyNode("epic", config.CategoryWaiting),
				childNode("story", "epic", config.CategoryDoing),
				childNode("task", "story", config.CategoryTodo),
			},
			id:   "task",
			want: issue.Readiness{ID: "task", HeldBy: "epic"},
		},
		{
			name: "dropped parent holds its children",
			nodes: []issue.Node{
				readyNode("epic", config.CategoryDropped),
				childNode("task", "epic", config.CategoryTodo),
			},
			id:   "task",
			want: issue.Readiness{ID: "task", HeldBy: "epic"},
		},
		{
			name: "done parent does not hold its children",
			nodes: []issue.Node{
				readyNode("epic", config.CategoryDone),
				childNode("task", "epic", config.CategoryTodo),
			},
			id:   "task",
			want: issue.Readiness{ID: "task", IsReady: true},
		},
		{
			name: "standing parent with every child closed is not ready",
			nodes: []issue.Node{
				standingNode("epic"),
				childNode("task", "epic", config.CategoryDone),
			},
			id:   "epic",
			want: issue.Readiness{ID: "epic", IsStanding: true},
		},
		{
			name:  "standing issue without children is not ready",
			nodes: []issue.Node{standingNode("epic")},
			id:    "epic",
			want:  issue.Readiness{ID: "epic", IsStanding: true},
		},
		{
			name: "open child of a standing parent is ready",
			nodes: []issue.Node{
				standingNode("epic"),
				childNode("task", "epic", config.CategoryTodo),
			},
			id:   "task",
			want: issue.Readiness{ID: "task", IsReady: true},
		},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			verdicts := issue.Ready(current.nodes, config.Default())

			got := verdictOf(t, verdicts, current.id)

			if !reflect.DeepEqual(got, current.want) {
				t.Errorf("Ready(%s) = %+v, want %+v", current.id, got, current.want)
			}
		})
	}
}

func TestReady_OnlyTodo_JudgedInNodeOrder(t *testing.T) {
	nodes := []issue.Node{
		readyNode("triaged", config.CategoryTodo),
		readyNode("started", config.CategoryDoing),
		readyNode("parked", config.CategoryWaiting),
		readyNode("finished", config.CategoryDone),
		readyNode("next", config.CategoryTodo),
	}

	verdicts := issue.Ready(nodes, config.Default())

	ids := make([]string, 0, len(verdicts))
	for _, verdict := range verdicts {
		ids = append(ids, verdict.ID)
	}
	if want := []string{"triaged", "next"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("judged %v, want %v", ids, want)
	}
}

func TestReady_HideLabel_MarksIssueHidden(t *testing.T) {
	cfg := config.Default()
	cfg.Ready.Hide = []string{"owner-run"}
	nodes := []issue.Node{
		{ID: "deploy", Category: config.CategoryTodo, Labels: []string{"ops", "owner-run"}},
		{ID: "login", Category: config.CategoryTodo, Labels: []string{"ops"}},
	}

	verdicts := issue.Ready(nodes, cfg)

	if deploy := verdictOf(t, verdicts, "deploy"); !deploy.IsHidden || !deploy.IsReady {
		t.Errorf("deploy = %+v, want hidden and ready", deploy)
	}
	if login := verdictOf(t, verdicts, "login"); login.IsHidden {
		t.Errorf("login = %+v, want not hidden", login)
	}
}

func TestReady_AliasClaimedTwice_BlocksAsMissing(t *testing.T) {
	nodes := []issue.Node{
		withDeps(readyNode("login", config.CategoryTodo), "old"),
		{ID: "first", Category: config.CategoryDone, Aliases: []string{"old"}},
		{ID: "second", Category: config.CategoryDone, Aliases: []string{"old"}},
	}

	verdicts := issue.Ready(nodes, config.Default())

	want := []issue.Blocker{{Dep: "old", Reason: issue.BlockMissing}}
	if got := verdictOf(t, verdicts, "login").Blocking; !reflect.DeepEqual(got, want) {
		t.Errorf("Blocking = %+v, want %+v", got, want)
	}
}
