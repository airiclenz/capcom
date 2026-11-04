package config

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// fuzzPath is the path FuzzParseConfig hands Parse.
const fuzzPath = "config.yaml"

// committedConfigPath is this repository's config file, relative to this package's directory.
const committedConfigPath = "../../.capcom/config.yaml"

// configSeeds are the hand-written inputs FuzzParseConfig starts from besides the committed config:
// the templates capcom init writes, a format newer than this capcom, a repeated top-level key and a
// scalar where a list belongs.
var configSeeds = []string{
	DefaultFile,
	KeylessFile,
	"format: 7\n",
	"types: [bug]\ntypes: [task]\n",
	"ready:\n  hide: wip\n",
}

// FuzzParseConfig parses arbitrary content as config.yaml: Parse never panics, and every error it
// returns names the path it was given.
func FuzzParseConfig(f *testing.F) {
	if data, ok := committedConfig(f); ok {
		f.Add(data)
	}
	for _, seed := range configSeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := Parse(fuzzPath, data)
		if err != nil && !strings.Contains(err.Error(), fuzzPath) {
			t.Errorf("Parse(%q) error %q does not name %q", data, err, fuzzPath)
		}
	})
}

// committedConfig returns the content of this repository's config file, and false when it is
// absent.
func committedConfig(f *testing.F) ([]byte, bool) {
	f.Helper()
	data, err := os.ReadFile(committedConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		f.Fatalf("read the committed config: %v", err)
	}
	return data, true
}
