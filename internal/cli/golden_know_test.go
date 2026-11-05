package cli

import "testing"

// TestGolden_KnowKinds pins the --json envelope of each know command's kind against the golden
// store, one golden file per case; each case runs on its own store.
func TestGolden_KnowKinds(t *testing.T) {
	tests := []struct {
		golden string
		args   []string
	}{
		{golden: "note", args: []string{"know", "show", "auth-refresh"}},
		{golden: "note-write", args: []string{"know", "add", "--id", "logout-order", "--parent", "auth-refresh",
			"--title", "Logout stops the refresh timer", "--label", "auth", "--ref", "internal/auth/refresh.go"}},
		{golden: "note-remove", args: []string{"know", "rm", "token-lifetimes"}},
		{golden: "know-tree", args: []string{"know", "tree"}},
		{golden: "know-search", args: []string{"know", "search", "refresh"}},
		{golden: "know-verify", args: []string{"know", "verify"}},
	}
	for _, test := range tests {
		t.Run(test.golden, func(t *testing.T) {
			root := newGoldenStore(t)

			stdout := mustRun(t, root, append(test.args, "--json")...)

			assertGoldenJSON(t, test.golden, root, stdout)
		})
	}
}

func TestGolden_KnowVerifyFindings_PinsTheFindingList(t *testing.T) {
	root := newGoldenStore(t)
	mustRun(t, root, "know", "add", "--id", "gone-ref", "--title", "A Note on a removed file",
		"--ref", "internal/removed.go")

	stdout, stderr, exitCode := runCLI(t, root, "know", "verify", "--json")

	if exitCode != ExitError || stderr != "" {
		t.Fatalf("exit = %d stderr = %q, want %d and no stderr", exitCode, stderr, ExitError)
	}
	assertGoldenJSON(t, "know-verify-findings", root, stdout)
}
