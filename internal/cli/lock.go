package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/store"
)

// Flags of the lock subcommands.
const (
	flagPurpose   = "purpose"
	flagHolder    = "holder"
	flagToken     = "token"
	flagTakeStale = "take-stale"
)

// LOCK= values: what a lock command did or found.
const (
	lockAcquired = "acquired"
	lockResumed  = "resumed"
	lockHeld     = "held"
	lockFree     = "free"
	lockReleased = "released"
	lockStale    = "stale"
)

// --kv keys of the lock commands, in output order after WORKTREE (status --all only).
const (
	kvKeyWorktree  = "WORKTREE"
	kvKeyLock      = "LOCK"
	kvKeyHolder    = "HOLDER"
	kvKeyPurpose   = "PURPOSE"
	kvKeySince     = "SINCE"
	kvKeyHeartbeat = "HEARTBEAT"
	kvKeyToken     = "TOKEN"
)

const kindLock = "lock"

// lockData is one lock in the --json data of the lock commands. The lock fields describe the
// record as it was found or written and are empty for a free lock; Token is set only for the
// caller's own lock.
type lockData struct {
	Lock      string `json:"lock"`
	Worktree  string `json:"worktree"`
	Holder    string `json:"holder,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
	Host      string `json:"host,omitempty"`
	User      string `json:"user,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Since     string `json:"since,omitempty"`
	Heartbeat string `json:"heartbeat,omitempty"`
	Stale     bool   `json:"stale"`
	Token     string `json:"token,omitempty"`
}

// lockTarget is the store of the current worktree and that worktree's lock.
type lockTarget struct {
	store *store.Store
	lock  lock.Worktree
}

// newLockCommand groups the Tree lock commands.
func (a *app) newLockCommand() *cobra.Command {
	lockCommand := &cobra.Command{
		Use:   "lock",
		Short: "Acquire, refresh, release and inspect this worktree's Tree lock",
		Args:  cobra.NoArgs,
	}
	lockCommand.AddCommand(a.newLockAcquireCommand())
	lockCommand.AddCommand(a.newLockBreakCommand())
	lockCommand.AddCommand(a.newLockCheckCommand())
	lockCommand.AddCommand(a.newLockReleaseCommand())
	lockCommand.AddCommand(a.newLockStatusCommand())
	lockCommand.AddCommand(a.newLockTouchCommand())
	return lockCommand
}

func (a *app) newLockAcquireCommand() *cobra.Command {
	var purpose, holder, token string
	var isTakeStale bool
	cmd := &cobra.Command{
		Use:   "acquire",
		Short: "Acquire this worktree's lock, or resume it with your own token",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target, err := openLockTarget()
			if err != nil {
				return err
			}
			claim, err := lock.CurrentClaim(target.store.WorktreeRoot, purpose, holder)
			if err != nil {
				return err
			}
			status, isResumed, err := target.lock.Acquire(claim, token, isTakeStale)
			state := lockAcquired
			if isResumed {
				state = lockResumed
			}
			return a.emitLock(cmd, state, status, status.Record.Token, err)
		}),
	}
	cmd.Flags().StringVar(&purpose, flagPurpose, "", "what the lock is held for")
	cmd.Flags().StringVar(&holder, flagHolder, "", "who holds the lock (default user@host)")
	cmd.Flags().StringVar(&token, flagToken, "", "your token from an earlier acquire, to resume the lock")
	cmd.Flags().BoolVar(&isTakeStale, flagTakeStale, false, "take the lock over when it is stale")
	_ = cmd.MarkFlagRequired(flagPurpose) // fails only for an undefined flag
	return a.withKV(cmd)
}

func (a *app) newLockTouchCommand() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "touch",
		Short: "Refresh the heartbeat of the lock you hold",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target, err := openLockTarget()
			if err != nil {
				return err
			}
			status, err := target.lock.Touch(token)
			return a.emitLock(cmd, lockHeld, status, token, err)
		}),
	}
	cmd.Flags().StringVar(&token, flagToken, "", "the token acquire returned")
	_ = cmd.MarkFlagRequired(flagToken) // fails only for an undefined flag
	return a.withKV(cmd)
}

func (a *app) newLockReleaseCommand() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release the lock you hold; releasing a free lock does nothing",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target, err := openLockTarget()
			if err != nil {
				return err
			}
			status, err := target.lock.Release(token)
			return a.emitLock(cmd, endedState(status), status, token, err)
		}),
	}
	cmd.Flags().StringVar(&token, flagToken, "", "the token acquire returned")
	_ = cmd.MarkFlagRequired(flagToken) // fails only for an undefined flag
	return a.withKV(cmd)
}

func (a *app) newLockBreakCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:   "break",
		Short: "Free this worktree's lock whoever holds it",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target, err := openLockTarget()
			if err != nil {
				return err
			}
			status, err := target.lock.Break()
			if err == nil && status.IsCorrupt {
				return a.emitDiscardedLock(cmd, status, target.lock.RecordPath())
			}
			return a.emitLock(cmd, endedState(status), status, "", err)
		}),
	})
}

func (a *app) newLockCheckCommand() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Exit 0 when this worktree's lock is free or yours, 3 when another holds it",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target, err := openLockTarget()
			if err != nil {
				return err
			}
			status, err := target.lock.Check(token)
			ownToken := ""
			if status.IsHeld {
				ownToken = token
			}
			return a.emitLock(cmd, foundState(status), status, ownToken, err)
		}),
	}
	cmd.Flags().StringVar(&token, flagToken, "", "your token; the lock counts as free when you hold it")
	return a.withKV(cmd)
}

func (a *app) newLockStatusCommand() *cobra.Command {
	var isAll bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show this worktree's lock, or with --all every worktree's",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target, err := openLockTarget()
			if err != nil {
				return err
			}
			if isAll {
				held, warnings, err := target.lock.List()
				if err != nil {
					return err
				}
				result := allLocksResult(held)
				result.Warnings = errorStrings(warnings)
				return a.emit(cmd, result)
			}
			status, err := target.lock.Read()
			return a.emitLock(cmd, foundState(status), status, "", err)
		}),
	}
	cmd.Flags().BoolVar(&isAll, flagAll, false, "show the locks of every worktree")
	return a.withKV(cmd)
}

// openLockTarget opens the store of the current worktree and that worktree's lock.
func openLockTarget() (lockTarget, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return lockTarget{}, err
	}
	opened, err := store.Open(cwd)
	if err != nil {
		return lockTarget{}, output.StoreFailure(err)
	}
	return lockTarget{store: opened, lock: lock.ForWorktree(opened)}, nil
}

// emitLock renders a lock command's outcome: status in state on success. A lock held by another
// session fails locked, and a touch of a free lock fails not_found; either failure still renders
// the lock as found, so --kv keeps its LOCK= record and closes it with the failure's code.
func (a *app) emitLock(cmd *cobra.Command, state string, status lock.Status, token string, err error) error {
	var held *lock.HeldError
	var notHeld *lock.NotHeldError
	switch {
	case errors.As(err, &held):
		failure := &output.Error{Code: output.CodeLocked, Message: held.Message(), Path: held.Path}
		return a.emit(cmd, failedLockResult(foundState(held.Status), held.Status, failure))
	case errors.As(err, &notHeld):
		failure := &output.Error{Code: output.CodeNotFound, Message: lock.ErrNotHeld.Error(), Path: notHeld.Path}
		return a.emit(cmd, failedLockResult(lockFree, status, failure))
	case err != nil:
		return err
	}
	data := newLockData(state, status, token)
	return a.emit(cmd, output.Result{
		Kind: kindLock,
		Data: data,
		Text: lockText(data),
		KV:   []output.KVRecord{{Pairs: lockPairs(data)}},
	})
}

// emitDiscardedLock renders a break that discarded status's corrupt record at path: the lock is
// now free, and a warning says the record was invalid and has been discarded.
func (a *app) emitDiscardedLock(cmd *cobra.Command, status lock.Status, path string) error {
	data := newLockData(lockFree, status, "")
	return a.emit(cmd, output.Result{
		Kind:     kindLock,
		Data:     data,
		Text:     lockText(data),
		KV:       []output.KVRecord{{Pairs: lockPairs(data)}},
		Warnings: []string{path + ": invalid lock record; discarded"},
	})
}

// failedLockResult is the partial result of a failed lock command: the lock as found, and the
// failure that explains it in place of the text.
func failedLockResult(state string, status lock.Status, failure *output.Error) output.Result {
	data := newLockData(state, status, "")
	return output.Result{
		Kind:    kindLock,
		Data:    data,
		KV:      []output.KVRecord{{Pairs: lockPairs(data), Status: string(failure.Code)}},
		Failure: failure,
	}
}

// allLocksResult lists every held lock of the repository, one --kv record per lock led by its
// WORKTREE line. With none held, --kv writes a lone STATUS line.
func allLocksResult(held []lock.Status) output.Result {
	data := make([]lockData, 0, len(held))
	records := make([]output.KVRecord, 0, len(held))
	var text strings.Builder
	for _, status := range held {
		entry := newLockData(foundState(status), status, "")
		data = append(data, entry)
		pairs := append([]output.KVPair{{Key: kvKeyWorktree, Value: entry.Worktree}}, lockPairs(entry)...)
		records = append(records, output.KVRecord{Pairs: pairs})
		text.WriteString(lockText(entry))
	}
	if len(held) == 0 {
		records = append(records, output.KVRecord{})
		text.WriteString("no locks held\n")
	}
	return output.Result{Kind: kindLock, Data: data, Text: text.String(), KV: records}
}

// foundState names a lock as read: held, stale or free.
func foundState(status lock.Status) string {
	switch {
	case !status.IsHeld:
		return lockFree
	case status.IsStale:
		return lockStale
	default:
		return lockHeld
	}
}

// endedState names the outcome of freeing a lock: released when it was held, free when it was not.
func endedState(status lock.Status) string {
	if status.IsHeld {
		return lockReleased
	}
	return lockFree
}

// newLockData describes status, reached in state; token is shown only when it is the caller's.
func newLockData(state string, status lock.Status, token string) lockData {
	data := lockData{Lock: state, Worktree: status.Key, Stale: status.IsStale}
	if !status.IsHeld {
		return data
	}
	record := status.Record
	data.Holder = record.Holder
	data.Purpose = record.Purpose
	data.Host = record.Host
	data.User = record.User
	data.Branch = record.Branch
	data.Since = record.Since.UTC().Format(time.RFC3339)
	data.Heartbeat = record.Heartbeat.UTC().Format(time.RFC3339)
	data.Token = token
	return data
}

// lockPairs is the --kv record of data, before its STATUS line.
func lockPairs(data lockData) []output.KVPair {
	return []output.KVPair{
		{Key: kvKeyLock, Value: data.Lock},
		{Key: kvKeyHolder, Value: data.Holder},
		{Key: kvKeyPurpose, Value: data.Purpose},
		{Key: kvKeySince, Value: data.Since},
		{Key: kvKeyHeartbeat, Value: data.Heartbeat},
		{Key: kvKeyToken, Value: data.Token},
	}
}

// lockText is the text line of data, plus a token line when the token is the caller's. Holder
// details follow only a lock that is in force; a released or free one is named alone.
func lockText(data lockData) string {
	line := data.Worktree + ": " + data.Lock
	if data.Holder != "" && data.Lock != lockReleased {
		line += fmt.Sprintf(
			" by %s since %s (heartbeat %s): %s",
			data.Holder,
			data.Since,
			data.Heartbeat,
			data.Purpose,
		)
	}
	line += "\n"
	if data.Token != "" && data.Lock != lockReleased {
		line += "token " + data.Token + "\n"
	}
	return line
}
