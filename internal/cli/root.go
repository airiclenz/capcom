// Package cli implements capcom's command line: the cobra command tree, the output contract
// (plain text, the versioned --json envelope, --kv KEY=VALUE lines) and the mapping from typed
// errors to exit codes. Run is the single entry point, used by the binary and by tests alike.
package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

const (
	flagJSON = "json"
	flagKV   = "kv"
	// argsTerminator ends flag parsing; nothing after it is a flag.
	argsTerminator = "--"
)

// annotationIndex is the cobra annotation a command sets to have the index ready before its body
// runs: indexRefresh brings it up to date, indexRebuild recreates it. Commands without it never
// open the store or the index.
const (
	annotationIndex = "capcom-index"
	indexRefresh    = "refresh"
	indexRebuild    = "rebuild"
)

// app carries one invocation's output streams and the renderer its global flags select. Run builds a
// fresh app per call so invocations share no state.
type app struct {
	stdout   io.Writer
	stderr   io.Writer
	isJSON   bool
	isKV     bool
	renderer Renderer
	// store and issueIndex are set for a command annotated with annotationIndex; Run closes the
	// index. indexReport is what preparing the index did, and its warnings go with the result.
	store       *store.Store
	issueIndex  *index.Index
	indexReport index.Report
}

// Run executes capcom with args (the program name excluded) against the given streams and
// returns the process exit code: 0 ok, 1 error, 2 usage, 3 not_found or locked. Results go to
// stdout; in text mode a failure goes to stderr, while --json and --kv report it on stdout.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	application := &app{stdout: stdout, stderr: stderr}
	root := application.newRootCommand()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if closeErr := application.closeIndex(); closeErr != nil && err == nil {
		err = asError(closeErr)
	}
	if err == nil {
		return ExitOK
	}

	failure, isTyped := err.(*Error)
	if isTyped && failure.isRendered {
		return failure.ExitCode()
	}
	if !isTyped {
		// cobra rejected the invocation before any command ran (unknown command, bad flag,
		// wrong argument count); command failures always arrive typed via runE.
		failure = UsageErrorf("%s", err.Error())
	}
	renderer := application.renderer
	if renderer == nil {
		// Parsing failed before the global flags were applied, so read them from args.
		renderer = newRenderer(scanFormat(args), stdout, stderr)
	}
	if renderErr := renderer.RenderError(failure); renderErr != nil {
		return ExitError
	}
	return failure.ExitCode()
}

// newRootCommand builds the command tree for one invocation.
func (a *app) newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:                "capcom",
		Short:              "A git-native issue tracker and knowledge store for coding agents",
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		PersistentPreRunE:  a.prepare,
	}
	root.PersistentFlags().BoolVar(&a.isJSON, flagJSON, false, "print a versioned JSON envelope")
	root.PersistentFlags().BoolVar(&a.isKV, flagKV, false, "print KEY=VALUE lines ending in STATUS=")
	root.AddCommand(a.newCloseCommand())
	root.AddCommand(a.newCreateCommand())
	root.AddCommand(a.newDepCommand())
	root.AddCommand(a.newInitCommand())
	root.AddCommand(a.newListCommand())
	root.AddCommand(a.newLogCommand())
	root.AddCommand(a.newMvCommand())
	root.AddCommand(a.newReadyCommand())
	root.AddCommand(a.newReindexCommand())
	root.AddCommand(a.newReopenCommand())
	root.AddCommand(a.newSearchCommand())
	root.AddCommand(a.newShowCommand())
	root.AddCommand(a.newUpdateCommand())
	root.AddCommand(a.newVersionCommand())
	return root
}

// prepare runs before every command body: it selects the renderer, then readies the index for a
// command that asks for it.
func (a *app) prepare(cmd *cobra.Command, args []string) error {
	if err := a.selectRenderer(cmd, args); err != nil {
		return err
	}
	return a.prepareIndex(cmd)
}

// prepareIndex opens the store and its index and refreshes or rebuilds it, once per command, as
// the command's annotationIndex asks.
func (a *app) prepareIndex(cmd *cobra.Command) error {
	mode := cmd.Annotations[annotationIndex]
	if mode == "" {
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return asError(err)
	}
	opened, err := store.Open(cwd)
	if err != nil {
		return storeFailure(err)
	}
	issueIndex, err := index.Open(opened)
	if err != nil {
		return asError(err)
	}
	a.store = opened
	a.issueIndex = issueIndex

	prepareIndex := issueIndex.Refresh
	if mode == indexRebuild {
		prepareIndex = issueIndex.Rebuild
	}
	report, err := prepareIndex()
	if err != nil {
		return asError(err)
	}
	a.indexReport = report
	return nil
}

// closeIndex releases the index a command opened, if any.
func (a *app) closeIndex() error {
	if a.issueIndex == nil {
		return nil
	}
	return a.issueIndex.Close()
}

// selectRenderer applies the global output flags once cobra has parsed them.
func (a *app) selectRenderer(_ *cobra.Command, _ []string) error {
	if a.isJSON && a.isKV {
		return UsageErrorf("--json and --kv cannot be combined")
	}
	format := formatText
	switch {
	case a.isJSON:
		format = formatJSON
	case a.isKV:
		format = formatKV
	}
	a.renderer = newRenderer(format, a.stdout, a.stderr)
	return nil
}

// runE adapts a command body to cobra: it types every failure the body returns, so Run can tell
// command failures from cobra's own usage errors.
func runE(body func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := body(cmd, args); err != nil {
			return asError(err)
		}
		return nil
	}
}

// emit renders a successful result in the selected format, the index's warnings ahead of the
// command's own. A command that offers no --kv records rejects --kv as a usage error. A result
// carrying a Failure is rendered with it, and emit then returns that failure, marked rendered, so
// the command still exits with its code.
func (a *app) emit(cmd *cobra.Command, result Result) error {
	if a.isKV && result.KV == nil {
		return UsageErrorf("%s does not support --kv", cmd.CommandPath())
	}
	warnings := make([]string, 0, len(a.indexReport.Warnings)+len(result.Warnings))
	for _, warning := range a.indexReport.Warnings {
		warnings = append(warnings, warning.String())
	}
	result.Warnings = append(warnings, result.Warnings...)
	if err := a.renderer.Render(result); err != nil {
		return err
	}
	if result.Failure == nil {
		return nil
	}
	rendered := *result.Failure
	rendered.isRendered = true
	return &rendered
}

// newVersionCommand reports the embedded release version.
func (a *app) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the capcom version",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			version := capcom.Version()
			return a.emit(cmd, Result{
				Kind: "version",
				Data: map[string]string{"version": version},
				Text: "capcom " + version + "\n",
				KV:   []KVRecord{{Pairs: []KVPair{{Key: "VERSION", Value: version}}}},
			})
		}),
	}
}

// scanFormat reads the global output flags straight from args, for failures cobra reports before
// it applies them. --json wins when both appear, so a JSON caller still gets an envelope.
func scanFormat(args []string) outputFormat {
	format := formatText
	for _, arg := range args {
		switch arg {
		case argsTerminator:
			return format
		case "--" + flagJSON, "--" + flagJSON + "=true":
			return formatJSON
		case "--" + flagKV, "--" + flagKV + "=true":
			format = formatKV
		}
	}
	return format
}
