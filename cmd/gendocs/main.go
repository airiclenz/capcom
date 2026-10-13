// Command gendocs writes capcom's Markdown CLI reference, one page per command, from the command
// tree the capcom binary runs. It is a development tool and is never shipped.
//
//	go run ./cmd/gendocs [-out docs/site/reference/cli]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/airiclenz/capcom/internal/cli"
	"github.com/airiclenz/capcom/internal/clidoc"
)

// defaultOutDir is where the committed CLI reference lives, relative to the repository root.
const defaultOutDir = "docs/site/reference/cli"

// outDirMode is the permission of the output directory when gendocs creates it.
const outDirMode = 0o755

func main() {
	out := flag.String("out", defaultOutDir, "directory the reference pages are written to")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

// run creates out when it is missing and writes the reference of the capcom command tree into it.
func run(out string) error {
	if err := os.MkdirAll(out, outDirMode); err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	return clidoc.WriteReference(cli.Command(), out)
}
