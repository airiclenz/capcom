//go:build unix

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// notExecutableFileMode is the mode of an agent file that exists but cannot be executed.
const notExecutableFileMode = 0o644

// shortAgentWaitDelay stands in for agentWaitDelay, so a test of an agent that ignores SIGTERM
// need not wait the full delay out.
const shortAgentWaitDelay = 300 * time.Millisecond

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

func TestRunAgent_KillsTheShellGroupsTheLedgerLists(t *testing.T) {
	if _, err := exec.LookPath("perl"); err != nil {
		t.Fatalf("perl starts the child in a process group of its own: %v", err)
	}
	dir := t.TempDir()
	transcript, err := os.Create(filepath.Join(dir, "transcript.log"))
	if err != nil {
		t.Fatalf("create the transcript: %v", err)
	}
	defer transcript.Close()
	seeded := workspace{root: dir, repo: dir, binDir: dir, waitDelay: shortAgentWaitDelay}
	script := `trap '' TERM
perl -e 'setpgrp; exec @ARGV' sleep 300 &
echo $! > child.pid
echo $! >> "$` + processGroupsVariable + `"
wait`

	run, err := seeded.runAgent(
		context.Background(), []string{"sh", "-c", script}, "", "", "", transcript, 300*time.Millisecond,
	)

	if err != nil || !run.IsTimedOut {
		t.Fatalf("runAgent = timed out %t, %v; want timed out, nil", run.IsTimedOut, err)
	}
	waitUntilGone(t, filepath.Join(dir, "child.pid"))
}

func TestProcessGroupLedger_LiveGroupsSkipsWhatTheSweepMustNotKill(t *testing.T) {
	const (
		live  = 999991
		ended = 999992
		torn  = 999993
	)
	tests := []struct {
		name      string
		contents  string
		isMissing bool
		want      []int
	}{
		{
			name: "a ledger with every line the sweep must skip",
			contents: strings.Join([]string{
				"0", "1", strconv.Itoa(syscall.Getpgrp()), "not a pgid",
				strconv.Itoa(ended), strconv.Itoa(live), "-" + strconv.Itoa(ended),
				strconv.Itoa(torn),
			}, "\n"),
			want: []int{live},
		},
		{name: "a ledger the agent never wrote", isMissing: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "process-groups")
			if !test.isMissing {
				if err := os.WriteFile(path, []byte(test.contents), processGroupsFileMode); err != nil {
					t.Fatalf("write the ledger: %v", err)
				}
			}

			groups, err := processGroupLedger(path).liveGroups()

			if err != nil || !slices.Equal(groups, test.want) {
				t.Errorf("liveGroups = %v, %v; want %v, nil", groups, err, test.want)
			}
		})
	}
}
