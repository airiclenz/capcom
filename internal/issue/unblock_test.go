package issue_test

import (
	"reflect"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
)

// withLabels returns node carrying labels.
func withLabels(node issue.Node, labels ...string) issue.Node {
	node.Labels = labels
	return node
}

func TestNewlyReady_Closing_ReturnsTheIssuesItReadies(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		nodes  []issue.Node
		hide   []string
		closed map[string]config.Category
		want   []string
	}{
		{
			name: "sole blocker closed done",
			nodes: []issue.Node{
				readyNode("schema", config.CategoryDoing),
				withDeps(readyNode("api", config.CategoryTodo), "schema"),
			},
			closed: map[string]config.Category{"schema": config.CategoryDone},
			want:   []string{"api"},
		},
		{
			name: "sole blocker dropped",
			nodes: []issue.Node{
				readyNode("schema", config.CategoryTodo),
				withDeps(readyNode("api", config.CategoryTodo), "schema"),
			},
			closed: map[string]config.Category{"schema": config.CategoryDropped},
			want:   []string{"api"},
		},
		{
			name: "one of two blockers closed",
			nodes: []issue.Node{
				readyNode("schema", config.CategoryDoing),
				readyNode("auth", config.CategoryTodo),
				withDeps(readyNode("api", config.CategoryTodo), "schema", "auth"),
			},
			closed: map[string]config.Category{"schema": config.CategoryDone},
			want:   nil,
		},
		{
			name: "both blockers closed together",
			nodes: []issue.Node{
				readyNode("schema", config.CategoryDoing),
				readyNode("auth", config.CategoryTodo),
				withDeps(readyNode("api", config.CategoryTodo), "schema", "auth"),
			},
			closed: map[string]config.Category{
				"schema": config.CategoryDone,
				"auth":   config.CategoryDone,
			},
			want: []string{"api"},
		},
		{
			name: "last open child closed readies the parent",
			nodes: []issue.Node{
				readyNode("epic", config.CategoryTodo),
				childNode("first", "epic", config.CategoryDone),
				childNode("second", "epic", config.CategoryDoing),
			},
			closed: map[string]config.Category{"second": config.CategoryDone},
			want:   []string{"epic"},
		},
		{
			name: "last open child closed leaves a standing parent out",
			nodes: []issue.Node{
				standingNode("epic"),
				childNode("first", "epic", config.CategoryDone),
				childNode("second", "epic", config.CategoryDoing),
			},
			closed: map[string]config.Category{"second": config.CategoryDone},
			want:   nil,
		},
		{
			name: "dependency written as an alias",
			nodes: []issue.Node{
				{ID: "schema", Category: config.CategoryDoing, Aliases: []string{"bd-a1"}},
				withDeps(readyNode("api", config.CategoryTodo), "bd-a1"),
			},
			closed: map[string]config.Category{"schema": config.CategoryDone},
			want:   []string{"api"},
		},
		{
			name: "hidden dependent left out",
			nodes: []issue.Node{
				readyNode("schema", config.CategoryDoing),
				withLabels(withDeps(readyNode("deploy", config.CategoryTodo), "schema"), "owner-run"),
				withDeps(readyNode("api", config.CategoryTodo), "schema"),
			},
			hide:   []string{"owner-run"},
			closed: map[string]config.Category{"schema": config.CategoryDone},
			want:   []string{"api"},
		},
		{
			name: "issue Ready before left out",
			nodes: []issue.Node{
				readyNode("schema", config.CategoryDoing),
				readyNode("docs", config.CategoryTodo),
				withDeps(readyNode("api", config.CategoryTodo), "schema"),
			},
			closed: map[string]config.Category{"schema": config.CategoryDone},
			want:   []string{"api"},
		},
		{
			name: "closed issue itself left out",
			nodes: []issue.Node{
				readyNode("schema", config.CategoryTodo),
			},
			closed: map[string]config.Category{"schema": config.CategoryDone},
			want:   nil,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cfg := config.Default()
			cfg.Ready.Hide = testCase.hide

			got := issue.NewlyReady(testCase.nodes, cfg, testCase.closed)

			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("NewlyReady = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestNewlyReady_Nodes_LeftUnchanged(t *testing.T) {
	t.Parallel()
	nodes := []issue.Node{
		readyNode("schema", config.CategoryDoing),
		withDeps(readyNode("api", config.CategoryTodo), "schema"),
	}

	issue.NewlyReady(nodes, config.Default(), map[string]config.Category{"schema": config.CategoryDone})

	if nodes[0].Category != config.CategoryDoing {
		t.Errorf("schema category = %q, want %q", nodes[0].Category, config.CategoryDoing)
	}
}
