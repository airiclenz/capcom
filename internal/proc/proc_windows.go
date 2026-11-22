package proc

import (
	"errors"
	"fmt"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

// listedProcess is one process of a toolhelp snapshot.
type listedProcess struct {
	parentPID uint32
	name      string
}

// openReader takes one toolhelp snapshot of every process and returns a reader over it, which
// adds each process's creation time from GetProcessTimes.
func openReader() (reader, error) {
	listed, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	read := func(pid int) (entry, error) {
		process, ok := listed[uint32(pid)]
		if !ok {
			return entry{}, fmt.Errorf("process %d: not in the snapshot", pid)
		}
		created, err := creationTime(uint32(pid))
		if err != nil {
			return entry{}, fmt.Errorf("process %d: creation time: %w", pid, err)
		}
		return entry{
			process:   Process{PID: pid, Started: strconv.FormatInt(created, 10), Name: process.name},
			parentPID: int(process.parentPID),
			order:     created,
		}, nil
	}
	return read, nil
}

// snapshotProcesses lists every process by PID from a toolhelp snapshot.
func snapshotProcesses() (map[uint32]listedProcess, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("process snapshot: %w", err)
	}
	// The snapshot is only read: a failure to close it changes nothing it listed.
	defer windows.CloseHandle(snapshot)
	listed := make(map[uint32]listedProcess)
	var current windows.ProcessEntry32
	current.Size = uint32(unsafe.Sizeof(current))
	err = windows.Process32First(snapshot, &current)
	for err == nil {
		listed[current.ProcessID] = listedProcess{
			parentPID: current.ParentProcessID,
			name:      windows.UTF16ToString(current.ExeFile[:]),
		}
		err = windows.Process32Next(snapshot, &current)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("process snapshot: %w", err)
	}
	return listed, nil
}

// creationTime returns the creation time of the process with the given PID, in nanoseconds
// since the Unix epoch.
func creationTime(pid uint32) (int64, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, err
	}
	// The handle only queried times: a failure to close it changes nothing they say.
	defer windows.CloseHandle(handle)
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	return created.Nanoseconds(), nil
}
