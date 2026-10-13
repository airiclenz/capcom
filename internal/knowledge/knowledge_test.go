package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/tree"
)

// notePath is the Note file the tests attribute findings to.
const notePath = ".capcom/knowledge/some-note.md"

// newWorktree returns a temp working tree holding a source file and a directory.
func newWorktree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "cli"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := "package cli\n\nfunc runCLI() {}\n"
	if err := os.WriteFile(filepath.Join(root, "internal", "cli", "harness.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return root
}

// parseNote parses content as the Note file notePath, failing the test on an error.
func parseNote(t *testing.T, content string) Note {
	t.Helper()
	note, err := Parse(notePath, []byte(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return note
}

func TestParse_FullNote_ReadsFieldsAndRefLines(t *testing.T) {
	t.Parallel()
	content := "---\ntitle: Harness rule\nlabels: [cli]\nrefs:\n  - internal/cli\n  - internal/cli/harness.go#runCLI\n" +
		"aliases: [old-rule]\n---\nBody text.\n"

	note := parseNote(t, content)

	if note.Title != "Harness rule" || !slices.Equal(note.Labels, []string{"cli"}) ||
		!slices.Equal(note.Aliases, []string{"old-rule"}) || note.Body != "Body text.\n" {
		t.Errorf("note = %+v", note)
	}
	if !slices.Equal(note.Refs, []string{"internal/cli", "internal/cli/harness.go#runCLI"}) ||
		note.RefLine(0) != 5 || note.RefLine(1) != 6 {
		t.Errorf("refs = %v at lines %d, %d; want lines 5 and 6", note.Refs, note.RefLine(0), note.RefLine(1))
	}
}

func TestParse_BadFiles_FailCitingTheLine(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		content string
		line    int
	}{
		"no fence":     {"title: x\n", 1},
		"never closed": {"---\ntitle: x\n", 1},
		"unknown key":  {"---\ntitle: x\nstatus: open\n---\n", 3},
		"repeated":     {"---\ntitle: x\ntitle: y\n---\n", 3},
		"refs scalar":  {"---\ntitle: x\nrefs: a.go\n---\n", 3},
		"empty ref":    {"---\ntitle: x\nrefs: [a.go, \"\"]\n---\n", 3},
	}

	for name, current := range cases {
		_, err := Parse(notePath, []byte(current.content))

		var failure *Error
		if !errors.As(err, &failure) || failure.Line != current.line || failure.Path != notePath {
			t.Errorf("%s: Parse error = %v; want %s at line %d", name, err, notePath, current.line)
		}
	}
}

func TestParse_ConflictMarkerInFrontmatter_CitesTheMarker(t *testing.T) {
	t.Parallel()
	content := "---\n<<<<<<< HEAD\ntitle: Mine\n=======\ntitle: Theirs\n>>>>>>> other-branch\n---\n"

	_, err := Parse(notePath, []byte(content))

	want := notePath + `:2: git conflict marker "<<<<<<< HEAD"; resolve the conflict and delete the markers`
	if err == nil || err.Error() != want {
		t.Errorf("Parse error = %v, want %s", err, want)
	}
}

func TestParse_ConflictMarkerOnlyInBody_KeepsTheYAMLError(t *testing.T) {
	t.Parallel()
	content := "---\ntitle: [x\n---\n<<<<<<< HEAD\n=======\n>>>>>>> other-branch\n"

	_, err := Parse(notePath, []byte(content))

	if err == nil || !strings.Contains(err.Error(), "invalid YAML frontmatter") {
		t.Errorf("Parse error = %v, want the YAML error", err)
	}
}

func TestValidate_NoTitleOrPathlessRef_ReportsBoth(t *testing.T) {
	t.Parallel()
	note := parseNote(t, "---\nlabels: [x]\nrefs: [\"#symbol\"]\n---\n")

	problems := Validate(notePath, note)

	if len(problems) != 2 || !strings.Contains(problems[0].Error(), "title is required") ||
		problems[1].Line != 3 || !strings.Contains(problems[1].Message, "names no path") {
		t.Errorf("problems = %v", problems)
	}
}

func TestValidate_EmptyLabel_Reported(t *testing.T) {
	t.Parallel()
	note := Note{Title: "Some note", Labels: []string{"cli", "", ""}}

	problems := Validate(notePath, note)

	want := notePath + ": labels: every entry must be non-empty text"
	if len(problems) != 1 || problems[0].Error() != want {
		t.Errorf("problems = %v, want [%s]", problems, want)
	}
}

func TestSerialize_ThenParse_RoundTrips(t *testing.T) {
	t.Parallel()
	original := Note{
		Title:        "Quoting: \"yes\" " + strings.Repeat("long title ", 12),
		Labels:       []string{"true", "cli"},
		Refs:         []string{"internal/cli/harness.go#func runCLI()", "docs/adr"},
		IsRetired:    true,
		SupersededBy: []string{"new-rule", "other-rule"},
		Aliases:      []string{"old-rule"},
		Body:         "\nBody.\n",
	}

	data := original.Serialize()
	parsed := parseNote(t, string(data))

	if parsed.Title != original.Title || !slices.Equal(parsed.Labels, original.Labels) ||
		!slices.Equal(parsed.Refs, original.Refs) || !parsed.IsRetired ||
		!slices.Equal(parsed.SupersededBy, original.SupersededBy) ||
		!slices.Equal(parsed.Aliases, original.Aliases) || parsed.Body != original.Body {
		t.Errorf("round trip of\n%s\n= %+v", data, parsed)
	}
	if strings.Count(string(data), "\n") != 10 {
		t.Errorf("serialised note is not one line per key:\n%s", data)
	}
}

func TestSerialize_KeyOrder_RetiredAndSupersededByBeforeAliases(t *testing.T) {
	t.Parallel()
	note := Note{
		Title:        "Old rule",
		Refs:         []string{"docs/adr"},
		IsRetired:    true,
		SupersededBy: []string{"new-rule"},
		Aliases:      []string{"older-rule"},
	}

	data := note.Serialize()

	want := "---\ntitle: Old rule\nrefs: [docs/adr]\nretired: true\nsuperseded-by: [new-rule]\n" +
		"aliases: [older-rule]\n---\n"
	if string(data) != want {
		t.Errorf("Serialize =\n%s\nwant\n%s", data, want)
	}
}

func TestParse_RetiredFalse_IsDroppedOnSerialize(t *testing.T) {
	t.Parallel()
	note := parseNote(t, "---\ntitle: Kept rule\nretired: false\n---\n")

	data := note.Serialize()

	if note.IsRetired || string(data) != "---\ntitle: Kept rule\n---\n" {
		t.Errorf("IsRetired = %v, Serialize =\n%s", note.IsRetired, data)
	}
}

func TestParse_RetiredNotABoolean_FailsCitingTheLine(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"yes", "True", "[true]", "1"} {
		_, err := Parse(notePath, []byte("---\ntitle: x\nretired: "+value+"\n---\n"))

		want := notePath + ":3: retired: expected true or false"
		if err == nil || err.Error() != want {
			t.Errorf("retired: %s: Parse error = %v, want %s", value, err, want)
		}
	}
}

func TestValidate_SupersededBy_ReportsBadEntriesAtTheirLines(t *testing.T) {
	t.Parallel()
	note := parseNote(t, "---\ntitle: x\nsuperseded-by:\n"+
		"  - new-rule\n"+
		"  - some-note\n"+
		"  - New_Rule\n"+
		"  - new-rule\n"+
		"---\n")

	problems := Validate(notePath, note)

	want := []string{
		notePath + `:5: superseded-by "some-note" names the note itself`,
		notePath + `:6: superseded-by "New_Rule" is not an id: use only a-z, 0-9, dots and hyphens`,
		notePath + `:7: superseded-by "new-rule" is listed more than once`,
	}
	got := make([]string, 0, len(problems))
	for _, problem := range problems {
		got = append(got, problem.Error())
	}
	if !slices.Equal(got, want) {
		t.Errorf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestValidate_DistinctSuccessors_HaveNoProblems(t *testing.T) {
	t.Parallel()
	note := parseNote(t, "---\ntitle: x\nretired: true\nsuperseded-by: [new-rule, v2.rule]\n---\n")

	if problems := Validate(notePath, note); problems != nil {
		t.Errorf("problems = %v, want none", problems)
	}
}

func TestSerialize_TitleWithColon_IsDoubleQuoted(t *testing.T) {
	t.Parallel()
	note := Note{Title: "Fix: parser", Labels: []string{"it's"}, Body: "\nBody.\n"}

	data := note.Serialize()

	want := "---\ntitle: \"Fix: parser\"\nlabels: [it's]\n---\n\nBody.\n"
	if string(data) != want {
		t.Errorf("Serialize =\n%s\nwant\n%s", data, want)
	}
}

func TestParse_UnknownKey_NamesTheNoteKeys(t *testing.T) {
	t.Parallel()

	_, err := Parse(notePath, []byte("---\ntitle: x\nstatus: open\n---\n"))

	want := notePath + `:3: unknown key "status"; a note has title, labels, refs, retired, superseded-by and aliases`
	if err == nil || err.Error() != want {
		t.Errorf("Parse error = %v, want %s", err, want)
	}
}

func TestVerify_Refs_FlagOnlyTheBrokenOnes(t *testing.T) {
	t.Parallel()
	root := newWorktree(t)
	note := parseNote(t, "---\ntitle: Refs\nrefs:\n"+
		"  - internal/cli\n"+
		"  - internal/cli/harness.go\n"+
		"  - internal/cli/harness.go#func runCLI\n"+
		"  - internal/cli/gone.go\n"+
		"  - internal/cli/harness.go#func missingSymbol\n"+
		"  - ../outside.go\n"+
		"  - internal/../../outside.go\n"+
		"  - /etc/passwd\n"+
		"  - internal/cli#runCLI\n"+
		"  - internal/cli/harness.go/sub\n"+
		"---\n")

	findings, err := Verify(root, notePath, "some-note", note)

	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	var got []string
	for _, finding := range findings {
		got = append(got, finding.String())
	}
	want := []string{
		notePath + `:7: ref "internal/cli/gone.go": no such file or directory`,
		notePath + `:8: ref "internal/cli/harness.go#func missingSymbol": symbol "func missingSymbol" is not in the file`,
		notePath + `:9: ref "../outside.go": escapes the working tree`,
		notePath + `:10: ref "internal/../../outside.go": escapes the working tree`,
		notePath + `:11: ref "/etc/passwd": escapes the working tree: write a path relative to its root`,
		notePath + `:12: ref "internal/cli#runCLI": is a directory; symbol "runCLI" needs a file`,
		notePath + `:13: ref "internal/cli/harness.go/sub": no such file or directory`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestVerify_NoteWithoutRefs_HasNoFindings(t *testing.T) {
	t.Parallel()
	note := parseNote(t, "---\ntitle: Plain\n---\nNothing to check.\n")

	findings, err := Verify(newWorktree(t), notePath, "some-note", note)

	if err != nil || len(findings) != 0 {
		t.Errorf("Verify = %v, %v; want no findings", findings, err)
	}
}

func TestVerify_SymlinkOutOfTheTree_IsFlagged(t *testing.T) {
	t.Parallel()
	root := newWorktree(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "elsewhere")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	note := parseNote(t, "---\ntitle: Link\nrefs: [elsewhere]\n---\n")

	findings, err := Verify(root, notePath, "some-note", note)

	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].Message, "symbolic link") {
		t.Errorf("Verify = %v, %v; want the link flagged", findings, err)
	}
}

func TestVerify_SymlinkLoop_IsAFindingNotAnError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	root := newWorktree(t)
	if err := os.Symlink("loop", filepath.Join(root, "loop")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	note := parseNote(t, "---\ntitle: Loop\nrefs: [loop/x, internal/cli/missing.go]\n---\n")

	findings, err := Verify(root, notePath, "some-note", note)

	if err != nil || len(findings) != 2 || !strings.Contains(findings[0].Message, "cannot be resolved") {
		t.Errorf("Verify = %v, %v; want the loop reported and the next Ref still checked", findings, err)
	}
}

func TestVerify_NameTooLong_IsAFindingNotAnError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("name-length errors differ on Windows")
	}
	note := parseNote(t, "---\ntitle: Long\nrefs: ["+strings.Repeat("a", 300)+"]\n---\n")

	findings, err := Verify(newWorktree(t), notePath, "some-note", note)

	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].Message, "cannot be resolved") {
		t.Errorf("Verify = %v, %v; want the too-long name reported", findings, err)
	}
}

func TestVerify_UnreadableSymbolFile_IsAFindingNotAnError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not kept on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	root := newWorktree(t)
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("sym\n"), 0o000); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	note := parseNote(t, "---\ntitle: Secret\nrefs: [secret.txt#sym]\n---\n")

	findings, err := Verify(root, notePath, "some-note", note)

	if err != nil || len(findings) != 1 || findings[0].Message != "cannot be resolved: permission denied" {
		t.Errorf("Verify = %v, %v; want the unreadable file reported", findings, err)
	}
}

func TestVerify_FIFORef_IsAFindingNotAHang(t *testing.T) {
	t.Parallel()
	root := newWorktree(t)
	makeFIFO(t, filepath.Join(root, "pipe"))
	note := parseNote(t, "---\ntitle: Pipe\nrefs: [pipe#needle]\n---\n")

	findings, err := verifyWithin(t, root, note)

	want := `is not a regular file; symbol "needle" needs a file`
	if err != nil || len(findings) != 1 || findings[0].Message != want {
		t.Errorf("Verify = %v, %v; want one Finding %q", findings, err, want)
	}
}

func TestVerify_OversizedRef_IsAFinding(t *testing.T) {
	t.Parallel()
	root := newWorktree(t)
	content := "needle\n" + strings.Repeat("x", 1<<20+1-len("needle\n"))
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	note := parseNote(t, "---\ntitle: Big\nrefs: [big.txt#needle]\n---\n")

	findings, err := Verify(root, notePath, "some-note", note)

	want := `is larger than 1 MiB; symbol "needle" is not searched`
	if err != nil || len(findings) != 1 || findings[0].Message != want {
		t.Errorf("Verify = %v, %v; want one Finding %q", findings, err, want)
	}
}

// verifyWithin runs Verify on note, failing the test when it has not returned after 5 s, as it
// does when it blocks opening a named pipe.
func verifyWithin(t *testing.T, root string, note Note) ([]Finding, error) {
	t.Helper()
	type result struct {
		findings []Finding
		err      error
	}
	done := make(chan result, 1)
	go func() {
		findings, err := Verify(root, notePath, "some-note", note)
		done <- result{findings: findings, err: err}
	}()
	select {
	case got := <-done:
		return got.findings, got.err
	case <-time.After(5 * time.Second):
		t.Fatal("Verify did not return within 5 s")
		return nil, nil
	}
}

func TestValidateRef_Forms(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"internal/cli":             true,
		"internal/cli/a.go#Run":    true,
		"../outside":               false,
		"/abs/path":                false,
		"a.go#":                    false,
		"#symbol":                  false,
		"internal/../internal/cli": true,
	}

	for ref, isValid := range cases {
		err := ValidateRef(ref)

		if (err == nil) != isValid {
			t.Errorf("ValidateRef(%q) = %v; want valid %v", ref, err, isValid)
		}
	}
	if err := ValidateRef("../x"); !errors.Is(err, ErrEscapes) {
		t.Errorf("ValidateRef(../x) = %v; want ErrEscapes", err)
	}
}

// newNoteTree writes files (slash paths to content) into a fresh Knowledge directory and returns
// the tree loaded from it, each node's Aliases read from its file.
func newNoteTree(t *testing.T, files map[string]string) *tree.Tree {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	notes, err := tree.Load(dir, func(path string) ([]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		note, err := Parse(path, data)
		if err != nil {
			return nil, nil
		}
		return note.Aliases, nil
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return notes
}

// successorsIn parses the Note file name of notes, a slash path below its directory, and returns
// its superseded-by entries.
func successorsIn(t *testing.T, notes *tree.Tree, name string) []string {
	t.Helper()
	path := filepath.Join(notes.Dir(), filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	note, err := Parse(path, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return note.SupersededBy
}

func TestPlanMove_Rename_RewritesOtherNotesSuccessors(t *testing.T) {
	cases := []struct {
		name       string
		successors string
		want       []string
	}{
		{"by id", "[old-rule, other]", []string{"new-rule", "other"}},
		{"by alias", "[older-rule]", []string{"new-rule"}},
		{"beside the new id", "[old-rule, new-rule]", []string{"new-rule"}},
		{"after the new id", "[new-rule, old-rule]", []string{"new-rule"}},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			notes := newNoteTree(t, map[string]string{
				"old-rule.md":   "---\ntitle: Old rule\naliases: [older-rule]\n---\n",
				"superseded.md": "---\ntitle: Superseded\nsuperseded-by: " + current.successors + "\n---\n",
			})

			move, err := PlanMove(notes, "old-rule", tree.Destination{ID: "new-rule"})
			if err == nil {
				err = move.Apply()
			}

			if err != nil {
				t.Fatalf("PlanMove and Apply: %v", err)
			}
			if got := successorsIn(t, notes, "superseded.md"); !slices.Equal(got, current.want) {
				t.Errorf("superseded-by = %v; want %v", got, current.want)
			}
		})
	}
}

func TestPlanMove_ReparentOnly_RewritesNoOtherNote(t *testing.T) {
	notes := newNoteTree(t, map[string]string{
		"old-rule.md":   "---\ntitle: Old rule\n---\n",
		"parent.md":     "---\ntitle: Parent\n---\n",
		"superseded.md": "---\ntitle: Superseded\nsuperseded-by: [old-rule]\n---\n",
	})

	move, err := PlanMove(notes, "old-rule", tree.Destination{ID: "old-rule", ParentID: "parent"})
	if err == nil {
		err = move.Apply()
	}

	if err != nil {
		t.Fatalf("PlanMove and Apply: %v", err)
	}
	if got := successorsIn(t, notes, "superseded.md"); !slices.Equal(got, []string{"old-rule"}) {
		t.Errorf("superseded-by = %v; want [old-rule] untouched", got)
	}
}

func TestPlanMove_UnparseableOtherNote_JoinsUnread(t *testing.T) {
	notes := newNoteTree(t, map[string]string{
		"old-rule.md": "---\ntitle: Old rule\n---\n",
		"broken.md":   "no frontmatter\n",
	})

	move, err := PlanMove(notes, "old-rule", tree.Destination{ID: "new-rule"})

	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if len(move.Unread) != 1 || filepath.Base(move.Unread[0]) != "broken.md" {
		t.Errorf("Unread = %v; want broken.md", move.Unread)
	}
}

func TestPlanRemove_DropsSuccessorEntriesNamingTheNote(t *testing.T) {
	notes := newNoteTree(t, map[string]string{
		"old-rule.md":  "---\ntitle: Old rule\naliases: [older-rule]\n---\n",
		"by-id.md":     "---\ntitle: By id\nsuperseded-by: [old-rule, other]\n---\n",
		"by-alias.md":  "---\ntitle: By alias\nsuperseded-by: [older-rule]\n---\n",
		"bystander.md": "---\ntitle: Bystander\nsuperseded-by: [other]\n---\n",
	})

	removal, err := PlanRemove(notes, "old-rule")
	if err == nil {
		_, err = removal.Apply()
	}

	if err != nil {
		t.Fatalf("PlanRemove and Apply: %v", err)
	}
	if !slices.Equal(removal.SupersededByDropped, []string{"by-alias", "by-id"}) {
		t.Errorf("SupersededByDropped = %v; want [by-alias by-id]", removal.SupersededByDropped)
	}
	if got := successorsIn(t, notes, "by-id.md"); !slices.Equal(got, []string{"other"}) {
		t.Errorf("by-id superseded-by = %v; want [other]", got)
	}
	data, err := os.ReadFile(filepath.Join(notes.Dir(), "by-alias.md"))
	if err != nil || strings.Contains(string(data), "superseded-by") {
		t.Errorf("by-alias.md = %q, %v; want the key gone", data, err)
	}
}

func TestPlanRemove_UnparseableOtherNote_JoinsUnread(t *testing.T) {
	notes := newNoteTree(t, map[string]string{
		"old-rule.md": "---\ntitle: Old rule\n---\n",
		"broken.md":   "no frontmatter\n",
	})

	removal, err := PlanRemove(notes, "old-rule")

	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if len(removal.Unread) != 1 || filepath.Base(removal.Unread[0]) != "broken.md" {
		t.Errorf("Unread = %v; want broken.md", removal.Unread)
	}
}
