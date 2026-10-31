package nodestore

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

func TestCommitFailure_ChangedRewrite_IsAConflictAtTheFile(t *testing.T) {
	s := newStore(t)
	path := writeNodeFile(t, s.IssuesDir(), "alpha.md", "changed since\n")
	var plan tree.Plan
	plan.AddRewrite(tree.Rewrite{Path: path, Original: []byte("as read\n"), Content: []byte("new\n")})
	_, err := plan.Apply()

	failure := Issues(s, nil).CommitFailure(err, nil)

	var conflict *ConflictError
	if !errors.As(failure, &conflict) {
		t.Fatalf("CommitFailure = %v, want a *ConflictError", failure)
	}
	if conflict.Path != ".capcom/issues/alpha.md" || conflict.Message != changedMessage {
		t.Errorf("conflict = %+v, want %q at .capcom/issues/alpha.md", conflict, changedMessage)
	}
}

func TestCommitFailure_CreateOverAFileThatAppeared_IsExistsNamingTheNode(t *testing.T) {
	s := newStore(t)
	writeNodeFile(t, s.IssuesDir(), "alpha.md", "appeared\n")
	betaPath := filepath.Join(s.IssuesDir(), "beta.md")
	var plan tree.Plan
	plan.AddCreate(betaPath, []byte("beta\n"))
	plan.AddCreate(filepath.Join(s.IssuesDir(), "alpha.md"), []byte("alpha\n"))
	_, err := plan.Apply()
	created := []Written{
		{ID: "beta", Path: ".capcom/issues/beta.md"},
		{ID: "alpha", Path: ".capcom/issues/alpha.md"},
	}

	failure := Issues(s, nil).CommitFailure(err, created)

	var exists *ExistsError
	if !errors.As(failure, &exists) {
		t.Fatalf("CommitFailure = %v, want an *ExistsError", failure)
	}
	if exists.ID != "alpha" || exists.Path != ".capcom/issues/alpha.md" {
		t.Errorf("exists = %+v, want alpha at .capcom/issues/alpha.md", exists)
	}
}

func TestCommitFailure_OtherFailure_IsReturnedAsItIs(t *testing.T) {
	s := newStore(t)
	cause := errors.New("disk full")

	failure := Issues(s, nil).CommitFailure(cause, nil)

	if failure != cause {
		t.Errorf("CommitFailure = %v, want %v", failure, cause)
	}
}
