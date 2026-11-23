package main

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeThresholds writes content as a thresholds file in dir and returns its path.
func writeThresholds(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "thresholds.yaml")
	if err := os.WriteFile(path, []byte(content), runFileMode); err != nil {
		t.Fatalf("write the thresholds file: %v", err)
	}
	return path
}

func TestLoadThresholds_ReadsEachModelsThreshold(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]int
	}{
		{name: "two models", content: "models:\n  qwen: 7\n  claude: 0\n", want: map[string]int{"qwen": 7, "claude": 0}},
		{name: "no models", content: "models: {}\n", want: map[string]int{}},
		{name: "an empty file", content: "", want: map[string]int{}},
		{name: "a comment only", content: "# no thresholds yet\n", want: map[string]int{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeThresholds(t, t.TempDir(), test.content)

			thresholds, err := loadThresholds(path)

			if err != nil || !maps.Equal(thresholds, test.want) {
				t.Errorf("loadThresholds = %v, %v; want %v", thresholds, err, test.want)
			}
		})
	}
}

func TestLoadThresholds_RefusesABadFileNamingIt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "a negative threshold", content: "models:\n  qwen: -1\n", want: "negative"},
		{name: "an unknown top-level key", content: "models: {}\nmodel: {}\n", want: "model"},
		{name: "a threshold that is not a number", content: "models:\n  qwen: many\n", want: "many"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeThresholds(t, t.TempDir(), test.content)

			_, err := loadThresholds(path)

			if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), test.want) {
				t.Errorf("loadThresholds error = %v, want one naming %s and %q", err, path, test.want)
			}
		})
	}
}

func TestLoadThresholds_RefusesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such.yaml")

	_, err := loadThresholds(path)

	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Errorf("loadThresholds error = %v, want a not-exist error naming %s", err, path)
	}
}

func TestLoadThresholds_ParsesTheCheckedInFile(t *testing.T) {
	_, err := loadThresholds("thresholds.yaml")

	if err != nil {
		t.Errorf("loadThresholds(thresholds.yaml) = %v, want the checked-in file to parse", err)
	}
}

func TestLookupThreshold_FindsTheNamedModel(t *testing.T) {
	path := writeThresholds(t, t.TempDir(), "models:\n  qwen: 7\n")

	threshold, err := lookupThreshold(path, "qwen")

	if err != nil || threshold != (runThreshold{name: "qwen", minPassed: 7}) {
		t.Errorf("lookupThreshold = %+v, %v; want qwen's 7", threshold, err)
	}
}

func TestLookupThreshold_NoFlagsIsNoThreshold(t *testing.T) {
	threshold, err := lookupThreshold("", "")

	if err != nil || threshold != (runThreshold{}) {
		t.Errorf("lookupThreshold = %+v, %v; want no threshold", threshold, err)
	}
}

func TestEnforceThreshold_JudgesThePassedCount(t *testing.T) {
	threshold := runThreshold{name: "qwen", minPassed: 2}
	tests := []struct {
		name       string
		threshold  runThreshold
		passed     int
		wantErr    error
		wantStdout string
	}{
		{name: "no threshold", passed: 0},
		{name: "met exactly", threshold: threshold, passed: 2, wantStdout: "threshold qwen: met (2)\n"},
		{
			name: "below", threshold: threshold, passed: 1,
			wantErr: belowThresholdError{name: "qwen", passed: 1, total: 3, minPassed: 2},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout strings.Builder

			err := enforceThreshold(&stdout, test.threshold, test.passed, 3)

			if err != test.wantErr || stdout.String() != test.wantStdout {
				t.Errorf("enforceThreshold = %v, stdout %q; want %v, %q", err, stdout.String(), test.wantErr, test.wantStdout)
			}
		})
	}
}

func TestBelowThresholdError_NamesTheShortfall(t *testing.T) {
	err := belowThresholdError{name: "qwen", passed: 1, total: 3, minPassed: 2}

	if got, want := err.Error(), "threshold qwen: 1/3 passed, need 2"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
