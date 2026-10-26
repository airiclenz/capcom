package output

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"strings"
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
		{NothingReadyErrorf("x"), ExitNothingReady},
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
		{"untyped", errors.New(`unknown command "frob"`), nil, ExitUsage, "capcom: unknown command \"frob\"\nhint: capcom --help\n"},
		{"invalid", Errorf(CodeInvalid, "bad title"), nil, ExitError, "capcom: bad title\nhint: capcom --help\n"},
		{"locked", Errorf(CodeLocked, "tree held"), nil, ExitNotFound, "capcom: tree held\nhint: capcom lock status\n"},
		{"usage", UsageErrorf("too many args"), nil, ExitUsage, "capcom: too many args\nhint: capcom --help\n"},
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

// declaredCodes returns the names and values of every constant of type Code this package declares,
// read from its source, so a Code added later is found without being listed here.
func declaredCodes(t *testing.T) map[string]Code {
	t.Helper()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	fileSet := token.NewFileSet()
	codes := map[string]Code{}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, source, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		for _, declaration := range file.Decls {
			collectCodeConstants(t, declaration, codes)
		}
	}
	return codes
}

// collectCodeConstants adds to codes each constant of type Code that declaration declares.
func collectCodeConstants(t *testing.T, declaration ast.Decl, codes map[string]Code) {
	t.Helper()
	general, isGeneral := declaration.(*ast.GenDecl)
	if !isGeneral || general.Tok != token.CONST {
		return
	}
	for _, spec := range general.Specs {
		value := spec.(*ast.ValueSpec)
		typeName, isNamed := value.Type.(*ast.Ident)
		if !isNamed || typeName.Name != "Code" {
			continue
		}
		for position, name := range value.Names {
			literal := value.Values[position].(*ast.BasicLit)
			codes[name.Name] = Code(literal.Value[1 : len(literal.Value)-1])
		}
	}
}

func TestDefaultHints_EveryCode_HasOne(t *testing.T) {
	codes := declaredCodes(t)

	if len(codes) == 0 {
		t.Fatal("found no Code constants")
	}
	for name, code := range codes {
		if defaultHints[code] == "" {
			t.Errorf("%s (%q) has no default hint", name, code)
		}
	}
}

func TestError_WithCommand_FillsCommandInHint(t *testing.T) {
	cases := []struct {
		name    string
		failure *Error
		path    string
		want    string
	}{
		{"default named", Errorf(CodeInvalid, "bad"), "capcom update", "capcom update --help"},
		{"default unnamed", Errorf(CodeInvalid, "bad"), "", "capcom --help"},
		{"override named", &Error{Code: CodeInvalid, Hint: "capcom <command> --json"}, "capcom know show", "capcom know show --json"},
		{"no placeholder", Errorf(CodeNotFound, "gone"), "capcom update", "capcom search <text>"},
		{"first name kept", Errorf(CodeInvalid, "bad").WithCommand("capcom dep add"), "capcom dep", "capcom dep add --help"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			named := testCase.failure.WithCommand(testCase.path)

			if got := named.hintText(); got != testCase.want {
				t.Errorf("hint = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestInCommand_TypesAndNamesEveryError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := InCommand(nil, "capcom show"); got != nil {
			t.Errorf("InCommand(nil) = %v, want nil", got)
		}
	})
	t.Run("untyped", func(t *testing.T) {
		got := InCommand(errors.New("accepts 1 arg(s)"), "capcom show").(*Error)

		if got.ExitCode() != ExitUsage || got.hintText() != "capcom show --help" {
			t.Errorf("InCommand(untyped) = exit %d hint %q, want usage exit and capcom show --help", got.ExitCode(), got.hintText())
		}
	})
	t.Run("typed", func(t *testing.T) {
		original := Errorf(CodeLocked, "tree held")

		got := InCommand(original, "capcom start").(*Error)

		if got == original || original.commandPath != "" || got.commandPath != "capcom start" {
			t.Errorf("InCommand(typed) = %+v from %+v, want a named copy", got, original)
		}
	})
}
