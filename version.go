// Package capcom is the module root. It only embeds release metadata that the binary reports;
// the program itself lives under cmd/ and internal/.
package capcom

import (
	_ "embed"
	"strings"
)

// versionFile holds the raw contents of the VERSION file at the module root. It is embedded
// here because //go:embed cannot reach a parent directory from internal/cli.
//
//go:embed VERSION
var versionFile string

// Version returns the release version recorded in the VERSION file, without surrounding
// whitespace.
func Version() string {
	return strings.TrimSpace(versionFile)
}
