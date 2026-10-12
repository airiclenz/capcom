package lock

import (
	"fmt"
	"os"
	"strings"

	"github.com/airiclenz/capcom/internal/gitx"
)

// Environment variables naming the user a lock records: USER off Windows, USERNAME on it.
var userVariables = []string{"USER", "USERNAME"}

// CurrentClaim describes the calling session for a lock on the worktree at worktreeRoot: purpose,
// holder (user@host, or the host alone with no user, unless given), and the host, user and
// checked-out branch of that worktree. The branch is empty on a detached HEAD. It fails when the
// host name cannot be read or git cannot report the branch.
func CurrentClaim(worktreeRoot, purpose, holder string) (Claim, error) {
	host, err := os.Hostname()
	if err != nil {
		return Claim{}, fmt.Errorf("reading host name: %w", err)
	}
	branch, err := gitx.CurrentBranch(worktreeRoot)
	if err != nil {
		return Claim{}, err
	}

	user := currentUser()
	if holder == "" {
		holder = host
		if user != "" {
			holder = user + "@" + host
		}
	}
	return Claim{
		Purpose: purpose,
		Holder:  holder,
		Host:    host,
		User:    user,
		Branch:  branch,
	}, nil
}

// currentUser returns the login name from the environment, or "" when none is set.
func currentUser() string {
	for _, variable := range userVariables {
		if name := strings.TrimSpace(os.Getenv(variable)); name != "" {
			return name
		}
	}
	return ""
}
