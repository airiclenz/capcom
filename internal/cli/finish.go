package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/briefing"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/lock"
)

// kindFinish is the --json kind of capcom finish.
const kindFinish = "finish"

// flagKeepLock leaves the Tree lock held after finish closes the Issue.
const flagKeepLock = "keep-lock"

// kvKeyNext is the --kv key naming the Issue finish says to work on next.
const kvKeyNext = "NEXT"

// The help text of capcom finish.
const (
	finishLong = "Close an Issue and free this worktree's Tree lock in one step, then name the Ready Issue to " +
		"work on next.\n\n" +
		"--reason is required; the Issue moves to the first done status, and the output gives " +
		"close's line and its \"now ready:\" line, a lock line, and a last line naming the top Ready " +
		"Issue as capcom next chooses it, or \"" + nothingReadyMessage + "\".\n\n" +
		"The lock is released through the Lock token capcom kept for the Issue when start took the " +
		"lock, and that kept token is deleted; --token is optional and, when given, is used in place " +
		"of the kept token.\n\n" +
		"When neither holds the lock, the lock is left alone and the lock line names capcom lock " +
		"status.\n\n" +
		"--keep-lock leaves the lock held and needs no token.\n\n" +
		"An Issue already Closed is reported already_closed and the lock is still released.\n\n" +
		"An Issue that does not exist or is standing fails the command and the lock is not touched.\n\n" +
		"--kv gives one record with ID=, LOCK= and NEXT=, and --json gives the close, the lock and " +
		"the next Issue."
	finishExample = "  capcom finish login-timeout-race --reason \"Fixed in 3f2a1c9\"\n" +
		"  capcom finish login-timeout-race --reason \"Fixed in 3f2a1c9\" --keep-lock   # more work under this lock\n" +
		"  capcom finish login-timeout-race --reason \"Fixed in 3f2a1c9\" --token <token> --kv"
)

// finishData is the --json data of capcom finish: the close as close reports it, the lock as finish
// left it (never with a token), and the Issue to work on next, null when nothing is Ready.
type finishData struct {
	Close closeData       `json:"close"`
	Lock  lockData        `json:"lock"`
	Next  *finishNextData `json:"next"`
}

// finishNextData names the top Ready Issue after the close.
type finishNextData struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// finishLockOutcome is what finish did to the Tree lock: the lock as it now is, a note the lock line
// carries when the lock was not released, and a warning when its record could not be read.
type finishLockOutcome struct {
	data    lockData
	note    string
	warning string
}

// newFinishCommand closes one Issue, frees the Tree lock the caller's token or the token kept for
// the Issue holds, and names the Ready Issue to work on next. A failed close fails finish before the lock is looked at; after a
// close, nothing the lock does changes the exit code.
func (a *app) newFinishCommand() *cobra.Command {
	var reason, token string
	var isKeepLock bool
	cmd := &cobra.Command{
		Use:               "finish <id|alias|prefix> --reason <text>",
		Short:             "Close an Issue and free this worktree's Tree lock",
		Long:              finishLong,
		Example:           finishExample,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeIssueIDs(1),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if err := requireReason(cmd, reason); err != nil {
				return err
			}
			return a.finish(cmd, args[0], reason, token, isKeepLock)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&reason, flagReason, "", "why the Issue is closed (required)")
	flags.StringVar(&token, flagToken, "", "your token from start or lock acquire, used in place of the kept one")
	flags.BoolVar(&isKeepLock, flagKeepLock, false, "leave the Tree lock held")
	return a.withKV(cmd)
}

// finish closes arg's Issue as close does, then frees or keeps the lock, then renders the close,
// the lock and the Issue to work on next.
func (a *app) finish(cmd *cobra.Command, arg string, reason string, token string, isKeepLock bool) error {
	status, _ := a.store.Config.DefaultStatus(config.CategoryDone) // config.yaml always declares one
	closed, err := a.closeResult([]string{arg}, status, reason)
	if err != nil {
		return err
	}
	if closed.Failure != nil {
		return closed.Failure
	}
	outcome := closed.Data.([]closeData)[0]

	held := finishLock(a.openLockTarget().lock, outcome.ID, token, isKeepLock)

	next, err := a.nextAfterFinish()
	if err != nil {
		return err
	}
	return a.emit(cmd, finishResult(closed.Text, outcome, held, next))
}

// finishLock releases worktree's lock for token, or for the Lock token kept for the Issue id when
// token is empty, or with isKeepLock only reads it. A lock neither token holds is left alone, and a
// record or kept token that cannot be read is reported; none of them is a failure.
func finishLock(worktree lock.Worktree, id string, token string, isKeepLock bool) finishLockOutcome {
	if isKeepLock {
		status, err := worktree.Read()
		if err != nil {
			return unreadableFinishLock(worktree, err)
		}
		return finishLockOutcome{data: newLockData(lockStateName(status.State()), status, ""), note: "kept (--keep-lock)"}
	}

	sessionToken, err := worktree.SessionToken(token, id)
	if err != nil {
		return unreadableKeptToken(worktree, err)
	}
	status, err := worktree.Release(sessionToken)
	var held *lock.HeldError
	switch {
	case errors.As(err, &held):
		return finishLockOutcome{
			data: newLockData(lockStateName(held.Status.State()), held.Status, ""),
			note: "left alone: " + notHoldingReason(id, token, sessionToken) + "; run capcom lock status",
		}
	case err != nil:
		return unreadableFinishLock(worktree, err)
	}
	return finishLockOutcome{data: newLockData(endedState(status), status, "")}
}

// notHoldingReason says why finish did not release the lock for the Issue id: the --token given
// does not hold it, the token kept for id does not, or neither was there to try.
func notHoldingReason(id string, token string, sessionToken string) string {
	switch {
	case token != "":
		return "--token does not hold it"
	case sessionToken != "":
		return "the kept token for " + id + " does not hold it"
	}
	return "no --token given and no kept token for " + id
}

// unreadableKeptToken reports worktree's lock, left alone because the kept Lock token failed to
// read with err; the lock is described as Read finds it.
func unreadableKeptToken(worktree lock.Worktree, err error) finishLockOutcome {
	status, readErr := worktree.Read()
	if readErr != nil {
		return unreadableFinishLock(worktree, readErr)
	}
	return finishLockOutcome{
		data:    newLockData(lockStateName(status.State()), status, ""),
		note:    "left alone: the kept token cannot be read; run capcom lock status",
		warning: err.Error(),
	}
}

// unreadableFinishLock reports worktree's lock whose record failed to read with err: invalid when
// the Lock overview finds it holds no valid lock, otherwise unreadable. The record is left in place.
func unreadableFinishLock(worktree lock.Worktree, err error) finishLockOutcome {
	state := lockUnreadable
	if overview, overviewErr := worktree.Overview(); overviewErr == nil && overview.State == lock.StateInvalid {
		state = lockInvalid
	}
	return finishLockOutcome{
		data:    lockData{Lock: state, Worktree: worktree.Key()},
		note:    "left alone: its record cannot be read; run capcom lock status, then capcom lock break to free it",
		warning: err.Error(),
	}
}

// nextAfterFinish refreshes the index the close changed and returns the top Ready Issue as capcom
// next chooses it, or nil when nothing is Ready.
func (a *app) nextAfterFinish() (*finishNextData, error) {
	if _, err := a.issueIndex.Refresh(); err != nil {
		return nil, err
	}
	options := &readyOptions{limit: nextLimit}
	listed, err := briefing.ReadyList(a.issueIndex, a.store.Config, options.listOptions(a.store.WorktreeRoot))
	if err != nil {
		return nil, err
	}
	if len(listed) == 0 {
		return nil, nil
	}
	return &finishNextData{ID: listed[0].Entry.ID, Title: listed[0].Entry.Title}, nil
}

// finishResult renders finish: closeText as close prints it, the lock line, and the next line.
func finishResult(closeText string, outcome closeData, held finishLockOutcome, next *finishNextData) output.Result {
	var text strings.Builder
	text.WriteString(closeText)
	line := strings.TrimSuffix(lockText(held.data), "\n")
	if held.note != "" {
		line += "; " + held.note
	}
	text.WriteString(line + "\n")
	nextID := ""
	if next == nil {
		text.WriteString(nothingReadyMessage + "\n")
	} else {
		nextID = next.ID
		text.WriteString("next: " + next.ID + "  " + next.Title + "\n")
	}

	result := output.Result{
		Kind: kindFinish,
		Data: finishData{Close: outcome, Lock: held.data, Next: next},
		Text: text.String(),
		KV: []output.KVRecord{{
			Pairs: []output.KVPair{
				{Key: "ID", Value: outcome.ID},
				{Key: kvKeyLock, Value: held.data.Lock},
				{Key: kvKeyNext, Value: nextID},
			},
			Status: outcome.Outcome,
		}},
	}
	if held.warning != "" {
		result.Warnings = []string{held.warning}
	}
	return result
}
