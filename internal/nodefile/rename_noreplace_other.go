//go:build !linux && !darwin && !windows

package nodefile

import (
	"errors"
	"os"
)

// noReplaceRename always fails with errors.ErrUnsupported in an *os.LinkError: this platform has
// no rename that refuses an existing newPath, so placeExclusive falls back to its copy.
func noReplaceRename(oldPath string, newPath string) error {
	return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: errors.ErrUnsupported}
}
