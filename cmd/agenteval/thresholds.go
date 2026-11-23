package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"
)

// thresholdsFile is the shape of a thresholds file: each model's name mapped to the fewest tasks
// a run of it must pass.
type thresholdsFile struct {
	Models map[string]int `yaml:"models"`
}

// runThreshold is the pass count a run must reach, for the model --name names. Its zero value,
// with no name, is a run without a threshold.
type runThreshold struct {
	name      string
	minPassed int
}

// belowThresholdError is a run whose passed count fell short of its model's threshold. The run's
// report is already written when it is returned.
type belowThresholdError struct {
	name      string
	passed    int
	total     int
	minPassed int
}

func (e belowThresholdError) Error() string {
	return fmt.Sprintf("threshold %s: %d/%d passed, need %d", e.name, e.passed, e.total, e.minPassed)
}

// loadThresholds reads the thresholds file at path, a YAML map `models: {<name>: <min passed>}`.
// An empty file holds no thresholds. A file that cannot be read or parsed, an unknown top-level
// key and a negative threshold are errors that name path.
func loadThresholds(path string) (map[string]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var file thresholdsFile
	if err := decoder.Decode(&file); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for name, minPassed := range file.Models {
		if minPassed < 0 {
			return nil, fmt.Errorf("%s: model %q: threshold %d is negative", path, name, minPassed)
		}
	}
	return file.Models, nil
}

// lookupThreshold returns the threshold of the model name in the thresholds file at path, or the
// zero runThreshold when neither is given. Either one without the other, a file loadThresholds
// refuses, and a name the file has no entry for are errors.
func lookupThreshold(path, name string) (runThreshold, error) {
	switch {
	case path == "" && name == "":
		return runThreshold{}, nil
	case name == "":
		return runThreshold{}, errors.New("--thresholds needs --name")
	case path == "":
		return runThreshold{}, errors.New("--name needs --thresholds")
	}
	thresholds, err := loadThresholds(path)
	if err != nil {
		return runThreshold{}, fmt.Errorf("--thresholds: %w", err)
	}
	minPassed, isFound := thresholds[name]
	if !isFound {
		return runThreshold{}, fmt.Errorf("--name: %s has no threshold for %q", path, name)
	}
	return runThreshold{name: name, minPassed: minPassed}, nil
}

// enforceThreshold judges a run's passed count of total against threshold: below it is a
// belowThresholdError, met prints a line saying so to stdout, and without a threshold it is
// silent.
func enforceThreshold(stdout io.Writer, threshold runThreshold, passed, total int) error {
	if threshold.name == "" {
		return nil
	}
	if passed < threshold.minPassed {
		return belowThresholdError{name: threshold.name, passed: passed, total: total, minPassed: threshold.minPassed}
	}
	fmt.Fprintf(stdout, "threshold %s: met (%d)\n", threshold.name, threshold.minPassed)
	return nil
}
