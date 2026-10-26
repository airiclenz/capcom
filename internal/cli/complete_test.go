package cli

import (
	"slices"
	"strings"
	"testing"
)

// completeDirective is the directive line ending the output of every id completion:
// ShellCompDirectiveKeepOrder (32) together with ShellCompDirectiveNoFileComp (4).
const completeDirective = ":36"

// newCompleteStore creates a store holding open and closed Issues, one with an alias, and two Notes.
func newCompleteStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"standing-parents.md": readIssue("Standing parents", "open", "standing: true\n", ""),
		"stale-docs.md":       readIssue("Stale docs", "open", "aliases: [old-docs]\n", ""),
		"auth-flow.md":        readIssue("Auth flow", "open", "", ""),
		"shipped-login.md":    readIssue("Shipped login", "done", "", ""),
		"abandoned-cache.md":  readIssue("Abandoned cache", "dropped", "", ""),
	})
	writeNoteFiles(t, root, map[string]string{
		"store-layout.md":  "---\ntitle: Store layout\n---\n\nFiles in .capcom.\n",
		"release-steps.md": "---\ntitle: Release steps\nretired: true\n---\n\nRetired.\n",
	})
	return root
}

// runComplete runs capcom __complete with args in dir and returns its output lines: one candidate
// per line, then the directive line.
func runComplete(t *testing.T, dir string, args ...string) []string {
	t.Helper()
	stdout, stderr, exitCode := runCLI(t, dir, append([]string{"__complete"}, args...)...)
	if exitCode != ExitOK {
		t.Fatalf("__complete %v exit = %d\nstdout: %s\nstderr: %s", args, exitCode, stdout, stderr)
	}
	return strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
}

func TestComplete_IssueArguments_ListMatchingIDsWithTitles(t *testing.T) {
	root := newCompleteStore(t)
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"show prefix", []string{"show", "stand"}, []string{"standing-parents\tStanding parents"}},
		{"show alias", []string{"show", "old"}, []string{"old-docs\tStale docs"}},
		{"show second argument", []string{"show", "auth-flow", "st"},
			[]string{"stale-docs\tStale docs", "standing-parents\tStanding parents"}},
		{"context prefix", []string{"context", "au"}, []string{"auth-flow\tAuth flow"}},
		{"dep add second argument", []string{"dep", "add", "auth-flow", "sh"}, []string{"shipped-login\tShipped login"}},
		{"related rm second argument", []string{"related", "rm", "auth-flow", "au"}, []string{"auth-flow\tAuth flow"}},
		{"mv first argument", []string{"mv", "ab"}, []string{"abandoned-cache\tAbandoned cache"}},
		{"ui issues id", []string{"ui", "issues", "stan"}, []string{"standing-parents\tStanding parents"}},
		{"ui knowledge id", []string{"ui", "knowledge", "st"}, []string{"store-layout\tStore layout"}},
		{"ui knowledge retired id", []string{"ui", "knowledge", "rel"}, []string{"release-steps\tRelease steps"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runComplete(t, root, test.args...)

			want := append(slices.Clone(test.want), completeDirective)
			if !slices.Equal(got, want) {
				t.Errorf("__complete %v =\n%q\nwant\n%q", test.args, got, want)
			}
		})
	}
}

func TestComplete_EmptyPrefix_ListsOpenIssuesThenClosedEachByID(t *testing.T) {
	root := newCompleteStore(t)

	got := runComplete(t, root, "close", "")

	want := []string{
		"auth-flow\tAuth flow",
		"stale-docs\tStale docs",
		"old-docs\tStale docs",
		"standing-parents\tStanding parents",
		"abandoned-cache\tAbandoned cache",
		"shipped-login\tShipped login",
		completeDirective,
	}
	if !slices.Equal(got, want) {
		t.Errorf("__complete close \"\" =\n%q\nwant\n%q", got, want)
	}
}

func TestComplete_ArgumentPastTheIDs_HasNoCandidates(t *testing.T) {
	root := newCompleteStore(t)
	tests := []struct {
		name string
		args []string
	}{
		{"mv new id", []string{"mv", "auth-flow", ""}},
		{"log text", []string{"log", "auth-flow", ""}},
		{"update second argument", []string{"update", "auth-flow", ""}},
		{"ui unknown tab", []string{"ui", "nope", ""}},
		{"ui third argument", []string{"ui", "issues", "auth-flow", ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runComplete(t, root, test.args...)

			if want := []string{completeDirective}; !slices.Equal(got, want) {
				t.Errorf("__complete %v = %q, want %q", test.args, got, want)
			}
		})
	}
}

func TestComplete_UITab_ListsTabNamesInTabBarOrder(t *testing.T) {
	root := newCompleteStore(t)

	got := runComplete(t, root, "ui", "")

	if want := []string{"issues", "ready", "knowledge", completeDirective}; !slices.Equal(got, want) {
		t.Errorf("__complete ui \"\" = %q, want %q", got, want)
	}
}

func TestComplete_OutsideAStore_HasNoCandidatesAndNoFiles(t *testing.T) {
	tests := []struct {
		name string
		dir  func(t *testing.T) string
	}{
		{"outside a git repository", func(t *testing.T) string { return t.TempDir() }},
		{"git repository without a store", newGitRepo},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := test.dir(t)

			got := runComplete(t, dir, "show", "")

			if want := []string{completeDirective}; !slices.Equal(got, want) {
				t.Errorf("__complete show \"\" = %q, want %q", got, want)
			}
		})
	}
}
