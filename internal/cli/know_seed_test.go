package cli

import (
	"bytes"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/airiclenz/capcom/internal/seed"
)

// madeUpSeedFS is a base set made up for tests: a root Note and two Notes below it.
var madeUpSeedFS = fstest.MapFS{
	"house.md":               {Data: []byte("---\ntitle: House rules\n---\n\nRead the rules below.\n")},
	"house/house-commits.md": {Data: []byte("---\ntitle: One change per commit\n---\n\nSplit unrelated changes.\n")},
	"house/house-tests.md":   {Data: []byte("---\ntitle: Run the tests first\n---\n\nA red suite is fixed first.\n")},
}

// loadMadeUpSeed loads madeUpSeedFS as the base set.
func loadMadeUpSeed() (seed.Set, error) {
	return seed.LoadFS(madeUpSeedFS)
}

// runSeedCLI is runCLI with loadSeed as the loader of the base set in place of seed.Load.
func runSeedCLI(
	t testing.TB,
	dir string,
	loadSeed func() (seed.Set, error),
	args ...string,
) (stdout, stderr string, exitCode int) {
	t.Helper()
	t.Chdir(dir)
	setHermeticGitEnv(t)

	var stdoutBuffer, stderrBuffer bytes.Buffer
	application := &app{stdout: &stdoutBuffer, stderr: &stderrBuffer, loadSeed: loadSeed}
	exitCode = application.run(args, strings.NewReader(""))
	return stdoutBuffer.String(), stderrBuffer.String(), exitCode
}

// newSeedStore returns the root of a git repository with an empty capcom store.
func newSeedStore(t *testing.T) string {
	t.Helper()
	root := newGitRepo(t)
	mustRun(t, root, "init")
	return root
}

// shippedSeed returns the base set the binary embeds.
func shippedSeed(t *testing.T) seed.Set {
	t.Helper()
	set, err := seed.Load()
	if err != nil {
		t.Fatalf("seed.Load: %v", err)
	}
	return set
}

// seedChildren returns the Notes of set directly below its root, at least two.
func seedChildren(t *testing.T, set seed.Set) []seed.Note {
	t.Helper()
	var children []seed.Note
	for _, note := range set.Notes() {
		if note.ParentID == set.Root().ID {
			children = append(children, note)
		}
	}
	if len(children) < 2 {
		t.Fatalf("the base set has %d Notes below its root, want two or more", len(children))
	}
	return children
}

// seededChildFile is the slash path below .capcom/knowledge/ a seed writes child to, a Note
// directly below the root of set.
func seededChildFile(set seed.Set, child seed.Note) string {
	return set.Root().ID + "/" + child.ID + ".md"
}

// seedOutcomes reads the text of capcom know seed into the outcome of each id.
func seedOutcomes(t *testing.T, stdout string) map[string]string {
	t.Helper()
	outcomes := make(map[string]string)
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		id, outcome, isNoteLine := strings.Cut(line, "  ")
		if !isNoteLine {
			continue
		}
		outcomes[id] = outcome
	}
	return outcomes
}

// assertSeedOutcomes fails t unless stdout names every Note of set, each with the outcome want
// gives for its id, or fallback when want has none.
func assertSeedOutcomes(t *testing.T, set seed.Set, stdout string, fallback string, want map[string]string) {
	t.Helper()
	outcomes := seedOutcomes(t, stdout)
	if len(outcomes) != len(set.Notes()) {
		t.Errorf("know seed named %d Notes, want %d:\n%s", len(outcomes), len(set.Notes()), stdout)
	}
	for _, note := range set.Notes() {
		wanted, isSpecial := want[note.ID]
		if !isSpecial {
			wanted = fallback
		}
		if outcomes[note.ID] != wanted {
			t.Errorf("outcome of %s = %q, want %q", note.ID, outcomes[note.ID], wanted)
		}
	}
}

// knowledgeFiles reads every file below .capcom/knowledge/ into its content by slash path.
func knowledgeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	dir := knowledgePath(root, "")
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir %s: %v", dir, err)
	}
	return files
}

// writeNoteFile replaces the content of the Note file at the slash path name below
// .capcom/knowledge/.
func writeNoteFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(knowledgePath(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestKnowSeed_FreshStore_AddsEveryNoteAndCheckPasses(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedAdded, nil)
	if got := len(knowledgeFiles(t, root)); got != len(set.Notes()) {
		t.Errorf("the store holds %d Note files, want %d", got, len(set.Notes()))
	}
	for _, note := range set.Notes() {
		shown := mustRun(t, root, "know", "show", note.ID)
		if !strings.Contains(shown, "title: "+note.Title+"\n") || !strings.Contains(shown, "seed:"+set.Root().ID) {
			t.Errorf("know show %s =\n%s\nwant the set's title and the seed Label", note.ID, shown)
		}
		if parent := "parent: " + note.ParentID + "\n"; note.ParentID != "" && !strings.Contains(shown, parent) {
			t.Errorf("know show %s lacks %q", note.ID, parent)
		}
	}
	if checked, stderr, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after know seed = %d\n%s%s", exitCode, checked, stderr)
	}
}

func TestKnowSeed_SecondRun_SkipsEveryNoteAsSameAndChangesNoFile(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	mustRun(t, root, "know", "seed")
	before := knowledgeFiles(t, root)

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedSame, nil)
	if !maps.Equal(before, knowledgeFiles(t, root)) {
		t.Errorf("the second know seed changed a Note file")
	}
}

func TestKnowSeed_EditedNote_KeepsItsEditAndSkipsAsDiffers(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	name := seededChildFile(set, child)
	mustRun(t, root, "know", "seed")
	edited := readNoteFile(t, root, name) + "\nThis repository adds a line of its own.\n"
	writeNoteFile(t, root, name, edited)

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedSame, map[string]string{child.ID: seedSkippedDiffers})
	if got := readNoteFile(t, root, name); got != edited {
		t.Errorf("the edited Note = %q, want the edit kept", got)
	}
}

func TestKnowSeed_CRLFText_IsSame(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	name := seededChildFile(set, child)
	mustRun(t, root, "know", "seed")
	writeNoteFile(t, root, name, strings.ReplaceAll(readNoteFile(t, root, name), "\n", "\r\n"))

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedSame, nil)
}

func TestKnowSeed_ChildIDHeldByTheReposOwnNote_SkipsItAndAddsTheRest(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	mustRun(t, root, "know", "add", "--id", child.ID, "--title", "The repository's own Note")
	own := readNoteFile(t, root, child.ID+".md")

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedAdded, map[string]string{child.ID: seedSkippedDiffers})
	if got := readNoteFile(t, root, child.ID+".md"); got != own {
		t.Errorf("the repository's own Note = %q, want it untouched", got)
	}
	if noteFileExists(t, root, seededChildFile(set, child)) {
		t.Errorf("know seed wrote a second file for %s", child.ID)
	}
}

func TestKnowSeed_IDHeldAsAnAlias_SkipsItNamingTheNote(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	mustRun(t, root, "know", "seed")
	mustRun(t, root, "know", "mv", child.ID, "renamed-rule")

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedSame, map[string]string{child.ID: "skipped (alias of renamed-rule)"})
	if noteFileExists(t, root, seededChildFile(set, child)) {
		t.Errorf("know seed added %s again beside the Note that holds it as an Alias", child.ID)
	}
}

func TestKnowSeed_RetiredChild_StaysRetired(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	mustRun(t, root, "know", "seed")
	mustRun(t, root, "know", "update", child.ID, "--retire")
	retired := readNoteFile(t, root, seededChildFile(set, child))

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedSame, map[string]string{child.ID: seedSkippedRetired})
	if got := readNoteFile(t, root, seededChildFile(set, child)); got != retired {
		t.Errorf("the retired Note = %q, want it untouched", got)
	}
}

func TestKnowSeed_RetiredRoot_SkipsEveryNoteAndAddsNone(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	mustRun(t, root, "know", "seed")
	mustRun(t, root, "know", "rm", child.ID)
	mustRun(t, root, "know", "update", set.Root().ID, "--retire")

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedRetired, nil)
	if noteFileExists(t, root, seededChildFile(set, child)) {
		t.Errorf("know seed added %s below a retired root", child.ID)
	}
}

func TestKnowSeed_RemovedChild_IsAddedAgain(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	mustRun(t, root, "know", "seed")
	seeded := readNoteFile(t, root, seededChildFile(set, child))
	mustRun(t, root, "know", "rm", child.ID)

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedSame, map[string]string{child.ID: seedAdded})
	if got := readNoteFile(t, root, seededChildFile(set, child)); got != seeded {
		t.Errorf("the Note added again = %q, want %q", got, seeded)
	}
}

func TestKnowSeed_MovedChild_IsSkippedWhereItIs(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	child := seedChildren(t, set)[0]
	mustRun(t, root, "know", "seed")
	mustRun(t, root, "know", "mv", child.ID, "--parent", ".")

	stdout := mustRun(t, root, "know", "seed")

	assertSeedOutcomes(t, set, stdout, seedSkippedSame, nil)
	if noteFileExists(t, root, seededChildFile(set, child)) || !noteFileExists(t, root, child.ID+".md") {
		t.Errorf("know seed did not leave %s where it was moved to", child.ID)
	}
}

func TestKnowSeed_RootWithoutTheSeedLabel_FailsAndWritesNothing(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	rootID := set.Root().ID
	mustRun(t, root, "know", "add", "--id", rootID, "--title", "The repository's own Note")
	before := knowledgeFiles(t, root)

	stdout, stderr, exitCode := runCLI(t, root, "know", "seed")

	wantFile := ".capcom/knowledge/" + rootID + ".md: "
	if exitCode != ExitError || stdout != "" || !strings.Contains(stderr, wantFile) ||
		!strings.Contains(stderr, "seed:"+rootID) {
		t.Errorf("know seed = %d\nstdout = %q\nstderr = %q\nwant an error at %s naming the seed Label",
			exitCode, stdout, stderr, wantFile)
	}
	if !maps.Equal(before, knowledgeFiles(t, root)) {
		t.Errorf("know seed wrote below a root Note that was not seeded")
	}
	jsonOutput, _, _ := runCLI(t, root, "know", "seed", "--json")
	failure, _ := decodeEnvelope(t, jsonOutput)["error"].(map[string]any)
	if failure["code"] != string(CodeInvalid) {
		t.Errorf("know seed --json error = %v, want code invalid", failure)
	}
}

func TestKnowSeed_DryRun_PrintsTheOutcomesAndWritesNothing(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	mustRun(t, root, "agents")
	guide := readAgentsGuide(t, root)

	stdout := mustRun(t, root, "know", "seed", "--dry-run")

	assertSeedOutcomes(t, set, stdout, seedAdded, nil)
	if !strings.HasSuffix(stdout, dryRunLine+"\n") {
		t.Errorf("know seed --dry-run = %q, want it to end in %q", stdout, dryRunLine)
	}
	if files := knowledgeFiles(t, root); len(files) != 0 {
		t.Errorf("know seed --dry-run wrote %v", files)
	}
	if got := readAgentsGuide(t, root); got != guide {
		t.Errorf("know seed --dry-run changed the agent guide")
	}
	jsonOutput := mustRun(t, root, "know", "seed", "--dry-run", "--json")
	data, _ := decodeEnvelope(t, jsonOutput)["data"].(map[string]any)
	notes, _ := data["notes"].([]any)
	if data["dry_run"] != true || len(notes) != len(set.Notes()) {
		t.Errorf("know seed --dry-run --json data = %v, want dry_run and every Note", data)
	}
}

func TestKnowSeed_AgentGuide_GainsExactlyOneLine(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	mustRun(t, root, "know", "add", "--id", "own-rule", "--title", "The repository's own rule")
	mustRun(t, root, "agents")
	before := strings.Split(readAgentsGuide(t, root), "\n")

	stdout := mustRun(t, root, "know", "seed")

	guide := readAgentsGuide(t, root)
	after := strings.Split(guide, "\n")
	wantLine := "- `" + set.Root().ID + "`: " + set.Root().Title + "\n"
	if len(after) != len(before)+1 || !strings.Contains(guide, wantLine) {
		t.Errorf("the agent guide went from %d to %d lines, want one more, the line %q:\n%s",
			len(before), len(after), wantLine, guide)
	}
	if !strings.HasSuffix(stdout, "refreshed: AGENTS.md\n") {
		t.Errorf("know seed = %q, want it to report the refreshed block", stdout)
	}
	if checked, stderr, exitCode := runCLI(t, root, "check"); exitCode != ExitOK {
		t.Errorf("check after know seed = %d\n%s%s", exitCode, checked, stderr)
	}
}

func TestKnowSeed_FailedWrite_StopsAfterTheOutcomesSoFar(t *testing.T) {
	root := newSeedStore(t)
	set := shippedSeed(t)
	children := seedChildren(t, set)
	blocked, last := children[0], children[len(children)-1]
	// A node file in a directory without a node file of its own is left out of the tree, and
	// still claims its id.
	stray := knowledgePath(root, "stray/"+blocked.ID+".md")
	if err := os.MkdirAll(filepath.Dir(stray), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeNoteFile(t, root, "stray/"+blocked.ID+".md", "---\ntitle: A stray Note\n---\n")

	stdout, stderr, exitCode := runCLI(t, root, "know", "seed")

	if exitCode != ExitError || !strings.Contains(stderr, "stray/"+blocked.ID+".md") {
		t.Errorf("know seed = %d, stderr = %q; want an error naming the stray file", exitCode, stderr)
	}
	outcomes := seedOutcomes(t, stdout)
	if len(outcomes) != 1 || outcomes[set.Root().ID] != seedAdded {
		t.Errorf("know seed outcomes = %v, want only the root Note, added", outcomes)
	}
	if !noteFileExists(t, root, set.Root().ID+".md") || noteFileExists(t, root, seededChildFile(set, last)) {
		t.Errorf("want the root Note written and nothing after the failed write")
	}
}

func TestKnowSeed_LoaderOfTheApp_ReplacesTheEmbeddedSet(t *testing.T) {
	root := newSeedStore(t)
	set, err := loadMadeUpSeed()
	if err != nil {
		t.Fatalf("seed.LoadFS: %v", err)
	}

	stdout, stderr, exitCode := runSeedCLI(t, root, loadMadeUpSeed, "know", "seed")

	if exitCode != ExitOK {
		t.Fatalf("know seed = %d\n%s%s", exitCode, stdout, stderr)
	}
	assertSeedOutcomes(t, set, stdout, seedAdded, nil)
	if got := len(knowledgeFiles(t, root)); got != len(set.Notes()) {
		t.Errorf("the store holds %d Note files, want %d", got, len(set.Notes()))
	}
}

func TestKnowSeed_KV_IsRefusedBeforeAnyWrite(t *testing.T) {
	root := newSeedStore(t)

	_, _, exitCode := runCLI(t, root, "know", "seed", "--kv")

	if exitCode != ExitUsage {
		t.Errorf("know seed --kv = %d, want a usage error", exitCode)
	}
	if files := knowledgeFiles(t, root); len(files) != 0 {
		t.Errorf("know seed --kv wrote %d files before it was refused", len(files))
	}
}
