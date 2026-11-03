//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// notExecutableFileMode is the mode of an agent file that exists but cannot be executed.
const notExecutableFileMode = 0o644

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

func TestRunAgent_ACancelledAgentThatExitsCleanlyIsNoStartFailure(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.log")
	transcript, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("create the transcript: %v", err)
	}
	defer transcript.Close()
	seeded := workspace{root: dir, repo: dir, binDir: dir}
	started := filepath.Join(dir, "started")
	script := `trap 'exit 0' TERM; touch "$0"; sleep 300 & wait`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cancelOnceExists(ctx, started, cancel)

	run, err := seeded.runAgent(ctx, []string{"sh", "-c", script, started}, "", "", "", transcript, time.Minute)

	if err != nil || run.IsTimedOut {
		t.Fatalf("runAgent = timed out %t, %v; want not timed out, nil", run.IsTimedOut, err)
	}
	text, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("read the transcript: %v", err)
	}
	if want := "agenteval: the agent ended with " + context.Canceled.Error(); !strings.Contains(string(text), want) {
		t.Errorf("transcript = %q, want it to note %q", text, want)
	}
}

func TestRunAgent_AnAgentThatCannotStartIsAnError(t *testing.T) {
	dir := t.TempDir()
	transcript, err := os.Create(filepath.Join(dir, "transcript.log"))
	if err != nil {
		t.Fatalf("create the transcript: %v", err)
	}
	defer transcript.Close()
	seeded := workspace{root: dir, repo: dir, binDir: dir}
	notExecutable := filepath.Join(dir, "not-executable")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), notExecutableFileMode); err != nil {
		t.Fatalf("write the agent: %v", err)
	}

	_, err = seeded.runAgent(
		context.Background(), []string{notExecutable}, "", "", "", transcript, time.Minute,
	)

	if err == nil || !strings.Contains(err.Error(), "start the agent") {
		t.Errorf("runAgent error = %v, want one saying it could not start the agent", err)
	}
}
