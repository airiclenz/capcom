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

func TestFormatVersion_IsTheNewestNamedFormat(t *testing.T) {
	t.Parallel()
	named := []int{MissingFormat, FormatRelated, FormatSupersession, FormatAgents, FormatStanding}

	for i := 1; i < len(named); i++ {
		if named[i] != named[i-1]+1 {
			t.Errorf("named formats = %v, want each one past the one before", named)
			break
		}
	}
	if newest := named[len(named)-1]; FormatVersion != newest {
		t.Errorf("FormatVersion = %d, want the newest named format %d", FormatVersion, newest)
	}
}

func TestParse_DefaultFile_DeclaresCurrentFormat(t *testing.T) {
	t.Parallel()

	config, err := Parse(testPath, []byte(DefaultFile))

	formatLine := fmt.Sprintf("\nformat: %d\n", FormatVersion)
	if err != nil || config.Format != FormatVersion || !strings.Contains(DefaultFile, formatLine) {
		t.Errorf("Parse(DefaultFile) = format %d, %v; want a real format: %d line", config.Format, err, FormatVersion)
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
		{"format alone", "format: 6\n", 1},
		{"after other keys", "types: [bug]\n\nformat: 6\n", 3},
		{"unknown key above format", "colour: blue\nformat: 6\n", 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(testPath, []byte(testCase.content))

			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("err = %v, want *config.Error", err)
			}
			want := fmt.Sprintf("%s:%d: store format 6 is newer than this capcom reads (up to 5); upgrade capcom",
				testPath, testCase.wantLine)
			if err.Error() != want {
				t.Errorf("err = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestKeylessFile_Parse_DefaultsAtFormatOne(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(testPath, []byte(KeylessFile))

	want := Default()
	want.Format = MissingFormat
	if err != nil || !reflect.DeepEqual(parsed, want) {
		t.Errorf("Parse(KeylessFile) = %+v, %v; want %+v", parsed, err, want)
	}
}

func TestRaiseFormat_KeylessFile_BecomesDefaultFile(t *testing.T) {
	t.Parallel()

	raised, isChanged, err := RaiseFormat(testPath, []byte(KeylessFile), FormatVersion)

	if err != nil || !isChanged || string(raised) != DefaultFile {
		t.Errorf("RaiseFormat = %v, %v,\n%s\nwant DefaultFile", isChanged, err, raised)
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
		{"document start", "---\n" + reviewStatuses, "---\nformat: 2\n" + reviewStatuses},
		{"document start with a comment", "--- # mine\n" + reviewStatuses, "--- # mine\nformat: 2\n" + reviewStatuses},
		{"comment before the document start", "# c\n---\n" + reviewStatuses, "# c\n---\nformat: 2\n" + reviewStatuses},
		{"directive before the document start", "%YAML 1.1\n---\n" + reviewStatuses,
			"%YAML 1.1\n---\nformat: 2\n" + reviewStatuses},
		{"commented key before the document start", "# format: 1\n---\n" + reviewStatuses,
			"# format: 1\n---\nformat: 2\n" + reviewStatuses},
		{"document start at end of file", "# c\n---", "# c\n---\nformat: 2\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			raised, isChanged, err := RaiseFormat(testPath, []byte(testCase.content), 2)

			if err != nil || !isChanged || string(raised) != testCase.want {
				t.Fatalf("RaiseFormat(%q) = %q, %v, %v; want %q, true, nil",
					testCase.content, raised, isChanged, err, testCase.want)
			}
			config, err := Parse(testPath, raised)
			if err != nil || config.Format != 2 {
				t.Errorf("Parse(raised) = format %d, %v; want format 2", config.Format, err)
			}
			original, err := Parse(testPath, []byte(testCase.content))
			if err != nil || !reflect.DeepEqual(config.Statuses, original.Statuses) {
				t.Errorf("Parse(raised) statuses = %+v, want %+v (%v)", config.Statuses, original.Statuses, err)
			}
		})
	}
}

// reviewStatuses is a statuses key with a status the default configuration does not have.
const reviewStatuses = "statuses:\n  open: todo\n  review: doing\n  done: done\n"

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
		raised, isChanged, err := RaiseFormat(testPath, []byte(testCase.content), testCase.version)

		if err != nil || isChanged || string(raised) != testCase.content {
			t.Errorf("RaiseFormat(%q, %d) = %q, %v, %v; want it unchanged",
				testCase.content, testCase.version, raised, isChanged, err)
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

func TestCategory_IsClosed_OnlyDoneAndDropped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		category Category
		want     bool
	}{
		{CategoryTodo, false},
		{CategoryDoing, false},
		{CategoryWaiting, false},
		{CategoryDone, true},
		{CategoryDropped, true},
		{Category("reviewing"), false},
	}
	for _, tc := range cases {
		t.Run(string(tc.category), func(t *testing.T) {
			t.Parallel()

			got := tc.category.IsClosed()

			if got != tc.want {
				t.Errorf("Category(%q).IsClosed() = %v, want %v", tc.category, got, tc.want)
			}
		})
	}

	closed := ClosedCategories()

	if want := []Category{CategoryDone, CategoryDropped}; !reflect.DeepEqual(closed, want) {
		t.Errorf("ClosedCategories() = %v, want %v", closed, want)
	}
	closed[0] = CategoryTodo
	if again := ClosedCategories(); again[0] != CategoryDone {
		t.Errorf("ClosedCategories() after the caller changed its slice = %v, want a fresh slice", again)
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
		{"days just above the bound", "elsewhere:\n  days: 36501\n", 2},
		{"days far above the bound", "elsewhere:\n  days: 999999\n", 2},
		{"plans not a list", "\nplans: docs/plans\n", 2},
		{"not a mapping", "- open\n", 1},
		{"format zero", "format: 0\n", 1},
		{"negative format", "types: [bug]\nformat: -1\n", 2},
		{"non-integer format", "format: x\n", 1},
		{"fractional format", "format: 1.5\n", 1},
		{"format not a scalar", "format:\n  - 1\n", 2},
		{"second document", "format: 2\n---\nstatuses:\n  open: todo\n  done: done\n", 2},
		{"guides not a list", "format: 4\nagents:\n  guides: CLAUDE.md\n", 3},
		{"unknown agents key", "format: 4\nagents:\n  guides: []\n  hosts: [claude]\n", 4},
		{"guide is AGENTS.md", "format: 4\nagents:\n  guides: [CLAUDE.md, AGENTS.md]\n", 3},
		{"guide is AGENTS.md once cleaned", "format: 4\nagents:\n  guides:\n    - ./AGENTS.md\n", 4},
		{"guide outside the repository", "format: 4\nagents:\n  guides:\n    - ../x.md\n", 4},
		{"absolute guide", "format: 4\nagents:\n  guides:\n    - /etc/x.md\n", 4},
		{"guide with a backslash", "format: 4\nagents:\n  guides:\n    - 'docs\\AI.md'\n", 4},
		{"duplicate guide", "format: 4\nagents:\n  guides:\n    - CLAUDE.md\n    - CLAUDE.md\n", 5},
		{"duplicate guide once cleaned", "format: 4\nagents:\n  guides:\n    - docs/AI.md\n    - ./docs/AI.md\n", 5},
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

func TestParse_AgentsGuides_LoadsAtFormatFour(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"key absent", "format: 4\n", []string{"CLAUDE.md", "GEMINI.md"}},
		{"empty list", "format: 4\nagents:\n  guides: []\n", []string{}},
		{"guide in a subdirectory", "format: 4\nagents:\n  guides: [docs/AI.md]\n", []string{"docs/AI.md"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			config, err := Parse(testPath, []byte(testCase.content))

			if err != nil || !reflect.DeepEqual(config.Agents.Guides, testCase.want) {
				t.Errorf("Parse(%q) = guides %q, %v; want %q", testCase.content, config.Agents.Guides, err, testCase.want)
			}
		})
	}
}

func TestParse_AgentsBelowFormatFour_FailsAtTheKeyNamingFormatFour(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		content  string
		wantLine int
	}{
		{"format 3", "format: 3\ntypes: [bug]\nagents:\n  guides: [CLAUDE.md]\n", 3},
		{"no format key", "agents:\n  guides: []\n", 1},
		{"invalid section", "format: 2\nagents: none\n", 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(testPath, []byte(testCase.content))

			var failure *Error
			if !errors.As(err, &failure) || failure.Line != testCase.wantLine ||
				!strings.Contains(failure.Message, "set format: 4") {
				t.Errorf("err = %v, want a *config.Error at line %d naming format: 4", err, testCase.wantLine)
			}
		})
	}
}

func TestParse_ElsewhereDaysAtBound_Loads(t *testing.T) {
	t.Parallel()

	config, err := Parse(testPath, []byte("elsewhere:\n  days: 36500\n"))

	if err != nil || config.Elsewhere.Days != 36500 {
		t.Errorf("Parse = elsewhere.days %d, %v; want 36500, nil", config.Elsewhere.Days, err)
	}
}

func TestParse_ElsewhereDaysAboveBound_ErrorNamesBound(t *testing.T) {
	t.Parallel()

	_, err := Parse(testPath, []byte("elsewhere:\n  days: 36501\n"))

	if err == nil || !strings.Contains(err.Error(), "0 to 36500") {
		t.Errorf("err = %v, want an error naming the bound 0 to 36500", err)
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

func TestParse_ContentAfterDocumentEnd_FailsAsInvalidYAMLNamingItsLine(t *testing.T) {
	t.Parallel()

	_, err := Parse(testPath, []byte("types: [bug]\n...\ncolour: blue\n"))

	var failure *Error
	if !errors.As(err, &failure) || !strings.HasPrefix(failure.Message, "invalid YAML: yaml: line 2: ") {
		t.Errorf("err = %v, want a *config.Error with an invalid YAML message naming line 2", err)
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
