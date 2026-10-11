package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
)

// initData is the --json data of capcom init.
type initData struct {
	// Root is the absolute store directory.
	Root string `json:"root"`
	// Created lists the paths init created, relative to the working tree root; empty on a re-run.
	Created []string `json:"created"`
}

// newInitCommand creates the store in the current git working tree, or does nothing when it
// already exists.
func (a *app) newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the capcom store (.capcom/) in this git repository",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opened, created, err := store.Init(cwd)
			if err != nil {
				return storeFailure(err)
			}
			if created == nil {
				created = []string{}
			}
			return a.emit(cmd, Result{
				Kind: "init",
				Data: initData{Root: opened.Root, Created: created},
				Text: initText(opened.Root, created),
			})
		}),
	}
}

func initText(root string, created []string) string {
	if len(created) == 0 {
		return "capcom store already initialized at " + root + "\n"
	}
	var text strings.Builder
	text.WriteString("initialized capcom store at " + root + "\n")
	for _, path := range created {
		text.WriteString("created " + path + "\n")
	}
	return text.String()
}

// storeFailure types a failure to open or create the store: a bad config.yaml is invalid at its
// file and line, a missing store is not_initialized, and a directory outside git is invalid.
func storeFailure(err error) *Error {
	var configFailure *config.Error
	switch {
	case errors.As(err, &configFailure):
		return &Error{
			Code:    CodeInvalid,
			Message: configFailure.Message,
			Path:    configFailure.Path,
			Line:    configFailure.Line,
		}
	case errors.Is(err, store.ErrNotInitialized):
		return Errorf(CodeNotInitialized, "%s", err.Error())
	default:
		return Errorf(CodeInvalid, "%s", err.Error())
	}
}
