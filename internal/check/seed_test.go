package check_test

import (
	"fmt"
	"path"
	"reflect"
	"testing"

	"github.com/airiclenz/capcom/internal/check"
	knowledgepkg "github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/seed"
	"github.com/airiclenz/capcom/internal/store"
)

const (
	// seedingVersion is the capcom version the fixtures were seeded by.
	seedingVersion = "0.0.1"
	// earlierSeedLine is a body line of the text an earlier capcom seeded a Note with.
	earlierSeedLine = "An earlier capcom shipped this line.\n"
)

// baseSet returns the base set the binary embeds.
func baseSet(t *testing.T) seed.Set {
	t.Helper()
	set, err := seed.Load()
	if err != nil {
		t.Fatalf("seed.Load: %v", err)
	}
	return set
}

// childOf returns a Note of set directly below its root.
func childOf(t *testing.T, set seed.Set) seed.Note {
	t.Helper()
	for _, note := range set.Notes() {
		if note.ParentID == set.Root().ID {
			return note
		}
	}
	t.Fatalf("the base set has no Note below its root")
	return seed.Note{}
}

// seedPath is the slash path, relative to the working tree root, of the file a seed writes the
// Note of set with the given id to: below the directory of each of its ancestors.
func seedPath(t *testing.T, set seed.Set, id string) string {
	t.Helper()
	name := id + ".md"
	for current := id; ; {
		note, isInSet := set.Note(current)
		if !isInSet {
			t.Fatalf("the base set holds no Note %q", current)
		}
		if note.ParentID == "" {
			return knowledge + name
		}
		name = path.Join(note.ParentID, name)
		current = note.ParentID
	}
}

// seededFile is the Note file a seed by capcom seedingVersion writes for text, a Note of set or
// an earlier text of one: its title and body with the two seed Labels.
func seededFile(set seed.Set, text seed.Note) knowledgepkg.Note {
	return knowledgepkg.Note{Title: text.Title, Body: text.Body, Labels: set.Labels(text, seedingVersion)}
}

// earlierText is note with the text an earlier capcom shipped for it.
func earlierText(note seed.Note) seed.Note {
	note.Body += earlierSeedLine
	return note
}

// seededStore returns the files of a store freshly seeded with set, by path relative to the
// working tree root.
func seededStore(t *testing.T, set seed.Set) map[string]knowledgepkg.Note {
	t.Helper()
	files := map[string]knowledgepkg.Note{}
	for _, note := range set.Notes() {
		files[seedPath(t, set, note.ID)] = seededFile(set, note)
	}
	return files
}

// writeSeedStore writes files, and extra as it is, into a new store and returns its root.
func writeSeedStore(t *testing.T, files map[string]knowledgepkg.Note, extra map[string]string) string {
	t.Helper()
	root := newRepo(t)
	content := map[string]string{}
	for name, note := range files {
		content[name] = string(note.Serialize())
	}
	for name, text := range extra {
		content[name] = text
	}
	writeFiles(t, root, content)
	return root
}

// runSeedWarnings opens the store in root and returns its seed warnings, failing the test on
// error.
func runSeedWarnings(t *testing.T, root string) []string {
	t.Helper()
	opened, err := store.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	warnings, err := check.SeedWarnings(opened)
	if err != nil {
		t.Fatalf("SeedWarnings: %v", err)
	}
	return warnings
}

// wantOutdatedWarning is the warning for the outdated seeded Note of set with the given id.
func wantOutdatedWarning(t *testing.T, set seed.Set, id string) string {
	t.Helper()
	return fmt.Sprintf(
		"%s: seeded Note %s is outdated: capcom know seed --update replaces it, "+
			"capcom know seed --show %s prints the new text",
		seedPath(t, set, id), id, id,
	)
}

func TestSeedWarnings_NoOutdatedNote_WarnsOfNothing(t *testing.T) {
	set := baseSet(t)
	child := childOf(t, set)
	childPath := seedPath(t, set, child.ID)
	rootPath := seedPath(t, set, set.Root().ID)

	unseeded := map[string]knowledgepkg.Note{knowledge + "own-note.md": {Title: "Own note", Body: "\nText.\n"}}
	edited := seededStore(t, set)
	editedNote := edited[childPath]
	editedNote.Body += "The repository's own line.\n"
	edited[childPath] = editedNote
	retired := seededStore(t, set)
	retiredNote := seededFile(set, earlierText(child))
	retiredNote.IsRetired = true
	retired[childPath] = retiredNote
	belowRetired := seededStore(t, set)
	belowRetired[childPath] = seededFile(set, earlierText(child))
	retiredRoot := belowRetired[rootPath]
	retiredRoot.IsRetired = true
	belowRetired[rootPath] = retiredRoot

	cases := map[string]map[string]knowledgepkg.Note{
		"a store that was never seeded":        unseeded,
		"a freshly seeded store":               seededStore(t, set),
		"an edited Note":                       edited,
		"a retired outdated Note":              retired,
		"an outdated Note below a retired one": belowRetired,
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			if warnings := runSeedWarnings(t, writeSeedStore(t, files, nil)); len(warnings) != 0 {
				t.Errorf("SeedWarnings = %q, want none", warnings)
			}
		})
	}
}

func TestSeedWarnings_OutdatedNote_WarnsOnceNamingItsFileAndTheUpdate(t *testing.T) {
	set := baseSet(t)
	child := childOf(t, set)
	files := seededStore(t, set)
	files[seedPath(t, set, child.ID)] = seededFile(set, earlierText(child))

	warnings := runSeedWarnings(t, writeSeedStore(t, files, nil))

	if want := []string{wantOutdatedWarning(t, set, child.ID)}; !reflect.DeepEqual(warnings, want) {
		t.Errorf("SeedWarnings = %q, want %q", warnings, want)
	}
}

func TestSeedWarnings_StoreWithAProblem_StillWarnsWithoutAnError(t *testing.T) {
	set := baseSet(t)
	child := childOf(t, set)
	files := seededStore(t, set)
	files[seedPath(t, set, child.ID)] = seededFile(set, earlierText(child))
	unparseable := map[string]string{knowledge + "broken-note.md": "---\ntitle: [unclosed\n---\n"}

	root := writeSeedStore(t, files, unparseable)

	warnings := runSeedWarnings(t, root)

	if problems := runCheck(t, root); len(problems) == 0 {
		t.Fatalf("Run reports no problem, want one for the unparseable Note")
	}
	if want := []string{wantOutdatedWarning(t, set, child.ID)}; !reflect.DeepEqual(warnings, want) {
		t.Errorf("SeedWarnings = %q, want %q", warnings, want)
	}
}

func TestRun_OutdatedSeededNote_IsNoProblem(t *testing.T) {
	set := baseSet(t)
	child := childOf(t, set)
	files := seededStore(t, set)
	files[seedPath(t, set, child.ID)] = seededFile(set, earlierText(child))

	problems := runCheck(t, writeSeedStore(t, files, nil))

	if len(problems) != 0 {
		t.Errorf("Run = %v, want no problem for an outdated seeded Note", problems)
	}
}
