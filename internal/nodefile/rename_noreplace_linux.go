package nodefile

import (
	"os"

	"golang.org/x/sys/unix"
)

// noReplaceRename renames oldPath to newPath with renameat2's RENAME_NOREPLACE, which the kernel
// refuses atomically, with EEXIST (an error wrapping fs.ErrExist), when newPath exists. A kernel or
// file system without the flag fails with EINVAL or ENOSYS. The error is an *os.LinkError.
func noReplaceRename(oldPath string, newPath string) error {
	err := unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: err}
	}
	return nil
}
