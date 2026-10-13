package frontmatter_test

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/airiclenz/capcom/internal/frontmatter"
)

// pair is the two-key node the schema tests decode into and encode from.
type pair struct {
	name  string
	count int
}

// pairSchema reads name as text and count, written always, as a whole number through Raw.
var pairSchema = frontmatter.NewSchema(
	"a pair",
	frontmatter.Text("name", func(node *pair) *string { return &node.name }),
	frontmatter.Raw(
		"count",
		func(value frontmatter.Field, node *pair) error {
			text, _ := value.Scalar()
			count, err := strconv.Atoi(text)
			if err != nil {
				return value.Errorf("count: expected a whole number")
			}
			node.count = count
			return nil
		},
		func(node pair, out *frontmatter.Encoder) { out.Raw("count", strconv.Itoa(node.count)) },
	),
)

func TestSchema_Encode_ThenDecode_RoundTrips(t *testing.T) {
	original := pair{name: "a: b", count: 3}
	const body = "Body text.\n"

	data := pairSchema.Encode(original, body)
	var decoded pair
	document, err := pairSchema.Decode(testPath, data, &decoded)

	if err != nil {
		t.Fatalf("Decode: %v\n%s", err, data)
	}
	if decoded != original || document.Body != body {
		t.Errorf("round trip = %+v, body %q; want %+v, body %q", decoded, document.Body, original, body)
	}
	const want = "---\nname: \"a: b\"\ncount: 3\n---\nBody text.\n"
	if string(data) != want {
		t.Errorf("Encode = %q, want %q", data, want)
	}
}

func TestSchema_Kind_NamesTheNounAndKeysInFieldOrder(t *testing.T) {
	kind := pairSchema.Kind()

	if kind.Noun != "a pair" || !slices.Equal(kind.Keys, []string{"name", "count"}) {
		t.Errorf("Kind = %+v, want noun %q and keys [name count]", kind, "a pair")
	}
}

func TestSchema_Decode_Failures_CiteFileLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    frontmatter.Error
	}{
		{
			name:    "unknown key names the kind's keys",
			content: "---\nname: x\ncolour: red\n---\n",
			want: frontmatter.Error{
				Path:    testPath,
				Line:    3,
				Message: `unknown key "colour"; a pair has name and count`,
			},
		},
		{
			name:    "raw decode error at its line",
			content: "---\nname: x\ncount: many\n---\n",
			want: frontmatter.Error{
				Path:    testPath,
				Line:    3,
				Message: "count: expected a whole number",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var node pair

			_, err := pairSchema.Decode(testPath, []byte(tc.content), &node)

			var got *frontmatter.Error
			if !errors.As(err, &got) {
				t.Fatalf("Decode error = %v, want *frontmatter.Error", err)
			}
			if *got != tc.want {
				t.Errorf("Decode error = %+v, want %+v", *got, tc.want)
			}
		})
	}
}
