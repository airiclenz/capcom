package main

import (
	"fmt"
	"strings"
)

// nextClaimCloseIssues are the three Ready issues of next-claim-close, one each of priority 1, 2
// and 3, listed in an order that is not their priority's, so an agent taking the first one it sees
// picks the wrong one.
var nextClaimCloseIssues = []seededIssue{
	{
		id:       "tidy-log-output",
		title:    "Tidy the log output",
		body:     "The debug log repeats the request id on every line.",
		priority: 3,
	},
	{
		id:       "fix-login-timeout",
		title:    "Fix the login timeout",
		body:     "A login that takes longer than five seconds fails with a blank page.",
		priority: 1,
	},
	{
		id:       "update-readme",
		title:    "Update the README",
		body:     "The README still names the old install command.",
		priority: 2,
	},
}

// nextClaimCloseUrgent is the id of next-claim-close's most urgent issue, the one the agent must
// close.
const nextClaimCloseUrgent = "fix-login-timeout"

// nextClaimCloseScenario asks the agent to work the most urgent of three Ready issues through:
// claim it, note progress, close it with a reason. It passes when that issue alone is closed, with
// a reason and at least one Log entry.
func nextClaimCloseScenario() scenario {
	return scenario{
		id: "next-claim-close",
		prompt: "Work on the most urgent ready issue in this repository: claim it, note your " +
			"progress on it, and close it with a reason. Leave the other issues alone.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, nextClaimCloseIssues)
		},
		check: checkNextClaimClose,
	}
}

// checkNextClaimClose judges next-claim-close from the issue files.
func checkNextClaimClose(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	for _, seeded := range nextClaimCloseIssues {
		stored, isFound := issues[seeded.id]
		switch {
		case !isFound:
			return failed("%s is gone", seeded.id)
		case seeded.id != nextClaimCloseUrgent && stored.IsClosed:
			return failed("%s (priority %d) was closed, not only the most urgent", seeded.id, seeded.priority)
		case seeded.id != nextClaimCloseUrgent:
			continue
		case !stored.IsClosed:
			return failed("%s, the most urgent issue, is not closed", seeded.id)
		case stored.Issue.Reason == "":
			return failed("%s was closed without a reason", seeded.id)
		case len(logEntries(stored)) == 0:
			return failed("%s has no Log entry", seeded.id)
		}
	}
	return passed()
}

// resumeLostTokenIssue is the id of the issue resume-lost-token's seed claims, Tree lock and all.
const resumeLostTokenIssue = "cache-layer"

// resumeLostTokenIssues is resume-lost-token's one issue. It has no Log entry, so any entry the
// check finds is the agent's.
var resumeLostTokenIssues = []seededIssue{
	{
		id:       resumeLostTokenIssue,
		title:    "Put a cache in front of the catalogue queries",
		body:     "Every page view runs the same three catalogue queries against the database.",
		priority: 2,
	},
}

// resumeLostTokenScenario seeds a claimed issue whose Lock token the agent is never given, and asks
// it to resume the work, log progress and release the lock. It passes when the lock is free,
// cache-layer has a Log entry, and the agent never broke the lock to get there.
func resumeLostTokenScenario() scenario {
	return scenario{
		id: "resume-lost-token",
		prompt: "You were working on the issue cache-layer when your session restarted, and the Lock " +
			"token you held is lost. Pick the work up again where you left it, note your progress on " +
			"cache-layer, and release the lock when you are done.\n",
		seed: func(env seedEnv) error {
			if err := createIssues(env, resumeLostTokenIssues); err != nil {
				return err
			}
			if _, err := env.capcom("start", resumeLostTokenIssue); err != nil {
				return fmt.Errorf("start %s: %w", resumeLostTokenIssue, err)
			}
			return nil
		},
		check: checkResumeLostToken,
	}
}

// checkResumeLostToken judges resume-lost-token from the lock state, cache-layer's Log and the
// call log.
func checkResumeLostToken(repo string, calls []shimCall) outcome {
	for _, call := range calls {
		if isLockBreak(call) {
			return failed("the lock was broken (capcom %s), not resumed", strings.Join(call.Args, " "))
		}
	}
	state, err := lockState(repo)
	if err != nil {
		return failed("cannot read the Tree lock: %v", err)
	}
	if state.IsHeld {
		return failed("the Tree lock is still held (%s)", state.Record.Purpose)
	}
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	stored, isFound := issues[resumeLostTokenIssue]
	switch {
	case !isFound:
		return failed("%s is gone", resumeLostTokenIssue)
	case len(logEntries(stored)) == 0:
		return failed("%s has no Log entry", resumeLostTokenIssue)
	}
	return passed()
}

// isLockBreak reports whether call ran `capcom lock break`: its first two arguments that are not
// flags are lock and break, so a global flag before them does not hide it.
func isLockBreak(call shimCall) bool {
	var words []string
	for _, arg := range call.Args {
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
		}
	}
	return len(words) >= 2 && words[0] == "lock" && words[1] == "break"
}

// The ids blocked-start works with: the issue the prompt asks to start, and the open issue it
// waits for, which the agent must start instead.
const (
	blockedStartIssue   = "deploy"
	blockedStartBlocker = "ci-green"
)

// blockedStartIssues are blocked-start's two issues: deploy waits for the open ci-green.
var blockedStartIssues = []seededIssue{
	{
		id:       blockedStartBlocker,
		title:    "Make the CI pipeline green",
		body:     "Two integration tests fail on every run since the database upgrade.",
		priority: 2,
	},
	{
		id:       blockedStartIssue,
		title:    "Deploy the new release",
		body:     "Roll the new release out to production.",
		priority: 1,
		deps:     []string{blockedStartBlocker},
	},
}

// blockedStartScenario asks the agent to start an issue that waits for an open one, and to start
// what blocks it when it cannot. It passes when ci-green is active and deploy is not.
func blockedStartScenario() scenario {
	return scenario{
		id: "blocked-start",
		prompt: "Start work on the issue deploy. If it cannot be started yet, start the work that " +
			"blocks it instead.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, blockedStartIssues)
		},
		check: checkBlockedStart,
	}
}

// checkBlockedStart judges blocked-start from the two issues' statuses.
func checkBlockedStart(repo string, _ []shimCall) outcome {
	cfg, issues, err := loadConfigAndIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	blocked, isBlockedFound := issues[blockedStartIssue]
	blocker, isBlockerFound := issues[blockedStartBlocker]
	switch {
	case !isBlockedFound:
		return failed("%s is gone", blockedStartIssue)
	case !isBlockerFound:
		return failed("%s is gone", blockedStartBlocker)
	case isActive(cfg, blocked):
		return failed("%s is active, though it waits for %s", blockedStartIssue, blockedStartBlocker)
	case !isActive(cfg, blocker):
		return failed(
			"%s, which blocks %s, is not active (status %q)",
			blockedStartBlocker, blockedStartIssue, blocker.Issue.Status,
		)
	}
	return passed()
}

// reopenWithReasonIssue is the id of the issue reopen-with-reason's seed closes by mistake.
const reopenWithReasonIssue = "login-bug"

// reopenWithReasonIssues is reopen-with-reason's one issue, closed with the reason "fixed". Closing
// writes no Log entry, so any entry the check finds is the agent's.
var reopenWithReasonIssues = []seededIssue{
	{
		id:          reopenWithReasonIssue,
		title:       "Login fails after a password change",
		body:        "A user who changed their password is sent back to the login page on every attempt.",
		priority:    2,
		closeReason: "fixed",
	},
}

// reopenWithReasonScenario seeds an issue closed by mistake and asks the agent to reopen it, saying
// why. It passes when login-bug is open and has a Log entry.
func reopenWithReasonScenario() scenario {
	return scenario{
		id: "reopen-with-reason",
		prompt: "The issue login-bug was closed by mistake: the bug still reproduces. Reopen it, " +
			"saying why.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, reopenWithReasonIssues)
		},
		check: checkReopenWithReason,
	}
}

// checkReopenWithReason judges reopen-with-reason from login-bug's status and Log.
func checkReopenWithReason(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	stored, isFound := issues[reopenWithReasonIssue]
	switch {
	case !isFound:
		return failed("%s is gone", reopenWithReasonIssue)
	case stored.IsClosed:
		return failed("%s is still closed (status %q)", reopenWithReasonIssue, stored.Issue.Status)
	case len(logEntries(stored)) == 0:
		return failed("%s has no Log entry", reopenWithReasonIssue)
	}
	return passed()
}
