package output

import (
	"errors"
	"fmt"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
)

func TestError_Error_PrefixesKnownLocation(t *testing.T) {
	cases := []struct {
		name    string
		failure Error
		want    string
	}{
		{"no location", Error{Message: "boom"}, "boom"},
		{"path only", Error{Message: "boom", Path: "a.md"}, "a.md: boom"},
		{"path and line", Error{Message: "boom", Path: "a.md", Line: 7}, "a.md:7: boom"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := testCase.failure.Error(); got != testCase.want {
				t.Errorf("Error() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestError_ExitCode_MapsCodes(t *testing.T) {
	cases := []struct {
		failure *Error
		want    int
	}{
		{Errorf(CodeNotFound, "x"), ExitNotFound},
		{Errorf(CodeLocked, "x"), ExitNotFound},
		{Errorf(CodeExists, "x"), ExitError},
		{Errorf(CodeInvalid, "x"), ExitError},
		{Errorf(CodeConflict, "x"), ExitError},
		{Errorf(CodeNotInitialized, "x"), ExitError},
		{UsageErrorf("x"), ExitUsage},
	}
	for _, testCase := range cases {
		t.Run(string(testCase.failure.Code), func(t *testing.T) {
			t.Parallel()

			if got := testCase.failure.ExitCode(); got != testCase.want {
				t.Errorf("ExitCode() = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestStoreFailure_MapsInitAndOpenErrors(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode Code
		wantLine int
	}{
		{"not initialized", fmt.Errorf("/repo/.capcom: %w", store.ErrNotInitialized), CodeNotInitialized, 0},
		{"outside git", fmt.Errorf("/tmp: %w", store.ErrNotGitRepository), CodeInvalid, 0},
		{"bad config", &config.Error{Path: ".capcom/config.yaml", Line: 5, Message: "bad"}, CodeInvalid, 5},
		{"other", errors.New("disk full"), CodeInvalid, 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := StoreFailure(testCase.err)

			if failure.Code != testCase.wantCode || failure.Line != testCase.wantLine {
				t.Errorf("failure = %+v, want code %s line %d", failure, testCase.wantCode, testCase.wantLine)
			}
		})
	}
}
