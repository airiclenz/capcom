package main

import (
	"context"
	"fmt"
)

// The publish steps, by the name status and publish report them under.
const (
	stepTag      = "tag"
	stepRelease  = "release"
	stepHomebrew = "homebrew"
	stepScoop    = "scoop"
	stepWinget   = "winget"
	stepSmoke    = "smoke"
)

// stepState is where a publish step stands for one version.
type stepState string

// The states a probe reports. Only stateDone lets publish skip a step; stateUnknown means the
// probe could not tell, and is never read as done.
const (
	// stateDone: the remote already holds the step's result for the version.
	stateDone stepState = "done"
	// statePending: publish has the step to do.
	statePending stepState = "pending"
	// stateSetupPending: the channel's one-time setup — its tap or bucket repository — has not
	// landed; publish skips the step. publish also reports winget so when its first `komac
	// submit` fails, keeping the rendered manifests for a human to submit.
	stateSetupPending stepState = "setup-pending"
	// stateUnknown: the probe could not run, or found the remote in a state publish cannot
	// converge from; the detail names which.
	stateUnknown stepState = "unknown"
)

// stepReport is one publish step's probed state. Its JSON keys are a contract the release skills
// read, like the rest of status's report.
type stepReport struct {
	Name   string    `json:"name"`
	State  stepState `json:"state"`
	Detail string    `json:"detail"`
}

// stepProbe asks the remote, read-only, where one publish step stands for version.
type stepProbe func(ctx context.Context, env probeEnv, version releaseVersion) stepReport

// probeEnv is what the probes read the remotes through: capcom's own checkout and GitHub
// repository through repo, and every other repository — the tap, the bucket, winget-pkgs —
// through commands, run in dir.
type probeEnv struct {
	repo     repo
	commands runner
	dir      string
}

// publishStep is one step publish runs, declared once for status and publish alike: the probe
// that asks the remote, read-only, whether the step is done; what a dry run says the step would
// do; the work itself, run only while the probe reports the step pending; and what the step
// table says once it has run ("done by this run" when empty); and, for a package channel, the
// check the smoke verb runs on what the channel serves (nil for none). A converge that returns a
// setupPendingError has not failed: the step is reported setup-pending and the run goes on.
// No function an entry names may read publishSteps: that would be an initialisation cycle.
type publishStep struct {
	name       string
	probe      stepProbe
	describe   func(run *publishRun) string
	converge   func(ctx context.Context, run *publishRun) error
	doneDetail string
	smoke      func(ctx context.Context, run *smokeRun)
}

// publishSteps are the steps publish runs, in the order it runs them. status and publish both
// read this one table, so what status reports done is exactly what publish skips. The channel
// steps (homebrew.go, scoop.go, winget.go) run after the release, whose SHA256SUMS they render
// from, and the smoke step (smoke.go) runs last, over everything the others published; the smoke
// verb runs the channels' smoke checks in this table's order.
var publishSteps = []publishStep{
	{name: stepTag, probe: probeTag, describe: describeTag, converge: convergeTag},
	{name: stepRelease, probe: probeRelease, describe: describeRelease, converge: convergeRelease},
	{name: stepHomebrew, probe: probeHomebrew, describe: describeHomebrew, converge: convergeHomebrew,
		smoke: func(ctx context.Context, run *smokeRun) { run.checkHomebrew(ctx) }},
	{name: stepScoop, probe: probeScoop, describe: describeScoop, converge: convergeScoop,
		smoke: func(ctx context.Context, run *smokeRun) { run.checkScoop(ctx) }},
	{name: stepWinget, probe: probeWinget, describe: describeWinget, converge: convergeWinget,
		doneDetail: wingetSubmittedDetail},
	{name: stepSmoke, probe: probeSmoke, describe: describeSmoke, converge: convergeSmoke,
		doneDetail: smokePassedDetail},
}

// probeSteps probes every publish step for version, in publish order.
func probeSteps(ctx context.Context, env probeEnv, version releaseVersion) []stepReport {
	steps := make([]stepReport, 0, len(publishSteps))
	for _, step := range publishSteps {
		steps = append(steps, step.probe(ctx, env, version))
	}
	return steps
}

// unknownStep is a step whose probe could not tell where it stands.
func unknownStep(name, format string, args ...any) stepReport {
	return stepReport{Name: name, State: stateUnknown, Detail: fmt.Sprintf(format, args...)}
}
