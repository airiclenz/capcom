package frontmatter_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/frontmatter"
)

// encoded is the frontmatter a test writes and reads back: title and reason as text, labels as a
// list.
type encoded struct {
	title  string
	labels []string
	reason string
}

func (v encoded) encode() []byte {
	var encoder frontmatter.Encoder
	encoder.Text("title", v.title)
	encoder.List("labels", v.labels)
	encoder.Text("reason", v.reason)
	return encoder.File("")
}

func decodeEncoded(t *testing.T, data []byte) encoded {
	t.Helper()
	var decoded encoded
	_, err := frontmatter.Decode(testPath, data, issueKind, func(field frontmatter.Field) error {
		var err error
		switch field.Key {
		case "title":
			decoded.title, err = field.Text()
		case "reason":
			decoded.reason, err = field.Text()
		case "labels":
			decoded.labels, _, err = field.List()
		}
		return err
	})
	if err != nil {
		t.Fatalf("Decode: %v\n%s", err, data)
	}
	return decoded
}

func TestEncoder_TrickyText_RoundTrips(t *testing.T) {
	values := []string{
		"123", "1.5", "true", "yes", "null", "~", "2026-10-11",
		"a: b", "a:b", "a #b", "#x", "- x", "-x", "? x", "&anchor", "*alias", "!tag", "%x", "@x",
		"`x`", "'quoted'", `"quoted"`, `back\slash`, " leading", "trailing ", "multi\nline",
		"tab\there", "bell\a", "line\u2028separator", "[x]", "{x}", "x, y", "x]", "ünïcode ✓",
		"plain words", "non\uFFFEcharacter", "non\uFFFFcharacter", "\uFFFE", "\uFFFF",
	}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			original := encoded{title: value, labels: []string{value, "z"}, reason: value}

			serialized := original.encode()
			parsed := decodeEncoded(t, serialized)

			if parsed.title != value || parsed.reason != value || !slices.Equal(parsed.labels, original.labels) {
				t.Errorf("round trip of %q gave title %q, reason %q, labels %q from:\n%s",
					value, parsed.title, parsed.reason, parsed.labels, serialized)
			}
			if again := parsed.encode(); !bytes.Equal(again, serialized) {
				t.Errorf("second encoding differs:\n%s\nvs\n%s", again, serialized)
			}
		})
	}
}

func TestEncoder_PlainText_IsUnquoted(t *testing.T) {
	original := encoded{title: "Session refresh races the login timeout", labels: []string{"apogee-kk0.4"}}

	serialized := string(original.encode())

	if !strings.Contains(serialized, "\ntitle: Session refresh races the login timeout\n") ||
		!strings.Contains(serialized, "\nlabels: [apogee-kk0.4]\n") {
		t.Errorf("Encoder quoted plain text:\n%s", serialized)
	}
}

func TestEncoder_UnsafeText_IsDoubleQuotedOnOneLine(t *testing.T) {
	original := encoded{title: "a: b\nsecond line", labels: []string{"x, y"}}

	serialized := string(original.encode())

	if !strings.Contains(serialized, "\ntitle: \"a: b\\nsecond line\"\n") ||
		!strings.Contains(serialized, "\nlabels: [\"x, y\"]\n") {
		t.Errorf("Encoder did not double-quote on one line:\n%s", serialized)
	}
}

func TestEncoder_LongText_StaysOnOneLine(t *testing.T) {
	title := strings.Repeat("a long title word ", 12) + "end"

	serialized := string(encoded{title: title}.encode())

	if !strings.Contains(serialized, "\ntitle: "+title+"\n") {
		t.Errorf("long title was not written on one line:\n%s", serialized)
	}
}

func TestEncoder_File_WritesKeysInOrderThenBody(t *testing.T) {
	var encoder frontmatter.Encoder
	encoder.Text("title", "Login")
	encoder.Raw("priority", "0")
	encoder.List("labels", []string{"a", "b"})
	encoder.Raw("created", "2026-10-11T09:30:00Z")

	file := string(encoder.File("Body text.\n"))

	want := "---\ntitle: Login\npriority: 0\nlabels: [a, b]\ncreated: 2026-10-11T09:30:00Z\n---\nBody text.\n"
	if file != want {
		t.Errorf("File = %q, want %q", file, want)
	}
}

func TestEncoder_EmptyValues_AreLeftOut(t *testing.T) {
	var encoder frontmatter.Encoder
	encoder.Text("title", "")
	encoder.List("labels", nil)
	encoder.List("aliases", []string{})
	encoder.Raw("closed", "")

	file := string(encoder.File(""))

	if file != "---\n---\n" {
		t.Errorf("File = %q, want an empty frontmatter", file)
	}
}
