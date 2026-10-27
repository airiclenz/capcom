package lock

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// mustSessionToken returns the token SessionToken gives key for token and purpose, failing the
// test on error.
func mustSessionToken(t *testing.T, locks *Locks, key, token, purpose string) string {
	t.Helper()
	kept, err := locks.SessionToken(key, token, purpose)
	if err != nil {
		t.Fatalf("SessionToken(%s, %q, %q): %v", key, token, purpose, err)
	}
	return kept
}

// writeTokenFileContent writes data as key's token file for purpose.
func writeTokenFileContent(t *testing.T, locks *Locks, key, purpose, data string) {
	t.Helper()
	if err := os.MkdirAll(locks.dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(locks.tokenPath(key, purpose), []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// assertExists fails the test unless a file exists at path exactly when want is true.
func assertExists(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Stat(path)
	if isPresent := err == nil; isPresent != want {
		t.Errorf("%s exists = %v (%v), want %v", filepath.Base(path), isPresent, err, want)
	}
}

func TestAcquire_WithPurpose_WritesTheTokenFile(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)

	acquired := mustAcquire(t, locks, MainWorktreeKey)

	path := locks.tokenPath(MainWorktreeKey, testClaim.Purpose)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if want := acquired.Token + "\n" + testClaim.Purpose + "\n"; string(data) != want {
		t.Errorf("token file = %q, want %q", data, want)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file mode = %o, want 600", perm)
	}
}

func TestAcquire_EmptyPurpose_KeepsNoToken(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	claim := testClaim
	claim.Purpose = ""

	if _, _, err := locks.Acquire(MainWorktreeKey, claim, "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	paths, err := locks.tokenFilePaths(MainWorktreeKey)
	if err != nil {
		t.Fatalf("tokenFilePaths: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("token files = %v, want none", paths)
	}
	if kept := mustSessionToken(t, locks, MainWorktreeKey, "", ""); kept != "" {
		t.Errorf("SessionToken = %q, want empty", kept)
	}
}

func TestSessionToken_Cases(t *testing.T) {
	t.Parallel()
	const flagToken = "fedcba9876543210fedcba9876543210"
	cases := []struct {
		name      string
		token     string
		purpose   string
		isKeptOwn bool
		want      string
	}{
		{name: "kept token for the purpose", purpose: testClaim.Purpose, isKeptOwn: true},
		{name: "another purpose", purpose: "item 13"},
		{name: "empty purpose", purpose: ""},
		{name: "flag token wins", token: flagToken, purpose: testClaim.Purpose, want: flagToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			locks, _ := openTest(t)
			acquired := mustAcquire(t, locks, MainWorktreeKey)
			want := tc.want
			if tc.isKeptOwn {
				want = acquired.Token
			}

			kept := mustSessionToken(t, locks, MainWorktreeKey, tc.token, tc.purpose)

			if kept != want {
				t.Errorf("SessionToken = %q, want %q", kept, want)
			}
		})
	}
}

func TestSessionToken_MalformedOrForeignFile_GivesNoToken(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"no line end", "0123456789abcdef0123456789abcdef"},
		{"empty token", "\n" + testClaim.Purpose + "\n"},
		{"purpose of another", "0123456789abcdef0123456789abcdef\nitem 13\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			locks, _ := openTest(t)
			writeTokenFileContent(t, locks, MainWorktreeKey, testClaim.Purpose, tc.data)

			kept := mustSessionToken(t, locks, MainWorktreeKey, "", testClaim.Purpose)

			if kept != "" {
				t.Errorf("SessionToken = %q, want empty", kept)
			}
		})
	}
}

func TestSessionToken_PurposeOverLines_RoundTrips(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	claim := testClaim
	claim.Purpose = "item 12\nsecond line"

	acquired, _, err := locks.Acquire(MainWorktreeKey, claim, "", false)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if kept := mustSessionToken(t, locks, MainWorktreeKey, "", claim.Purpose); kept != acquired.Token {
		t.Errorf("SessionToken = %q, want %q", kept, acquired.Token)
	}
}

func TestSessionToken_UnreadableFile_FailsNamingIt(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	path := locks.tokenPath(MainWorktreeKey, testClaim.Purpose)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	_, err := locks.SessionToken(MainWorktreeKey, "", testClaim.Purpose)

	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want one naming %s", err, path)
	}
}

func TestAcquire_Resume_RewritesTheTokenFile(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	acquired := mustAcquire(t, locks, MainWorktreeKey)
	path := locks.tokenPath(MainWorktreeKey, testClaim.Purpose)
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, isResumed, err := locks.Acquire(MainWorktreeKey, testClaim, acquired.Token, false)

	if err != nil || !isResumed {
		t.Fatalf("Acquire = resumed %v, %v; want a resume", isResumed, err)
	}
	if kept := mustSessionToken(t, locks, MainWorktreeKey, "", testClaim.Purpose); kept != acquired.Token {
		t.Errorf("SessionToken after resume = %q, want %q", kept, acquired.Token)
	}
}

func TestAcquire_ResumeUnderNewPurpose_MovesTheTokenFile(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	alpha := Claim{Purpose: "alpha", Holder: "agent-a"}
	beta := Claim{Purpose: "beta", Holder: "agent-a"}
	const otherToken = "fedcba9876543210fedcba9876543210"
	writeTokenFileContent(t, locks, MainWorktreeKey, "gamma", otherToken+"\ngamma\n")
	acquired, _, err := locks.Acquire(MainWorktreeKey, alpha, "", false)
	if err != nil {
		t.Fatalf("Acquire(alpha): %v", err)
	}

	_, isResumed, err := locks.Acquire(MainWorktreeKey, beta, acquired.Token, false)

	if err != nil || !isResumed {
		t.Fatalf("Acquire(beta) = resumed %v, %v; want a resume", isResumed, err)
	}
	assertExists(t, locks.tokenPath(MainWorktreeKey, alpha.Purpose), false)
	assertExists(t, locks.tokenPath(MainWorktreeKey, "gamma"), true)
	if kept := mustSessionToken(t, locks, MainWorktreeKey, "", alpha.Purpose); kept != "" {
		t.Errorf("SessionToken(alpha) = %q, want none", kept)
	}
	if kept := mustSessionToken(t, locks, MainWorktreeKey, "", beta.Purpose); kept != acquired.Token {
		t.Errorf("SessionToken(beta) = %q, want %q", kept, acquired.Token)
	}
	paths, err := locks.tokenFilePaths(MainWorktreeKey)
	if err != nil {
		t.Fatalf("tokenFilePaths: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("token files = %v, want beta's and the other token's", paths)
	}
}

func TestRelease_DeletesOnlyTheReleasedTokensFiles(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	acquired := mustAcquire(t, locks, MainWorktreeKey)
	makeLinkedGitDir(t, locks, foreignKey)
	mustAcquire(t, locks, foreignKey)

	if _, err := locks.Release(MainWorktreeKey, acquired.Token); err != nil {
		t.Fatalf("Release: %v", err)
	}

	assertExists(t, locks.tokenPath(MainWorktreeKey, testClaim.Purpose), false)
	assertExists(t, locks.tokenPath(foreignKey, testClaim.Purpose), true)
}

func TestRelease_FreeLock_DeletesTheTokenFile(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	const keptToken = "0123456789abcdef0123456789abcdef"
	writeTokenFileContent(t, locks, MainWorktreeKey, testClaim.Purpose, keptToken+"\n"+testClaim.Purpose+"\n")
	writeTokenFileContent(t, locks, MainWorktreeKey, "item 13", "fedcba9876543210fedcba9876543210\nitem 13\n")

	if _, err := locks.Release(MainWorktreeKey, keptToken); err != nil {
		t.Fatalf("Release: %v", err)
	}

	assertExists(t, locks.tokenPath(MainWorktreeKey, testClaim.Purpose), false)
	assertExists(t, locks.tokenPath(MainWorktreeKey, "item 13"), true)
}

func TestRelease_HeldByAnother_DeletesNothing(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	const otherToken = "fedcba9876543210fedcba9876543210"
	writeTokenFileContent(t, locks, MainWorktreeKey, "item 13", otherToken+"\nitem 13\n")

	_, err := locks.Release(MainWorktreeKey, otherToken)

	asHeldError(t, err)
	assertExists(t, locks.tokenPath(MainWorktreeKey, testClaim.Purpose), true)
	assertExists(t, locks.tokenPath(MainWorktreeKey, "item 13"), true)
}

func TestBreak_DeletesEveryTokenFileOfTheKeyOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		arrange func(t *testing.T, locks *Locks)
	}{
		{"held lock", func(t *testing.T, locks *Locks) { mustAcquire(t, locks, MainWorktreeKey) }},
		{"corrupt record", func(t *testing.T, locks *Locks) { writeRecord(t, locks, MainWorktreeKey, "{") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			locks, _ := openTest(t)
			const dottedKey = MainWorktreeKey + ".b"
			tc.arrange(t, locks)
			writeTokenFileContent(t, locks, MainWorktreeKey, "item 13", "fedcba9876543210fedcba9876543210\nitem 13\n")
			mustAcquire(t, locks, dottedKey)

			if _, err := locks.Break(MainWorktreeKey); err != nil {
				t.Fatalf("Break: %v", err)
			}

			paths, err := locks.tokenFilePaths(MainWorktreeKey)
			if err != nil {
				t.Fatalf("tokenFilePaths: %v", err)
			}
			if len(paths) != 0 {
				t.Errorf("token files of %s = %v, want none", MainWorktreeKey, paths)
			}
			assertExists(t, locks.tokenPath(dottedKey, testClaim.Purpose), true)
		})
	}
}
