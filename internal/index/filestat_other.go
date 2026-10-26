//go:build !linux && !darwin && !windows

package index

import "io/fs"

// changeStamp returns (0, ""): this system's change time and file identity are not read, so a
// node file is compared by size and mtime alone.
func changeStamp(_ string, _ fs.FileInfo) (ctime int64, identity string) {
	return 0, ""
}
