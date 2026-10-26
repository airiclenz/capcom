package gitx

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// pathSeparator splits the paths `check-ignore -z` reads and prints.
	pathSeparator = "\x00"
	// noneIgnoredExit is the exit code of `check-ignore` when it ignores none of the paths; any
	// other failure, such as 128 for a path outside the repository, is a real git error.
	noneIgnoredExit = 1
)

// IgnoredPaths returns the paths git ignores in the working tree at dir, in the order and the
// spelling the caller gave them. A path is relative to dir or absolute, and must lie inside the
// working tree; it need not exist. A tracked file is never ignored, even when an ignore pattern
// matches it. No paths, or none ignored, returns an empty result and no error; a path outside the
// working tree, or dir outside any, fails with an *Error carrying git's stderr. It runs one
// `git check-ignore` per call.
func IgnoredPaths(dir string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	slashPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		slashPaths = append(slashPaths, filepath.ToSlash(path))
	}
	input := strings.Join(slashPaths, pathSeparator) + pathSeparator
	output, err := runWithInput(dir, []byte(input), "check-ignore", "-z", "--stdin")
	if err != nil {
		var exitFailure *exec.ExitError
		if errors.As(err, &exitFailure) && exitFailure.ExitCode() == noneIgnoredExit {
			return nil, nil
		}
		return nil, err
	}

	ignored := make(map[string]bool)
	for _, printed := range strings.Split(output, pathSeparator) {
		if printed != "" {
			ignored[printed] = true
		}
	}
	var result []string
	for index, path := range paths {
		if ignored[slashPaths[index]] {
			result = append(result, path)
		}
	}
	return result, nil
}
