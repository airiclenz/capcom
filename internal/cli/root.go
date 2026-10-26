package cli

import (
	"io"
	"os"
	"strings"

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
		"SQLite index lives under the git directory and is never committed.\n\n" +
		"Every command except capcom ui prints plain text, or with --json a versioned JSON envelope.\n\n" +
		"Run capcom <command> --help for a command's flags and examples."
	rootExample = "  capcom init                  # create .capcom/ in this git repository\n" +
		"  capcom prime                 # the session-start briefing\n" +
		"  capcom ready                 # the Issues ready to work on, most urgent first\n" +
		"  capcom know tree --depth 2   # the top two levels of the Knowledge tree\n" +
		"  capcom ready --json          # the same list as a JSON envelope"
	versionLong = "Print the version of this capcom binary as capcom <version>.\n\n" +
		"--kv prints it as a VERSION line followed by STATUS, and --json inside a versioned JSON " +
		"envelope."
	versionExample = "  capcom version          # capcom <version>\n" +
		"  capcom version --kv     # VERSION=<version>, then STATUS=ok\n" +
		"  capcom version --json   # {version} in a JSON envelope"
)

// helpTemplate lays out a command's --help: its Short, then the usage section of usageTemplate,
// then its Long, so the examples come before the rules.
const helpTemplate = `{{with .Short}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}{{with .Long}}
{{. | trimTrailingWhitespaces}}
{{end}}`

// usageTemplate is cobra's default usage template with Examples moved below Available Commands, so
// a group lists its subcommands first. capcom declares no command groups or help topics, so their
// sections are left out.
const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

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
	// isCompleting reports whether the invocation is cobra's __complete or __completeNoDesc, the
	// only one that registers flag completions (app.completeFlag).
	isCompleting bool
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
// locked, 4 nothing Ready (capcom next). Results go to stdout; in text mode a failure goes to stderr, while --json and --kv
// report it on stdout. A failure names the command cobra ran, or failed to parse, as the one that
// failed, so its hint can name that command.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	application := &app{stdout: stdout, stderr: stderr, isCompleting: isCompletionRequest(args)}
	root := application.newRootCommand()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	executed, err := root.ExecuteC()
	if closeErr := application.closeIndex(); closeErr != nil && err == nil {
		err = output.AsError(closeErr)
	}
	if executed != nil {
		err = output.InCommand(err, executed.CommandPath())
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
		Use:               "capcom",
		Short:             "A git-native issue tracker and knowledge store for coding agents",
		Long:              rootLong,
		Example:           rootExample,
		SilenceErrors:     true,
		SilenceUsage:      true,
		PersistentPreRunE: a.prepare,
	}
	root.SetFlagErrorFunc(unknownCommandOnFlagError)
	root.SetHelpTemplate(helpTemplate)
	root.SetUsageTemplate(usageTemplate)
	root.PersistentFlags().BoolVar(&a.isJSON, flagJSON, false, "print a versioned JSON envelope")
	root.AddCommand(a.newAgentsCommand())
	root.AddCommand(a.newCheckCommand())
	root.AddCommand(a.newCloseCommand())
	root.AddCommand(a.newContextCommand())
	root.AddCommand(a.newCreateCommand())
	root.AddCommand(a.newDepCommand())
	root.AddCommand(a.newFinishCommand())
	root.AddCommand(a.newFmtCommand())
	root.AddCommand(a.newInitCommand())
	root.AddCommand(a.newKnowCommand())
	root.AddCommand(a.newListCommand())
	root.AddCommand(a.newLockCommand())
	root.AddCommand(a.newLogCommand())
	root.AddCommand(a.newMergeDriverCommand())
	root.AddCommand(newMvCommand(a, issueVerbs))
	root.AddCommand(a.newNextCommand())
	root.AddCommand(a.newPrimeCommand())
	root.AddCommand(a.newReadyCommand())
	root.AddCommand(a.newRecapCommand())
	root.AddCommand(a.newReindexCommand())
	root.AddCommand(a.newRelatedCommand())
	root.AddCommand(a.newReopenCommand())
	root.AddCommand(newRmCommand(a, issueVerbs))
	root.AddCommand(a.newSearchCommand())
	root.AddCommand(a.newShowCommand())
	root.AddCommand(a.newStartCommand())
	root.AddCommand(a.newUICommand())
	root.AddCommand(a.newUpdateCommand())
	root.AddCommand(a.newVersionCommand())
	guardSingleValueFlags(root)
	return withSubcommandDispatch(root)
}

// suggestionDistance is the largest edit distance at which a mistyped name still has the closest
// valid one suggested.
const suggestionDistance = 2

// withSubcommandDispatch gives group, a command with subcommands and no body of its own, the body
// every such group shares and returns it: called alone it prints its help, and a first argument
// that names none of its subcommands is refused as an unknown command, naming the closest one.
// Without it cobra prints the help and exits 0 for a mistyped subcommand.
func withSubcommandDispatch(group *cobra.Command) *cobra.Command {
	group.Args = cobra.ArbitraryArgs
	group.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return unknownCommandError(cmd, args[0])
	}
	return group
}

// unknownCommandError is the usage error for word given where a subcommand of group belongs. Its
// Suggestion is the command path below capcom of the closest subcommand cobra finds within
// suggestionDistance of word, or that word starts; it is empty when none is that close.
func unknownCommandError(group *cobra.Command, word string) *output.Error {
	failure := output.UsageErrorf("unknown command %q for %q", word, group.CommandPath())
	group.SuggestionsMinimumDistance = suggestionDistance
	suggestions := group.SuggestionsFor(word)
	if len(suggestions) == 0 {
		return failure
	}
	path := group.CommandPath() + " " + closestName(word, suggestions)
	failure.Suggestion = strings.TrimSpace(strings.TrimPrefix(path, group.Root().Name()))
	return failure
}

// closestName returns the candidate with the smallest case-insensitive edit distance to word,
// the earliest one on a tie. cobra returns its suggestions in command order, not by distance, so
// know tre would otherwise suggest know rm before know tree. candidates must not be empty.
func closestName(word string, candidates []string) string {
	closest := candidates[0]
	closestDistance := editDistance(word, closest)
	for _, candidate := range candidates[1:] {
		if distance := editDistance(word, candidate); distance < closestDistance {
			closest, closestDistance = candidate, distance
		}
	}
	return closest
}

// editDistance is the Levenshtein distance between left and right, ignoring case.
func editDistance(left, right string) int {
	leftRunes := []rune(strings.ToLower(left))
	rightRunes := []rune(strings.ToLower(right))
	previousRow := make([]int, len(rightRunes)+1)
	for column := range previousRow {
		previousRow[column] = column
	}
	for row, leftRune := range leftRunes {
		currentRow := make([]int, len(rightRunes)+1)
		currentRow[0] = row + 1
		for column, rightRune := range rightRunes {
			substitution := previousRow[column]
			if leftRune != rightRune {
				substitution++
			}
			currentRow[column+1] = min(substitution, previousRow[column+1]+1, currentRow[column]+1)
		}
		previousRow = currentRow
	}
	return previousRow[len(rightRunes)]
}

// unknownCommandOnFlagError is the flag error function of every command. A group parses its flags
// before its body reads the arguments, so in capcom updat x --status active the unknown --status
// would hide the mistyped command; when the group's first argument names none of its subcommands,
// that unknown command is the error. Otherwise an unknown long flag names the closest flag of cmd
// (unknownFlagError), and any other flag error is returned unchanged.
func unknownCommandOnFlagError(cmd *cobra.Command, err error) error {
	if !cmd.HasSubCommands() {
		return unknownFlagError(cmd, err)
	}
	args := cmd.Flags().Args()
	if len(args) == 0 || namesSubcommand(cmd, args[0]) {
		return unknownFlagError(cmd, err)
	}
	return unknownCommandError(cmd, args[0])
}

// namesSubcommand reports whether word is the name or an alias of one of group's subcommands.
func namesSubcommand(group *cobra.Command, word string) bool {
	for _, subcommand := range group.Commands() {
		if subcommand.Name() == word || subcommand.HasAlias(word) {
			return true
		}
	}
	return false
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
// carrying a Failure is rendered with it, named as cmd's failure, and emit then returns that
// failure, marked rendered, so the command still exits with its code.
func (a *app) emit(cmd *cobra.Command, result output.Result) error {
	if a.isKV && result.KV == nil {
		return output.UsageErrorf("%s does not support --kv", cmd.CommandPath())
	}
	if result.Failure != nil {
		result.Failure = result.Failure.WithCommand(cmd.CommandPath())
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
