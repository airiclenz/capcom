//go:build unix

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// endedGroupPrefix opens a ledger line recording that a group listed before has no process left.
const endedGroupPrefix = "-"

// processGroupLedger is the file, named by AGENTEVAL_PROCESS_GROUPS, where the agent loop lists
// the process group of every shell command it starts, one decimal pgid per line, and a line
// -<pgid> once such a group has no process left. The harness's sweep reads it to reach the groups
// outside the agent's own. Empty, it records nothing.
type processGroupLedger string

// shellArgv is the argv that runs command through the POSIX shell.
func shellArgv(command string) []string {
	return []string{"sh", "-c", command}
}

// isolateProcessGroup starts process in a process group of its own and has its cancellation kill
// that whole group, so a background job or a child holding the output pipe dies with it.
func isolateProcessGroup(process *exec.Cmd) {
	process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	process.Cancel = func() error {
		return syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
	}
}

// isolateAgentProcessGroup starts agent in a process group of its own and has its cancellation
// send that whole group SIGTERM, so the agent and everything it started can wind down. The sweep
// it returns sends SIGKILL to whatever is still in the group and to every group ledger lists that
// is not recorded as ended: the shell commands an agenteval agent loop started in groups of their
// own, which a loop that outlives the wait delay leaves running. Call it once agent has returned,
// so no process the agent or its loop started outlives the run. The sweep does nothing for an
// agent that never started, and a group already empty is no error. A listed group that ended
// without a -<pgid> line, because the loop was killed before writing it or a background job kept
// the group alive past the loop's probe, leaves a window: should the system hand that pgid to a
// new group before the sweep, the sweep kills that group.
func isolateAgentProcessGroup(agent *exec.Cmd, ledger processGroupLedger) (sweep func() error) {
	agent.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	agent.Cancel = func() error {
		return syscall.Kill(-agent.Process.Pid, syscall.SIGTERM)
	}
	return func() error {
		if agent.Process == nil {
			return nil
		}
		failures := []error{killProcessGroup(agent.Process.Pid)}
		listed, err := ledger.liveGroups()
		failures = append(failures, err)
		for _, pgid := range listed {
			failures = append(failures, killProcessGroup(pgid))
		}
		return errors.Join(failures...)
	}
}

// killProcessGroup sends SIGKILL to every process in the group pgid; a group already empty is no
// error.
func killProcessGroup(pgid int) error {
	err := syscall.Kill(-pgid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// recordStarted appends pgid, the group a started shell command leads, to the ledger.
func (l processGroupLedger) recordStarted(pgid int) error {
	return l.append(strconv.Itoa(pgid))
}

// recordEnded appends -<pgid> to the ledger when no process is left in the group pgid, so the
// sweep never kills a later group the system hands the same pgid; a group with a process left
// in it, a background job of the command, stays listed for the sweep.
func (l processGroupLedger) recordEnded(pgid int) error {
	if !errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
		return nil
	}
	return l.append(endedGroupPrefix + strconv.Itoa(pgid))
}

// append adds line to the ledger in a single write, creating the file when it does not exist yet,
// so lines from concurrent writers never interleave. An empty ledger appends nothing.
func (l processGroupLedger) append(line string) error {
	if l == "" {
		return nil
	}
	file, err := os.OpenFile(string(l), os.O_APPEND|os.O_CREATE|os.O_WRONLY, processGroupsFileMode)
	if err != nil {
		return fmt.Errorf("record the process group: %w", err)
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		return errors.Join(fmt.Errorf("record the process group: %w", err), file.Close())
	}
	return file.Close()
}

// liveGroups returns the groups the ledger lists and does not record as ended, in the order
// listed. It counts only lines that end in a newline, so a line torn by a writer killed mid-write
// is skipped, and skips a line that is no pgid, a pgid of 1 or less (0 and -1 address the caller's
// own group and every process, not one group) and the harness's own group. A ledger that does not
// exist lists nothing.
func (l processGroupLedger) liveGroups() ([]int, error) {
	if l == "" {
		return nil, nil
	}
	text, err := os.ReadFile(string(l))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the process group ledger: %w", err)
	}
	complete := text[:strings.LastIndex(string(text), "\n")+1]

	var listed []int
	ended := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(complete), "\n"), "\n") {
		ending, isEnded := strings.CutPrefix(line, endedGroupPrefix)
		pgid, err := strconv.Atoi(ending)
		if err != nil {
			continue
		}
		if isEnded {
			ended[pgid] = true
			continue
		}
		listed = append(listed, pgid)
	}

	own := syscall.Getpgrp()
	live := make([]int, 0, len(listed))
	for _, pgid := range listed {
		if pgid > 1 && pgid != own && !ended[pgid] {
			live = append(live, pgid)
		}
	}
	return live, nil
}
