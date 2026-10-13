package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// The names the gate reads remote state through.
const (
	remoteName   = "origin"
	mainBranch   = "main"
	remoteMain   = remoteName + "/" + mainBranch
	ciWorkflow   = "ci.yml"
	shortSHASize = 12
)

// peelablePaths are the only files a commit may touch for the gate to look past it: the two a
// cut's own commits write (the changelog rollup and the VERSION bump), which a version bump
// pushed on its own writes too.
var peelablePaths = []string{changelogFile, versionFile}

// gateReport is the CI gate's verdict: whether a cut may proceed, the commit it was judged on,
// and every condition that blocks it, each named.
type gateReport struct {
	IsOK     bool     `json:"ok"`
	Commit   string   `json:"commit"`
	Failures []string `json:"failures"`
	// Head is the commit HEAD resolved to when the gate judged it, empty when it did not
	// resolve: publish reads VERSION at it, checks it against origin and pushes it, so what is
	// pushed is what the gate cleared. It is not part of status's JSON.
	Head string `json:"-"`
}

// checkGate judges the release gate over checkout. The gate commit G is HEAD with its trailing
// first-parent commits that touch only CHANGELOG.md and/or VERSION peeled off. The gate clears
// when the tree is clean, G is on origin/main, origin/main is contained in HEAD, and ci.yml has a
// successful run for G or for a peeled commit origin/main contains — a push runs CI on its head
// alone, so a feature pushed with a VERSION bump leaves the green run on the bump.
// A check that cannot run is a failure too, named with its cause; the gate has no bypass.
func checkGate(ctx context.Context, checkout repo) gateReport {
	report := gateReport{Failures: []string{}}
	fail := func(format string, args ...any) {
		report.Failures = append(report.Failures, fmt.Sprintf(format, args...))
	}

	if err := checkout.FetchMain(ctx); err != nil {
		fail("could not fetch %s: %v", remoteName, err)
	}

	isClean, err := checkout.IsClean(ctx)
	switch {
	case err != nil:
		fail("could not read the working tree's status: %v", err)
	case !isClean:
		fail("the working tree is not clean")
	}

	head, err := checkout.Resolve(ctx, "HEAD")
	if err != nil {
		fail("could not resolve HEAD: %v", err)
		return report
	}
	report.Head = head
	gate, peeled, err := peelBumps(ctx, checkout, head)
	if err != nil {
		fail("could not find the gate commit: %v", err)
		return report
	}
	report.Commit = gate

	isPushed, err := checkout.IsAncestor(ctx, gate, remoteMain)
	switch {
	case err != nil:
		fail("could not check whether %s is on %s: %v", short(gate), remoteMain, err)
	case !isPushed:
		fail("gate commit %s is not on %s: push it first", short(gate), remoteMain)
	}

	isCurrent, err := checkout.IsAncestor(ctx, remoteMain, head)
	switch {
	case err != nil:
		fail("could not check whether HEAD contains %s: %v", remoteMain, err)
	case !isCurrent:
		fail("%s has commits HEAD does not: pull first", remoteMain)
	}

	if err := checkGreenRun(ctx, checkout, gate, peeled); err != nil {
		fail("%v", err)
	}

	report.IsOK = len(report.Failures) == 0
	return report
}

// peelBumps walks HEAD's first-parent line back past every commit whose own changes touch only
// peelablePaths, and returns the first commit it stops at (the gate commit) and the commits it
// walked past, newest first. It stops at any commit that is not an ordinary one-parent commit — a
// merge is never peeled — and at one that touches nothing at all.
func peelBumps(ctx context.Context, checkout repo, head string) (string, []string, error) {
	var peeled []string
	commit := head
	for {
		parents, err := checkout.Parents(ctx, commit)
		if err != nil {
			return "", nil, fmt.Errorf("read the parents of %s: %w", short(commit), err)
		}
		if len(parents) != 1 {
			return commit, peeled, nil
		}
		parent := parents[0]

		changed, err := checkout.ChangedPaths(ctx, parent, commit)
		if err != nil {
			return "", nil, fmt.Errorf("list the paths %s changes: %w", short(commit), err)
		}
		if !touchesOnlyPeelablePaths(changed) {
			return commit, peeled, nil
		}
		peeled = append(peeled, commit)
		commit = parent
	}
}

// touchesOnlyPeelablePaths reports whether changed names at least one path and nothing outside
// peelablePaths.
func touchesOnlyPeelablePaths(changed []string) bool {
	if len(changed) == 0 {
		return false
	}
	for _, path := range changed {
		if !slices.Contains(peelablePaths, path) {
			return false
		}
	}
	return true
}

// checkGreenRun returns nil when ci.yml has a successful run for the gate commit, or for one of
// the peeled commits that origin/main contains — the only ones a push could have run CI on in
// its place. Otherwise it returns the failure that names what was looked for.
func checkGreenRun(ctx context.Context, checkout repo, gate string, peeled []string) error {
	candidates := []string{gate}
	for _, commit := range peeled {
		isPushed, err := checkout.IsAncestor(ctx, commit, remoteMain)
		if err != nil {
			return fmt.Errorf("could not check whether %s is on %s: %w", short(commit), remoteMain, err)
		}
		if isPushed {
			candidates = append(candidates, commit)
		}
	}

	for _, commit := range candidates {
		isGreen, err := checkout.HasGreenRun(ctx, commit)
		if err != nil {
			return fmt.Errorf("could not read the %s runs for %s: %w", ciWorkflow, short(commit), err)
		}
		if isGreen {
			return nil
		}
	}

	shortCandidates := make([]string, 0, len(candidates))
	for _, commit := range candidates {
		shortCandidates = append(shortCandidates, short(commit))
	}
	return fmt.Errorf("%s has no successful run for %s", ciWorkflow, strings.Join(shortCandidates, " or "))
}

// short abbreviates a commit id for a message; the full id stays in the report's commit field.
func short(commit string) string {
	if len(commit) <= shortSHASize {
		return commit
	}
	return commit[:shortSHASize]
}
