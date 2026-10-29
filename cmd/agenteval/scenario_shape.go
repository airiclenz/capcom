package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/store"
)

// hasHeading reports whether body holds a line that is exactly heading, such as "## Acceptance".
func hasHeading(body, heading string) bool {
	for line := range strings.Lines(body) {
		if strings.TrimRight(line, " \t\r\n") == heading {
			return true
		}
	}
	return false
}

// fileWithDepBlocker is the id of file-with-dep's seeded issue, the one the new issue must wait
// for.
const fileWithDepBlocker = "auth-tokens"

// acceptanceHeading is the heading of the section that says how to tell an issue is done.
const acceptanceHeading = "## Acceptance"

// fileWithDepIssues is file-with-dep's seed: the one issue the new issue depends on.
var fileWithDepIssues = []seededIssue{
	{
		id:       fileWithDepBlocker,
		title:    "Issue signed auth tokens",
		body:     "Sessions live in server memory; the login endpoint should issue signed tokens instead.",
		priority: 2,
	},
}

// fileWithDepScenario asks the agent to file an issue that waits for a seeded one, with a why and an
// Acceptance section. It passes when exactly one issue was added, depending on auth-tokens, with an
// Acceptance section.
func fileWithDepScenario() scenario {
	return scenario{
		id: "file-with-dep",
		prompt: "File a new issue for rate limiting the login endpoint, so that a client is slowed " +
			"down after repeated failed logins. The work cannot start before the issue auth-tokens " +
			"is done. Say in the issue why it is needed, and give it an Acceptance section that " +
			"says how to tell it is done.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, fileWithDepIssues)
		},
		check: checkFileWithDep,
	}
}

// checkFileWithDep judges file-with-dep from the issue files.
func checkFileWithDep(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	if _, isFound := issues[fileWithDepBlocker]; !isFound {
		return failed("%s is gone", fileWithDepBlocker)
	}
	var added []storedIssue
	for id, stored := range issues {
		if id != fileWithDepBlocker {
			added = append(added, stored)
		}
	}
	if len(added) != 1 {
		return failed("%d issues were added, want one", len(added))
	}
	filed := added[0]
	switch {
	case !slices.Contains(filed.Issue.Deps, fileWithDepBlocker):
		return failed("%s does not depend on %s (deps: %v)", filed.ID, fileWithDepBlocker, filed.Issue.Deps)
	case !hasHeading(filed.Issue.Body, acceptanceHeading):
		return failed("%s has no %q section", filed.ID, acceptanceHeading)
	}
	return passed()
}

// The ids move-under-parent works with: the Parent and the top-level issue the agent moves below
// it.
const (
	moveUnderParentParent = "billing"
	moveUnderParentChild  = "invoice-pdf"
)

// moveUnderParentIssues are move-under-parent's two top-level issues.
var moveUnderParentIssues = []seededIssue{
	{
		id:       moveUnderParentParent,
		title:    "Billing",
		body:     "Everything about charging customers and the documents they receive.",
		priority: 2,
	},
	{
		id:       moveUnderParentChild,
		title:    "Attach the invoice as a PDF",
		body:     "The invoice mail links to a web page; customers want the PDF attached.",
		priority: 3,
	},
}

// moveUnderParentScenario asks the agent to move a top-level issue below another. It passes when
// invoice-pdf's file sits in billing's children directory and no longer at the top level.
func moveUnderParentScenario() scenario {
	return scenario{
		id: "move-under-parent",
		prompt: "The issue invoice-pdf belongs under the issue billing. Put it there, keeping its " +
			"id.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, moveUnderParentIssues)
		},
		check: checkMoveUnderParent,
	}
}

// checkMoveUnderParent judges move-under-parent from where invoice-pdf's file is.
func checkMoveUnderParent(repo string, _ []shimCall) outcome {
	opened, err := store.Open(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	fileName := moveUnderParentChild + markdownExtension
	movedPath := filepath.Join(opened.IssuesDir(), moveUnderParentParent, fileName)
	oldPath := filepath.Join(opened.IssuesDir(), fileName)
	isMoved, err := fileExists(movedPath)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	isLeft, err := fileExists(oldPath)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	switch {
	case !isMoved:
		return failed("%s does not exist", opened.Relative(movedPath))
	case isLeft:
		return failed("%s still exists", opened.Relative(oldPath))
	}
	return passed()
}

// fileExists reports whether path names a file or directory; an error other than its absence is
// returned.
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// The ids rename-id works with: the issue's id before and after the rename.
const (
	renameIDOld = "fix-thing"
	renameIDNew = "fix-upload-timeout"
)

// renameIDIssues are rename-id's seed: the issue to rename, then the two issues that depend on
// it.
var renameIDIssues = []seededIssue{
	{
		id:       renameIDOld,
		title:    "Fix the upload timeout",
		body:     "An upload over a hundred megabytes times out after thirty seconds.",
		priority: 2,
	},
	{
		id:       "upload-progress-bar",
		title:    "Show a progress bar for uploads",
		body:     "A long upload shows nothing until it is done.",
		priority: 3,
		deps:     []string{renameIDOld},
	},
	{
		id:       "resumable-uploads",
		title:    "Resume an interrupted upload",
		body:     "A dropped connection restarts the upload from the first byte.",
		priority: 3,
		deps:     []string{renameIDOld},
	},
}

// renameIDScenario asks the agent to rename an issue two others depend on, keeping references to
// it working. It passes when fix-upload-timeout exists with the alias fix-thing and both
// dependents' deps name fix-upload-timeout.
func renameIDScenario() scenario {
	return scenario{
		id: "rename-id",
		prompt: "The id of the issue fix-thing says nothing. Rename it to fix-upload-timeout, so that " +
			"everything that refers to it keeps working.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, renameIDIssues)
		},
		check: checkRenameID,
	}
}

// checkRenameID judges rename-id from the renamed issue's file and its dependents' deps.
func checkRenameID(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	renamed, isFound := issues[renameIDNew]
	switch {
	case !isFound:
		return failed("%s does not exist", renameIDNew)
	case !slices.Contains(renamed.Issue.Aliases, renameIDOld):
		return failed("%s has no alias %s (aliases: %v)", renameIDNew, renameIDOld, renamed.Issue.Aliases)
	}
	for _, seeded := range renameIDIssues[1:] {
		dependent, isFound := issues[seeded.id]
		switch {
		case !isFound:
			return failed("%s is gone", seeded.id)
		case !slices.Contains(dependent.Issue.Deps, renameIDNew):
			return failed("%s does not depend on %s (deps: %v)", seeded.id, renameIDNew, dependent.Issue.Deps)
		}
	}
	return passed()
}

// closeChildrenParent is the id of close-children's standing Parent, the grouping issue the agent
// must leave open.
const closeChildrenParent = "v2-migration"

// closeChildrenIssues are close-children's seed: the standing Parent, then its four children, the
// last of them already closed.
var closeChildrenIssues = []seededIssue{
	{
		id:         closeChildrenParent,
		title:      "Migrate to the v2 API",
		body:       "Groups the work of moving every client of the v1 API to v2.",
		priority:   2,
		isStanding: true,
	},
	{
		id:       "v2-auth-endpoints",
		title:    "Move the auth endpoints to v2",
		body:     "Login and token refresh still call the v1 routes.",
		priority: 2,
		parent:   closeChildrenParent,
	},
	{
		id:       "v2-orders-endpoints",
		title:    "Move the orders endpoints to v2",
		body:     "The order list and order detail pages still call the v1 routes.",
		priority: 2,
		parent:   closeChildrenParent,
	},
	{
		id:       "v2-client-sdk",
		title:    "Release the client SDK for v2",
		body:     "The published SDK only speaks v1.",
		priority: 3,
		parent:   closeChildrenParent,
	},
	{
		id:          "v2-docs",
		title:       "Document the v2 API",
		body:        "The reference pages describe only the v1 routes.",
		priority:    3,
		parent:      closeChildrenParent,
		closeReason: "Published with the v2 beta",
	},
}

// closeChildrenScenario asks the agent to close the remaining children of a standing Parent and
// leave the Parent itself alone. It passes when every child is closed with a reason and
// v2-migration is open and still standing.
func closeChildrenScenario() scenario {
	return scenario{
		id: "close-children",
		prompt: "The remaining work under the issue v2-migration is done. Close all of it, but not " +
			"v2-migration itself: it only groups the work and stays open.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, closeChildrenIssues)
		},
		check: checkCloseChildren,
	}
}

// checkCloseChildren judges close-children from the Parent's and its children's issue files.
func checkCloseChildren(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	parent, isFound := issues[closeChildrenParent]
	switch {
	case !isFound:
		return failed("%s is gone", closeChildrenParent)
	case parent.IsClosed:
		return failed("%s, the grouping issue, was closed", closeChildrenParent)
	case !parent.Issue.IsStanding:
		return failed("%s is no longer standing", closeChildrenParent)
	}
	for _, seeded := range closeChildrenIssues[1:] {
		child, isFound := issues[seeded.id]
		switch {
		case !isFound:
			return failed("%s is gone", seeded.id)
		case !child.IsClosed:
			return failed("%s is not closed", seeded.id)
		case child.Issue.Reason == "":
			return failed("%s was closed without a reason", seeded.id)
		}
	}
	return passed()
}
