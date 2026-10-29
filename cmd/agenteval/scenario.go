package main

import (
	"fmt"
	"strconv"
)

// scenario is one seeded capcom task: the prompt the agent reads on stdin, the seed that puts the
// task's starting state into a fresh repo, and the check that judges what the agent left behind.
//
// A prompt names ids and goals, never a capcom command: the agent learns capcom from the AGENTS.md
// block the workspace writes. A check reads only the repo's .capcom files, its lock state and the
// task's call log, never the agent's transcript.
type scenario struct {
	id     string
	prompt string
	// seed adds the task's starting state to a repo that already has a capcom store and its
	// AGENTS.md block; the workspace commits whatever it leaves.
	seed func(env seedEnv) error
	// check judges the repo after the agent ran, given the calls the agent made.
	check func(repo string, calls []shimCall) outcome
}

// seedEnv is what a scenario's seed works with: the repo, and a way to run the capcom build under
// test in it, never through the shim, so seeding is not counted as the agent's calls.
type seedEnv struct {
	repo string
	// capcom runs the build under test with args in repo and returns its stdout; a non-zero exit is
	// an error carrying its stderr.
	capcom func(args ...string) (string, error)
}

// outcome is a check's verdict on one task: whether the agent did it, and why not when it did not.
type outcome struct {
	Passed bool
	Why    string
}

// passed is the verdict of a task the agent did.
func passed() outcome {
	return outcome{Passed: true}
}

// failed is the verdict of a task the agent did not do, with the reason formatted from format and
// args.
func failed(format string, args ...any) outcome {
	return outcome{Why: fmt.Sprintf(format, args...)}
}

// scenarios returns every scenario, in the order a run runs them and the report lists them. Ids are
// stable: a report row, a transcript file and `run --only` all name a scenario by its id.
func scenarios() []scenario {
	return []scenario{
		nextClaimCloseScenario(),
	}
}

// findScenario returns the scenario with id, and false when there is none.
func findScenario(id string) (scenario, bool) {
	for _, candidate := range scenarios() {
		if candidate.id == id {
			return candidate, true
		}
	}
	return scenario{}, false
}

// seededIssue is an issue a seed creates: its id, title, body and priority.
type seededIssue struct {
	id       string
	title    string
	body     string
	priority int
}

// createIssues creates each of issues in env's repo, in order.
func createIssues(env seedEnv, issues []seededIssue) error {
	for _, seeded := range issues {
		_, err := env.capcom(
			"create", "--id", seeded.id, "--title", seeded.title, "--body", seeded.body,
			"--priority", strconv.Itoa(seeded.priority),
		)
		if err != nil {
			return fmt.Errorf("create %s: %w", seeded.id, err)
		}
	}
	return nil
}

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
