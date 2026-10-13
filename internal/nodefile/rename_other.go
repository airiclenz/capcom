//go:build !windows

package nodefile

import (
	"fmt"
	"os"
)

// replaceRename renames the written file temp over path once: off Windows a reader holding path
// open never blocks the rename. The error names path.
func replaceRename(temp string, path string, _ bool) error {
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
