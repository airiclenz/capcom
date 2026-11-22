//go:build !linux && !darwin && !windows

package proc

import "errors"

// openReader fails: this system has no process reader, so Ancestry is always empty.
func openReader() (reader, error) {
	return nil, errors.New("process ancestry: unsupported on this system")
}
