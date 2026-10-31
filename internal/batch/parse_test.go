package batch_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/airiclenz/capcom/internal/batch"
	"github.com/airiclenz/capcom/internal/ops"
)

// testFileName is the apply file name every test parses under.
const testFileName = "batch.yaml"

// parseOne parses data, which must hold exactly one valid operation, and returns it.
func parseOne(t *testing.T, data string) batch.Op {
	t.Helper()
	operations, err := batch.Parse(testFileName, []byte(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(operations) != 1 {
		t.Fatalf("Parse returned %d operations, want 1", len(operations))
	}
	return operations[0]
}

// textOf returns a pointer to text, the way an ops.IssueUpdate holds a changed string.
func textOf(text string) *string {
	return &text
}

// numberOf returns a pointer to number, the way ops holds a given integer.
func numberOf(number int) *int {
	return &number
}

// flagOf returns a pointer to flag, the way an ops.IssueUpdate holds a changed standing mark.
func flagOf(flag bool) *bool {
	return &flag
}

func TestParse_OneValidFilePerOp_ReturnsTheTypedOperations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data string
		want batch.Op
	}{
		{
			name: "create with every key",
			data: "- op: create\n  id: cache-warmup\n  title: Warm the cache\n  why: Cold starts.\n" +
				"  acceptance: [Warm after deploy, \"no a,b split\"]\n  dep: login-flow, refresh-retry\n" +
				"  label: [ops, \"a,b\"]\n  parent: deploys\n  plan: deploy-plan\n  priority: 1\n" +
				"  standing: false\n  status: todo\n  type: bug\n",
			want: batch.Op{
				Kind: batch.KindCreate, Position: 1, Line: 1, ID: "cache-warmup",
				Create: ops.NewIssue{
					ID: "cache-warmup", Parent: "deploys", Title: "Warm the cache", Status: "todo", Type: "bug",
					Priority: numberOf(1), Labels: []string{"ops", "a", "b"}, Plan: "deploy-plan",
					Deps: []string{"login-flow", "refresh-retry"}, Why: "Cold starts.",
					Acceptance: []string{"Warm after deploy", "no a,b split"},
				},
			},
		},
		{
			name: "update",
			data: "- op: update\n  id: login\n  if-match: 1f1cd6021b07dbfc\n  title: New title\n  status: active\n" +
				"  type: bug\n  priority: 0\n  plan: \"\"\n  no-standing: true\n  add-label: a,b\n" +
				"  remove-label: [c]\n  body-append: [\"one, two\", three]\n  body-replace-old: [x, y]\n" +
				"  body-replace-new: [X, Y]\n",
			want: batch.Op{
				Kind: batch.KindUpdate, Position: 1, Line: 1, ID: "login", IfMatch: "1f1cd6021b07dbfc",
				Update: ops.IssueUpdate{
					Title: textOf("New title"), Status: textOf("active"), Type: textOf("bug"),
					Priority: numberOf(0), Plan: textOf(""), IsStanding: flagOf(false),
					AddLabels: []string{"a", "b"}, RemoveLabels: []string{"c"},
					Replacements: []ops.Replacement{{Old: "x", New: "X"}, {Old: "y", New: "Y"}},
					Appends:      []string{"one, two", "three"},
				},
			},
		},
		{
			name: "dep add",
			data: "- op: dep add\n  id: refresh-retry\n  dep: login-flow\n  if-match: abc\n",
			want: batch.Op{Kind: batch.KindDepAdd, Position: 1, Line: 1, ID: "refresh-retry", IfMatch: "abc", Dep: "login-flow"},
		},
		{
			name: "close",
			data: "- op: close\n  id: login\n  reason: fixed\n  status: dropped\n  if-match: abc\n",
			want: batch.Op{Kind: batch.KindClose, Position: 1, Line: 1, ID: "login", IfMatch: "abc", Status: "dropped", Reason: "fixed"},
		},
		{
			name: "log",
			data: "- op: log\n  id: login\n  text: reproduced\n  if-match: abc\n",
			want: batch.Op{Kind: batch.KindLog, Position: 1, Line: 1, ID: "login", IfMatch: "abc", Text: "reproduced"},
		},
		{
			name: "a number title is its text",
			data: "- op: create\n  id: year\n  title: 2026\n",
			want: batch.Op{
				Kind: batch.KindCreate, Position: 1, Line: 1, ID: "year",
				Create: ops.NewIssue{ID: "year", Title: "2026"},
			},
		},
		{
			name: "update standing true",
			data: "- op: update\n  id: requests\n  standing: true\n",
			want: batch.Op{
				Kind: batch.KindUpdate, Position: 1, Line: 1, ID: "requests",
				Update: ops.IssueUpdate{IsStanding: flagOf(true)},
			},
		},
		{
			name: "no-standing false changes nothing",
			data: "- op: update\n  id: requests\n  no-standing: false\n",
			want: batch.Op{Kind: batch.KindUpdate, Position: 1, Line: 1, ID: "requests"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := parseOne(t, test.data)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("Parse =\n%+v\nwant\n%+v", got, test.want)
			}
		})
	}
}

func TestParse_CreateWithABody_ReturnsTheBodyInline(t *testing.T) {
	t.Parallel()

	got := parseOne(t, "- op: create\n  id: a\n  title: b\n  body: |\n    First line.\n  standing: true\n")

	if got.Create.Body == nil {
		t.Fatal("Create.Body is nil, want the inline body")
	}
	body, err := got.Create.Body()
	if err != nil || body != "First line.\n" || !got.Create.IsStanding {
		t.Errorf("Body() = %q, %v; IsStanding = %v, want \"First line.\\n\", nil; true", body, err, got.Create.IsStanding)
	}
}

func TestParse_SeveralOperations_KeepTheirPositionsAndLines(t *testing.T) {
	t.Parallel()
	data := "# a batch\n- op: log\n  id: a\n  text: one\n\n- op: close\n  id: a\n  reason: done\n"

	operations, err := batch.Parse(testFileName, []byte(data))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := [][2]int{}
	for _, operation := range operations {
		got = append(got, [2]int{operation.Position, operation.Line})
	}
	if want := [][2]int{{1, 2}, {2, 6}}; !reflect.DeepEqual(got, want) {
		t.Errorf("positions and lines = %v, want %v", got, want)
	}
}

func TestParse_IfMatch_IsTakenByEveryOpButCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data string
	}{
		{"update", "- op: update\n  id: a\n  title: b\n  if-match: e1\n"},
		{"dep add", "- op: dep add\n  id: a\n  dep: b\n  if-match: e1\n"},
		{"close", "- op: close\n  id: a\n  reason: b\n  if-match: e1\n"},
		{"log", "- op: log\n  id: a\n  text: b\n  if-match: e1\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := parseOne(t, test.data)

			if got.IfMatch != "e1" {
				t.Errorf("IfMatch = %q, want e1", got.IfMatch)
			}
		})
	}
}

func TestParse_InvalidFile_IsRefusedAtThePositionAndLine(t *testing.T) {
	t.Parallel()
	// valid is a first operation that parses, so each refusal below sits in operation 2 at line 4.
	const valid = "- op: log\n  id: a\n  text: b\n"
	tests := []struct {
		name         string
		fileName     string
		data         string
		wantPosition int
		wantError    string
	}{
		{"empty file", "", "", 0, "batch.yaml:1: the file holds no operations; give a list of them"},
		{"empty list", "", "[]\n", 0, "batch.yaml:1: the list holds no operations"},
		{"not a list", "", "op: log\n", 0, "batch.yaml:1: the file is a mapping; give a list of operations"},
		{
			"not YAML", "", valid + "- op: log\n  id: a\n   bad: x\n", 0,
			"batch.yaml:6: invalid YAML: mapping values are not allowed in this context",
		},
		{
			"a second document", "", valid + "---\n" + valid, 0,
			"batch.yaml:4: a second YAML document; an apply file holds one list of operations",
		},
		{"stdin is named stdin", "-", "", 0, "stdin:1: the file holds no operations; give a list of them"},
		{"not a mapping", "", valid + "- log\n", 2, `batch.yaml:4: operation 2: is "log"; give a mapping of op and flag names`},
		{
			"no op", "", valid + "- id: a\n", 2,
			"batch.yaml:4: operation 2: no op key; give one of create, update, dep add, close or log",
		},
		{
			"unknown op", "", valid + "- op: delete\n", 2,
			`batch.yaml:4: operation 2: unknown op "delete"; give one of create, update, dep add, close or log`,
		},
		{
			"unknown key kv", "", valid + "- op: log\n  id: a\n  text: b\n  kv: true\n", 2,
			`batch.yaml:7: operation 2 (log): unknown key "kv"; log takes op, id, text, if-match`,
		},
		{
			"if-match on create", "", valid + "- op: create\n  id: a\n  title: b\n  if-match: e1\n", 2,
			`batch.yaml:7: operation 2 (create): unknown key "if-match"; create takes op, id, title, body, ` +
				`why, acceptance, dep, label, parent, plan, priority, standing, status, type`,
		},
		{
			"body-file", "", valid + "- op: create\n  id: a\n  title: b\n  body-file: body.md\n", 2,
			"batch.yaml:7: operation 2 (create): body-file is refused: apply files take text inline",
		},
		{
			"body-append-file", "", valid + "- op: update\n  id: a\n  body-append-file: \"-\"\n", 2,
			"batch.yaml:6: operation 2 (update): body-append-file is refused: apply files take text inline",
		},
		{
			"missing title", "", valid + "- op: create\n  id: a\n", 2,
			"batch.yaml:4: operation 2 (create): missing required key title",
		},
		{
			"missing reason", "", valid + "- op: close\n  id: a\n", 2,
			"batch.yaml:4: operation 2 (close): missing required key reason",
		},
		{
			"missing dep", "", valid + "- op: dep add\n  id: a\n", 2,
			"batch.yaml:4: operation 2 (dep add): missing required key dep",
		},
		{
			"missing id", "", valid + "- op: log\n  text: b\n", 2,
			"batch.yaml:4: operation 2 (log): missing required key id",
		},
		{
			"title a mapping", "", valid + "- op: create\n  id: a\n  title:\n    b: c\n", 2,
			"batch.yaml:7: operation 2 (create): title is a mapping; give a string",
		},
		{
			"priority not an integer", "", valid + "- op: update\n  id: a\n  priority: high\n", 2,
			`batch.yaml:6: operation 2 (update): priority is "high"; give an integer`,
		},
		{
			"standing not true or false", "", valid + "- op: update\n  id: a\n  standing: yes\n", 2,
			`batch.yaml:6: operation 2 (update): standing is "yes"; give true or false`,
		},
		{
			"label a mapping", "", valid + "- op: create\n  id: a\n  title: b\n  label: {c: d}\n", 2,
			"batch.yaml:7: operation 2 (create): label is a mapping; give a string or a list of strings",
		},
		{
			"label item a list", "", valid + "- op: create\n  id: a\n  title: b\n  label: [c, [d]]\n", 2,
			"batch.yaml:7: operation 2 (create): label is a list; give a string",
		},
		{
			"an alias", "", valid + "- op: create\n  id: a\n  title: &t b\n  why: *t\n", 2,
			"batch.yaml:7: operation 2 (create): why is an alias; give a string",
		},
		{
			"a repeated key", "", valid + "- op: log\n  id: a\n  text: b\n  text: c\n", 2,
			"batch.yaml:7: operation 2 (log): text given twice; give it once",
		},
		{
			"standing with no-standing", "", valid + "- op: update\n  id: a\n  standing: true\n  no-standing: true\n", 2,
			"batch.yaml:7: operation 2 (update): standing and no-standing cannot be combined",
		},
		{
			"update with only id", "", valid + "- op: update\n  id: a\n", 2,
			"batch.yaml:4: operation 2 (update): update needs at least one flag that changes the issue",
		},
		{
			"update with only id and if-match", "", valid + "- op: update\n  id: a\n  if-match: e1\n", 2,
			"batch.yaml:4: operation 2 (update): update needs at least one flag that changes the issue",
		},
		{
			"uneven replacement pairs", "", valid + "- op: update\n  id: a\n  body-replace-old: [x, y]\n  body-replace-new: z\n", 2,
			"batch.yaml:4: operation 2 (update): 2 body-replace-old and 1 body-replace-new given; give them in pairs",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileName := test.fileName
			if fileName == "" {
				fileName = testFileName
			}

			operations, err := batch.Parse(fileName, []byte(test.data))

			var refusal *batch.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("Parse = %v, %v; want a *batch.Error", operations, err)
			}
			if refusal.Position != test.wantPosition || err.Error() != test.wantError {
				t.Errorf("Parse error = position %d %q, want position %d %q",
					refusal.Position, err.Error(), test.wantPosition, test.wantError)
			}
		})
	}
}
