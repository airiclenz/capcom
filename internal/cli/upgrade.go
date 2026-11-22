package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
)

// kindUpgrade is the --json kind of capcom upgrade.
const kindUpgrade = "upgrade"

// flagFormat names the store format capcom upgrade raises the store to.
const flagFormat = "format"

// upgradeData is the --json data of capcom upgrade: the format the store declared before and after
// the run, whether the run changed config.yaml, and what each format it added brings, [] when it
// changed nothing.
type upgradeData struct {
	From    int               `json:"from"`
	To      int               `json:"to"`
	Changed bool              `json:"changed"`
	Adds    []upgradeAddition `json:"adds"`
}

// upgradeAddition is one format an upgrade added and what that format brings.
type upgradeAddition struct {
	Format int    `json:"format"`
	Adds   string `json:"adds"`
}

// Help text of capcom upgrade.
const (
	upgradeLong = "Raise the store format that .capcom/config.yaml declares to --format, by default the " +
		"newest this capcom reads, creating config.yaml when there is none. It lists what each added " +
		"format brings.\n\n" +
		"capcom releases that read only an older format can no longer read the store afterwards: raise it " +
		"once everyone sharing the repository runs a capcom that reads the new format.\n\n" +
		"A store already at that format or above is left as it is; capcom never lowers a store format."
	upgradeExample = "  capcom upgrade              # raise the store to the newest format this capcom reads\n" +
		"  capcom upgrade --format 5   # raise it to format 5 and no further\n" +
		"  capcom upgrade --json       # {from, to, changed, adds} in a JSON envelope"
)

// newUpgradeCommand raises the format config.yaml declares to --format, config.FormatVersion by
// default, through store.Store.RaiseFormat, which creates a missing config.yaml (ADR 0019). A
// --format outside 1 to config.FormatVersion is a usage error. A store already at that format or
// above is left untouched and the run exits 0. Only a run that changed config.yaml is noted as a
// write of no Issue for the Tree lock warning (app.noteWrite).
func (a *app) newUpgradeCommand() *cobra.Command {
	target := config.FormatVersion
	cmd := &cobra.Command{
		Use:         "upgrade [--format <n>]",
		Short:       "Raise the store format on request",
		Long:        upgradeLong,
		Example:     upgradeExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			if target < config.MissingFormat || target > config.FormatVersion {
				return output.UsageErrorf("--%s %d: must be from %d to %d",
					flagFormat, target, config.MissingFormat, config.FormatVersion)
			}
			from := a.store.Config.Format
			isChanged, err := a.store.RaiseFormat(target)
			if err != nil {
				return output.StoreFailure(err)
			}
			if isChanged {
				a.noteWrite()
			}
			return a.emit(cmd, upgradeResult(from, target, isChanged))
		}),
	}
	cmd.Flags().IntVar(&target, flagFormat, config.FormatVersion,
		"the store format to raise the store to")
	return cmd
}

// upgradeResult renders an upgrade of a store at format from towards format target: the formats
// it added, one line each, and the releases that can no longer read the store, or when it changed
// nothing, the format the store is already at.
func upgradeResult(from, target int, isChanged bool) output.Result {
	data := upgradeData{From: from, To: from, Changed: isChanged, Adds: []upgradeAddition{}}
	if !isChanged {
		text := fmt.Sprintf("store is already at format %d\n", from)
		return output.Result{Kind: kindUpgrade, Data: data, Text: text}
	}

	data.To = target
	var text strings.Builder
	fmt.Fprintf(&text, "store format %d → %d\n", from, target)
	for _, addition := range config.FormatAdditions(from, target) {
		data.Adds = append(data.Adds, upgradeAddition{Format: addition.Format, Adds: addition.Adds})
		fmt.Fprintf(&text, "  %d: %s\n", addition.Format, addition.Adds)
	}
	fmt.Fprintf(&text, "capcom releases that read at most format %d can no longer read this store\n", target-1)
	return output.Result{Kind: kindUpgrade, Data: data, Text: text.String()}
}
