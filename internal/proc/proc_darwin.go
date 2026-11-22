package proc

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// openReader returns the reader of the kern.proc.pid sysctl.
func openReader() (reader, error) {
	return readKinfo, nil
}

// readKinfo reads the process with the given PID through the kern.proc.pid sysctl. Its start
// order is the start time in microseconds since the epoch.
func readKinfo(pid int) (entry, error) {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return entry{}, err
	}
	started := info.Proc.P_starttime
	order := int64(started.Sec)*1_000_000 + int64(started.Usec)
	return entry{
		process: Process{
			PID:     pid,
			Started: strconv.FormatInt(order, 10),
			Name:    unix.ByteSliceToString(info.Proc.P_comm[:]),
		},
		parentPID: int(info.Eproc.Ppid),
		order:     order,
	}, nil
}
