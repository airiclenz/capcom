package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// maxConflictExit is the highest exit code git merge-file uses to count conflicts; it exits 255
// when the merge itself fails.
const maxConflictExit = 127

// MergeLabels are the names git merge-file writes after the conflict markers: Ours after
// `<<<<<<<`, Base after `|||||||` (in the diff3 style) and Theirs after `>>>>>>>`.
type MergeLabels struct {
	Base   string
	Ours   string
	Theirs string
}

// MergeFile merges the files at oursPath and theirsPath against their common ancestor at basePath
// with `git merge-file -p` and returns the merged bytes and whether they hold conflict markers,
// labelled with labels. It reads the three files and writes none of them. A conflict is not an
// error; a merge git cannot do (a missing or unreadable input) fails with an *Error carrying git's
// stderr and returns no bytes. It needs no repository.
func MergeFile(basePath, oursPath, theirsPath string, labels MergeLabels) ([]byte, bool, error) {
	args := []string{
		"merge-file", "-p",
		"-L", labels.Ours, "-L", labels.Base, "-L", labels.Theirs,
		oursPath, basePath, theirsPath,
	}
	command := exec.Command(gitProgram, args...)
	command.Env = append(os.Environ(), optionalLocksOff)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	if err == nil {
		return stdout.Bytes(), false, nil
	}
	var exitFailure *exec.ExitError
	if !errors.As(err, &exitFailure) {
		return nil, false, fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
	}
	// Unlike Run, a non-zero exit here can be a result: merge-file exits with the number of
	// conflicts it left, and its stdout then holds the merge with their markers.
	if code := exitFailure.ExitCode(); code > 0 && code <= maxConflictExit {
		return stdout.Bytes(), true, nil
	}
	return nil, false, &Error{Args: args, Stderr: stderr.String(), Err: err}
}
