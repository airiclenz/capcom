package main

import (
	"fmt"
	"strconv"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
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
		fileWithDepScenario(),
		findLockNoteScenario(),
		resumeLostTokenScenario(),
		findByContentScenario(),
		contextBeforeStartScenario(),
		blockedStartScenario(),
		moveUnderParentScenario(),
		renameIDScenario(),
		addNoteScenario(),
		reopenWithReasonScenario(),
		closeChildrenScenario(),
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

// seededIssue is an issue a seed creates: its id, title, body and priority, the ids of the issues
// it depends on, the basename of its plan (empty for none), the id of its Parent (empty for the top
// level), whether it is a standing Parent, and the reason it is closed with (empty to leave it
// open).
type seededIssue struct {
	id          string
	title       string
	body        string
	priority    int
	deps        []string
	plan        string
	parent      string
	isStanding  bool
	closeReason string
}

// createIssues creates each of issues in env's repo, in order, so an issue must come after the
// issues it depends on and its Parent; one with a close reason is closed right after it is created.
func createIssues(env seedEnv, issues []seededIssue) error {
	for _, seeded := range issues {
		args := []string{
			"create", "--id", seeded.id, "--title", seeded.title, "--body", seeded.body,
			"--priority", strconv.Itoa(seeded.priority),
		}
		for _, dep := range seeded.deps {
			args = append(args, "--dep", dep)
		}
		if seeded.plan != "" {
			args = append(args, "--plan", seeded.plan)
		}
		if seeded.parent != "" {
			args = append(args, "--parent", seeded.parent)
		}
		if seeded.isStanding {
			args = append(args, "--standing")
		}
		if _, err := env.capcom(args...); err != nil {
			return fmt.Errorf("create %s: %w", seeded.id, err)
		}
		if seeded.closeReason == "" {
			continue
		}
		if _, err := env.capcom("close", seeded.id, "--reason", seeded.closeReason); err != nil {
			return fmt.Errorf("close %s: %w", seeded.id, err)
		}
	}
	return nil
}

// seededNote is a Note a seed adds: its id, the id of the Note it sits below (empty for the top
// level), its title and its body.
type seededNote struct {
	id     string
	parent string
	title  string
	body   string
}

// createNotes adds each of notes to env's repo, in order, so a parent must come before its
// children. The body is appended after the add, since know add takes a body only from a file.
func createNotes(env seedEnv, notes []seededNote) error {
	for _, seeded := range notes {
		args := []string{"know", "add", "--id", seeded.id, "--title", seeded.title}
		if seeded.parent != "" {
			args = append(args, "--parent", seeded.parent)
		}
		if _, err := env.capcom(args...); err != nil {
			return fmt.Errorf("add the Note %s: %w", seeded.id, err)
		}
		if _, err := env.capcom("know", "update", seeded.id, "--body-append", seeded.body); err != nil {
			return fmt.Errorf("write the body of the Note %s: %w", seeded.id, err)
		}
	}
	return nil
}

// isActive reports whether stored is being worked on: its status falls in the doing category of
// cfg.
func isActive(cfg config.Config, stored storedIssue) bool {
	category, _ := cfg.CategoryOf(stored.Issue.Status)
	return category == config.CategoryDoing
}

// loadConfigAndIssues returns the config of repo's store and its issues, as loadIssues reads them.
func loadConfigAndIssues(repo string) (config.Config, map[string]storedIssue, error) {
	opened, err := store.Open(repo)
	if err != nil {
		return config.Config{}, nil, err
	}
	issues, err := loadIssues(repo)
	if err != nil {
		return config.Config{}, nil, err
	}
	return opened.Config, issues, nil
}
