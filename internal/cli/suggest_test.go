package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestEditDistance_CountsEditsIgnoringCase(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"status", "status", 0},
		{"STATUS", "status", 0},
		{"statsu", "status", 2},
		{"jsno", "json", 2},
		{"", "json", 4},
		{"tre", "tree", 1},
		{"zzzzqq", "json", 6},
	}
	for _, testCase := range cases {
		t.Run(testCase.left+"/"+testCase.right, func(t *testing.T) {
			got := editDistance(testCase.left, testCase.right)

			if got != testCase.want {
				t.Errorf("editDistance(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
			}
		})
	}
}

// newSuggestionCommand returns a command with the own flags stay, star, status and a hidden
// secret, below a parent whose persistent flag json it inherits.
func newSuggestionCommand() *cobra.Command {
	parent := &cobra.Command{Use: "parent"}
	parent.PersistentFlags().Bool("json", false, "")
	child := &cobra.Command{Use: "child", Run: func(*cobra.Command, []string) {}}
	child.Flags().Bool("stay", false, "")
	child.Flags().Bool("star", false, "")
	child.Flags().String("status", "", "")
	child.Flags().Bool("secret", false, "")
	if err := child.Flags().MarkHidden("secret"); err != nil {
		panic(err)
	}
	parent.AddCommand(child)
	return child
}

func TestClosestFlagName_PicksTheClosestVisibleFlag(t *testing.T) {
	cases := map[string]string{
		"statsu":  "status",
		"jsno":    "json",
		"sta":     "star",
		"STATU":   "status",
		"secre":   "",
		"zzzzqq":  "",
		"":        "",
		"statusX": "status",
	}
	for typed, want := range cases {
		t.Run(typed, func(t *testing.T) {
			command := newSuggestionCommand()

			got := closestFlagName(command, typed)

			if got != want {
				t.Errorf("closestFlagName(%q) = %q, want %q", typed, got, want)
			}
		})
	}
}
