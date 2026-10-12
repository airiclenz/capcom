package rawfm_test

import (
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/rawfm"
)

func TestAliases_RawFrontmatter_ReturnsTheListedAliases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "flow form",
			content: "---\ntitle: Some issue\naliases: [first-name, 'second-name', \"third-name\"]\n---\nBody.\n",
			want:    []string{"first-name", "second-name", "third-name"},
		},
		{
			name:    "block form",
			content: "---\ntitle: Some issue\naliases:\n  - first-name\n  - second-name\nstatus: open\n---\n",
			want:    []string{"first-name", "second-name"},
		},
		{
			name: "conflict marker before title",
			content: "---\n<<<<<<< HEAD\ntitle: Mine\n=======\ntitle: Theirs\n>>>>>>> other-branch\n" +
				"status: open\naliases: [first-name]\n---\n",
			want: []string{"first-name"},
		},
		{
			name: "aliases line on both sides of a conflict hunk",
			content: "---\ntitle: Some issue\n<<<<<<< HEAD\naliases: [first-name, mine-only]\n=======\n" +
				"aliases: [first-name, theirs-only]\n>>>>>>> other-branch\n---\n",
			want: []string{"first-name", "mine-only", "theirs-only"},
		},
		{
			name: "block list across a conflict hunk",
			content: "---\ntitle: Some issue\naliases:\n<<<<<<< HEAD\n  - mine-only\n=======\n  - theirs-only\n" +
				">>>>>>> other-branch\nstatus: open\n---\n",
			want: []string{"mine-only", "theirs-only"},
		},
		{
			name:    "flow list missing its closing bracket",
			content: "---\ntitle: Some issue\naliases: [first-name, second-name\n---\n",
			want:    []string{"first-name", "second-name"},
		},
		{
			name:    "CRLF line endings",
			content: "---\r\ntitle: Some issue\r\naliases:\r\n  - first-name\r\n---\r\n",
			want:    []string{"first-name"},
		},
		{
			name:    "frontmatter never closed",
			content: "---\ntitle: Some issue\naliases: [first-name]",
			want:    []string{"first-name"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := rawfm.Aliases([]byte(tc.content))

			if !slices.Equal(got, tc.want) {
				t.Errorf("Aliases = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAliases_NoAliasesKey_ReturnsNone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{name: "no key", content: "---\ntitle: Some issue\nstatus: open\n---\n"},
		{name: "key only in the body", content: "---\ntitle: Some issue\n---\naliases: [body-text]\n"},
		{name: "nested key", content: "---\ntitle: Some issue\nextra:\n  aliases: [nested-name]\n---\n"},
		{name: "no frontmatter", content: "aliases: [first-name]\n"},
		{name: "empty file", content: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := rawfm.Aliases([]byte(tc.content))

			if len(got) != 0 {
				t.Errorf("Aliases = %q, want none", got)
			}
		})
	}
}
