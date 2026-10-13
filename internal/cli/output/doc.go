// Package output holds capcom's command-line output contract: the Result a command hands over, the
// text, --json and --kv Renderers that write it, the typed Error with its closed set of Codes and
// the process exit codes they map to, and StoreFailure, which types a failure to open the store.
// It never imports internal/cli, so every package of the command line can build and inspect a
// typed failure.
package output
