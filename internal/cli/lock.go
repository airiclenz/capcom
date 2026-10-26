package cli

import (
	"errors"
	"fmt"
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
	lockAcquired   = "acquired"
	lockResumed    = "resumed"
	lockHeld       = "held"
	lockFree       = "free"
	lockReleased   = "released"
	lockStale      = "stale"
	lockInvalid    = "invalid"
	lockUnreadable = "unreadable"
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

// The help text of capcom lock and its subcommands.
const (
	lockLong = "Manage this worktree's Tree lock: the advisory claim one agent session holds on one working " +
		"tree while it edits.\n\n" +
		"acquire claims it, prints a Lock token and keeps that token for the session in a token file " +
		"under the git directory; touch renews its heartbeat with that token, and release frees it.\n\n" +
		"A lock whose heartbeat is older than the TTL (lock.ttl in config.yaml, two hours by default), " +
		"or dated more than one TTL ahead by a clock that ran fast, is stale: it stays held until its holder resumes it, another session takes it over with " +
		"acquire --take-stale, or someone breaks it.\n\n" +
		"Each git worktree has a lock of its own, and capcom installs no git hooks: the lock binds " +
		"only the sessions that check it."
	lockExample = "  capcom lock acquire --purpose \"implementing the release plan\"   # prints and keeps the token\n" +
		"  capcom lock touch --token <token>     # renew the heartbeat\n" +
		"  capcom lock status --all              # every worktree's lock\n" +
		"  capcom lock release --token <token>   # free it when the work is done"
	lockAcquireLong = "Claim this worktree's Tree lock for --purpose, which is required.\n\n" +
		"A free lock gets a new Lock token, printed on a token line; touch and release need it.\n\n" +
		"capcom keeps the Lock token for the session in a token file under the git directory: " +
		"acquiring again with the same --purpose and no --token resumes your lock, even a stale one, " +
		"refreshes its heartbeat and prints the token again.\n\n" +
		"A --token from an earlier acquire is used in place of the kept one.\n\n" +
		"A lock another session holds fails locked, exit code 3, unless it is stale and --take-stale " +
		"is given, which takes it over with a new token.\n\n" +
		"--holder names the holder, user@host by default."
	lockAcquireExample = "  capcom lock acquire --purpose \"implementing the release plan\"   # prints and keeps the token\n" +
		"  capcom lock acquire --purpose \"implementing the release plan\"   # again: resume after a restart\n" +
		"  capcom lock acquire --purpose \"finishing the release\" --take-stale   # take over a stale lock"
	lockTouchLong = "Refresh the heartbeat of the Tree lock you hold, so that it does not go stale; --token, the " +
		"Lock token acquire printed, is required.\n\n" +
		"Touch it more often than the lock TTL while the work goes on.\n\n" +
		"It prints the lock as it now stands.\n\n" +
		"It fails not_found when nobody holds the lock and locked when another session holds it, both " +
		"with exit code 3."
	lockTouchExample = "  capcom lock touch --token <token>\n" +
		"  capcom lock touch --token <token> --kv   # KEY=VALUE lines ending in STATUS="
	lockReleaseLong = "Free the Tree lock you hold; --token, the Lock token acquire printed, is required.\n\n" +
		"It also deletes the kept token files holding that token.\n\n" +
		"Releasing a lock that is already free succeeds and only deletes those token files, so it is " +
		"safe on every exit path.\n\n" +
		"It prints the worktree and released, or free when there was nothing to release.\n\n" +
		"A lock another session holds fails locked, exit code 3, and deletes nothing; break frees a " +
		"lock whose holder is gone."
	lockReleaseExample = "  capcom lock release --token <token>\n" +
		"  capcom lock release --token <token> --kv   # KEY=VALUE lines ending in STATUS="
	lockBreakLong = "Free this worktree's Tree lock whoever holds it, without its Lock token, and delete this " +
		"worktree's kept token files, so no session resumes the lock from one.\n\n" +
		"Use it when the holder is gone, for a stale lock nobody will resume, or when the lock record " +
		"itself is invalid: such a record is discarded, with a warning.\n\n" +
		"The session that held the lock is not told; its next touch fails not_found.\n\n" +
		"It prints the worktree and released, or free when nobody held the lock."
	lockBreakExample = "  capcom lock break        # free the lock whoever holds it\n" +
		"  capcom lock break --kv   # KEY=VALUE lines ending in STATUS="
	lockCheckLong = "Exit 0 when this worktree's Tree lock is free or held by your --token, and fail locked, " +
		"exit code 3, when another session holds it, stale or not.\n\n" +
		"Without --token any held lock counts as another's.\n\n" +
		"It prints the lock's state, free, held or stale, with the holder, purpose, since time and " +
		"heartbeat of a held lock.\n\n" +
		"A script or hook runs it before it edits the working tree."
	lockCheckExample = "  capcom lock check                   # exit 3 when any session holds the lock\n" +
		"  capcom lock check --token <token>   # exit 0 when the lock is free or yours"
	lockStatusLong = "Show this worktree's Tree lock: free, or held or stale with its holder, purpose, since " +
		"time and heartbeat.\n\n" +
		"A lock record that does not parse shows as invalid, and one that cannot be read as " +
		"unreadable, each with a warning naming the file; the command still exits 0.\n\n" +
		"--all shows every held lock in the repository, one per worktree, or \"no locks held\", with " +
		"one warning per invalid or unreadable record.\n\n" +
		"It only reads the lock, and it never prints a Lock token."
	lockStatusExample = "  capcom lock status         # this worktree's lock\n" +
		"  capcom lock status --all   # every worktree's lock"
)

// newLockCommand groups the Tree lock commands.
func (a *app) newLockCommand() *cobra.Command {
	lockCommand := &cobra.Command{
		Use:     "lock",
		Short:   "Acquire, refresh, release and inspect this worktree's Tree lock",
		Long:    lockLong,
		Example: lockExample,
	}
	lockCommand.AddCommand(a.newLockAcquireCommand())
	lockCommand.AddCommand(a.newLockBreakCommand())
	lockCommand.AddCommand(a.newLockCheckCommand())
	lockCommand.AddCommand(a.newLockReleaseCommand())
	lockCommand.AddCommand(a.newLockStatusCommand())
	lockCommand.AddCommand(a.newLockTouchCommand())
	return withSubcommandDispatch(lockCommand)
}

func (a *app) newLockAcquireCommand() *cobra.Command {
	var purpose, holder, token string
	var isTakeStale bool
	cmd := &cobra.Command{
		Use:         "acquire",
		Short:       "Acquire this worktree's lock, or resume it with your own token",
		Long:        lockAcquireLong,
		Example:     lockAcquireExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target := a.openLockTarget()
			claim, err := lock.CurrentClaim(target.store.WorktreeRoot, purpose, holder)
			if err != nil {
				return err
			}
			sessionToken, err := target.lock.SessionToken(token, purpose)
			if err != nil {
				return err
			}
			status, isResumed, err := target.lock.Acquire(claim, sessionToken, isTakeStale)
			state := lockAcquired
			if isResumed {
				state = lockResumed
			}
			return a.emitLock(cmd, state, status, status.Record.Token, err)
		}),
	}
	cmd.Flags().StringVar(&purpose, flagPurpose, "", "what the lock is held for")
	cmd.Flags().StringVar(&holder, flagHolder, "", "who holds the lock (default user@host)")
	cmd.Flags().StringVar(&token, flagToken, "", "your token from an earlier acquire, used in place of the kept one")
	cmd.Flags().BoolVar(&isTakeStale, flagTakeStale, false, "take the lock over when it is stale")
	_ = cmd.MarkFlagRequired(flagPurpose) // fails only for an undefined flag
	return a.withKV(cmd)
}

func (a *app) newLockTouchCommand() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:         "touch",
		Short:       "Refresh the heartbeat of the lock you hold",
		Long:        lockTouchLong,
		Example:     lockTouchExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target := a.openLockTarget()
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
		Use:         "release",
		Short:       "Release the lock you hold; releasing a free lock succeeds",
		Long:        lockReleaseLong,
		Example:     lockReleaseExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target := a.openLockTarget()
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
		Use:         "break",
		Short:       "Free this worktree's lock whoever holds it",
		Long:        lockBreakLong,
		Example:     lockBreakExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target := a.openLockTarget()
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
		Use:         "check",
		Short:       "Exit 0 when this worktree's lock is free or yours, 3 when another holds it",
		Long:        lockCheckLong,
		Example:     lockCheckExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			target := a.openLockTarget()
			status, err := target.lock.Check(token)
			ownToken := ""
			if status.IsHeld {
				ownToken = token
			}
			return a.emitLock(cmd, lockStateName(status.State()), status, ownToken, err)
		}),
	}
	cmd.Flags().StringVar(&token, flagToken, "", "your token; the lock counts as free when you hold it")
	return a.withKV(cmd)
}

func (a *app) newLockStatusCommand() *cobra.Command {
	var isAll bool
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show this worktree's lock, or with --all every worktree's",
		Long:        lockStatusLong,
		Example:     lockStatusExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			overview, err := a.openLockTarget().lock.Overview()
			if err != nil {
				return err
			}
			if isAll {
				return a.emit(cmd, allLocksResult(overview))
			}
			return a.emit(cmd, ownLockResult(overview))
		}),
	}
	cmd.Flags().BoolVar(&isAll, flagAll, false, "show the locks of every worktree")
	return a.withKV(cmd)
}

// lockStatusHint is the hint of a lock command refused by the lock's state: capcom lock status shows
// who holds this worktree's lock, if anyone.
const lockStatusHint = "capcom lock status"

// corruptLockHint is the hint of a command that met a lock record holding no valid lock: capcom lock
// status shows the record as invalid, and capcom lock break discards it.
const corruptLockHint = "capcom lock status, then capcom lock break"

// corruptLockFailure types err, a read of the lock record at path that holds no valid lock
// (lock.ErrCorruptRecord), as invalid: the record cannot be read, and the hint names the commands
// that show and discard it. The message drops err's leading path, as Error prefixes Path itself.
func corruptLockFailure(path string, err error) *output.Error {
	return &output.Error{
		Code:    output.CodeInvalid,
		Message: "the lock record cannot be read: " + strings.TrimPrefix(err.Error(), path+": "),
		Path:    path,
		Hint:    corruptLockHint,
	}
}

// openLockTarget pairs the store the root opened with the current worktree's lock.
func (a *app) openLockTarget() lockTarget {
	return lockTarget{store: a.store, lock: lock.ForWorktree(a.store)}
}

// emitLock renders a lock command's outcome: status in state on success. A lock held by another
// session fails locked, and a touch of a free lock fails not_found; either failure hints capcom
// lock status and still renders the lock as found, so --kv keeps its LOCK= record and closes it
// with the failure's code. A record that holds no valid lock fails invalid (corruptLockFailure).
func (a *app) emitLock(cmd *cobra.Command, state string, status lock.Status, token string, err error) error {
	var held *lock.HeldError
	var notHeld *lock.NotHeldError
	switch {
	case errors.As(err, &held):
		failure := &output.Error{Code: output.CodeLocked, Message: held.Message(), Path: held.Path, Hint: lockStatusHint}
		return a.emit(cmd, failedLockResult(lockStateName(held.Status.State()), held.Status, failure))
	case errors.As(err, &notHeld):
		failure := &output.Error{
			Code:    output.CodeNotFound,
			Message: lock.ErrNotHeld.Error(),
			Path:    notHeld.Path,
			Hint:    lockStatusHint,
		}
		return a.emit(cmd, failedLockResult(lockFree, status, failure))
	case errors.Is(err, lock.ErrCorruptRecord):
		return corruptLockFailure(a.openLockTarget().lock.RecordPath(), err)
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

// ownLockResult is overview's own worktree lock in its state. An invalid or unreadable record of
// its own is reported in that state, with one warning naming the file; another worktree's record
// is not looked at.
func ownLockResult(overview lock.Overview) output.Result {
	data := newLockData(lockStateName(overview.State), overview.Own, "")
	result := output.Result{
		Kind: kindLock,
		Data: data,
		Text: lockText(data),
		KV:   []output.KVRecord{{Pairs: lockPairs(data)}},
	}
	for _, problem := range overview.Problems {
		if problem.Key == overview.Key {
			result.Warnings = append(result.Warnings, problem.Error())
		}
	}
	return result
}

// allLocksResult lists every held lock of overview, one --kv record per lock led by its WORKTREE
// line, with one warning per record that is invalid or unreadable. With none held, --kv writes a
// lone STATUS line.
func allLocksResult(overview lock.Overview) output.Result {
	held := overview.Locks
	data := make([]lockData, 0, len(held))
	records := make([]output.KVRecord, 0, len(held))
	var text strings.Builder
	for _, status := range held {
		entry := newLockData(lockStateName(status.State()), status, "")
		data = append(data, entry)
		pairs := append([]output.KVPair{{Key: kvKeyWorktree, Value: entry.Worktree}}, lockPairs(entry)...)
		records = append(records, output.KVRecord{Pairs: pairs})
		text.WriteString(lockText(entry))
	}
	if len(held) == 0 {
		records = append(records, output.KVRecord{})
		text.WriteString("no locks held\n")
	}
	return output.Result{
		Kind:     kindLock,
		Data:     data,
		Text:     text.String(),
		KV:       records,
		Warnings: errorStrings(overview.Problems),
	}
}

// lockStateName names a lock state as LOCK=, the JSON lock field and the text line give it; it is
// the one mapping every lock command and prime use.
func lockStateName(state lock.State) string {
	switch state {
	case lock.StateHeld:
		return lockHeld
	case lock.StateStale:
		return lockStale
	case lock.StateInvalid:
		return lockInvalid
	case lock.StateUnreadable:
		return lockUnreadable
	default:
		return lockFree
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
