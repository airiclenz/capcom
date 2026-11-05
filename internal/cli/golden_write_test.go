package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// goldenApplyFile is the relative name the apply goldens write their apply file under, in the
// store's root; apply echoes it as an error's path, so the failure golden needs no <DIR>.
const goldenApplyFile = "golden-batch.yaml"

// goldenBranch is the branch the lock goldens check out, so the lock's branch does not follow
// git's default branch name.
const goldenBranch = "main"

// goldenUser is the user the lock goldens run as, so the holder is user@host on every host; the
// normaliser still replaces it with <USER>.
const goldenUser = "golden-user"

// TestGolden_WriteKinds pins the --json envelope of each write command's kind against the golden
// store, one golden file per case; each case runs on its own store.
func TestGolden_WriteKinds(t *testing.T) {
	tests := []struct {
		golden string
		args   []string
	}{
		{golden: "issue-write", args: []string{"create", "--id", "login-hint", "--title", "Hint at the password rules",
			"--priority", "2", "--label", "auth", "--body", "Show the rules before the first try."}},
		{golden: "close", args: []string{"close", "login-copy", "--reason", "reworded"}},
		{golden: "issue-remove", args: []string{"rm", "login-copy"}},
	}
	for _, test := range tests {
		t.Run(test.golden, func(t *testing.T) {
			root := newGoldenStore(t)

			stdout := mustRun(t, root, append(test.args, "--json")...)

			assertGoldenJSON(t, test.golden, root, stdout)
		})
	}
}

// TestGolden_LockKinds pins the --json envelope of the commands that take or free the Tree lock;
// the token, host and user are normalised away.
func TestGolden_LockKinds(t *testing.T) {
	tests := []struct {
		golden string
		setup  [][]string
		args   []string
	}{
		{golden: "lock", args: []string{"lock", "acquire", "--purpose", "golden work"}},
		{golden: "start", args: []string{"start", "session-timeout"}},
		{
			golden: "finish",
			setup:  [][]string{{"start", "session-timeout"}},
			args:   []string{"finish", "session-timeout", "--reason", "fixed the race"},
		},
	}
	for _, test := range tests {
		t.Run(test.golden, func(t *testing.T) {
			t.Setenv("USER", goldenUser)
			root := newGoldenStore(t)
			runGit(t, root, "branch", "-m", goldenBranch)
			for _, setup := range test.setup {
				mustRun(t, root, setup...)
			}

			stdout := mustRun(t, root, append(test.args, "--json")...)

			assertGoldenJSON(t, test.golden, root, stdout)
		})
	}
}

// TestGolden_Fmt_PinsTheRewrittenPaths swaps two frontmatter keys of an Issue file, so fmt
// rewrites it and the golden pins a non-empty paths list.
func TestGolden_Fmt_PinsTheRewrittenPaths(t *testing.T) {
	root := newGoldenStore(t)
	issuePath := filepath.Join(root, ".capcom", "issues", "login-copy.md")
	content, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", issuePath, err)
	}
	const canonicalOrder = "title: Reword the login errors\nstatus: open\n"
	if !strings.Contains(string(content), canonicalOrder) {
		t.Fatalf("%s = %q, want it to hold %q", issuePath, content, canonicalOrder)
	}
	reordered := strings.Replace(string(content), canonicalOrder, "status: open\ntitle: Reword the login errors\n", 1)
	if err := os.WriteFile(issuePath, []byte(reordered), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", issuePath, err)
	}

	stdout := mustRun(t, root, "fmt", "--json")

	assertGoldenJSON(t, "fmt", root, stdout)
}

func TestGolden_Apply_PinsTheAppliedBatch(t *testing.T) {
	root := newGoldenStore(t)
	writeGoldenFile(t, root, goldenApplyFile, "- op: create\n  id: login-audit\n  title: Audit the login\n"+
		"- op: log\n  id: login-copy\n  text: drafted new wording\n"+
		"- op: close\n  id: login-copy\n  reason: reworded\n")

	stdout := mustRun(t, root, "apply", goldenApplyFile, "--json")

	assertGoldenJSON(t, "apply", root, stdout)
}

func TestGolden_ApplyFailure_PinsTheErrorPathAndLine(t *testing.T) {
	root := newGoldenStore(t)
	writeGoldenFile(t, root, goldenApplyFile, "- op: log\n  id: login-copy\n  text: drafted new wording\n"+
		"- op: update\n  id: no-such-issue\n  priority: 1\n")

	stdout, _, exitCode := runCLI(t, root, "apply", goldenApplyFile, "--json")

	if exitCode != output.ExitNotFound {
		t.Fatalf("exit = %d, want %d", exitCode, output.ExitNotFound)
	}
	assertGoldenJSON(t, "apply-error", root, stdout)
}

func TestGolden_Merge_PinsTheMergedPath(t *testing.T) {
	root := newGoldenStore(t)
	basePath, oursPath, theirsPath := writeDriverInputs(
		t,
		t.TempDir(),
		driverIssueFile("Login", driverEntryBase),
		driverIssueFile("Login", driverEntryBase, driverEntryOurs),
		driverIssueFile("Login", driverEntryBase, driverEntryTheirs),
	)

	stdout := mustRun(t, root, "merge-driver", basePath, oursPath, theirsPath, mergeDriverPath, "--json")

	assertGoldenJSON(t, "merge", root, stdout)
}

// TestGolden_Errors pins the error envelope in each documented form: a plain error, one with a
// suggestion for a mistyped command or flag, one with the valid values of a flag, and a partial
// failure that keeps its kind and carries both data and error.
func TestGolden_Errors(t *testing.T) {
	tests := []struct {
		golden   string
		args     []string
		exitCode int
	}{
		{golden: "error", args: []string{"dep", "add", "session-timeout", "refresh-retry"}, exitCode: ExitError},
		{golden: "error-unknown-command", args: []string{"know", "shwo", "auth-refresh"}, exitCode: output.ExitUsage},
		{golden: "error-unknown-flag", args: []string{"update", "login-copy", "--statsu", "active"}, exitCode: output.ExitUsage},
		{golden: "error-valid", args: []string{"update", "login-copy", "--type", "bugg"}, exitCode: ExitError},
		{golden: "close-not-found", args: []string{"close", "nosuch", "--reason", "x"}, exitCode: output.ExitNotFound},
	}
	for _, test := range tests {
		t.Run(test.golden, func(t *testing.T) {
			root := newGoldenStore(t)

			stdout, _, exitCode := runCLI(t, root, append(test.args, "--json")...)

			if exitCode != test.exitCode {
				t.Fatalf("exit = %d, want %d", exitCode, test.exitCode)
			}
			assertGoldenJSON(t, test.golden, root, stdout)
		})
	}
}
