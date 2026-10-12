package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
		Title:   "Quoting: \"yes\" " + strings.Repeat("long title ", 12),
		Labels:  []string{"true", "cli"},
		Refs:    []string{"internal/cli/harness.go#func runCLI()", "docs/adr"},
		Aliases: []string{"old-rule"},
		Body:    "\nBody.\n",
	}

	data := original.Serialize()
	parsed := parseNote(t, string(data))

	if parsed.Title != original.Title || !slices.Equal(parsed.Labels, original.Labels) ||
		!slices.Equal(parsed.Refs, original.Refs) || !slices.Equal(parsed.Aliases, original.Aliases) ||
		parsed.Body != original.Body {
		t.Errorf("round trip of\n%s\n= %+v", data, parsed)
	}
	if strings.Count(string(data), "\n") != 8 {
		t.Errorf("serialised note is not one line per key:\n%s", data)
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
