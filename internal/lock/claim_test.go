package lock

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testBranch = "trunk"

// newClaimRepo creates a hermetic git repository whose unborn branch is testBranch and returns its
// symlink-free root.
func newClaimRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	environment := map[string]string{
		"GIT_CONFIG_GLOBAL":       os.DevNull,
		"GIT_CONFIG_NOSYSTEM":     "1",
		"GIT_CEILING_DIRECTORIES": filepath.Dir(root),
		"GIT_AUTHOR_NAME":         "Test Author",
		"GIT_AUTHOR_EMAIL":        "author@example.com",
		"GIT_COMMITTER_NAME":      "Test Committer",
		"GIT_COMMITTER_EMAIL":     "committer@example.com",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "symbolic-ref", "HEAD", "refs/heads/"+testBranch)
	return root
}

// runGit runs a git command for test setup and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

// setUser sets the login-name variables CurrentClaim reads; "" leaves the user unset.
func setUser(t *testing.T, name string) {
	t.Helper()
	for _, variable := range userVariables {
		t.Setenv(variable, name)
	}
}

// mustHostname returns the host name CurrentClaim is expected to record.
func mustHostname(t *testing.T) string {
	t.Helper()
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("Hostname: %v", err)
	}
	return host
}

func TestCurrentClaim_DescribesSession(t *testing.T) {
	host := mustHostname(t)
	tests := []struct {
		name   string
		user   string
		holder string
		want   Claim
	}{
		{
			name: "no holder defaults to user@host",
			user: "airic",
			want: Claim{Purpose: "item 30", Holder: "airic@" + host, Host: host, User: "airic", Branch: testBranch},
		},
		{
			name:   "a given holder is kept",
			user:   "airic",
			holder: "agent-a",
			want:   Claim{Purpose: "item 30", Holder: "agent-a", Host: host, User: "airic", Branch: testBranch},
		},
		{
			name: "no user defaults the holder to the host",
			want: Claim{Purpose: "item 30", Holder: host, Host: host, Branch: testBranch},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newClaimRepo(t)
			setUser(t, test.user)

			claim, err := CurrentClaim(root, "item 30", test.holder)

			if err != nil {
				t.Fatalf("CurrentClaim: %v", err)
			}
			if claim != test.want {
				t.Errorf("claim = %+v, want %+v", claim, test.want)
			}
		})
	}
}

func TestCurrentClaim_DetachedHead_HasNoBranch(t *testing.T) {
	root := newClaimRepo(t)
	runGit(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	runGit(t, root, "checkout", "-q", "--detach")

	claim, err := CurrentClaim(root, "item 30", "")

	if err != nil {
		t.Fatalf("CurrentClaim: %v", err)
	}
	if claim.Branch != "" {
		t.Errorf("Branch = %q, want empty on a detached HEAD", claim.Branch)
	}
}

func TestCurrentClaim_NotARepository_Fails(t *testing.T) {
	root := newClaimRepo(t)
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, dirPermissions); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", root)

	_, err := CurrentClaim(outside, "item 30", "")

	if err == nil {
		t.Fatal("CurrentClaim outside a repository: want an error, got nil")
	}
}
