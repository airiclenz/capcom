package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/airiclenz/capcom"
	"github.com/airiclenz/capcom/internal/cli/output"
)

// goldenSourceDir is the package directory whose non-test sources TestGolden_KindsCovered scans
// for the kinds the code emits; it is made absolute when the test binary starts, as goldenJSONDir
// is.
var goldenSourceDir = absolutePath(".")

// goldenLockHolder is the holder the --kv goldens that need the Tree lock held take it as.
const goldenLockHolder = "golden-agent"

var (
	// kindLiteralPattern matches the kind a result names as a string literal: Kind: "issue".
	kindLiteralPattern = regexp.MustCompile(`\bKind:\s*"([a-z][a-z-]*)"`)
	// kindConstantPattern matches a kind constant's declaration: kindLock = "lock".
	kindConstantPattern = regexp.MustCompile(`\bkind[A-Z]\w*\s*=\s*"([a-z][a-z-]*)"`)
)

// emittedKinds returns the kinds the non-test sources of internal/cli name, as Kind: literals and
// kind constants, plus output.KindError, the kind of every failure envelope.
func emittedKinds(t *testing.T) map[string]bool {
	t.Helper()
	sources, err := filepath.Glob(filepath.Join(goldenSourceDir, "*.go"))
	if err != nil {
		t.Fatalf("Glob %s: %v", goldenSourceDir, err)
	}
	kinds := map[string]bool{output.KindError: true}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", source, err)
		}
		for _, pattern := range []*regexp.Regexp{kindLiteralPattern, kindConstantPattern} {
			for _, match := range pattern.FindAllSubmatch(content, -1) {
				kinds[string(match[1])] = true
			}
		}
	}
	return kinds
}

// goldenKinds returns the kinds the envelopes under testdata/json carry, read from each file's
// kind member.
func goldenKinds(t *testing.T) map[string]bool {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(goldenJSONDir, "*"+goldenJSONSuffix))
	if err != nil {
		t.Fatalf("Glob %s: %v", goldenJSONDir, err)
	}
	kinds := map[string]bool{}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", path, err)
		}
		var envelope struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(content, &envelope); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		kinds[envelope.Kind] = true
	}
	return kinds
}

func TestGoldenHarness_CompareGolden_NormalisesKVRecords(t *testing.T) {
	want := "VERSION=<VERSION>\nETAG=<ETAG>\nHOLDER=<USER>@<HOST>\nSINCE=<TIME>\nTOKEN=<TOKEN>\nHOLDER=agent-a\nSTATUS=ok\n"
	got := "VERSION=" + capcom.Version() + "\nETAG=803b58eea3012260\nHOLDER=ada@build-host\nSINCE=2026-11-05T07:58:12Z\n" +
		"TOKEN=64fe804b1b93ad6677b7974a424b3822\nHOLDER=agent-a\nSTATUS=ok\n"
	tests := []struct {
		name      string
		got       string
		isChanged bool
	}{
		{name: "volatile values only", got: got, isChanged: false},
		{name: "renamed key", got: strings.Replace(got, "SINCE=", "SINCE_AT=", 1), isChanged: true},
		{name: "changed holder", got: strings.Replace(got, "HOLDER=agent-a", "HOLDER=agent-b", 1), isChanged: true},
		{name: "changed status", got: strings.Replace(got, "STATUS=ok", "STATUS=closed", 1), isChanged: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := compareGolden(want, test.got, "")

			if (err != nil) != test.isChanged {
				t.Errorf("compareGolden error = %v, want a change reported: %t", err, test.isChanged)
			}
		})
	}
}

// TestGolden_KindsCovered fails when the code emits a kind that no golden file under
// testdata/json pins.
func TestGolden_KindsCovered(t *testing.T) {
	emitted := emittedKinds(t)
	if !emitted[kindLock] || !emitted["version"] {
		t.Fatalf("emitted kinds = %v, want the scan to find kind constants and Kind: literals", emitted)
	}

	pinned := goldenKinds(t)

	for kind := range emitted {
		if !pinned[kind] {
			t.Errorf("kind %q has no golden file: add a TestGolden_* case writing testdata/json/%s%s",
				kind, kind, goldenJSONSuffix)
		}
	}
}

// kvGoldenName is the name of cmd's --kv golden file: its path below the root command, spaces as
// dashes.
func kvGoldenName(path, rootName string) string {
	return strings.ReplaceAll(strings.TrimPrefix(path, rootName+" "), " ", "-")
}

// TestGolden_KVCovered fails when a command that offers --kv has no testdata/kv/<command>.golden.
func TestGolden_KVCovered(t *testing.T) {
	for _, cmd := range availableCommands() {
		if cmd.Flags().Lookup(flagKV) == nil {
			continue
		}
		name := kvGoldenName(cmd.CommandPath(), cmd.Root().Name())

		_, err := os.Stat(filepath.Join(goldenKVDir, name+goldenKVSuffix))

		if err != nil {
			t.Errorf("%s offers --%s but has no golden file: %v", cmd.CommandPath(), flagKV, err)
		}
	}
}

// TestGolden_KV pins the --kv records of each command that offers --kv and needs no Tree lock held
// by someone else, one golden file per command; each case runs on its own golden store.
func TestGolden_KV(t *testing.T) {
	tests := []struct {
		golden string
		setup  [][]string
		args   []string
	}{
		{golden: "apply", args: []string{"apply", goldenApplyFile}},
		{golden: "close", args: []string{"close", "login-copy", "--reason", "reworded"}},
		{golden: "create", args: []string{"create", "--id", "login-hint", "--title", "Hint at the password rules",
			"--body", "Show the rules before the first try."}},
		{
			golden: "finish",
			setup:  [][]string{{"start", "session-timeout"}},
			args:   []string{"finish", "session-timeout", "--reason", "fixed the race"},
		},
		{golden: "log", args: []string{"log", "session-timeout", "reproduced in one tab"}},
		{golden: "mv", args: []string{"mv", "login-copy", "login-wording"}},
		{golden: "next", args: []string{"next"}},
		{golden: "reopen", args: []string{"reopen", "old-login-page", "--reason", "still linked"}},
		{golden: "rm", args: []string{"rm", "login-copy"}},
		{golden: "show", args: []string{"show", "session-timeout"}},
		{golden: "start", args: []string{"start", "session-timeout"}},
		{golden: "update", args: []string{"update", "login-copy", "--priority", "2"}},
		{golden: "version", args: []string{"version"}},
		{golden: "dep-add", args: []string{"dep", "add", "login-copy", "session-timeout"}},
		{golden: "dep-rm", args: []string{"dep", "rm", "refresh-retry", "session-timeout"}},
		{golden: "lock-acquire", args: []string{"lock", "acquire", "--purpose", "golden work"}},
		{golden: "related-add", args: []string{"related", "add", "refresh-retry", "login-copy"}},
		{golden: "related-rm", args: []string{"related", "rm", "login-copy", "session-timeout"}},
	}
	for _, test := range tests {
		t.Run(test.golden, func(t *testing.T) {
			t.Setenv("USER", goldenUser)
			root := newGoldenStore(t)
			runGit(t, root, "branch", "-m", goldenBranch)
			writeGoldenFile(t, root, goldenApplyFile, "- op: create\n  id: login-audit\n  title: Audit the login\n"+
				"- op: close\n  id: login-copy\n  reason: reworded\n")
			for _, setup := range test.setup {
				mustRun(t, root, setup...)
			}

			stdout := mustRun(t, root, append(test.args, "--"+flagKV)...)

			assertGoldenKV(t, test.golden, root, stdout)
		})
	}
}

// TestGolden_KVHeldLock pins the --kv records of the lock commands that act on a held Tree lock;
// each case takes the lock as goldenLockHolder first and passes its token where the command needs
// one.
func TestGolden_KVHeldLock(t *testing.T) {
	tests := []struct {
		golden string
		args   func(token string) []string
	}{
		{golden: "lock-break", args: func(string) []string { return []string{"lock", "break"} }},
		{golden: "lock-check", args: func(token string) []string { return []string{"lock", "check", "--token", token} }},
		{golden: "lock-release", args: func(token string) []string { return []string{"lock", "release", "--token", token} }},
		{golden: "lock-status", args: func(string) []string { return []string{"lock", "status"} }},
		{golden: "lock-touch", args: func(token string) []string { return []string{"lock", "touch", "--token", token} }},
	}
	for _, test := range tests {
		t.Run(test.golden, func(t *testing.T) {
			t.Setenv("USER", goldenUser)
			root := newGoldenStore(t)
			runGit(t, root, "branch", "-m", goldenBranch)
			token := acquireToken(t, root, goldenLockHolder)

			stdout := mustRun(t, root, append(test.args(token), "--"+flagKV)...)

			assertGoldenKV(t, test.golden, root, stdout)
		})
	}
}

// TestExitCodes produces each exit code json-output.md documents, the two meanings of 3 told apart
// by the error code, plus check exiting 1 on a finding and prime exiting 0 on a broken store.
func TestExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, root string)
		args     []string
		exitCode int
		code     output.Code
	}{
		{name: "0 success", args: []string{"version"}, exitCode: ExitOK},
		{name: "1 error", args: []string{"dep", "add", "session-timeout", "refresh-retry"}, exitCode: ExitError,
			code: output.CodeInvalid},
		{name: "2 usage", args: []string{"update", "login-copy", "--statsu", "active"}, exitCode: output.ExitUsage,
			code: output.CodeInvalid},
		{name: "3 not found", args: []string{"show", "nosuch"}, exitCode: output.ExitNotFound, code: output.CodeNotFound},
		{
			name: "3 locked",
			setup: func(t *testing.T, root string) {
				t.Helper()
				acquireToken(t, root, goldenLockHolder)
			},
			args:     []string{"lock", "acquire", "--purpose", "other work", "--holder", "other-agent"},
			exitCode: output.ExitNotFound,
			code:     output.CodeLocked,
		},
		{
			name: "4 nothing ready",
			setup: func(t *testing.T, root string) {
				t.Helper()
				for _, id := range []string{"session-timeout", "refresh-retry", "login-copy"} {
					mustRun(t, root, "close", id, "--reason", "done")
				}
			},
			args:     []string{"next"},
			exitCode: output.ExitNothingReady,
		},
		{
			name: "check finding is 1",
			setup: func(t *testing.T, root string) {
				t.Helper()
				mustRun(t, root, "know", "add", "--id", "gone-ref", "--title", "A Note on a removed file",
					"--ref", "internal/removed.go")
			},
			args:     []string{"check"},
			exitCode: ExitError,
		},
		{
			name: "prime on a broken store is 0",
			setup: func(t *testing.T, root string) {
				t.Helper()
				writeGoldenFile(t, root, ".capcom/config.yaml", "statuses:\n  open: nonsense\n")
			},
			args:     []string{"prime"},
			exitCode: ExitOK,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newGoldenStore(t)
			if test.setup != nil {
				test.setup(t, root)
			}

			stdout, stderr, exitCode := runCLI(t, root, append(test.args, "--json")...)

			if exitCode != test.exitCode {
				t.Fatalf("exit = %d, want %d\nstdout = %s\nstderr = %s", exitCode, test.exitCode, stdout, stderr)
			}
			var envelope output.Envelope
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("stdout = %q: %v", stdout, err)
			}
			var code output.Code
			if envelope.Error != nil {
				code = envelope.Error.Code
			}
			if code != test.code {
				t.Errorf("error code = %q, want %q\nstdout = %s", code, test.code, stdout)
			}
		})
	}
}
