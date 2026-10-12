package rawfm_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/rawfm"
)

// frontmatter is a closed issue frontmatter of six lines, so a body appended to it starts at line 7.
const frontmatter = "---\ntitle: Some issue\nstatus: open\ntype: task\npriority: 2\n---\n"

// logConflict is a Log body as a merge of two concurrent `capcom log` calls leaves it, from line 7
// on below frontmatter: its markers are at lines 12, 14 and 16.
const logConflict = "\nThe body.\n\n## Log\n\n" +
	"<<<<<<< HEAD\n- 2026-10-11 alice: reproduced it.\n=======\n" +
	"- 2026-10-11 bob: found the cause.\n>>>>>>> other-branch\n"

// markerLines returns the 1-based lines of markers, in their order.
func markerLines(markers []rawfm.ConflictMarker) []int {
	lines := make([]int, 0, len(markers))
	for _, marker := range markers {
		lines = append(lines, marker.Line)
	}
	return lines
}

func TestConflictMarkers_FileContent_ReturnsTheMarkerLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    []int
	}{
		{
			name:    "markers in the Log",
			content: frontmatter + logConflict,
			want:    []int{12, 14, 16},
		},
		{
			name:    "markers in the Log with CRLF line endings",
			content: strings.ReplaceAll(frontmatter+logConflict, "\n", "\r\n"),
			want:    []int{12, 14, 16},
		},
		{
			name: "diff3 markers with the base section",
			content: frontmatter +
				"\n<<<<<<< HEAD\n- mine\n||||||| base\n=======\n- theirs\n>>>>>>> other-branch\n",
			want: []int{8, 10, 11, 13},
		},
		{
			name:    "a setext heading underline alone",
			content: frontmatter + "\nA heading\n=======\n\nText.\n",
			want:    []int{},
		},
		{
			name:    "markers inside a backtick code fence",
			content: frontmatter + "\n```text\n<<<<<<< HEAD\n=======\n>>>>>>> other-branch\n```\n",
			want:    []int{},
		},
		{
			name: "markers inside a tilde code fence holding a shorter backtick fence",
			content: frontmatter +
				"\n~~~~\n<<<<<<< HEAD\n```\n=======\n>>>>>>> other-branch\n~~~~\n",
			want: []int{},
		},
		{
			name:    "markers after a closed code fence",
			content: frontmatter + "\n```\ncode\n```\n" + logConflict,
			want:    []int{16, 18, 20},
		},
		{
			name: "markers in the frontmatter",
			content: "---\ntitle: Some issue\n<<<<<<< HEAD\nstatus: done\n=======\n" +
				"status: active\n>>>>>>> other-branch\ntype: task\n---\n",
			want: []int{3, 5, 7},
		},
		{
			name:    "a code fence in a frontmatter that is never closed",
			content: "---\ntitle: Some issue\n```\n<<<<<<< HEAD\n=======\n>>>>>>> other-branch\n",
			want:    []int{4, 5, 6},
		},
		{
			name:    "markers in a file without frontmatter",
			content: "<<<<<<< HEAD\nmine\n=======\ntheirs\n>>>>>>> other-branch\n",
			want:    []int{1, 3, 5},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			markers := rawfm.ConflictMarkers([]byte(testCase.content))

			if got := markerLines(markers); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("ConflictMarkers lines = %v; want %v", got, testCase.want)
			}
		})
	}
}

func TestConflictMarkers_CRLFContent_TrimsTheCarriageReturnFromTheText(t *testing.T) {
	t.Parallel()
	content := strings.ReplaceAll(frontmatter+logConflict, "\n", "\r\n")

	markers := rawfm.ConflictMarkers([]byte(content))

	want := []rawfm.ConflictMarker{
		{Line: 12, Text: "<<<<<<< HEAD"},
		{Line: 14, Text: "======="},
		{Line: 16, Text: ">>>>>>> other-branch"},
	}
	if !reflect.DeepEqual(markers, want) {
		t.Errorf("ConflictMarkers = %+v; want %+v", markers, want)
	}
}

func TestConflictMessage_Marker_QuotesTheMarkerAndSaysHowToResolve(t *testing.T) {
	t.Parallel()
	marker := rawfm.ConflictMarker{Line: 12, Text: "<<<<<<< HEAD"}

	message := rawfm.ConflictMessage(marker)

	want := `git conflict marker "<<<<<<< HEAD"; resolve the conflict and delete the markers`
	if message != want {
		t.Errorf("ConflictMessage = %q; want %q", message, want)
	}
}
