package nearest

import (
	"slices"
	"testing"
)

func TestEditDistance_CountsEditsIgnoringCase(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"status", "status", 0},
		{"STATUS", "status", 0},
		{"statsu", "status", 2},
		{"jsno", "json", 2},
		{"", "json", 4},
		{"tre", "tree", 1},
		{"zzzzqq", "json", 6},
	}
	for _, testCase := range cases {
		t.Run(testCase.left+"/"+testCase.right, func(t *testing.T) {
			got := EditDistance(testCase.left, testCase.right)

			if got != testCase.want {
				t.Errorf("EditDistance(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
			}
		})
	}
}

func TestClosestNames_SuggestsCloseNamesInRankOrder(t *testing.T) {
	names := []string{
		"adr-0004", "go-tests", "log", "loge", "login-flow", "login-timeout-race", "logon", "logs", "spec-0004", "standing-parents",
	}
	cases := []struct {
		name  string
		typed string
		want  []string
	}{
		{"one edit away", "standng-parents", []string{"standing-parents"}},
		{"distance before shared words", "login-flaw", []string{"login-flow", "login-timeout-race"}},
		{"a shared word only", "login-redirect", []string{"login-flow", "login-timeout-race"}},
		{"more shared words first", "login-timeout-retry", []string{"login-timeout-race", "login-flow"}},
		{"at most three, then by name", "logx", []string{"log", "loge", "logs"}},
		{"a word of fewer than three letters is not shared", "go-qqqqq", nil},
		{"digits alone are no shared word", "rfc-0004", nil},
		{"nothing close", "zzzzzz", nil},
		{"empty", "", nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Ranked(testCase.typed, names)

			if !slices.Equal(got, testCase.want) {
				t.Errorf("Ranked(%q) = %v, want %v", testCase.typed, got, testCase.want)
			}
		})
	}
}
