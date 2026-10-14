package lock

import "errors"

// State names the state of one worktree's Tree lock in a Lock overview.
type State string

// The states of a worktree's Tree lock.
const (
	// StateFree is a worktree nobody holds a lock on.
	StateFree State = "free"
	// StateHeld is a worktree whose lock is held and renewed in time.
	StateHeld State = "held"
	// StateStale is a worktree whose lock is held but not renewed within the TTL.
	StateStale State = "stale"
	// StateInvalid is a worktree whose lock record was read but holds no valid lock, so acquiring
	// it fails.
	StateInvalid State = "invalid"
	// StateUnreadable is a worktree whose lock record or guard cannot be read at all.
	StateUnreadable State = "unreadable"
)

// RecordProblem is one lock record the Lock overview could not read as a lock.
type RecordProblem struct {
	// Key names the record's worktree.
	Key string
	// Path is the record's file.
	Path string
	// State is StateInvalid for a record that holds no valid lock, StateUnreadable for one whose
	// record or guard cannot be read.
	State State
	// Err is the read's error, unchanged; it names the file that failed.
	Err error
}

// Error returns Err's message.
func (p RecordProblem) Error() string {
	return p.Err.Error()
}

// Unwrap returns Err.
func (p RecordProblem) Unwrap() error {
	return p.Err
}

// Overview is the Lock overview: every held lock of the repository, one worktree's state, and one
// problem per record that could not be read as a lock.
type Overview struct {
	// Locks is every held lock, stale or not, ordered by key; the worktree's own is among them.
	Locks []Status
	// Key names the worktree whose state the overview gives.
	Key string
	// State is that worktree's lock state.
	State State
	// Own is that worktree's lock; only Key is set unless State is StateHeld or StateStale.
	Own Status
	// Problems holds one entry per record that is invalid or unreadable, ordered by key, the
	// worktree's own included.
	Problems []RecordProblem
}

// Overview returns the Lock overview of the repository, with key's state. A record that is
// invalid or cannot be read becomes one problem beside the other locks; only a lock directory that
// cannot be listed fails. A missing lock directory gives an empty overview with key free, and
// nothing is created.
func (l *Locks) Overview(key string) (Overview, error) {
	keys, err := l.recordKeys(key)
	if err != nil {
		return Overview{}, err
	}

	overview := Overview{Locks: []Status{}, Key: key, State: StateFree, Own: Status{Key: key}}
	for _, recordKey := range keys {
		status, err := l.Read(recordKey)
		if err != nil {
			problem := RecordProblem{
				Key:   recordKey,
				Path:  l.RecordPath(recordKey),
				State: StateUnreadable,
				Err:   err,
			}
			if errors.Is(err, errCorruptRecord) {
				problem.State = StateInvalid
			}
			overview.Problems = append(overview.Problems, problem)
			if recordKey == key {
				overview.State = problem.State
			}
			continue
		}
		if !status.IsHeld {
			continue
		}
		overview.Locks = append(overview.Locks, status)
		if recordKey == key {
			overview.Own = status
			overview.State = status.State()
		}
	}
	return overview, nil
}

// State names the state of a lock read from a valid record, or of a worktree with no record: free,
// held or stale. Only the Lock overview names an invalid or unreadable record.
func (s Status) State() State {
	switch {
	case !s.IsHeld:
		return StateFree
	case s.IsStale:
		return StateStale
	default:
		return StateHeld
	}
}
