// Command capcom is a git-native issue tracker and knowledge store for coding agents.
package main

import (
	"os"

	"github.com/airiclenz/capcom/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
