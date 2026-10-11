package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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

func TestParse_EmptyDocument_ReturnsDefaultAtFormatOne(t *testing.T) {
	t.Parallel()
	want := Default()
	want.Format = 1
	for _, content := range []string{"", "# only a comment\n", "~\n"} {
		config, err := Parse(testPath, []byte(content))

		if err != nil || !reflect.DeepEqual(config, want) {
			t.Errorf("Parse(%q) = %+v, %v; want the default config at format 1", content, config, err)
		}
	}
}

func TestParse_DefaultFile_DeclaresCurrentFormat(t *testing.T) {
	t.Parallel()

	config, err := Parse(testPath, []byte(DefaultFile))

	if err != nil || config.Format != 2 || !strings.Contains(DefaultFile, "\nformat: 2\n") {
		t.Errorf("Parse(DefaultFile) = format %d, %v; want a real format: 2 line", config.Format, err)
	}
}

func TestDefault_ElsewhereAndLock_HaveDocumentedDefaults(t *testing.T) {
	t.Parallel()

	config := Default()

	if config.Elsewhere.Days != 30 || config.Lock.TTL != 2*time.Hour {
		t.Errorf("elsewhere.days = %d lock.ttl = %v, want 30 and 2h", config.Elsewhere.Days, config.Lock.TTL)
	}
}

func TestDefault_DoingCategory_HasActiveStatus(t *testing.T) {
	t.Parallel()

	status, hasDoing := Default().DefaultStatus(CategoryDoing)

	if !hasDoing || status != "active" {
		t.Errorf("DefaultStatus(doing) = %q, %v, want \"active\", true", status, hasDoing)
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

func TestParse_Format_MissingOrOneLoads(t *testing.T) {
	t.Parallel()
	for _, content := range []string{"types: [bug]\n", "format: 1\ntypes: [bug]\n"} {
		config, err := Parse(testPath, []byte(content))

		if err != nil || config.Format != 1 {
			t.Errorf("Parse(%q) = format %d, %v; want format 1", content, config.Format, err)
		}
	}
}

func TestParse_FormatTwo_Loads(t *testing.T) {
	t.Parallel()

	config, err := Parse(testPath, []byte("format: 2\ntypes: [bug]\n"))

	if err != nil || config.Format != 2 {
		t.Errorf("Parse = format %d, %v; want format 2", config.Format, err)
	}
}

func TestParse_NewerFormat_FailsAsTooNewWithLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		content  string
		wantLine int
	}{
		{"format alone", "format: 3\n", 1},
		{"after other keys", "types: [bug]\n\nformat: 3\n", 3},
		{"unknown key above format", "colour: blue\nformat: 3\n", 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(testPath, []byte(testCase.content))

			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("err = %v, want *config.Error", err)
			}
			want := fmt.Sprintf("%s:%d: store format 3 is newer than this capcom reads (up to 2); upgrade capcom",
				testPath, testCase.wantLine)
			if err.Error() != want {
				t.Errorf("err = %q, want %q", err.Error(), want)
			}
		})
	}
}

// formatOneTemplate is the config.yaml `capcom init` wrote while the store format was 1: the
// current template with the format key left as a comment under its own comment line.
func formatOneTemplate(t *testing.T) string {
	t.Helper()
	current := formatComment + "\nformat: 2\n"
	legacy := "# The store format version; left as a comment, since a missing key means format 1.\n# format: 1\n"
	if !strings.Contains(DefaultFile, current) {
		t.Fatalf("DefaultFile does not carry %q", current)
	}
	return strings.Replace(DefaultFile, current, legacy, 1)
}

func TestRaiseFormat_FormatOneTemplate_BecomesCurrentTemplate(t *testing.T) {
	t.Parallel()

	raised, isChanged := RaiseFormat([]byte(formatOneTemplate(t)), 2)

	if !isChanged || string(raised) != DefaultFile {
		t.Errorf("RaiseFormat = %v,\n%s\nwant the current template", isChanged, raised)
	}
}

func TestRaiseFormat_RewritesOrInsertsOneFormatKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"no format key", "# mine\ntypes: [bug]\n", "format: 2\n# mine\ntypes: [bug]\n"},
		{"empty file", "", "format: 2\n"},
		{"commented key without the template comment", "# keep\n# format: 1\ntypes: [bug]\n",
			"# keep\nformat: 2\ntypes: [bug]\n"},
		{"format 1", "types: [bug]\nformat: 1 # old\nlock:\n  ttl: 1h\n", "types: [bug]\nformat: 2\nlock:\n  ttl: 1h\n"},
		{"real key wins over a commented one", "# format: 1\nformat: 1\n", "# format: 1\nformat: 2\n"},
		{"CRLF kept", "types: [bug]\r\nformat: 1\r\n", "types: [bug]\r\nformat: 2\r\n"},
		{"CRLF on insert", "types: [bug]\r\n", "format: 2\r\ntypes: [bug]\r\n"},
		{"no final newline", "types: [bug]\nformat: 1", "types: [bug]\nformat: 2"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			raised, isChanged := RaiseFormat([]byte(testCase.content), 2)

			if !isChanged || string(raised) != testCase.want {
				t.Fatalf("RaiseFormat(%q) = %q, %v; want %q, true", testCase.content, raised, isChanged, testCase.want)
			}
			config, err := Parse(testPath, raised)
			if err != nil || config.Format != 2 {
				t.Errorf("Parse(raised) = format %d, %v; want format 2", config.Format, err)
			}
		})
	}
}

func TestRaiseFormat_AtOrAboveVersion_IsUnchanged(t *testing.T) {
	t.Parallel()
	cases := []struct {
		content string
		version int
	}{
		{DefaultFile, 2},
		{"format: 3\n", 2},
		{"# format: 1\n", 1},
	}
	for _, testCase := range cases {
		raised, isChanged := RaiseFormat([]byte(testCase.content), testCase.version)

		if isChanged || string(raised) != testCase.content {
			t.Errorf("RaiseFormat(%q, %d) = %q, %v; want it unchanged",
				testCase.content, testCase.version, raised, isChanged)
		}
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
		{"format zero", "format: 0\n", 1},
		{"negative format", "types: [bug]\nformat: -1\n", 2},
		{"non-integer format", "format: x\n", 1},
		{"fractional format", "format: 1.5\n", 1},
		{"format not a scalar", "format:\n  - 1\n", 2},
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
