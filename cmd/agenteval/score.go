package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// taskScore is what one task's call log adds up to: how many capcom calls the agent made, how
// many of them failed, and how many of those failures repeated an earlier one.
type taskScore struct {
	Calls            int
	Failed           int
	RepeatedFailures int
}

// add returns the sum of s and other, the way a report's totals row adds its tasks up.
func (s taskScore) add(other taskScore) taskScore {
	return taskScore{
		Calls:            s.Calls + other.Calls,
		Failed:           s.Failed + other.Failed,
		RepeatedFailures: s.RepeatedFailures + other.RepeatedFailures,
	}
}

// readCallLog returns the calls of the call log at path, in the order the shim appended them.
// A log that does not exist holds no calls, since the shim only creates it on the first call;
// blank lines are skipped. A line that is not a call fails the read with an error naming the
// file and the line.
func readCallLog(path string) ([]shimCall, error) {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var calls []shimCall
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var call shimCall
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			return nil, fmt.Errorf("%s:%d: not a call log line: %w", path, lineNumber, err)
		}
		calls = append(calls, call)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return calls, nil
}

// score adds up one task's calls. Every call with a non-zero exit code is failed, whatever the
// code; a failed call is also a repeated failure when an earlier failed call of the same task had
// the same arguments and the same exit code, which is an agent retrying what it was already told
// does not work.
func score(calls []shimCall) taskScore {
	result := taskScore{Calls: len(calls)}
	seenFailures := map[string]bool{}
	for _, call := range calls {
		if call.Exit == 0 {
			continue
		}
		result.Failed++
		key := failureKey(call)
		if seenFailures[key] {
			result.RepeatedFailures++
		}
		seenFailures[key] = true
	}
	return result
}

// failureKey identifies a failed call by its exit code and arguments. The arguments are quoted,
// so two different argument lists never share a key.
func failureKey(call shimCall) string {
	return fmt.Sprintf("%d %q", call.Exit, call.Args)
}
