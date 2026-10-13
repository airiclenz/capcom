//go:build windows

package nodefile

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// renameBackoff is the wait before each retry of a rename a reader blocks: with the first try,
// five attempts in all.
var renameBackoff = [...]time.Duration{
	10 * time.Millisecond,
	20 * time.Millisecond,
	40 * time.Millisecond,
	80 * time.Millisecond,
}

// beforeRenameRetry, when set, is called with path after each backoff wait, before the target is
// rechecked and the rename retried, so a test can change the target there. It is nil in
// production.
var beforeRenameRetry func(path string)

// replaceRename renames the written file temp over path. Windows refuses to replace a file another
// process holds open, with ERROR_ACCESS_DENIED or ERROR_SHARING_VIOLATION, so such a failure is
// retried after each wait in renameBackoff; the last failure is returned. When isReplacing, path
// is rechecked before each retry, and a path gone meanwhile fails with checkReplaceTarget's error
// instead of being recreated. Every error names path.
func replaceRename(temp string, path string, isReplacing bool) error {
	err := os.Rename(temp, path)
	for _, wait := range renameBackoff {
		if !isHeldOpen(err) {
			break
		}
		time.Sleep(wait)
		if beforeRenameRetry != nil {
			beforeRenameRetry(path)
		}
		if isReplacing {
			if checkErr := checkReplaceTarget(path); checkErr != nil {
				return checkErr
			}
		}
		err = os.Rename(temp, path)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// isHeldOpen reports whether err is Windows refusing a rename because another process has the
// target open.
func isHeldOpen(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
