package nodefile

import (
	"os"

	"golang.org/x/sys/windows"
)

// noReplaceRename renames oldPath to newPath with MoveFileEx without MOVEFILE_REPLACE_EXISTING,
// which Windows refuses atomically, with ERROR_ALREADY_EXISTS or ERROR_FILE_EXISTS (errors wrapping
// fs.ErrExist), when newPath exists. The paths are passed as they are, without the long-path
// prefix os adds: a path past MAX_PATH fails here and placeExclusive falls back to its copy. The
// error is an *os.LinkError.
func noReplaceRename(oldPath string, newPath string) error {
	from, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: err}
	}
	to, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: err}
	}
	if err := windows.MoveFileEx(from, to, 0); err != nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: err}
	}
	return nil
}
