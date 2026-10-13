package output

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

// failingWriter rejects every write, standing in for a closed output stream.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stream closed") }

func TestExit_MapsEachOutcome(t *testing.T) {
	notFound := Errorf(CodeNotFound, "issue x not found")
	cases := []struct {
		name       string
		err        error
		stderr     io.Writer
		wantCode   int
		wantStderr string
	}{
		{"nil", nil, nil, ExitOK, ""},
		{"rendered not_found", Rendered(notFound), nil, ExitNotFound, ""},
		{"untyped", errors.New(`unknown command "frob"`), nil, ExitUsage, "capcom: unknown command \"frob\"\n"},
		{"invalid", Errorf(CodeInvalid, "bad title"), nil, ExitError, "capcom: bad title\n"},
		{"locked", Errorf(CodeLocked, "tree held"), nil, ExitNotFound, "capcom: tree held\n"},
		{"usage", UsageErrorf("too many args"), nil, ExitUsage, "capcom: too many args\n"},
		{"failing renderer", Errorf(CodeNotFound, "gone"), failingWriter{}, ExitError, ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			errStream := testCase.stderr
			if errStream == nil {
				errStream = &stderr
			}

			got := Exit(NewRenderer(FormatText, &stdout, errStream), testCase.err)

			if got != testCase.wantCode {
				t.Errorf("Exit() = %d, want %d", got, testCase.wantCode)
			}
			if stderr.String() != testCase.wantStderr || stdout.Len() != 0 {
				t.Errorf("stdout %q, stderr %q, want stdout empty and stderr %q", stdout.String(), stderr.String(), testCase.wantStderr)
			}
		})
	}
}

func TestRendered_MarksACopy(t *testing.T) {
	failure := Errorf(CodeInvalid, "x")

	marked := Rendered(failure)

	if !marked.isRendered || failure.isRendered || marked == failure {
		t.Errorf("Rendered: marked %+v, original %+v, want only a distinct copy marked", marked, failure)
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
