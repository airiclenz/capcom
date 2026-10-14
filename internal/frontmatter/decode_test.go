package frontmatter_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/frontmatter"
)

const testPath = ".capcom/knowledge/a.md"

var (
	issueKind = frontmatter.Kind{
		Noun: "an issue",
		Keys: []string{
			"title", "status", "type", "priority", "labels", "deps", "related", "plan", "aliases",
			"created", "closed", "reason",
		},
	}
	noteKind = frontmatter.Kind{
		Noun: "a note",
		Keys: []string{"title", "labels", "refs", "retired", "superseded-by", "aliases"},
	}
)

// listKeys are the keys the test callback reads as lists; priority it reads as a whole number and
// every other key as text.
var listKeys = []string{"labels", "deps", "related", "aliases", "refs"}

// decoded is what the test callback collected from the fields Decode handed it.
type decoded struct {
	texts     map[string]string
	lists     map[string][]string
	listLines map[string][]int
}

// decode runs Decode with a callback that reads every field the way a kind would.
func decode(
	t *testing.T,
	kind frontmatter.Kind,
	content string,
) (frontmatter.Document, decoded, error) {
	t.Helper()
	got := decoded{
		texts:     map[string]string{},
		lists:     map[string][]string{},
		listLines: map[string][]int{},
	}
	apply := func(field frontmatter.Field) error {
		switch {
		case field.Key == "priority":
			text, isScalar := field.Scalar()
			if _, err := strconv.Atoi(text); !isScalar || err != nil {
				return field.Errorf("priority: expected a whole number from 0 to 4")
			}
		case slices.Contains(listKeys, field.Key):
			entries, lines, err := field.List()
			if err != nil {
				return err
			}
			got.lists[field.Key], got.listLines[field.Key] = entries, lines
		default:
			text, err := field.Text()
			if err != nil {
				return err
			}
			got.texts[field.Key] = text
		}
		return nil
	}
	document, err := frontmatter.Decode(testPath, []byte(content), kind, apply)
	return document, got, err
}

func TestDecode_Failures_CiteFileLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		kind    frontmatter.Kind
		content string
		want    string
	}{
		{
			name:    "no opening fence, issue",
			kind:    issueKind,
			content: "title: T\n",
			want:    testPath + ":1: an issue file must start with a --- line",
		},
		{
			name:    "no opening fence, note",
			kind:    noteKind,
			content: "title: T\n",
			want:    testPath + ":1: a note file must start with a --- line",
		},
		{
			name:    "never closed",
			kind:    noteKind,
			content: "---\ntitle: T\n",
			want:    testPath + ":1: the frontmatter opened here is never closed by a --- line",
		},
		{
			name:    "unknown key, issue",
			kind:    issueKind,
			content: "---\ntitle: T\nstatus: open\nupdated: 2026-10-11T09:30:00Z\n---\n",
			want: testPath + `:4: unknown key "updated"; an issue has title, status, type, ` +
				`priority, labels, deps, related, plan, aliases, created, closed and reason`,
		},
		{
			name:    "unknown key, note",
			kind:    noteKind,
			content: "---\ntitle: T\nstatus: open\n---\n",
			want:    testPath + `:3: unknown key "status"; a note has title, labels, refs, retired, superseded-by and aliases`,
		},
		{
			name:    "duplicate key",
			kind:    noteKind,
			content: "---\ntitle: T\ntitle: U\n---\n",
			want:    testPath + `:3: duplicate key "title"`,
		},
		{
			name:    "frontmatter not a mapping",
			kind:    noteKind,
			content: "---\n- a\n---\n",
			want:    testPath + ":2: the frontmatter must be a mapping of keys",
		},
		{
			name:    "title not text",
			kind:    noteKind,
			content: "---\ntitle: [a]\n---\n",
			want:    testPath + ":2: title: expected text",
		},
		{
			name:    "labels not a list",
			kind:    noteKind,
			content: "---\ntitle: T\nlabels: ui\n---\n",
			want:    testPath + ":3: labels: expected a list such as [a, b]",
		},
		{
			name:    "empty flow entry",
			kind:    issueKind,
			content: "---\ndeps: [a, '']\n---\n",
			want:    testPath + ":2: deps: every entry must be non-empty text",
		},
		{
			name:    "empty block entry cites its own line",
			kind:    noteKind,
			content: "---\ntitle: T\nrefs:\n  - a.go\n  - ''\n---\n",
			want:    testPath + ":5: refs: every entry must be non-empty text",
		},
		{
			name:    "callback failure at the value line",
			kind:    issueKind,
			content: "---\ntitle: T\npriority: high\n---\n",
			want:    testPath + ":3: priority: expected a whole number from 0 to 4",
		},
		{
			name:    "content after a document end marker",
			kind:    noteKind,
			content: "---\ntitle: a\n...\nbogus: 1\n---\n",
			want:    testPath + ":3: invalid YAML frontmatter: did not find expected <document start>",
		},
		{
			name:    "second document",
			kind:    noteKind,
			content: "---\ntitle: a\n--- \nbogus: 1\n---\n",
			want:    testPath + ":3: only one YAML document is allowed",
		},
		{
			name:    "conflict marker overrides the failure it causes",
			kind:    noteKind,
			content: "---\n<<<<<<< HEAD\ntitle: A\n=======\ntitle: B\n>>>>>>> other\n---\n",
			want: testPath + `:2: git conflict marker "<<<<<<< HEAD"; ` +
				"resolve the conflict and delete the markers",
		},
		{
			name:    "conflict marker in the body leaves the failure alone",
			kind:    noteKind,
			content: "---\ntitle: T\ntitle: U\n---\n<<<<<<< HEAD\n",
			want:    testPath + `:3: duplicate key "title"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := decode(t, tc.kind, tc.content)

			var decodeError *frontmatter.Error
			if !errors.As(err, &decodeError) || decodeError.Error() != tc.want {
				t.Errorf("Decode error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestDecode_YAMLSyntaxError_CitesFrontmatterLine(t *testing.T) {
	t.Parallel()
	content := "---\ntitle: T\nlabels: [open\nrefs: [a]\n---\n"

	_, _, err := decode(t, noteKind, content)

	// yaml.v3 counts parser errors (unlike scanner errors) from 0, so the cited line may sit one
	// above the fault; it must still fall inside the frontmatter.
	var decodeError *frontmatter.Error
	if !errors.As(err, &decodeError) {
		t.Fatalf("Decode error = %v, want a *frontmatter.Error", err)
	}
	if decodeError.Line < 2 || decodeError.Line > 4 ||
		!strings.HasPrefix(decodeError.Message, "invalid YAML frontmatter: ") {
		t.Errorf("Decode error = %v, want an invalid-YAML error on lines 2 to 4", decodeError)
	}
}

func TestDecode_ListEntryLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    []int
	}{
		{name: "flow style", content: "---\ntitle: T\nrefs: [a.go, b.go]\n---\n", want: []int{3, 3}},
		{
			name:    "block style",
			content: "---\ntitle: T\nrefs:\n  - a.go\n\n  - b.go\n---\n",
			want:    []int{4, 6},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, got, err := decode(t, noteKind, tc.content)

			if err != nil {
				t.Fatalf("Decode error = %v", err)
			}
			if !slices.Equal(got.lists["refs"], []string{"a.go", "b.go"}) ||
				!slices.Equal(got.listLines["refs"], tc.want) {
				t.Errorf(
					"refs = %v at lines %v, want [a.go b.go] at %v",
					got.lists["refs"],
					got.listLines["refs"],
					tc.want,
				)
			}
		})
	}
}

func TestDecode_CRLF_ReadsAsLF(t *testing.T) {
	t.Parallel()
	content := "---\r\ntitle: T\r\nlabels: [a, b]\r\n---\r\nline one\r\nline two\r\n"

	document, got, err := decode(t, noteKind, content)

	if err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if document.Body != "line one\nline two\n" || got.texts["title"] != "T" ||
		!slices.Equal(got.lists["labels"], []string{"a", "b"}) {
		t.Errorf(
			"Decode = body %q, title %q, labels %v",
			document.Body,
			got.texts["title"],
			got.lists["labels"],
		)
	}
}

func TestDecode_KeyLinesAndBody(t *testing.T) {
	t.Parallel()
	content := "---\ntitle: T\n\nlabels: [a]\n---\nbody\n---\nmore\n"

	document, _, err := decode(t, noteKind, content)

	if err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if document.Lines["title"] != 2 || document.Lines["labels"] != 4 || len(document.Lines) != 2 {
		t.Errorf("Lines = %v, want title 2 and labels 4", document.Lines)
	}
	if document.Body != "body\n---\nmore\n" {
		t.Errorf("Body = %q, want the content after the first closing fence", document.Body)
	}
}

func TestDecode_EmptyValues_ReadAsZero(t *testing.T) {
	t.Parallel()
	content := "---\ntitle:\nlabels: []\naliases:\n---\n"

	_, got, err := decode(t, noteKind, content)

	if err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if got.texts["title"] != "" || got.lists["labels"] != nil || got.lists["aliases"] != nil {
		t.Errorf(
			"Decode = title %q, labels %v, aliases %v, want all empty",
			got.texts["title"],
			got.lists["labels"],
			got.lists["aliases"],
		)
	}
}

func TestDecode_NoKeys_CallsNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		body    string
	}{
		{name: "empty frontmatter", content: "---\n---\nbody\n", body: "body\n"},
		{name: "comment only", content: "---\n# nothing\n---\n", body: ""},
		{name: "closing fence at end of file", content: "---\n---", body: ""},
		{name: "closing fence with a lone CR at end of file", content: "---\n---\r", body: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			isCalled := false
			apply := func(frontmatter.Field) error {
				isCalled = true
				return nil
			}

			document, err := frontmatter.Decode(testPath, []byte(tc.content), noteKind, apply)

			isEmpty := document.Lines != nil && len(document.Lines) == 0 && document.Body == tc.body
			if err != nil || isCalled || !isEmpty {
				t.Errorf(
					"Decode = %+v, %v, called %v; want body %q, no lines and no call",
					document,
					err,
					isCalled,
					tc.body,
				)
			}
		})
	}
}

func TestField_ScalarAndIsEmpty(t *testing.T) {
	t.Parallel()
	content := "---\ntitle: 'quoted: text'\nstatus:\nlabels: [a]\n---\n"
	kind := frontmatter.Kind{Noun: "an issue", Keys: []string{"title", "status", "labels"}}
	type reading struct {
		text              string
		isScalar, isEmpty bool
	}
	got := map[string]reading{}

	_, err := frontmatter.Decode(testPath, []byte(content), kind, func(field frontmatter.Field) error {
		text, isScalar := field.Scalar()
		got[field.Key] = reading{text: text, isScalar: isScalar, isEmpty: field.IsEmpty()}
		return nil
	})

	want := map[string]reading{
		"title":  {text: "quoted: text", isScalar: true},
		"status": {isScalar: true, isEmpty: true},
		"labels": {},
	}
	if err != nil || len(got) != len(want) {
		t.Fatalf("Decode = %v, %v", got, err)
	}
	for key, reading := range want {
		if got[key] != reading {
			t.Errorf("%s = %+v, want %+v", key, got[key], reading)
		}
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()

	content := []byte("---\r\ntitle: T\r\n---\r\nbody\r\n")

	front, body, err := frontmatter.Split(testPath, content, "a note")

	if err != nil || front != "title: T\n" || body != "body\n" {
		t.Errorf("Split = %q, %q, %v; want the frontmatter and body with LF endings", front, body, err)
	}
}

func TestSplit_Fences(t *testing.T) {
	t.Parallel()
	const neverClosed = testPath + ":1: the frontmatter opened here is never closed by a --- line"
	const noOpening = testPath + ":1: a note file must start with a --- line"
	cases := []struct {
		name    string
		content string
		front   string
		body    string
		wantErr string
	}{
		{
			name:    "CRLF fences",
			content: "---\r\ntitle: T\r\n---\r\nbody\r\n",
			front:   "title: T\n",
			body:    "body\n",
		},
		{name: "byte order mark before the opening fence", content: "\ufeff---\ntitle: T\n---\n", wantErr: noOpening},
		{name: "trailing space after the closing fence", content: "---\ntitle: T\n--- \n", wantErr: neverClosed},
		{name: "missing closing fence", content: "---\ntitle: T\n", wantErr: neverClosed},
		{name: "lone opening fence", content: "---\n", wantErr: neverClosed},
		{name: "closing fence with a lone CR at end of file", content: "---\ntitle: T\n---\r", front: "title: T\n"},
		{
			name:    "fence lines in the body are kept",
			content: "---\ntitle: T\n---\r\nabove\r\n---\r\nbelow\n",
			front:   "title: T\n",
			body:    "above\n---\nbelow\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			front, body, err := frontmatter.Split(testPath, []byte(tc.content), "a note")

			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if gotErr != tc.wantErr || front != tc.front || body != tc.body {
				t.Errorf(
					"Split = %q, %q, %q; want %q, %q, %q",
					front,
					body,
					gotErr,
					tc.front,
					tc.body,
					tc.wantErr,
				)
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  frontmatter.Error
		want string
	}{
		{
			name: "with a line",
			err:  frontmatter.Error{Path: "a.md", Line: 3, Message: "m"},
			want: "a.md:3: m",
		},
		{name: "without a line", err: frontmatter.Error{Path: "a.md", Message: "m"}, want: "a.md: m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.err.Error()

			if got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
