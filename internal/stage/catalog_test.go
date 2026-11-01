package stage_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/tree"
)

// newCatalogStage returns the Catalog of a stage over a store holding the issues parser-fix,
// lint-rules, token-a, token-b and report, with parser-cleanup, lint-style, token-c, layout-a and
// layout-b staged as creates and report staged active.
func newCatalogStage(t *testing.T) ops.Catalog {
	t.Helper()
	s, x := newIssueStore(t, map[string]string{
		"parser-fix": issueText("Fix the parser", "open"),
		"lint-rules": issueText("Lint rules", "open"),
		"token-a":    issueText("Token A", "open"),
		"token-b":    issueText("Token B", "open"),
		"report":     issueText("Report", "open"),
	})
	staged, overlay := newStaged(s, x)
	for _, id := range []string{"parser-cleanup", "lint-style", "token-c", "layout-a", "layout-b"} {
		if _, err := staged.CreateIssue(ops.NewIssue{ID: id, Title: id, Created: createdAt}); err != nil {
			t.Fatalf("CreateIssue %s: %v", id, err)
		}
	}
	if _, err := staged.Claim("report", ""); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	return overlay.Catalog(x)
}

func TestCatalog_Resolve_FindsTheIssuesAsStaged(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantID     string
		wantStatus string
	}{
		{name: "a staged create by its id", ref: "parser-cleanup", wantID: "parser-cleanup", wantStatus: "open"},
		{name: "a staged create by its citation spelling", ref: "Parser Cleanup", wantID: "parser-cleanup", wantStatus: "open"},
		{name: "a staged create by its prefix", ref: "parser-c", wantID: "parser-cleanup", wantStatus: "open"},
		{name: "an index issue by its prefix", ref: "parser-f", wantID: "parser-fix", wantStatus: "open"},
		{name: "a staged edit through the index", ref: "rep", wantID: "report", wantStatus: "active"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newCatalogStage(t)

			entry, err := c.Resolve(test.ref)

			if err != nil || entry.ID != test.wantID || entry.Status != test.wantStatus {
				t.Errorf("Resolve(%q) = %s %s, %v; want %s %s", test.ref, entry.ID, entry.Status, err,
					test.wantID, test.wantStatus)
			}
		})
	}
}

func TestCatalog_Resolve_PrefixOfSeveralIssues_IsAmbiguous(t *testing.T) {
	tests := []struct {
		name           string
		ref            string
		wantCandidates []string
	}{
		{name: "one index id and one staged id", ref: "lint", wantCandidates: []string{"lint-rules", "lint-style"}},
		{name: "two index ids and one staged id", ref: "token", wantCandidates: []string{"token-a", "token-b", "token-c"}},
		{name: "two staged ids", ref: "layout", wantCandidates: []string{"layout-a", "layout-b"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newCatalogStage(t)

			_, err := c.Resolve(test.ref)

			var ambiguous *tree.AmbiguousError
			if !errors.As(err, &ambiguous) || !slices.Equal(ambiguous.Candidates, test.wantCandidates) {
				t.Errorf("Resolve(%q) err = %v, want a *tree.AmbiguousError of %v", test.ref, err, test.wantCandidates)
			}
		})
	}
}

func TestCatalog_Resolve_NoMatch_IsTheIndexNotFound(t *testing.T) {
	c := newCatalogStage(t)

	_, err := c.Resolve("missing")

	var notFound *tree.NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("Resolve err = %v, want a *tree.NotFoundError", err)
	}
}

// aliasedIssueText returns the text of an open issue titled title whose aliases line lists
// aliases.
func aliasedIssueText(title string, aliases string) string {
	return strings.Replace(issueText(title, "open"), "\n---\n", "\naliases: ["+aliases+"]\n---\n", 1)
}

func TestCatalog_Resolve_AliasTheStageDropped_ResolvesAsTheCommittedTreeWould(t *testing.T) {
	isNotFound := func(err error) bool {
		var notFound *tree.NotFoundError
		return errors.As(err, &notFound)
	}
	isUnusable := func(err error) bool {
		var unusable *tree.UnusableError
		return errors.As(err, &unusable)
	}
	tests := []struct {
		name       string
		indexIDs   []string
		createIDs  []string
		stagedText string
		ref        string
		wantID     string
		isWantErr  func(error) bool
	}{
		{name: "no other match is not found", ref: "old", isWantErr: isNotFound},
		{name: "an index id it prefixes", indexIDs: []string{"older-thing"}, ref: "old", wantID: "older-thing"},
		{name: "a staged create it prefixes", createIDs: []string{"old-new"}, ref: "old", wantID: "old-new"},
		{name: "an alias the edit keeps", ref: "kept", wantID: "x"},
		{
			name:       "an unparsed staged file still listing it",
			stagedText: "---\ntitle: [unclosed\naliases: [old, kept]\n---\n",
			ref:        "old",
			isWantErr:  isUnusable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{"x": aliasedIssueText("X", "old, kept")}
			for _, id := range test.indexIDs {
				files[id] = issueText(id, "open")
			}
			s, x := newIssueStore(t, files)
			staged, overlay := newStaged(s, x)
			for _, id := range test.createIDs {
				if _, err := staged.CreateIssue(ops.NewIssue{ID: id, Title: id, Created: createdAt}); err != nil {
					t.Fatalf("CreateIssue %s: %v", id, err)
				}
			}
			stagedText := test.stagedText
			if stagedText == "" {
				stagedText = aliasedIssueText("X", "kept")
			}
			path := filepath.Join(s.IssuesDir(), "x.md")
			if err := overlay.ReplaceFile(path, strings.NewReader(stagedText)); err != nil {
				t.Fatalf("ReplaceFile: %v", err)
			}

			entry, err := overlay.Catalog(x).Resolve(test.ref)

			if test.isWantErr != nil {
				if !test.isWantErr(err) {
					t.Errorf("Resolve(%q) = %s, %v; want the error of case %q", test.ref, entry.ID, err, test.name)
				}
				return
			}
			if err != nil || entry.ID != test.wantID {
				t.Errorf("Resolve(%q) = %s, %v; want %s", test.ref, entry.ID, err, test.wantID)
			}
		})
	}
}

func TestCatalog_List_MergesTheStagedIssuesInPathOrder(t *testing.T) {
	tests := []struct {
		name    string
		filter  index.Filter
		wantIDs []string
	}{
		{name: "open issues", filter: index.Filter{}, wantIDs: []string{"alpha", "child", "beta", "report"}},
		{
			name:    "closed included",
			filter:  index.Filter{IsClosedIncluded: true},
			wantIDs: []string{"alpha", "child", "beta", "gamma", "report"},
		},
		{name: "limited", filter: index.Filter{Limit: 2}, wantIDs: []string{"alpha", "child"}},
		{name: "by status", filter: index.Filter{Statuses: []string{"active"}}, wantIDs: []string{"report"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, x := newIssueStore(t, map[string]string{
				"alpha":  issueText("Alpha", "open"),
				"gamma":  issueText("Gamma", "open"),
				"report": issueText("Report", "open"),
			})
			staged, overlay := newStaged(s, x)
			for _, spec := range []ops.NewIssue{
				{ID: "beta", Title: "Beta", Created: createdAt},
				{ID: "child", Parent: "alpha", Title: "Child", Created: createdAt},
			} {
				if _, err := staged.CreateIssue(spec); err != nil {
					t.Fatalf("CreateIssue %s: %v", spec.ID, err)
				}
			}
			if _, err := staged.Close("gamma", "", "done", "shipped", closedAt); err != nil {
				t.Fatalf("Close: %v", err)
			}
			if _, err := staged.Claim("report", ""); err != nil {
				t.Fatalf("Claim: %v", err)
			}

			entries, err := overlay.Catalog(x).List(test.filter)

			ids := make([]string, 0, len(entries))
			for _, entry := range entries {
				ids = append(ids, entry.ID)
			}
			if err != nil || !slices.Equal(ids, test.wantIDs) {
				t.Errorf("List = %v, %v; want %v", ids, err, test.wantIDs)
			}
		})
	}
}
