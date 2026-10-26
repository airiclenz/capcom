// Package lock implements the Tree lock (ADR 0005): one advisory lock per worktree, held by an
// agent session through a token and kept live by a heartbeat. The records of all of a clone's
// worktrees live together under `<git-common-dir>/capcom/locks/<key>.json`, never in git. Every
// read-modify-write of a record runs under an OS file lock on a sibling `<key>.lck` file, which is
// held only for that moment; the record itself is replaced atomically. Beside them, each
// `<key>.<sha256 of purpose>.token` token file keeps the Lock token a session acquired with that
// purpose (ADR 0016), so a later command resumes or releases the lock without being handed it.
package lock

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// Layout of the lock directory under the git common dir.
const (
	capcomDir       = "capcom"
	locksDir        = "locks"
	recordExtension = ".json"
	guardExtension  = ".lck"
	hiddenPrefix    = "."
)

// Worktree keys: the primary worktree's record is `_main`, a linked worktree's `wt-<name>`, so a
// linked worktree named `_main` still gets a record of its own.
const (
	MainWorktreeKey      = "_main"
	linkedWorktreePrefix = "wt-"
)

const (
	tokenBytes       = 16
	dirPermissions   = 0o755
	guardPermissions = 0o644
)

// ErrNotHeld reports a touch of a worktree whose lock nobody holds.
var ErrNotHeld = errors.New("no lock is held on this worktree")

// errCorruptRecord marks a record file that was read but holds no lock: it does not parse, or it
// carries no token. Break discards such a record and the Lock overview reports it as invalid; an
// I/O error reading a record never matches it.
var errCorruptRecord = errors.New("invalid lock record")

// Record is one worktree's lock as stored on disk.
type Record struct {
	// Token is the Lock token the holder presents to touch, release and re-acquire. It tells
	// sessions apart, and a token file beside the record keeps it in the open (ADR 0016).
	Token string `json:"token"`
	// Purpose says what the holder is doing.
	Purpose string `json:"purpose"`
	// Holder names the session holding the lock.
	Holder string `json:"holder"`
	// Host, User and Branch describe where the lock was acquired.
	Host   string `json:"host"`
	User   string `json:"user"`
	Branch string `json:"branch"`
	// Since is when the lock was acquired; Heartbeat when it was last acquired, resumed or touched.
	Since     time.Time `json:"since"`
	Heartbeat time.Time `json:"heartbeat"`
}

// Claim describes a session asking for a lock.
type Claim struct {
	Purpose string
	Holder  string
	Host    string
	User    string
	Branch  string
}

// Status is the state of one worktree's lock.
type Status struct {
	// Key names the worktree: MainWorktreeKey or `wt-<linked worktree name>`.
	Key string
	// Record is the lock record; zero when IsHeld is false.
	Record Record
	// IsHeld reports whether a record exists, stale or not.
	IsHeld bool
	// IsStale reports a held lock whose heartbeat is older than the TTL.
	IsStale bool
	// IsCorrupt reports a record that held no valid lock and was discarded; only Break sets it,
	// and IsHeld is then false.
	IsCorrupt bool
}

// NotHeldError reports a touch of a free lock; it matches ErrNotHeld under errors.Is.
type NotHeldError struct {
	// Path is the lock record's file.
	Path string
}

// Error names the record file, then gives ErrNotHeld's message.
func (e *NotHeldError) Error() string {
	return e.Path + ": " + ErrNotHeld.Error()
}

// Unwrap returns ErrNotHeld.
func (e *NotHeldError) Unwrap() error {
	return ErrNotHeld
}

// HeldError reports a lock held by a session other than the caller.
type HeldError struct {
	// Path is the lock record's file.
	Path string
	// Status is the lock as found.
	Status Status
	// Age is how long the lock has been held.
	Age time.Duration
	// HeartbeatAge is how long ago the holder last refreshed it.
	HeartbeatAge time.Duration
}

// Error names the record file, then gives Message.
func (e *HeldError) Error() string {
	return e.Path + ": " + e.Message()
}

// Message names the holder, the purpose, the age and whether the lock is stale.
func (e *HeldError) Message() string {
	stale := ""
	if e.Status.IsStale {
		stale = ", stale"
	}
	record := e.Status.Record
	return fmt.Sprintf(
		"locked by %s (%s) for %s, heartbeat %s ago%s",
		record.Holder,
		record.Purpose,
		e.Age.Round(time.Second),
		e.HeartbeatAge.Round(time.Second),
		stale,
	)
}

// Locks is the lock directory of one repository.
type Locks struct {
	dir string
	ttl time.Duration
}

// Open returns the locks kept under gitCommonDir; a lock whose heartbeat is older than ttl is
// stale. Nothing is touched on disk until a lock is read or written.
func Open(gitCommonDir string, ttl time.Duration) *Locks {
	return &Locks{dir: filepath.Join(gitCommonDir, capcomDir, locksDir), ttl: ttl}
}

// WorktreeKey returns the lock key of the worktree whose git dir is gitDir: MainWorktreeKey for
// the primary worktree (its git dir is the common dir), else `wt-` plus the linked worktree's
// name, which is the last element of its git dir (`<common>/worktrees/<name>`).
func WorktreeKey(gitDir, gitCommonDir string) string {
	if filepath.Clean(gitDir) == filepath.Clean(gitCommonDir) {
		return MainWorktreeKey
	}
	return linkedWorktreePrefix + filepath.Base(gitDir)
}

// RecordPath returns the file that holds key's lock record.
func (l *Locks) RecordPath(key string) string {
	return filepath.Join(l.dir, key+recordExtension)
}

// Acquire takes key's lock for claim. A free lock gets a new token. When token matches the
// holder's, the lock is resumed, even a stale one: its heartbeat is refreshed, its purpose and
// holder replaced by claim's, and isResumed is true. A lock another session holds fails with a
// *HeldError, unless isTakeStale is set and that lock is stale, in which case it is taken over
// with a new token. The lock's token is kept in the token file for claim's purpose, written before
// the record, so a failed write takes no lock; an empty purpose keeps no token. A resume moves the
// token file: every other token file of key keeping the token is deleted, so one token has one
// token file and a session naming the purpose it was kept for before fails like any other.
func (l *Locks) Acquire(key string, claim Claim, token string, isTakeStale bool) (Status, bool, error) {
	var acquired Status
	isResumed := false
	err := l.withGuard(key, func(now time.Time) error {
		current, err := l.read(key, now)
		if err != nil {
			return err
		}

		record := Record{
			Purpose:   claim.Purpose,
			Holder:    claim.Holder,
			Host:      claim.Host,
			User:      claim.User,
			Branch:    claim.Branch,
			Since:     now,
			Heartbeat: now,
		}
		switch {
		case current.IsHeld && token != "" && current.Record.Token == token:
			isResumed = true
			record.Token = token
			record.Since = current.Record.Since
		case current.IsHeld && !(isTakeStale && current.IsStale):
			return l.heldError(current, now)
		default:
			record.Token, err = newToken()
			if err != nil {
				return err
			}
		}
		if err := l.writeTokenFile(key, record.Token, claim.Purpose); err != nil {
			return err
		}
		if isResumed {
			if err := l.removeTokenFiles(key, token, l.keptTokenPath(key, claim.Purpose)); err != nil {
				return err
			}
		}
		if err := l.write(key, record); err != nil {
			return err
		}
		acquired = Status{Key: key, Record: record, IsHeld: true}
		return nil
	})
	return acquired, isResumed, err
}

// Touch refreshes the heartbeat of key's lock for the holder of token. It fails with a
// *NotHeldError, returning the free lock, when nobody holds it, and with a *HeldError when another
// session holds it.
func (l *Locks) Touch(key, token string) (Status, error) {
	var touched Status
	err := l.withGuard(key, func(now time.Time) error {
		current, err := l.read(key, now)
		if err != nil {
			return err
		}
		if !current.IsHeld {
			touched = current
			return &NotHeldError{Path: l.RecordPath(key)}
		}
		if current.Record.Token != token {
			return l.heldError(current, now)
		}

		current.Record.Heartbeat = now
		if err := l.write(key, current.Record); err != nil {
			return err
		}
		touched = Status{Key: key, Record: current.Record, IsHeld: true}
		return nil
	})
	return touched, err
}

// Release frees key's lock for the holder of token and returns the lock as it was, then deletes
// every token file of key that keeps token. Releasing a free lock succeeds and deletes those token
// files too; a lock another session holds fails with a *HeldError and deletes nothing.
func (l *Locks) Release(key, token string) (Status, error) {
	var released Status
	err := l.withGuard(key, func(now time.Time) error {
		current, err := l.read(key, now)
		if err != nil {
			released = current
			return err
		}
		if current.IsHeld && current.Record.Token != token {
			return l.heldError(current, now)
		}
		released = current
		if current.IsHeld {
			if err := l.remove(key); err != nil {
				return err
			}
		}
		return l.removeTokenFiles(key, token, "")
	})
	return released, err
}

// Break frees key's lock whoever holds it, deletes every token file of key, and returns the lock as
// it was. A record that holds no valid lock is discarded too, and the free Status returned says so
// with IsCorrupt; a record that cannot be read at all fails and is left in place, its token files
// with it.
func (l *Locks) Break(key string) (Status, error) {
	var broken Status
	err := l.withGuard(key, func(now time.Time) error {
		current, err := l.read(key, now)
		switch {
		case errors.Is(err, errCorruptRecord):
			current = Status{Key: key, IsCorrupt: true}
		case err != nil:
			broken = current
			return err
		}
		broken = current
		if err := l.remove(key); err != nil {
			return err
		}
		return l.removeAllTokenFiles(key)
	})
	return broken, err
}

// Read returns the state of key's lock. It reads without the guard, as a record is replaced
// atomically, and creates nothing.
func (l *Locks) Read(key string) (Status, error) {
	return l.read(key, time.Now().UTC())
}

// Check returns key's lock when it is free or held by token, and fails with a *HeldError when
// another session holds it, stale or not; an empty token holds nothing. It reads without the guard,
// as a record is replaced atomically, and creates nothing.
func (l *Locks) Check(key, token string) (Status, error) {
	now := time.Now().UTC()
	current, err := l.read(key, now)
	if err != nil {
		return Status{}, err
	}
	if current.IsHeld && (token == "" || current.Record.Token != token) {
		return Status{}, l.heldError(current, now)
	}
	return current, nil
}

// recordKeys returns the key of every record file in the lock directory, ordered. It skips hidden
// files and directories, except that ownKey's entry is kept even when it is a directory, so reading
// it reports the problem; an empty ownKey keeps no directory. A missing lock directory has no keys
// and is not created; only a lock directory that cannot be listed fails.
func (l *Locks) recordKeys(ownKey string) ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if errors.Is(err, fs.ErrNotExist) {
		// Windows reports a file in the directory's place as not found too.
		if _, statErr := os.Lstat(l.dir); errors.Is(statErr, fs.ErrNotExist) {
			return []string{}, nil
		}
		err = errors.New("not a directory")
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", l.dir, err)
	}

	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		key, isRecord := strings.CutSuffix(name, recordExtension)
		isKept := !entry.IsDir() || (ownKey != "" && key == ownKey)
		if isRecord && isKept && !strings.HasPrefix(name, hiddenPrefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// withGuard runs body while holding the OS file lock on key's guard file, passing it the current
// time. The guard is held only for the call.
func (l *Locks) withGuard(key string, body func(now time.Time) error) error {
	if err := os.MkdirAll(l.dir, dirPermissions); err != nil {
		return fmt.Errorf("%s: %w", l.dir, err)
	}
	guardPath := filepath.Join(l.dir, key+guardExtension)
	guard, err := os.OpenFile(guardPath, os.O_RDWR|os.O_CREATE, guardPermissions)
	if err != nil {
		return fmt.Errorf("%s: %w", guardPath, err)
	}
	if err := lockFile(guard); err != nil {
		closeErr := guard.Close()
		return fmt.Errorf("%s: locking: %w", guardPath, errors.Join(err, closeErr))
	}

	bodyErr := body(time.Now().UTC())
	unlockErr := unlockFile(guard)
	closeErr := guard.Close()
	if bodyErr != nil {
		return bodyErr
	}
	if err := errors.Join(unlockErr, closeErr); err != nil {
		return fmt.Errorf("%s: unlocking: %w", guardPath, err)
	}
	return nil
}

// read loads key's record, if any, and judges its staleness at now. A record that does not parse or
// carries no token fails wrapping errCorruptRecord. Writers call it holding the guard; readers
// call it without, as write replaces a record by an atomic rename, so they see a whole record.
func (l *Locks) read(key string, now time.Time) (Status, error) {
	path := l.RecordPath(key)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Status{Key: key}, nil
	}
	if err != nil {
		return Status{}, fmt.Errorf("%s: %w", path, err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Status{}, fmt.Errorf("%s: %w: %w", path, errCorruptRecord, err)
	}
	if record.Token == "" {
		return Status{}, fmt.Errorf("%s: %w: no token", path, errCorruptRecord)
	}
	return Status{
		Key:     key,
		Record:  record,
		IsHeld:  true,
		IsStale: now.Sub(record.Heartbeat) > l.ttl,
	}, nil
}

// write replaces key's record atomically. The caller holds the guard.
func (l *Locks) write(key string, record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", l.RecordPath(key), err)
	}
	data = append(data, '\n')
	return nodefile.WriteFileAtomic(l.RecordPath(key), bytes.NewReader(data))
}

// remove deletes key's record. The caller holds the guard.
func (l *Locks) remove(key string) error {
	return removeFile(l.RecordPath(key))
}

// heldError reports current, held by another session, as seen at now.
func (l *Locks) heldError(current Status, now time.Time) *HeldError {
	return &HeldError{
		Path:         l.RecordPath(current.Key),
		Status:       current,
		Age:          now.Sub(current.Record.Since),
		HeartbeatAge: now.Sub(current.Record.Heartbeat),
	}
}

// newToken returns a fresh 128-bit random token in hex.
func newToken() (string, error) {
	buffer := make([]byte, tokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generating lock token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
