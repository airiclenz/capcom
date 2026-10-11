package config

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

const testPath = ".capcom/config.yaml"

func TestParse_DefaultFile_EqualsDefault(t *testing.T) {
	t.Parallel()

	config, err := Parse(testPath, []byte(DefaultFile))

	if err != nil {
		t.Fatalf("Parse(DefaultFile): %v", err)
	}
	if !reflect.DeepEqual(config, Default()) {
		t.Errorf("Parse(DefaultFile) = %+v, want %+v", config, Default())
	}
}

func TestParse_EmptyDocument_ReturnsDefault(t *testing.T) {
	t.Parallel()
	for _, content := range []string{"", "# only a comment\n", "~\n"} {
		config, err := Parse(testPath, []byte(content))

		if err != nil || !reflect.DeepEqual(config, Default()) {
			t.Errorf("Parse(%q) = %+v, %v; want the default config", content, config, err)
		}
	}
}

func TestDefault_ElsewhereAndLock_HaveDocumentedDefaults(t *testing.T) {
	t.Parallel()

	config := Default()

	if config.Elsewhere.Days != 30 || config.Lock.TTL != 2*time.Hour {
		t.Errorf("elsewhere.days = %d lock.ttl = %v, want 30 and 2h", config.Elsewhere.Days, config.Lock.TTL)
	}
}

func TestParse_PartialConfig_KeepsDefaultsForMissingKeys(t *testing.T) {
	t.Parallel()
	content := "lock:\n  ttl: 90m\n"

	config, err := Parse(testPath, []byte(content))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if config.Lock.TTL != 90*time.Minute {
		t.Errorf("lock.ttl = %v, want 90m", config.Lock.TTL)
	}
	if config.Elsewhere.Days != 30 || !reflect.DeepEqual(config.Statuses, Default().Statuses) {
		t.Errorf("config = %+v, want the remaining keys at their defaults", config)
	}
}

func TestParse_Statuses_PreservesFileOrder(t *testing.T) {
	t.Parallel()
	content := "statuses:\n  triage: todo\n  open: todo\n  review: doing\n  shipped: done\n"

	config, err := Parse(testPath, []byte(content))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []Status{
		{Name: "triage", Category: CategoryTodo},
		{Name: "open", Category: CategoryTodo},
		{Name: "review", Category: CategoryDoing},
		{Name: "shipped", Category: CategoryDone},
	}
	if !reflect.DeepEqual(config.Statuses, want) {
		t.Errorf("statuses = %+v, want %+v", config.Statuses, want)
	}
}

func TestConfig_DefaultStatus_IsFirstListedInCategory(t *testing.T) {
	t.Parallel()
	content := "statuses:\n  open: todo\n  shipped: done\n  triage: todo\n  merged: done\n"
	config, err := Parse(testPath, []byte(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	todoStatus, hasTodo := config.DefaultStatus(CategoryTodo)
	doneStatus, hasDone := config.DefaultStatus(CategoryDone)
	_, hasWaiting := config.DefaultStatus(CategoryWaiting)

	if todoStatus != "open" || !hasTodo || doneStatus != "shipped" || !hasDone {
		t.Errorf("defaults = %q/%v %q/%v, want open and shipped", todoStatus, hasTodo, doneStatus, hasDone)
	}
	if hasWaiting {
		t.Errorf("DefaultStatus(waiting) found a status, want none")
	}
}

func TestConfig_CategoryOf_LooksUpConfiguredStatus(t *testing.T) {
	t.Parallel()
	config := Default()

	category, isKnown := config.CategoryOf("parked")
	_, isUnknownKnown := config.CategoryOf("planned")

	if category != CategoryWaiting || !isKnown || isUnknownKnown {
		t.Errorf("CategoryOf = %q/%v, unknown known = %v", category, isKnown, isUnknownKnown)
	}
}

func TestParse_InvalidConfig_FailsWithPathAndLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		content  string
		wantLine int
	}{
		{"unknown top-level key", "types: [bug]\ncolour: blue\n", 2},
		{"unknown nested key", "lock:\n  ttl: 1h\n  owner: me\n", 3},
		{"invalid category", "statuses:\n  open: todo\n  review: reviewing\n  done: done\n", 3},
		{"no todo status", "statuses:\n  parked: waiting\n  done: done\n", 2},
		{"no done status", "statuses:\n  open: todo\n", 2},
		{"duplicate status", "statuses:\n  open: todo\n  done: done\n  open: doing\n", 4},
		{"empty types", "types: []\n", 1},
		{"duplicate type", "types:\n  - bug\n  - bug\n", 3},
		{"bad ttl", "lock:\n  ttl: soon\n", 2},
		{"negative ttl", "lock:\n  ttl: -1h\n", 2},
		{"negative days", "elsewhere:\n  days: -2\n", 2},
		{"fractional days", "elsewhere:\n  days: 1.5\n", 2},
		{"plans not a list", "\nplans: docs/plans\n", 2},
		{"not a mapping", "- open\n", 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(testPath, []byte(testCase.content))

			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("err = %v, want *config.Error", err)
			}
			if failure.Path != testPath || failure.Line != testCase.wantLine {
				t.Errorf("error at %s:%d (%v), want %s:%d", failure.Path, failure.Line, err, testPath, testCase.wantLine)
			}
		})
	}
}

func TestParse_MalformedYAML_NamesFile(t *testing.T) {
	t.Parallel()

	_, err := Parse(testPath, []byte("statuses: [open\n"))

	var failure *Error
	if !errors.As(err, &failure) || failure.Path != testPath {
		t.Errorf("err = %v, want a *config.Error naming %s", err, testPath)
	}
}

func TestError_Error_PrefixesPathAndLine(t *testing.T) {
	t.Parallel()

	withLine := (&Error{Path: "c.yaml", Line: 3, Message: "bad"}).Error()
	withoutLine := (&Error{Path: "c.yaml", Message: "bad"}).Error()

	if withLine != "c.yaml:3: bad" || withoutLine != "c.yaml: bad" {
		t.Errorf("Error() = %q / %q", withLine, withoutLine)
	}
}
