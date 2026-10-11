package issue_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/issue"
)

// testPath is the file name the tests hand to Parse, so errors can be matched exactly.
const testPath = ".capcom/issues/login-timeout-race.md"

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return data
}

func mustParse(t *testing.T, data []byte) issue.Issue {
	t.Helper()
	parsed, err := issue.Parse(testPath, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return parsed
}

func TestParseSerialize_CanonicalGoldens_AreIdentity(t *testing.T) {
	goldens := []string{
		"minimal.md",
		"full.md",
		"priority-zero.md",
		"flow-labels.md",
		"fence-in-code-block.md",
		"utf8.md",
	}
	for _, name := range goldens {
		t.Run(name, func(t *testing.T) {
			data := readGolden(t, name)

			serialized := mustParse(t, data).Serialize()

			if !bytes.Equal(serialized, data) {
				t.Errorf("round trip changed the file:\n--- got ---\n%s\n--- want ---\n%s", serialized, data)
			}
		})
	}
}

func TestParse_Full_ReadsEveryField(t *testing.T) {
	data := readGolden(t, "full.md")

	parsed := mustParse(t, data)

	want := issue.Issue{
		Title:    "Session refresh races the login timeout",
		Status:   "done",
		Type:     "feature",
		Priority: 1,
		Labels:   []string{"owner-run", "invariant"},
		Deps:     []string{"token-store-rework", "clock-skew-guard"},
		Plan:     "2026-10-11 - 00 - capcom-v1-issues-plan",
		Aliases:  []string{"apogee-kk0.4", "old-session-race"},
		Created:  time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC),
		Closed:   time.Date(2026, 10, 12, 17, 5, 42, 0, time.UTC),
		Reason:   "Fixed in 3f2a1c9: refresh now waits on the timeout lock.",
	}
	got := parsed
	got.Body = ""
	if got.Title != want.Title || got.Status != want.Status || got.Type != want.Type ||
		got.Priority != want.Priority || got.Plan != want.Plan || got.Reason != want.Reason ||
		!slices.Equal(got.Labels, want.Labels) || !slices.Equal(got.Deps, want.Deps) ||
		!slices.Equal(got.Aliases, want.Aliases) || !got.Created.Equal(want.Created) ||
		!got.Closed.Equal(want.Closed) {
		t.Errorf("Parse = %+v, want %+v", got, want)
	}
	if !strings.HasPrefix(parsed.Body, "\nThe refresh path") || !strings.HasSuffix(parsed.Body, "two tabs.\n") {
		t.Errorf("Body = %q, want the text after the closing fence verbatim", parsed.Body)
	}
}

func TestParse_PriorityZero_IsKeptNotDefaulted(t *testing.T) {
	data := readGolden(t, "priority-zero.md")

	parsed := mustParse(t, data)

	if parsed.Priority != 0 {
		t.Errorf("Priority = %d, want 0", parsed.Priority)
	}
}

func TestParse_MissingPriority_IsDefault(t *testing.T) {
	data := []byte("---\ntitle: No priority\n---\n")

	parsed := mustParse(t, data)

	if parsed.Priority != issue.DefaultPriority {
		t.Errorf("Priority = %d, want %d", parsed.Priority, issue.DefaultPriority)
	}
}

func TestParse_UTF8_KeptVerbatim(t *testing.T) {
	data := readGolden(t, "utf8.md")

	parsed := mustParse(t, data)

	if parsed.Title != "Gerät meldet Überlauf — 日本語 ✓" {
		t.Errorf("Title = %q", parsed.Title)
	}
	if !slices.Equal(parsed.Labels, []string{"größe", "✓"}) {
		t.Errorf("Labels = %q", parsed.Labels)
	}
	if !strings.Contains(parsed.Body, "東京, emoji 🚀") {
		t.Errorf("Body = %q", parsed.Body)
	}
}

func TestParse_CRLF_SerializesAsLF(t *testing.T) {
	want := readGolden(t, "full.md")
	crlf := bytes.ReplaceAll(want, []byte("\n"), []byte("\r\n"))

	serialized := mustParse(t, crlf).Serialize()

	if !bytes.Equal(serialized, want) {
		t.Errorf("CRLF file serialised as:\n%q\nwant:\n%q", serialized, want)
	}
}

func TestParse_NonCanonical_SerializesCanonically(t *testing.T) {
	data := []byte(strings.Join([]string{
		"---",
		"# a comment",
		"reason: 'done: shipped'",
		"closed: 2026-10-12T19:05:42.75+02:00",
		"labels:",
		"  - b",
		"  - a",
		"deps: []",
		"related: []",
		"plan: ~",
		"title: \"Out of order\"",
		"status: done",
		"type: task",
		"created: 2026-10-11T09:30:00Z",
		"---",
		"body",
		"",
	}, "\n"))
	want := strings.Join([]string{
		"---",
		"title: Out of order",
		"status: done",
		"type: task",
		"priority: 2",
		"labels: [b, a]",
		"created: 2026-10-11T09:30:00Z",
		"closed: 2026-10-12T17:05:42Z",
		"reason: \"done: shipped\"",
		"---",
		"body",
		"",
	}, "\n")

	serialized := mustParse(t, data).Serialize()

	if string(serialized) != want {
		t.Errorf("Serialize =\n%s\nwant\n%s", serialized, want)
	}
}

func TestParse_Related_SerializesAfterDeps(t *testing.T) {
	data := []byte(strings.Join([]string{
		"---",
		"related: [clock-skew-guard, token-store-rework]",
		"plan: some-plan",
		"deps: [session-store]",
		"title: Linked",
		"---",
		"",
	}, "\n"))
	want := strings.Join([]string{
		"---",
		"title: Linked",
		"priority: 2",
		"deps: [session-store]",
		"related: [clock-skew-guard, token-store-rework]",
		"plan: some-plan",
		"---",
		"",
	}, "\n")

	parsed := mustParse(t, data)
	serialized := parsed.Serialize()

	if !slices.Equal(parsed.Related, []string{"clock-skew-guard", "token-store-rework"}) {
		t.Errorf("Related = %q, want both linked ids in file order", parsed.Related)
	}
	if string(serialized) != want {
		t.Errorf("Serialize =\n%s\nwant\n%s", serialized, want)
	}
	if again := mustParse(t, serialized).Serialize(); !bytes.Equal(again, serialized) {
		t.Errorf("canonical file changed on a second round trip:\n%s", again)
	}
}

func TestParse_Failures_CiteFileLine(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "unknown key",
			content: "---\ntitle: T\nstatus: open\nupdated: 2026-10-11T09:30:00Z\n---\n",
			want:    testPath + `:4: unknown key "updated"`,
		},
		{
			name:    "stored id",
			content: "---\nid: login-timeout-race\n---\n",
			want:    testPath + `:2: unknown key "id"`,
		},
		{
			name:    "duplicate key",
			content: "---\ntitle: T\ntitle: U\n---\n",
			want:    testPath + `:3: duplicate key "title"`,
		},
		{
			name:    "priority not a number",
			content: "---\ntitle: T\npriority: high\n---\n",
			want:    testPath + ":3: priority: expected a whole number from 0 to 4",
		},
		{
			name:    "bad timestamp",
			content: "---\ncreated: yesterday\n---\n",
			want:    testPath + ":2: created: expected an RFC3339 timestamp such as 2026-10-11T09:30:00Z",
		},
		{
			name:    "labels not a list",
			content: "---\ntitle: T\nlabels: ui\n---\n",
			want:    testPath + ":3: labels: expected a list such as [a, b]",
		},
		{
			name:    "empty list entry",
			content: "---\ndeps: [a, '']\n---\n",
			want:    testPath + ":2: deps: every entry must be non-empty text",
		},
		{
			name:    "empty related entry",
			content: "---\nrelated: ['']\n---\n",
			want:    testPath + ":2: related: every entry must be non-empty text",
		},
		{
			name:    "title not text",
			content: "---\ntitle: [a]\n---\n",
			want:    testPath + ":2: title: expected text",
		},
		{
			name:    "frontmatter not a mapping",
			content: "---\n- a\n---\n",
			want:    testPath + ":2: the frontmatter must be a mapping of keys",
		},
		{
			name:    "no opening fence",
			content: "title: T\n",
			want:    testPath + ":1: an issue file must start with a --- line",
		},
		{
			name:    "never closed",
			content: "---\ntitle: T\n",
			want:    testPath + ":1: the frontmatter opened here is never closed by a --- line",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := issue.Parse(testPath, []byte(tc.content))

			if err == nil || err.Error() != tc.want {
				t.Errorf("Parse error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestParse_YAMLSyntaxError_CitesFileLine(t *testing.T) {
	content := "---\ntitle: T\nstatus: [open\ntype: bug\n---\n"

	_, err := issue.Parse(testPath, []byte(content))

	// yaml.v3 counts parser errors (unlike scanner errors) from 0, so the cited line may sit one
	// above the fault; it must still fall inside the frontmatter.
	var parseError *issue.Error
	if !errors.As(err, &parseError) || parseError.Line < 2 || parseError.Line > 4 || parseError.Path != testPath {
		t.Errorf("Parse error = %v, want an *issue.Error citing a frontmatter line", err)
	}
}

func TestParse_EmptyFrontmatterAndBody(t *testing.T) {
	cases := []string{"---\n---\n", "---\n---", "---\n# only a comment\n---\n"}
	for _, content := range cases {
		t.Run(content, func(t *testing.T) {
			parsed, err := issue.Parse(testPath, []byte(content))

			if err != nil || parsed.Title != "" || parsed.Body != "" {
				t.Errorf("Parse(%q) = %+v, %v; want an empty issue", content, parsed, err)
			}
		})
	}
}

func TestSerialize_TrickyText_RoundTrips(t *testing.T) {
	values := []string{
		"123", "1.5", "true", "yes", "null", "~", "2026-10-11",
		"a: b", "a:b", "a #b", "#x", "- x", "-x", "? x", "&anchor", "*alias", "!tag", "%x", "@x",
		"`x`", "'quoted'", `"quoted"`, `back\slash`, " leading", "trailing ", "multi\nline",
		"tab\there", "bell\a", "line\u2028separator", "[x]", "{x}", "x, y", "x]", "ünïcode ✓",
		"plain words", "non\uFFFEcharacter", "non\uFFFFcharacter", "\uFFFE", "\uFFFF",
	}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			original := issue.Issue{Title: value, Reason: value, Labels: []string{value, "z"}}

			serialized := original.Serialize()
			parsed := mustParse(t, serialized)

			if parsed.Title != value || parsed.Reason != value || !slices.Equal(parsed.Labels, original.Labels) {
				t.Errorf("round trip of %q gave title %q, reason %q, labels %q from:\n%s",
					value, parsed.Title, parsed.Reason, parsed.Labels, serialized)
			}
			if again := parsed.Serialize(); !bytes.Equal(again, serialized) {
				t.Errorf("second serialisation differs:\n%s\nvs\n%s", again, serialized)
			}
		})
	}
}

func TestSerialize_PlainText_IsUnquoted(t *testing.T) {
	original := issue.Issue{Title: "Session refresh races the login timeout", Labels: []string{"apogee-kk0.4"}}

	serialized := string(original.Serialize())

	if !strings.Contains(serialized, "\ntitle: Session refresh races the login timeout\n") ||
		!strings.Contains(serialized, "\nlabels: [apogee-kk0.4]\n") {
		t.Errorf("Serialize quoted plain text:\n%s", serialized)
	}
}

func TestSerialize_LongTitle_StaysOnOneLine(t *testing.T) {
	title := strings.Repeat("a long title word ", 12) + "end"
	original := issue.Issue{Title: title}

	serialized := string(original.Serialize())

	if !strings.Contains(serialized, "\ntitle: "+title+"\n") {
		t.Errorf("long title was not written on one line:\n%s", serialized)
	}
}

func TestSerialize_EmptyIssue_WritesPriorityOnly(t *testing.T) {
	serialized := string(issue.Issue{}.Serialize())

	if serialized != "---\npriority: 0\n---\n" {
		t.Errorf("Serialize = %q, want only the always-written priority", serialized)
	}
}
