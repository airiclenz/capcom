//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunAgent_KillsEveryProcessTheAgentStarted(t *testing.T) {
	tests := []struct {
		name           string
		script         string
		timeout        time.Duration
		wantIsTimedOut bool
	}{
		{
			name:           "an agent past the timeout",
			script:         "sleep 300 & echo $! > child.pid; wait",
			timeout:        300 * time.Millisecond,
			wantIsTimedOut: true,
		},
		{
			name:    "an agent that finishes with a background child",
			script:  "sleep 300 & echo $! > child.pid",
			timeout: time.Minute,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			transcript, err := os.Create(filepath.Join(dir, "transcript.log"))
			if err != nil {
				t.Fatalf("create the transcript: %v", err)
			}
			defer transcript.Close()
			seeded := workspace{root: dir, repo: dir, binDir: dir}

			run, err := seeded.runAgent(
				context.Background(), []string{"sh", "-c", test.script}, "", "", "", transcript, test.timeout,
			)

			if err != nil || run.IsTimedOut != test.wantIsTimedOut {
				t.Fatalf("runAgent = timed out %t, %v; want timed out %t, nil", run.IsTimedOut, err, test.wantIsTimedOut)
			}
			waitUntilGone(t, filepath.Join(dir, "child.pid"))
		})
	}
}
