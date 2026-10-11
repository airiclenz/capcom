package edit_test

import (
	"errors"
	"regexp"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/edit"
)

func TestLabels_AddsMissingThenRemoves(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		add    []string
		remove []string
		want   []string
	}{
		{name: "add to none", add: []string{"auth"}, want: []string{"auth"}},
		{name: "add keeps order and skips present", labels: []string{"auth"}, add: []string{"ui", "auth"}, want: []string{"auth", "ui"}},
		{name: "remove present", labels: []string{"auth", "ui"}, remove: []string{"auth"}, want: []string{"ui"}},
		{name: "remove absent", labels: []string{"ui"}, remove: []string{"auth"}, want: []string{"ui"}},
		{name: "remove last", labels: []string{"ui"}, remove: []string{"ui"}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := slices.Clone(tc.labels)

			merged := edit.Labels(original, tc.add, tc.remove)

			if !slices.Equal(merged, tc.want) || (tc.want == nil && merged != nil) {
				t.Errorf("labels = %#v, want %#v", merged, tc.want)
			}
			if !slices.Equal(original, tc.labels) {
				t.Errorf("input labels changed to %#v", original)
			}
		})
	}
}

func TestLabels_EmptyAdd_Dropped(t *testing.T) {
	add := []string{"x", "", "y"}

	merged := edit.Labels(nil, add, nil)

	if want := []string{"x", "y"}; !slices.Equal(merged, want) {
		t.Errorf("labels = %#v, want %#v", merged, want)
	}
}

func TestAppendBody_AddsBlockAfterBlankLine(t *testing.T) {
	cases := []struct {
		name string
		body string
		text string
		want string
	}{
		{name: "empty body", body: "", text: "First.", want: "\nFirst.\n"},
		{name: "after paragraph", body: "\nIntro.\n", text: "More.\n", want: "\nIntro.\n\nMore.\n"},
		{name: "no final newline", body: "\nIntro.", text: "More.", want: "\nIntro.\n\nMore.\n"},
		{name: "literal at sign", body: "\nIntro.\n", text: "@alice", want: "\nIntro.\n\n@alice\n"},
		{name: "crlf text", body: "\nIntro.\n", text: "a\r\nb\r\n", want: "\nIntro.\n\na\nb\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := edit.AppendBody(tc.body, tc.text)

			if body != tc.want {
				t.Errorf("body = %q, want %q", body, tc.want)
			}
		})
	}
}

func TestReplaceBody_OneMatch_Replaces(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		old         string
		replacement string
		want        string
	}{
		{name: "plain", body: "\nThe old text stays old.\n", old: "The old", replacement: "The new", want: "\nThe new text stays old.\n"},
		{name: "crlf", body: "\nThe old\ntext stays old.\n", old: "The old\r\ntext", replacement: "A\r\nB", want: "\nA\nB stays old.\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := edit.ReplaceBody(tc.body, tc.old, tc.replacement)

			if err != nil || body != tc.want {
				t.Errorf("body = %q, err = %v, want %q", body, err, tc.want)
			}
		})
	}
}

func TestReplaceBody_NotExactlyOnce_Fails(t *testing.T) {
	for _, old := range []string{"absent", "old", ""} {
		t.Run(old, func(t *testing.T) {
			_, err := edit.ReplaceBody("\nThe old text stays old.\n", old, "new")

			if !errors.Is(err, edit.ErrReplaceMatch) {
				t.Errorf("err = %v, want ErrReplaceMatch", err)
			}
		})
	}
}

func TestETag_HashesContent(t *testing.T) {
	hexDigits := regexp.MustCompile(`^[0-9a-f]{16}$`)
	first := []byte("---\nid: first\n---\n\nBody.\n")
	second := []byte("---\nid: second\n---\n\nBody.\n")

	firstTag := edit.ETag(first)

	if !hexDigits.MatchString(firstTag) {
		t.Errorf("ETag = %q, want 16 lowercase hex digits", firstTag)
	}
	if again := edit.ETag(slices.Clone(first)); again != firstTag {
		t.Errorf("ETag of equal bytes = %q, want %q", again, firstTag)
	}
	if other := edit.ETag(second); other == firstTag {
		t.Errorf("ETag of different files are both %q", other)
	}
	if empty := edit.ETag(nil); empty != "cbf29ce484222325" {
		t.Errorf("ETag(nil) = %q, want the FNV-1a 64 offset basis", empty)
	}
}
