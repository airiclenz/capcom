package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

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

func TestSuggest_UpdateUnknownID_NamesTheClosestIssue(t *testing.T) {
	root := newIssueStore(t, map[string]string{"standing-parents.md": readIssue("Standing parents", "open", "", "")})

	_, stderr, exitCode := runCLI(t, root, "update", "standng-parents", "--title", "Parents")

	want := "capcom: id \"standng-parents\" not found; did you mean standing-parents?\nhint: capcom search <text>\n"
	if exitCode != ExitNotFound || stderr != want {
		t.Errorf("update exit = %d stderr = %q, want %q", exitCode, stderr, want)
	}
}

func TestSuggest_CloseUnknownID_NamesTheClosestIssue(t *testing.T) {
	root := newIssueStore(t, map[string]string{"standing-parents.md": readIssue("Standing parents", "open", "", "")})

	_, stderr, exitCode := runCLI(t, root, "close", "standng-parents", "--reason", "x")

	if exitCode != ExitNotFound || !strings.HasSuffix(stderr, "; did you mean standing-parents?\nhint: capcom search <text>\n") {
		t.Errorf("close exit = %d stderr = %q, want standing-parents suggested", exitCode, stderr)
	}
}

func TestSuggest_DepAddUnknownDependency_NamesTheClosestIssue(t *testing.T) {
	root := newReadStore(t)

	_, stderr, exitCode := runCLI(t, root, "dep", "add", "release-checklist", "old-ideas")

	if exitCode != ExitNotFound || !strings.HasSuffix(stderr, "; did you mean old-idea?\nhint: capcom search <text>\n") {
		t.Errorf("dep add exit = %d stderr = %q, want old-idea suggested", exitCode, stderr)
	}
}

func TestSuggest_KnowMvUnknownID_NamesTheClosestNote(t *testing.T) {
	root := newKnowStore(t)

	_, stderr, exitCode := runCLI(t, root, "know", "mv", "git-fixturez", "git-setup")

	want := "capcom: id \"git-fixturez\" not found; did you mean git-fixtures?\nhint: capcom know search <text>\n"
	if exitCode != ExitNotFound || stderr != want {
		t.Errorf("know mv exit = %d stderr = %q, want %q", exitCode, stderr, want)
	}
}
