// Package gitx runs the read-only git commands capcom needs. capcom never commits and never runs a
// writing git command (ADR 0004), so every call here only reads, and runs with
// GIT_OPTIONAL_LOCKS=0 so a read never takes the index lock a concurrent git command needs.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	gitProgram       = "git"
	optionalLocksOff = "GIT_OPTIONAL_LOCKS=0"
	discoverOutputs  = 3
)

// Sources of an author name, in the order AuthorName tries them.
const (
	authorNameVariable = "GIT_AUTHOR_NAME"
	userNameKey        = "user.name"
	userVariable       = "USER"
)

// ErrNoAuthor reports that no author name is configured anywhere AuthorName looks.
var ErrNoAuthor = errors.New("no author name: set GIT_AUTHOR_NAME, git config user.name or USER")

// ErrNotWorkTree reports a directory outside any git working tree: not in a repository at all, or
// inside a bare repository or a .git directory.
var ErrNotWorkTree = errors.New("not inside a git working tree")

// Error is a git command that ran and failed. Stderr holds git's own explanation.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

// Error renders the command and git's message.
func (e *Error) Error() string {
	message := strings.TrimSpace(e.Stderr)
	if message == "" {
		message = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), message)
}

// Unwrap exposes the process error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Paths locates the git working tree a directory belongs to. Every path is absolute and cleaned;
// WorktreeRoot also has its symlinks resolved.
type Paths struct {
	// WorktreeRoot is the top directory of the working tree.
	WorktreeRoot string
	// GitDir is this worktree's git directory (differs from GitCommonDir in a linked worktree).
	GitDir string
	// GitCommonDir is the git directory shared by every worktree of the repository.
	GitCommonDir string
}

// Discover finds the working tree containing dir. It fails with an error wrapping ErrNotWorkTree
// when dir is outside one; any other failure (git missing, dir unreadable) is returned as is.
func Discover(dir string) (Paths, error) {
	output, err := Run(
		dir,
		"rev-parse", "--path-format=absolute", "--show-toplevel", "--git-dir", "--git-common-dir",
	)
	if err != nil {
		var gitFailure *Error
		if errors.As(err, &gitFailure) {
			return Paths{}, fmt.Errorf("%s: %w (%s)", dir, ErrNotWorkTree, strings.TrimSpace(gitFailure.Stderr))
		}
		return Paths{}, err
	}

	lines := strings.Split(strings.TrimRight(output, "\r\n"), "\n")
	if len(lines) != discoverOutputs {
		return Paths{}, fmt.Errorf("git rev-parse in %s: unexpected output %q", dir, output)
	}
	worktreeRoot, err := filepath.EvalSymlinks(cleanPath(lines[0]))
	if err != nil {
		return Paths{}, fmt.Errorf("resolve worktree root: %w", err)
	}
	return Paths{
		WorktreeRoot: worktreeRoot,
		GitDir:       cleanPath(lines[1]),
		GitCommonDir: cleanPath(lines[2]),
	}, nil
}

// Run runs a read-only git command in dir and returns its stdout. A command that exits non-zero
// fails with an *Error carrying git's stderr. Callers must pass read-only commands only.
func Run(dir string, args ...string) (string, error) {
	command := exec.Command(gitProgram, args...)
	command.Dir = dir
	command.Env = append(os.Environ(), optionalLocksOff)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		var exitFailure *exec.ExitError
		if errors.As(err, &exitFailure) {
			return "", &Error{Args: args, Stderr: stderr.String(), Err: err}
		}
		return "", fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

// AuthorName returns the name a capcom write records as its author: the first non-empty of the
// GIT_AUTHOR_NAME environment variable, `git config user.name` as read in dir, and the USER
// environment variable. An unset user.name is not a failure; it fails with ErrNoAuthor when all
// three are empty, and with git's own error when git cannot run at all.
func AuthorName(dir string) (string, error) {
	if name := strings.TrimSpace(os.Getenv(authorNameVariable)); name != "" {
		return name, nil
	}
	output, err := Run(dir, "config", userNameKey)
	var gitFailure *Error
	if err != nil && !errors.As(err, &gitFailure) {
		return "", err
	}
	if name := strings.TrimSpace(output); name != "" {
		return name, nil
	}
	if name := strings.TrimSpace(os.Getenv(userVariable)); name != "" {
		return name, nil
	}
	return "", ErrNoAuthor
}

// cleanPath turns one path line of git output into a native, cleaned path; git prints forward
// slashes on every platform.
func cleanPath(line string) string {
	return filepath.Clean(filepath.FromSlash(strings.TrimRight(line, "\r")))
}
