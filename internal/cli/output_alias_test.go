package cli

import (
	"io"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// The cli tests name the output contract under the spellings it had before it moved to package
// output; these aliases keep those tests unchanged. Only the names the tests use are declared.

// Error is output.Error under its old spelling.
type Error = output.Error

// Result is output.Result under its old spelling.
type Result = output.Result

// SchemaVersion is output.SchemaVersion under its old spelling.
const SchemaVersion = output.SchemaVersion

// The output error codes under their old spellings.
const (
	CodeNotFound       = output.CodeNotFound
	CodeExists         = output.CodeExists
	CodeInvalid        = output.CodeInvalid
	CodeConflict       = output.CodeConflict
	CodeNotInitialized = output.CodeNotInitialized
)

// The output exit codes under their old spellings.
const (
	ExitOK       = output.ExitOK
	ExitError    = output.ExitError
	ExitUsage    = output.ExitUsage
	ExitNotFound = output.ExitNotFound
)

// The output kind, and --kv format, under their old spellings.
const (
	kindError = output.KindError
	formatKV  = output.FormatKV
)

// newRenderer is output.NewRenderer under its old spelling.
func newRenderer(format output.OutputFormat, stdout, stderr io.Writer) output.Renderer {
	return output.NewRenderer(format, stdout, stderr)
}

// storeFailure is output.StoreFailure under its old spelling.
func storeFailure(err error) *Error {
	return output.StoreFailure(err)
}
