// Package proc reads the calling process's ancestry and picks its harness anchor: the nearest
// ancestor that is not a shell or a wrapper command. Callers never see how an operating system
// names or stamps a process.
package proc

import (
	"os"
	"strings"
)

// maxAncestry bounds the walk up the parent chain, so a cycle in what the operating system
// reports can never loop forever.
const maxAncestry = 64

// Process is one process of an ancestry.
type Process struct {
	// PID is the process id.
	PID int
	// Started is an opaque start stamp: two Processes with equal PID and Started are the same
	// process, while a reused PID gets a different stamp. Its form differs per operating system,
	// so it is only ever compared, never parsed.
	Started string
	// Name is the executable's name as the operating system reports it, which may be truncated
	// (Linux keeps 15 bytes, macOS 16).
	Name string
}

// entry is one process as a reader sees it: the Process, its parent's PID and a start order
// that sorts an older process first.
type entry struct {
	process   Process
	parentPID int
	order     int64
}

// reader reads the process with the given PID; it fails when that process is gone or
// unreadable.
type reader func(pid int) (entry, error)

// wrapperNames are the shells and wrapper commands Anchor skips, lowercased and without `.exe`.
var wrapperNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "mksh": true, "ash": true,
	"fish": true, "tcsh": true, "csh": true, "busybox": true, "pwsh": true, "powershell": true,
	"cmd": true, "env": true, "timeout": true, "nice": true, "nohup": true, "xargs": true,
	"stdbuf": true, "time": true, "sudo": true, "doas": true,
}

// Ancestry returns the calling process's ancestors, its parent first. The walk stops before
// PID 1 or below, after 64 ancestors, at the first ancestor it cannot read, and at an ancestor
// that started after its child (a PID reused after the real parent exited). It works on Linux,
// macOS and Windows and returns nil on any other system or when even the caller cannot be read.
func Ancestry() []Process {
	read, err := openReader()
	if err != nil {
		// An unreadable ancestry is an empty one: callers treat it as "no anchor", never as a
		// failure of the command they run.
		return nil
	}
	return walk(os.Getpid(), read)
}

// walk returns the ancestors of the process with the given PID, read with read, under the
// stopping rules Ancestry documents.
func walk(pid int, read reader) []Process {
	child, err := read(pid)
	if err != nil {
		return nil
	}
	var chain []Process
	for len(chain) < maxAncestry && child.parentPID > 1 {
		parent, err := read(child.parentPID)
		if err != nil || parent.order > child.order {
			break
		}
		chain = append(chain, parent.process)
		child = parent
	}
	return chain
}

// Anchor returns the first Process of chain that is not a shell or a wrapper command, matched
// on its lowercased base name with any `.exe` suffix removed; false when every Process of chain
// is one, or chain is empty.
func Anchor(chain []Process) (Process, bool) {
	for _, candidate := range chain {
		if !isWrapper(candidate.Name) {
			return candidate, true
		}
	}
	return Process{}, false
}

// isWrapper reports whether name, a bare name or a path with either separator, names a shell
// or a wrapper command.
func isWrapper(name string) bool {
	base := strings.ToLower(name[strings.LastIndexAny(name, `/\`)+1:])
	return wrapperNames[strings.TrimSuffix(base, ".exe")]
}
