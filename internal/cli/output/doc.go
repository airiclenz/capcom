// Package output holds capcom's command-line error contract: the typed Error, its closed set of
// Codes and the process exit codes they map to. It never imports internal/cli, so every package
// of the command line can build and inspect a typed failure.
package output
