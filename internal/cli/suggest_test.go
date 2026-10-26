package cli

import (
	"slices"
	"strings"
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

func TestClosestNames_SuggestsCloseNamesInRankOrder(t *testing.T) {
	names := []string{
		"adr-0004", "go-tests", "log", "loge", "login-flow", "login-timeout-race", "logon", "logs", "spec-0004", "standing-parents",
	}
	cases := []struct {
		name  string
		typed string
		want  []string
	}{
		{"one edit away", "standng-parents", []string{"standing-parents"}},
		{"distance before shared words", "login-flaw", []string{"login-flow", "login-timeout-race"}},
		{"a shared word only", "login-redirect", []string{"login-flow", "login-timeout-race"}},
		{"more shared words first", "login-timeout-retry", []string{"login-timeout-race", "login-flow"}},
		{"at most three, then by name", "logx", []string{"log", "loge", "logs"}},
		{"a word of fewer than three letters is not shared", "go-qqqqq", nil},
		{"digits alone are no shared word", "rfc-0004", nil},
		{"nothing close", "zzzzzz", nil},
		{"empty", "", nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := closestNames(testCase.typed, names)

			if !slices.Equal(got, testCase.want) {
				t.Errorf("closestNames(%q) = %v, want %v", testCase.typed, got, testCase.want)
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

	want := "capcom: id \"git-fixturez\" not found; did you mean git-fixtures?\nhint: capcom search <text>\n"
	if exitCode != ExitNotFound || stderr != want {
		t.Errorf("know mv exit = %d stderr = %q, want %q", exitCode, stderr, want)
	}
}
