package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/ops"
)

// kindStart is the --json kind of capcom start.
const kindStart = "start"

// The help text of capcom start.
const (
	startLong = "Claim a Ready Issue and this worktree's Tree lock in one step, then print the Issue in full " +
		"and the Lock token to keep. The Issue moves to the first doing status and the lock is held " +
		"with the Issue id as its purpose; --token, the Lock token an earlier start or lock acquire " +
		"printed, resumes your lock instead of acquiring a new one. Starting an Issue already in doing " +
		"under your own --token changes nothing in its file and resumes the lock. It refuses, writing " +
		"nothing, an Issue that is Closed, standing, not Ready or already in doing without your lock, " +
		"and a lock another session holds (locked, exit code 3); each refusal names the command that " +
		"resolves it. --kv adds LOCK= and TOKEN= to show's record, and --json adds a lock object to " +
		"show's data."
	startExample = "  capcom start login-timeout-race                   # prints the Issue, then the token to keep\n" +
		"  capcom start login-timeout-race --token <token>   # resume after a restart\n" +
		"  capcom start login-timeout-race --kv              # TOKEN= carries the Lock token"
)

// startData is the --json data of capcom start: the Issue as show gives it, and the lock it holds,
// with the caller's token.
type startData struct {
	showData
	Lock lockData `json:"lock"`
}

// beforeStartClaim runs in start between the acquire of the lock and the claim of the Issue. It is
// nil in production; tests set it to change the Issue file at that moment.
var beforeStartClaim func(entry index.Entry)

// newStartCommand claims a Ready Issue and holds this worktree's Tree lock for it. Every refusal
// comes before any write; a claim that fails after a fresh acquire releases that lock again.
func (a *app) newStartCommand() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:         "start <id|alias|prefix>",
		Short:       "Claim a Ready Issue and this worktree's Tree lock",
		Long:        startLong,
		Example:     startExample,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			return a.start(cmd, args[0], token)
		}),
	}
	cmd.Flags().StringVar(&token, flagToken, "", "your token from an earlier start or lock acquire, to resume the lock")
	return a.withKV(cmd)
}

// start resolves arg, refuses an Issue start cannot claim or a lock another session holds, then
// acquires (or resumes) the lock, claims the Issue unless it is already in doing, and prints both.
func (a *app) start(cmd *cobra.Command, arg string, token string) error {
	entry, err := a.resolveIssue(arg)
	if err != nil {
		return err
	}
	isDoing := entry.Category == config.CategoryDoing
	if !isDoing {
		if err := a.refuseUnstartable(entry); err != nil {
			return err
		}
	}

	target := a.openLockTarget()
	checked, err := target.lock.Check(token)
	if err := a.refuseLock(entry, isDoing, checked, err); err != nil {
		return err
	}

	claim, err := lock.CurrentClaim(target.store.WorktreeRoot, entry.ID, "")
	if err != nil {
		return err
	}
	acquired, isResumed, err := target.lock.Acquire(claim, token, false)
	if err != nil {
		return startLockFailure(err)
	}
	state := lockAcquired
	if isResumed {
		state = lockResumed
	}

	if !isDoing {
		if err := a.claimForStart(target.lock, entry, acquired.Record.Token, isResumed); err != nil {
			return err
		}
	}
	return a.emitStarted(cmd, entry.ID, newLockData(state, acquired, acquired.Record.Token))
}

// refuseUnstartable refuses an Issue that is not in doing and cannot be claimed: Closed, standing,
// outside the todo category, not Ready, or in a store whose config.yaml declares no doing status.
// Each refusal names the command that resolves it.
func (a *app) refuseUnstartable(entry index.Entry) error {
	id := entry.ID
	todoStatus, _ := a.store.Config.DefaultStatus(config.CategoryTodo) // config.yaml always declares one
	switch {
	case entry.Category.IsClosed():
		return output.Errorf(
			output.CodeInvalid,
			"%s is closed (%s); run capcom reopen %s --reason <text> first",
			id, entry.Status, id,
		)
	case entry.IsStanding:
		return output.Errorf(
			output.CodeInvalid,
			"%s is standing: it only groups its children and is never claimed; "+
				"run capcom list --tree %s to pick one of them",
			id, id,
		)
	case entry.Category != config.CategoryTodo:
		return output.Errorf(
			output.CodeConflict,
			"%s is not Ready: status %q is not in the todo category; run capcom update %s --status %s first",
			id, entry.Status, id, todoStatus,
		)
	}

	verdict, err := a.readiness(id)
	if err != nil {
		return err
	}
	if !verdict.IsReady {
		return output.Errorf(
			output.CodeConflict,
			"%s is not Ready: %s; run %s",
			id, notReadyReasons(verdict), strings.Join(readinessResolvers(verdict), ", then "),
		)
	}
	if _, isDeclared := a.store.Config.DefaultStatus(config.CategoryDoing); !isDeclared {
		return output.Errorf(output.CodeInvalid, "%s; add one to statuses", ops.ErrNoDoingStatus.Error())
	}
	return nil
}

// readiness returns the ready rules' verdict on the todo Issue with id, judged against the whole
// issue tree in the index.
func (a *app) readiness(id string) (issue.Readiness, error) {
	nodes, err := a.issueNodes()
	if err != nil {
		return issue.Readiness{}, err
	}
	for _, verdict := range issue.Ready(nodes, a.store.Config) {
		if verdict.ID == id {
			return verdict, nil
		}
	}
	return issue.Readiness{}, output.Errorf(output.CodeConflict, "%s has no readiness verdict; run capcom show %s", id, id)
}

// readinessResolvers names the command that resolves each reason verdict is not Ready, in the
// order notReadyReasons gives the reasons.
func readinessResolvers(verdict issue.Readiness) []string {
	var resolvers []string
	if len(verdict.Blocking) > 0 {
		resolvers = append(resolvers, "capcom dep list "+verdict.ID)
	}
	if len(verdict.OpenChildren) > 0 {
		resolvers = append(resolvers, "capcom list --tree "+verdict.ID)
	}
	if verdict.HeldBy != "" {
		resolvers = append(resolvers, "capcom show "+verdict.HeldBy)
	}
	return resolvers
}

// refuseLock refuses on the lock as Check found it (checked, or checkErr): an Issue already in
// doing is started only under the caller's own held lock, and any other Issue only when the lock is
// free or the caller's.
func (a *app) refuseLock(entry index.Entry, isDoing bool, checked lock.Status, checkErr error) error {
	id := entry.ID
	var held *lock.HeldError
	isHeldByOther := errors.As(checkErr, &held)
	if checkErr != nil && !isHeldByOther {
		return checkErr
	}
	if !isDoing {
		if isHeldByOther {
			return startLockFailure(checkErr)
		}
		return nil
	}
	switch {
	case isHeldByOther:
		return &output.Error{
			Code: output.CodeConflict,
			Message: fmt.Sprintf(
				"%s is already in doing (%s) and the Tree lock is held without your token; "+
					"if the lock is yours, run capcom start %s --token <token>",
				id, entry.Status, id,
			),
			Path: held.Path,
		}
	case !checked.IsHeld:
		todoStatus, _ := a.store.Config.DefaultStatus(config.CategoryTodo) // config.yaml always declares one
		return output.Errorf(
			output.CodeConflict,
			"%s is already in doing (%s) and the Tree lock is free; run capcom lock acquire --purpose %s "+
				"to hold it, or capcom update %s --status %s to start it again",
			id, entry.Status, id, id, todoStatus,
		)
	}
	return nil
}

// startLockFailure types a lock another session holds as locked, naming capcom lock status; any
// other failure passes through.
func startLockFailure(err error) error {
	var held *lock.HeldError
	if !errors.As(err, &held) {
		return err
	}
	return &output.Error{
		Code:    output.CodeLocked,
		Message: held.Message() + "; run capcom lock status",
		Path:    held.Path,
	}
}

// claimForStart claims entry's Issue through ops after the lock was taken with token. A claim that
// fails releases a lock this start acquired fresh, so a refusal leaves no lock behind; a resumed
// lock stays held.
func (a *app) claimForStart(worktree lock.Worktree, entry index.Entry, token string, isResumed bool) error {
	if beforeStartClaim != nil {
		beforeStartClaim(entry)
	}
	_, claimErr := a.written(a.ops().Claim(entry.ID, ""))
	if claimErr == nil {
		return nil
	}
	if !isResumed {
		if _, err := worktree.Release(token); err != nil {
			return errors.Join(claimErr, err)
		}
	}
	return claimErr
}

// emitStarted renders the started Issue with id as show prints it, read after the claim, then the
// lock in held. The index is refreshed first, since the claim wrote the file through ops.
func (a *app) emitStarted(cmd *cobra.Command, id string, held lockData) error {
	if _, err := a.issueIndex.Refresh(); err != nil {
		return err
	}
	shown, _, err := a.showResult([]string{id})
	if err != nil {
		return err
	}
	if shown.Failure != nil {
		return shown.Failure
	}
	record := shown.KV[0]
	record.Pairs = append(
		record.Pairs,
		output.KVPair{Key: kvKeyLock, Value: held.Lock},
		output.KVPair{Key: kvKeyToken, Value: held.Token},
	)
	return a.emit(cmd, output.Result{
		Kind: kindStart,
		Data: startData{showData: shown.Data.([]showData)[0], Lock: held},
		Text: shown.Text + "\n" + lockText(held),
		KV:   []output.KVRecord{record},
	})
}
