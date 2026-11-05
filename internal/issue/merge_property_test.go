package issue_test

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/rawfm"
)

// propertyIterationsVariable names the environment variable that overrides how many seeded cases
// a property test runs.
const propertyIterationsVariable = "CAPCOM_PROPERTY_ITERATIONS"

// defaultPropertyIterations is how many seeded cases a property test runs without the override.
const defaultPropertyIterations = 200

// propertyParagraphs is how many paragraphs the body of a generated issue holds above its Log.
const propertyParagraphs = 3

// propertyLogHeading opens the Log section of a generated issue, its last section.
const propertyLogHeading = "## Log"

// propertyStart is the date of the first Log entry a generated base issue can hold.
var propertyStart = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// Values the generator draws a field from.
var (
	propertyStatuses = []string{"open", "in_progress", "blocked"}
	propertyLabels   = []string{"bug", "docs", "perf", "ui"}
	propertyDeps     = []string{"alpha", "beta", "gamma"}
)

// propertyFile is one generated version of an issue file: the frontmatter fields an edit can
// change, the body paragraphs above the Log and the Log entries, oldest first.
type propertyFile struct {
	title      string
	status     string
	priority   int
	labels     []string
	deps       []string
	paragraphs []string
	log        []string
	// lastDay is the day, counted from propertyStart, of the newest Log entry.
	lastDay int
}

// render returns the file in the canonical form Serialize writes, its Log the last section.
func (f propertyFile) render() []byte {
	body := "\n" + strings.Join(f.paragraphs, "\n\n") + "\n\n" + propertyLogHeading + "\n\n" +
		strings.Join(f.log, "\n") + "\n"
	return issue.Issue{
		Title:    f.title,
		Status:   f.status,
		Priority: f.priority,
		Labels:   f.labels,
		Deps:     f.deps,
		Body:     body,
	}.Serialize()
}

// clone returns a copy of the file that shares no slice with it.
func (f propertyFile) clone() propertyFile {
	f.labels = slices.Clone(f.labels)
	f.deps = slices.Clone(f.deps)
	f.paragraphs = slices.Clone(f.paragraphs)
	f.log = slices.Clone(f.log)
	return f
}

// propertyCase is one seeded merge: a base issue file and the two sides edited from it.
type propertyCase struct {
	base   propertyFile
	ours   propertyFile
	theirs propertyFile
}

// newPropertyCase generates the case for seed: a base issue with one to three Log entries, and
// ours and theirs, each base after one to five random edits.
func newPropertyCase(seed uint64) propertyCase {
	random := rand.New(rand.NewPCG(seed, seed))
	base := propertyFile{
		title:    "Base title",
		status:   randomPick(random, propertyStatuses),
		priority: random.IntN(5),
		labels:   randomSubset(random, propertyLabels),
		deps:     randomSubset(random, propertyDeps),
		lastDay:  -1,
	}
	for index := range propertyParagraphs {
		base.paragraphs = append(base.paragraphs, fmt.Sprintf("Base paragraph %d.", index))
	}
	for index := range 1 + random.IntN(3) {
		base.appendEntry(random, "Base", fmt.Sprintf("base note %d", index))
	}
	return propertyCase{
		base:   base,
		ours:   editedFile(random, base, "Ours"),
		theirs: editedFile(random, base, "Theirs"),
	}
}

// editedFile returns base after one to five random edits made by side. Each Log entry it appends
// is dated after every entry before it, and its text names side and the edit, so no two entries
// of a case are equal.
func editedFile(random *rand.Rand, base propertyFile, side string) propertyFile {
	edited := base.clone()
	for index := range 1 + random.IntN(5) {
		change := fmt.Sprintf("%s change %d", strings.ToLower(side), index)
		switch random.IntN(8) {
		case 0:
			edited.title = "Title by " + change
		case 1:
			edited.status = randomPick(random, propertyStatuses)
		case 2:
			edited.priority = random.IntN(5)
		case 3:
			edited.labels = randomSubset(random, propertyLabels)
		case 4:
			edited.deps = randomSubset(random, propertyDeps)
		case 5:
			edited.paragraphs[random.IntN(propertyParagraphs)] = "Paragraph by " + change + "."
		default:
			edited.appendEntry(random, side, change)
		}
	}
	return edited
}

// appendEntry appends a Log entry by author with text, dated one to three days after the newest
// entry, and continued on an indented second line one time in four.
func (f *propertyFile) appendEntry(random *rand.Rand, author string, text string) {
	f.lastDay += 1 + random.IntN(3)
	entry := fmt.Sprintf(
		"- %s %s: %s",
		propertyStart.AddDate(0, 0, f.lastDay).Format(time.DateOnly),
		author,
		text,
	)
	if random.IntN(4) == 0 {
		entry += "\n  more on " + text
	}
	f.log = append(f.log, entry)
}

// randomPick returns one of values.
func randomPick(random *rand.Rand, values []string) string {
	return values[random.IntN(len(values))]
}

// randomSubset returns some of values, in their order, possibly none.
func randomSubset(random *rand.Rand, values []string) []string {
	var subset []string
	for _, value := range values {
		if random.IntN(2) == 0 {
			subset = append(subset, value)
		}
	}
	return subset
}

// propertyIterations returns how many seeded cases a property test runs:
// CAPCOM_PROPERTY_ITERATIONS when set, defaultPropertyIterations otherwise.
func propertyIterations(t *testing.T) int {
	t.Helper()
	value := os.Getenv(propertyIterationsVariable)
	if value == "" {
		return defaultPropertyIterations
	}
	iterations, err := strconv.Atoi(value)
	if err != nil || iterations < 1 {
		t.Fatalf("%s=%q: expected a whole number above 0", propertyIterationsVariable, value)
	}
	return iterations
}

// gitMergeFile returns a TextMerge that runs git merge-file on the three versions, written to
// files in dir.
func gitMergeFile(dir string) issue.TextMerge {
	labels := gitx.MergeLabels{Base: "base", Ours: "ours", Theirs: "theirs"}
	return func(base []byte, ours []byte, theirs []byte) ([]byte, bool, error) {
		paths := make([]string, 0, 3)
		for index, content := range [][]byte{base, ours, theirs} {
			path := filepath.Join(dir, strconv.Itoa(index))
			if err := os.WriteFile(path, content, 0o600); err != nil {
				return nil, false, err
			}
			paths = append(paths, path)
		}
		return gitx.MergeFile(paths[0], paths[1], paths[2], labels)
	}
}

func TestMergeFile_RandomEdits_KeepEveryLogEntry(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	textMerge := gitMergeFile(t.TempDir())
	iterations := propertyIterations(t)
	clean := 0

	for seed := range uint64(iterations) {
		generated := newPropertyCase(seed)
		base, ours, theirs := generated.base.render(), generated.ours.render(), generated.theirs.render()

		merged, hasConflict, err := issue.MergeFile(base, ours, theirs, textMerge)

		problem := ""
		switch {
		case err != nil:
			problem = fmt.Sprintf("MergeFile failed: %v", err)
		case hasConflict:
			problem = checkConflicted(merged)
		default:
			clean++
			problem = checkClean(generated, merged)
		}
		if problem != "" {
			t.Fatalf(
				"seed %d: %s\n--- base:\n%s--- ours:\n%s--- theirs:\n%s--- merged:\n%s",
				seed, problem, base, ours, theirs, merged,
			)
		}
	}
	t.Logf("%d of %d seeded merges were clean", clean, iterations)
}

// checkConflicted returns what is wrong with merged, a merge that reported a conflict, or "" when
// rawfm.ConflictMarkers finds a marker in it.
func checkConflicted(merged []byte) string {
	if len(rawfm.ConflictMarkers(merged)) == 0 {
		return "the merge reported a conflict but holds no conflict marker"
	}
	return ""
}

// checkClean returns what is wrong with merged, a merge of generated that reported no conflict,
// or "" when it holds no conflict marker, parses, keeps every Log entry of ours and of theirs
// exactly once and in date order, and takes each field one side changed from that side.
func checkClean(generated propertyCase, merged []byte) string {
	if markers := rawfm.ConflictMarkers(merged); len(markers) > 0 {
		return fmt.Sprintf("the merge reported no conflict but holds %q", markers[0].Text)
	}
	parsed, err := issue.Parse("merged.md", merged)
	if err != nil {
		return fmt.Sprintf("the merge does not parse: %v", err)
	}
	if problem := checkLog(generated, merged); problem != "" {
		return problem
	}
	return checkFields(generated, parsed)
}

// checkLog returns "" when the Log of merged holds every entry of ours and of theirs exactly once,
// nothing else, and in date order; otherwise what is wrong.
func checkLog(generated propertyCase, merged []byte) string {
	entries, isEntryOnly := issue.LogEntries(merged)
	if !isEntryOnly {
		return "the merged Log holds a line that is not part of an entry"
	}
	want := slices.Clone(generated.ours.log)
	for _, entry := range generated.theirs.log {
		if !slices.Contains(want, entry) {
			want = append(want, entry)
		}
	}
	for _, entry := range want {
		if count := countOf(entries, entry); count != 1 {
			return fmt.Sprintf("the merged Log holds %q %d times, want once", entry, count)
		}
	}
	if len(entries) != len(want) {
		return fmt.Sprintf(
			"the merged Log holds %d entries, want the %d of ours and theirs", len(entries), len(want),
		)
	}
	for index := 1; index < len(entries); index++ {
		if entryDay(entries[index]) < entryDay(entries[index-1]) {
			return fmt.Sprintf("the merged Log holds %q after %q", entries[index], entries[index-1])
		}
	}
	return ""
}

// countOf returns how many of entries equal entry.
func countOf(entries []string, entry string) int {
	count := 0
	for _, candidate := range entries {
		if candidate == entry {
			count++
		}
	}
	return count
}

// entryDay returns the YYYY-MM-DD date of a Log entry, which sorts as text in date order.
func entryDay(entry string) string {
	return entry[len("- ") : len("- ")+len(time.DateOnly)]
}

// propertyField is one field checkFields compares: its name, how to read it from a generated
// file, and its value in the merge.
type propertyField struct {
	name   string
	values func(file propertyFile) string
	merged string
}

// checkFields returns "" when every field of merged that only one side of generated changed holds
// that side's value; otherwise the first field that does not.
func checkFields(generated propertyCase, merged issue.Issue) string {
	base, ours, theirs := generated.base, generated.ours, generated.theirs
	fields := []propertyField{
		{"title", func(file propertyFile) string { return file.title }, merged.Title},
		{"status", func(file propertyFile) string { return file.status }, merged.Status},
		{
			"priority",
			func(file propertyFile) string { return strconv.Itoa(file.priority) },
			strconv.Itoa(merged.Priority),
		},
		{
			"labels",
			func(file propertyFile) string { return fmt.Sprint(file.labels) },
			fmt.Sprint(merged.Labels),
		},
		{
			"deps",
			func(file propertyFile) string { return fmt.Sprint(file.deps) },
			fmt.Sprint(merged.Deps),
		},
	}
	paragraphs := bodyParagraphs(merged.Body)
	for index := range propertyParagraphs {
		mergedParagraph := ""
		if index < len(paragraphs) {
			mergedParagraph = paragraphs[index]
		}
		fields = append(fields, propertyField{
			fmt.Sprintf("paragraph %d", index),
			func(file propertyFile) string { return file.paragraphs[index] },
			mergedParagraph,
		})
	}

	for _, field := range fields {
		want, isOneSided := oneSidedValue(field.values(base), field.values(ours), field.values(theirs))
		if isOneSided && field.merged != want {
			return fmt.Sprintf(
				"the merged %s is %q, want %q, changed on one side only",
				field.name, field.merged, want,
			)
		}
	}
	return ""
}

// oneSidedValue returns the value of the side that changed a field, and true, when exactly one
// side's value differs from base's.
func oneSidedValue(base string, ours string, theirs string) (string, bool) {
	switch {
	case ours != base && theirs == base:
		return ours, true
	case theirs != base && ours == base:
		return theirs, true
	}
	return "", false
}

// bodyParagraphs returns the paragraphs of an issue body above its Log section.
func bodyParagraphs(body string) []string {
	above, _, _ := strings.Cut(body, propertyLogHeading)
	return strings.Split(strings.TrimSpace(above), "\n\n")
}
