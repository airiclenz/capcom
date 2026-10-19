package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

const (
	flagJSON = "json"
	flagKV   = "kv"
	// argsTerminator ends flag parsing; nothing after it is a flag.
	argsTerminator = "--"
)

// Help text of capcom itself and of capcom version.
const (
	rootLong = "capcom is a git-native issue tracker and knowledge store for coding agents. Issues and " +
		"Knowledge Notes are markdown files under .capcom/, versioned with the code; a disposable " +
		"SQLite index lives under the git directory and is never committed. Every command prints plain " +
		"text, or with --json a versioned JSON envelope. Run capcom <command> --help for a command's " +
		"flags and examples."
	rootExample = "  capcom init                  # create .capcom/ in this git repository\n" +
		"  capcom prime                 # the session-start briefing\n" +
		"  capcom ready                 # the Issues ready to work on, most urgent first\n" +
		"  capcom know tree --depth 2   # the top two levels of the Knowledge tree\n" +
		"  capcom ready --json          # the same list as a JSON envelope"
	versionLong = "Print the version of this capcom binary as capcom <version>. --kv prints it as a " +
		"VERSION line followed by STATUS, and --json inside a versioned JSON envelope."
	versionExample = "  capcom version          # capcom <version>\n" +
		"  capcom version --kv     # VERSION=<version>, then STATUS=ok\n" +
		"  capcom version --json   # {version} in a JSON envelope"
)

// annotationIndex is the cobra annotation a command sets to have the store, and with it the index,
// ready before its body runs: indexRefresh brings the index up to date, indexRebuild recreates it,
// and storeOnly opens the store without the index. Commands without it never open the store or
// the index here; prime goes without it on purpose and opens both itself, so that their failures
// become warnings instead of failing the session it briefs. init and check open the store
// themselves too: init creates it, and check reports a config failure as one of its problems.
const (
	annotationIndex = "capcom-index"
	indexRefresh    = "refresh"
	indexRebuild    = "rebuild"
	storeOnly       = "store"
)

// app carries one invocation's output streams and the renderer its output flags select. Run builds a
// fresh app per call so invocations share no state.
type app struct {
	stdout   io.Writer
	stderr   io.Writer
	isJSON   bool
	isKV     bool
	renderer output.Renderer
	// store is set for a command annotated with annotationIndex, and issueIndex for one that
	// asks for the index; Run closes the index. indexReport is what opening the index did.
	// indexWarnings are the index warnings that go with the result: the report's own, or the
	// renamed copies a move puts in their place.
	store         *store.Store
	issueIndex    *index.Index
	indexReport   index.Report
	indexWarnings []diagnose.Finding
	// writeWarnings are the warnings of the writes the command made, such as the store format an
	// edit raised (app.written); emit renders them after the index warnings and before the result's.
	writeWarnings []string
}

// Run executes capcom with args (the program name excluded) against the given streams and
// returns the process exit code output.Exit decides: 0 ok, 1 error, 2 usage, 3 not_found or
// locked. Results go to stdout; in text mode a failure goes to stderr, while --json and --kv
// report it on stdout.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	application := &app{stdout: stdout, stderr: stderr}
	root := application.newRootCommand()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if closeErr := application.closeIndex(); closeErr != nil && err == nil {
		err = output.AsError(closeErr)
	}
	renderer := application.renderer
	if renderer == nil {
		// Parsing failed before the output flags were applied, so read them from args.
		renderer = output.NewRenderer(scanFormat(args), stdout, stderr)
	}
	return output.Exit(renderer, err)
}

// Command returns a fresh capcom command tree whose output is discarded, for tools that read the
// tree's commands, flags and help text without running them, such as the CLI reference generator.
func Command() *cobra.Command {
	return (&app{stdout: io.Discard, stderr: io.Discard}).newRootCommand()
}

// newRootCommand builds the command tree for one invocation.
func (a *app) newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:                "capcom",
		Short:              "A git-native issue tracker and knowledge store for coding agents",
		Long:               rootLong,
		Example:            rootExample,
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		PersistentPreRunE:  a.prepare,
	}
	root.PersistentFlags().BoolVar(&a.isJSON, flagJSON, false, "print a versioned JSON envelope")
	root.AddCommand(a.newAgentsCommand())
	root.AddCommand(a.newCheckCommand())
	root.AddCommand(a.newCloseCommand())
	root.AddCommand(a.newCreateCommand())
	root.AddCommand(a.newDepCommand())
	root.AddCommand(a.newFmtCommand())
	root.AddCommand(a.newInitCommand())
	root.AddCommand(a.newKnowCommand())
	root.AddCommand(a.newListCommand())
	root.AddCommand(a.newLockCommand())
	root.AddCommand(a.newLogCommand())
	root.AddCommand(a.newMergeDriverCommand())
	root.AddCommand(newMvCommand(a, issueVerbs))
	root.AddCommand(a.newPrimeCommand())
	root.AddCommand(a.newReadyCommand())
	root.AddCommand(a.newReindexCommand())
	root.AddCommand(a.newRelatedCommand())
	root.AddCommand(a.newReopenCommand())
	root.AddCommand(newRmCommand(a, issueVerbs))
	root.AddCommand(a.newSearchCommand())
	root.AddCommand(a.newShowCommand())
	root.AddCommand(a.newUICommand())
	root.AddCommand(a.newUpdateCommand())
	root.AddCommand(a.newVersionCommand())
	guardSingleValueFlags(root)
	return root
}

// prepare runs before every command body: it selects the renderer, refuses a single-value flag
// given more than once, then readies the index for a command that asks for it.
func (a *app) prepare(cmd *cobra.Command, args []string) error {
	if err := a.selectRenderer(cmd, args); err != nil {
		return err
	}
	if err := refuseRepeatedFlags(cmd.Flags()); err != nil {
		return err
	}
	return a.prepareIndex(cmd)
}

// prepareIndex opens the store and its index, refreshed or rebuilt as the command's
// annotationIndex asks, once per command; storeOnly opens the store alone. A storeOnly command
// checks its required flags itself, ahead of cobra's own check (which runs only after this
// hook), so a missing required flag is still reported ahead of a store that does not open, as it
// was when the command's body opened the store.
func (a *app) prepareIndex(cmd *cobra.Command) error {
	mode := cmd.Annotations[annotationIndex]
	if mode == "" {
		return nil
	}
	if mode == storeOnly {
		if err := cmd.ValidateRequiredFlags(); err != nil {
			return err
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return output.AsError(err)
	}
	opened, err := store.Open(cwd)
	if err != nil {
		return output.StoreFailure(err)
	}
	if mode == storeOnly {
		a.store = opened
		return nil
	}
	indexMode := index.ModeRefresh
	if mode == indexRebuild {
		indexMode = index.ModeRebuild
	}
	issueIndex, err := index.Open(opened, indexMode)
	if err != nil {
		return output.AsError(err)
	}
	a.store = opened
	a.issueIndex = issueIndex
	a.indexReport = issueIndex.Report()
	a.indexWarnings = a.indexReport.Warnings
	return nil
}

// closeIndex releases the index a command opened, if any.
func (a *app) closeIndex() error {
	if a.issueIndex == nil {
		return nil
	}
	return a.issueIndex.Close()
}

// withKV registers --kv on cmd and returns it. Only the commands whose results carry KV records
// call it, so --kv shows in the help of exactly the commands that honour it, and any other command
// rejects it as an unknown flag.
func (a *app) withKV(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolVar(&a.isKV, flagKV, false, "print KEY=VALUE lines ending in STATUS=")
	return cmd
}

// selectRenderer applies the output flags once cobra has parsed them.
func (a *app) selectRenderer(_ *cobra.Command, _ []string) error {
	if a.isJSON && a.isKV {
		return output.UsageErrorf("--json and --kv cannot be combined")
	}
	format := output.FormatText
	switch {
	case a.isJSON:
		format = output.FormatJSON
	case a.isKV:
		format = output.FormatKV
	}
	a.renderer = output.NewRenderer(format, a.stdout, a.stderr)
	return nil
}

// runE adapts a command body to cobra: it types every failure the body returns, so Run can tell
// command failures from cobra's own usage errors.
func runE(body func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := body(cmd, args); err != nil {
			return output.AsError(err)
		}
		return nil
	}
}

// emit renders a successful result in the selected format, the index's warnings ahead of the
// command's own. A command offers --kv only through withKV; a result without KV records under
// --kv is therefore a command's own defect, still rejected as a usage error, not output. A result
// carrying a Failure is rendered with it, and emit then returns that failure, marked rendered, so
// the command still exits with its code.
func (a *app) emit(cmd *cobra.Command, result output.Result) error {
	if a.isKV && result.KV == nil {
		return output.UsageErrorf("%s does not support --kv", cmd.CommandPath())
	}
	warnings := make([]string, 0, len(a.indexWarnings)+len(a.writeWarnings)+len(result.Warnings))
	for _, warning := range a.indexWarnings {
		warnings = append(warnings, warning.String())
	}
	warnings = append(warnings, a.writeWarnings...)
	result.Warnings = append(warnings, result.Warnings...)
	if err := a.renderer.Render(result); err != nil {
		return err
	}
	if result.Failure == nil {
		return nil
	}
	return output.Rendered(result.Failure)
}

// newVersionCommand reports the embedded release version.
func (a *app) newVersionCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:     "version",
		Short:   "Print the capcom version",
		Long:    versionLong,
		Example: versionExample,
		Args:    cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			version := capcom.Version()
			return a.emit(cmd, output.Result{
				Kind: "version",
				Data: map[string]string{"version": version},
				Text: "capcom " + version + "\n",
				KV:   []output.KVRecord{{Pairs: []output.KVPair{{Key: "VERSION", Value: version}}}},
			})
		}),
	})
}

// scanFormat reads the output flags straight from args, for failures cobra reports before it
// applies them, such as --kv on a command that does not offer it. --json wins when both appear,
// so a JSON caller still gets an envelope.
func scanFormat(args []string) output.OutputFormat {
	format := output.FormatText
	for _, arg := range args {
		switch arg {
		case argsTerminator:
			return format
		case "--" + flagJSON, "--" + flagJSON + "=true":
			return output.FormatJSON
		case "--" + flagKV, "--" + flagKV + "=true":
			format = output.FormatKV
		}
	}
	return format
}
