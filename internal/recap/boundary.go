package recap

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/gitx"
)

// The forms a since value takes other than a git ref.
const (
	// sinceDateLayout is the layout of a since date, read as local midnight.
	sinceDateLayout = "2006-01-02"
	// sinceOptionPrefix starts a value git would read as an option, never a ref.
	sinceOptionPrefix = "-"
)

// sinceDaysPattern matches a since day count, <n>d.
var sinceDaysPattern = regexp.MustCompile(`^([0-9]+)d$`)

// Boundary resolves since, in the repository at worktreeRoot, to the commit a recap starts after:
// for a date YYYY-MM-DD (local midnight) or a day count <n>d (n days before now), the last commit on
// HEAD at or before that time, "" when there is none; else since names a commit itself. A value git
// would read as an option, or one naming no commit, fails wrapping ErrUnknownBoundary; a git failure
// is returned as it is.
func Boundary(worktreeRoot, since string, now time.Time) (string, error) {
	start, isTime := sinceTime(since, now)
	if isTime {
		return gitx.CommitBefore(worktreeRoot, start)
	}
	if strings.HasPrefix(since, sinceOptionPrefix) {
		return "", fmt.Errorf("%s: %w", since, ErrUnknownBoundary)
	}
	commit, err := gitx.ResolveCommit(worktreeRoot, since)
	if err != nil {
		return "", err
	}
	if commit == "" {
		return "", fmt.Errorf("%s: %w", since, ErrUnknownBoundary)
	}
	return commit, nil
}

// sinceTime reads since as a date, local midnight, or as <n>d, n days before now. isTime is false
// when since is neither, so it is a git ref.
func sinceTime(since string, now time.Time) (start time.Time, isTime bool) {
	if date, err := time.ParseInLocation(sinceDateLayout, since, time.Local); err == nil {
		return date, true
	}
	match := sinceDaysPattern.FindStringSubmatch(since)
	if match == nil {
		return time.Time{}, false
	}
	days, err := strconv.Atoi(match[1])
	if err != nil {
		return time.Time{}, false
	}
	return now.AddDate(0, 0, -days), true
}
