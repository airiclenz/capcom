package hooks_test

import (
	"errors"
	"testing"

	"github.com/airiclenz/capcom/internal/hooks"
)

// codexSessionStartGroup and codexPreCompactGroup are the groups Merge adds to the Codex events, as
// they are laid out inside an event's array.
const (
	codexSessionStartGroup = `      {
        "matcher": "startup|resume|clear",
        "hooks": [
          {
            "type": "command",
            "command": "capcom prime"
          }
        ]
      }`
	codexPreCompactGroup = `      {
        "matcher": "manual|auto",
        "hooks": [
          {
            "type": "command",
            "command": "capcom prime"
          }
        ]
      }`
)

// codexMerged is the hooks member Merge writes into Codex content without hooks.
const codexMerged = `  "hooks": {
    "SessionStart": [
` + codexSessionStartGroup + `
    ],
    "PreCompact": [
` + codexPreCompactGroup + `
    ]
  }`

func TestMerge_UsableContent_AddsTheMissingGroups(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantMerged string
	}{
		{"empty content", "", "{\n" + codexMerged + "\n}\n"},
		{"null", " null\n", "{\n" + codexMerged + "\n}\n"},
		{
			"member order and value bytes kept",
			`{"z": 1.50, "hooks": {"Stop": [{"x":1}]}, "a": "<&>"}`,
			`{
  "z": 1.50,
  "hooks": {
    "Stop": [
      {
        "x": 1
      }
    ],
    "SessionStart": [
` + codexSessionStartGroup + `
    ],
    "PreCompact": [
` + codexPreCompactGroup + `
    ]
  },
  "a": "<&>"
}
`,
		},
		{
			"foreign group shape kept",
			`{"hooks": {"SessionStart": ["not a group", {"hooks": "x"}]}}`,
			`{
  "hooks": {
    "SessionStart": [
      "not a group",
      {
        "hooks": "x"
      },
` + codexSessionStartGroup + `
    ],
    "PreCompact": [
` + codexPreCompactGroup + `
    ]
  }
}
`,
		},
		{
			"event already running capcom prime",
			`{"hooks": {"PreCompact": [{"hooks": [{"type": "command", "command": "capcom prime"}]}]}}`,
			`{
  "hooks": {
    "PreCompact": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "capcom prime"
          }
        ]
      }
    ],
    "SessionStart": [
` + codexSessionStartGroup + `
    ]
  }
}
`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged, isAdded, err := hooks.Merge([]byte(testCase.content), hooks.Codex)

			if err != nil || !isAdded || string(merged) != testCase.wantMerged {
				t.Errorf("Merge = %v, %v, merged:\n%s\nwant true, nil, merged:\n%s", isAdded, err, merged, testCase.wantMerged)
			}
		})
	}
}

func TestMerge_UnusableContent_FailsWithPathAndLine(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    hooks.Error
	}{
		{
			"syntax error",
			"{\n  \"a\": 1,\n  oops\n}\n",
			hooks.Error{Path: ".codex/hooks.json", Line: 3, Message: "invalid settings: invalid character 'o' looking for beginning of object key string"},
		},
		{
			"not an object",
			"[1, 2]\n",
			hooks.Error{Path: ".codex/hooks.json", Message: "invalid settings: must be a JSON object"},
		},
		{
			"trailing data",
			"{} {}\n",
			hooks.Error{Path: ".codex/hooks.json", Message: "invalid settings: unexpected data after the JSON object"},
		},
		{
			"hooks not an object",
			`{"hooks": [1]}`,
			hooks.Error{Path: ".codex/hooks.json", Message: "hooks must be a JSON object"},
		},
		{
			"event not an array",
			`{"hooks": {"PreCompact": {}}}`,
			hooks.Error{Path: ".codex/hooks.json", Message: "hooks.PreCompact must be an array of hook groups"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged, _, err := hooks.Merge([]byte(testCase.content), hooks.Codex)

			var failure *hooks.Error
			if !errors.As(err, &failure) || *failure != testCase.want || merged != nil {
				t.Errorf("Merge = %q, %v; want nil, %+v", merged, err, testCase.want)
			}
		})
	}
}

func TestMerge_SecondMerge_ChangesNothing(t *testing.T) {
	for _, target := range []hooks.Target{hooks.Claude, hooks.Codex} {
		t.Run(target.Path, func(t *testing.T) {
			first, _, err := hooks.Merge([]byte(`{"model": "x"}`), target)
			if err != nil {
				t.Fatalf("first Merge: %v", err)
			}

			second, isAdded, err := hooks.Merge(first, target)

			if err != nil || isAdded || string(second) != string(first) {
				t.Errorf("second Merge = %v, %v, merged:\n%s\nwant false, nil and the first result", isAdded, err, second)
			}
		})
	}
}

func TestSnippet_Claude_IsTheMergeOfEmptySettings(t *testing.T) {
	merged, _, err := hooks.Merge(nil, hooks.Claude)

	if snippet := hooks.Snippet(hooks.Claude); err != nil || string(snippet) != string(merged) {
		t.Errorf("Snippet = %s, want the merge of empty settings (err %v):\n%s", snippet, err, merged)
	}
}

func TestError_Error_NamesPathAndLine(t *testing.T) {
	cases := []struct {
		failure hooks.Error
		want    string
	}{
		{hooks.Error{Path: ".claude/settings.json", Line: 3, Message: "bad"}, ".claude/settings.json:3: bad"},
		{hooks.Error{Path: ".claude/settings.json", Message: "bad"}, ".claude/settings.json: bad"},
	}
	for _, testCase := range cases {
		if got := testCase.failure.Error(); got != testCase.want {
			t.Errorf("Error() = %q, want %q", got, testCase.want)
		}
	}
}
