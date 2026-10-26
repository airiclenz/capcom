//go:build windows

package index

import (
	"io/fs"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileBasicInfo is the FILE_BASIC_INFO that GetFileInformationByHandleEx fills for the
// FileBasicInfo class, which golang.org/x/sys/windows does not define. Its times are FILETIMEs,
// 100-nanosecond intervals since 1601.
type fileBasicInfo struct {
	CreationTime   int64
	LastAccessTime int64
	LastWriteTime  int64
	ChangeTime     int64
	FileAttributes uint32
	_              uint32
}

// changeStamp returns the NTFS change time of the node file at path, in Unix nanoseconds, and an
// empty identity: an edit moves the change time even when it restores the size and mtime. It
// opens the file for its attributes only, sharing it with every other access, and reads no file
// body. A file it cannot open or query gives (0, ""), and the file is compared by size and mtime
// alone.
func changeStamp(path string, _ fs.FileInfo) (ctime int64, identity string) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, ""
	}
	handle, err := windows.CreateFile(
		name,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return 0, ""
	}
	// The handle only read attributes: a failure to close it changes nothing the stamp says.
	defer windows.CloseHandle(handle)
	var info fileBasicInfo
	err = windows.GetFileInformationByHandleEx(
		handle, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		return 0, ""
	}
	changed := windows.Filetime{
		LowDateTime:  uint32(info.ChangeTime),
		HighDateTime: uint32(info.ChangeTime >> 32),
	}
	return changed.Nanoseconds(), ""
}
