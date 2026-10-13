package nodefile

import (
	"os"

	"golang.org/x/sys/unix"
)

// noReplaceRename renames oldPath to newPath with renamex_np's RENAME_EXCL, which the kernel
// refuses atomically, with EEXIST (an error wrapping fs.ErrExist), when newPath exists. A file
// system without the flag fails with ENOTSUP. The error is an *os.LinkError.
func noReplaceRename(oldPath string, newPath string) error {
	err := unix.RenameatxNp(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_EXCL)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: err}
	}
	return nil
}
