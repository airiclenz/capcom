package output

import "testing"

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
