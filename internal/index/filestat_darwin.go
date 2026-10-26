//go:build darwin

package index

import (
	"fmt"
	"io/fs"
	"syscall"
)

// changeStamp returns the change time of the node file info describes, in Unix nanoseconds, and
// its identity, its device and inode, both from info, the walk's lstat of the file at path: an
// edit moves the change time even when it restores the size and mtime, and a rename over the file
// gives it another inode. It reads no file body. A stat it cannot read gives (0, ""), and the
// file is compared by size and mtime alone.
func changeStamp(_ string, info fs.FileInfo) (ctime int64, identity string) {
	stat, isStat := info.Sys().(*syscall.Stat_t)
	if !isStat {
		return 0, ""
	}
	return stat.Ctimespec.Nano(), fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
}
