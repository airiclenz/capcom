package proc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Field positions in /proc/<pid>/stat counted after the comm's closing parenthesis, where the
// state (field 3) is position 0.
const (
	statParentField  = 1  // field 4, ppid
	statStartedField = 19 // field 22, starttime in clock ticks since boot
)

// openReader returns the reader of /proc.
func openReader() (reader, error) {
	return readStat, nil
}

// readStat reads the process with the given PID from /proc/<pid>/stat.
func readStat(pid int) (entry, error) {
	path := "/proc/" + strconv.Itoa(pid) + "/stat"
	data, err := os.ReadFile(path)
	if err != nil {
		return entry{}, err
	}
	return parseStat(pid, string(data))
}

// parseStat parses the content of /proc/<pid>/stat. The comm sits in parentheses and may hold
// spaces and parentheses itself, so the fields are split after its last closing parenthesis.
func parseStat(pid int, stat string) (entry, error) {
	open := strings.IndexByte(stat, '(')
	closing := strings.LastIndexByte(stat, ')')
	if open < 0 || closing < open {
		return entry{}, fmt.Errorf("stat of process %d: no comm in parentheses", pid)
	}
	fields := strings.Fields(stat[closing+1:])
	if len(fields) <= statStartedField {
		return entry{}, fmt.Errorf("stat of process %d: %d fields after the comm", pid, len(fields))
	}
	parentPID, err := strconv.Atoi(fields[statParentField])
	if err != nil {
		return entry{}, fmt.Errorf("stat of process %d: parent: %w", pid, err)
	}
	started := fields[statStartedField]
	order, err := strconv.ParseInt(started, 10, 64)
	if err != nil {
		return entry{}, fmt.Errorf("stat of process %d: start time: %w", pid, err)
	}
	return entry{
		process:   Process{PID: pid, Started: started, Name: stat[open+1 : closing]},
		parentPID: parentPID,
		order:     order,
	}, nil
}
