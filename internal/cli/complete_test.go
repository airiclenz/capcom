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

func TestComplete_NoteArguments_ListNotRetiredNoteIDsWithTitles(t *testing.T) {
	root := newCompleteStore(t)
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"know show empty prefix", []string{"know", "show", ""}, []string{"store-layout\tStore layout"}},
		{"know show retired", []string{"know", "show", "rel"}, nil},
		{"know update prefix", []string{"know", "update", "st"}, []string{"store-layout\tStore layout"}},
		{"know tree prefix", []string{"know", "tree", "st"}, []string{"store-layout\tStore layout"}},
		{"know mv first argument", []string{"know", "mv", "st"}, []string{"store-layout\tStore layout"}},
		{"know mv new id", []string{"know", "mv", "store-layout", ""}, nil},
		{"know rm prefix", []string{"know", "rm", "st"}, []string{"store-layout\tStore layout"}},
		{"know show second argument", []string{"know", "show", "store-layout", ""}, nil},
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

func TestComplete_FlagValues_ListWhatTheFlagAccepts(t *testing.T) {
	root := newCompleteStore(t)
	openStatuses := []string{"open\ttodo", "active\tdoing", "parked\twaiting"}
	types := []string{"bug", "feature", "task", "chore"}
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"know add parent", []string{"know", "add", "--parent", "st"}, []string{"store-layout\tStore layout"}},
		{"know add parent retired", []string{"know", "add", "--parent", "rel"}, nil},
		{"know mv parent", []string{"know", "mv", "store-layout", "--parent", "st"}, []string{"store-layout\tStore layout"}},
		{"know update add superseded by", []string{"know", "update", "store-layout", "--add-superseded-by", "st"},
			[]string{"store-layout\tStore layout"}},
		{"know update remove superseded by", []string{"know", "update", "store-layout", "--remove-superseded-by", "st"},
			[]string{"store-layout\tStore layout"}},
		{"mv parent", []string{"mv", "auth-flow", "--parent", "st"},
			[]string{"stale-docs\tStale docs", "standing-parents\tStanding parents"}},
		{"create parent", []string{"create", "--parent", "au"}, []string{"auth-flow\tAuth flow"}},
		{"list parent", []string{"list", "--parent", "sh"}, []string{"shipped-login\tShipped login"}},
		{"create dep", []string{"create", "--dep", "ab"}, []string{"abandoned-cache\tAbandoned cache"}},
		{"create status", []string{"create", "--status", ""}, openStatuses},
		{"update status", []string{"update", "auth-flow", "--status", ""}, openStatuses},
		{"reopen status", []string{"reopen", "shipped-login", "--status", ""}, openStatuses},
		{"close status", []string{"close", "auth-flow", "--status", ""}, []string{"done\tdone", "dropped\tdropped"}},
		{"list status", []string{"list", "--status", ""}, append(slices.Clone(openStatuses), "done\tdone", "dropped\tdropped")},
		{"list category", []string{"list", "--category", ""}, []string{"todo", "doing", "waiting", "done", "dropped"}},
		{"create type", []string{"create", "--type", ""}, types},
		{"update type prefix", []string{"update", "auth-flow", "--type", "b"}, []string{"bug"}},
		{"list type", []string{"list", "--type", ""}, types},
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

func TestComplete_CommaSeparatedFlag_CompletesThePartAfterTheLastComma(t *testing.T) {
	root := newCompleteStore(t)
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"list status", []string{"list", "--status", "open,d"}, []string{"open,done\tdone", "open,dropped\tdropped"}},
		{"list category", []string{"list", "--category", "todo,waiting,do"},
			[]string{"todo,waiting,doing", "todo,waiting,done"}},
		{"list type", []string{"list", "--type", "bug,f"}, []string{"bug,feature"}},
		{"create dep", []string{"create", "--dep", "auth-flow,sta"},
			[]string{"auth-flow,stale-docs\tStale docs", "auth-flow,standing-parents\tStanding parents"}},
		{"string array takes no commas", []string{"know", "update", "store-layout", "--add-superseded-by", "x,st"}, nil},
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

func TestComplete_CommandTreeOutsideCompletion_RegistersNoFlagCompletions(t *testing.T) {
	root := Command()

	create, _, err := root.Find([]string{"create"})
	if err != nil {
		t.Fatalf("Find create: %v", err)
	}

	if _, isRegistered := create.GetFlagCompletionFunc(flagStatus); isRegistered {
		t.Errorf("create --%s has a completion function outside __complete", flagStatus)
	}
}
