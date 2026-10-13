package clidoc

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// markdownExtension is the extension of every reference page, matching the names cobra/doc gives
// the files it writes and the links between them.
const markdownExtension = ".md"

// Sidebar positions of the reference pages: the root command's page leads, every other page
// follows in the site's own order.
const (
	rootSidebarOrder    = 0
	commandSidebarOrder = 1
)

// WriteReference writes one Markdown page per available command of root, nested commands
// included, into dir, which must exist. Each page opens with YAML frontmatter holding its title,
// description and sidebar order, carries no generation date, and links its parent and children by
// bare file name. cobra's help and completion commands get no page.
func WriteReference(root *cobra.Command, dir string) error {
	pages := make(map[string]*cobra.Command)
	collectPages(root, pages)

	var prependErr error
	prepender := func(filename string) string {
		cmd, ok := pages[filepath.Base(filename)]
		if !ok {
			if prependErr == nil {
				prependErr = fmt.Errorf("%s: no command for this reference page", filename)
			}
			return ""
		}
		return frontmatter(cmd)
	}
	linkHandler := func(link string) string {
		return filepath.Base(link)
	}

	if err := doc.GenMarkdownTreeCustom(root, dir, prepender, linkHandler); err != nil {
		return fmt.Errorf("write CLI reference to %s: %w", dir, err)
	}
	return prependErr
}

// collectPages disables cobra's generation date on cmd and every available command below it, and
// records each under the file name cobra/doc writes its page to. It visits the same commands
// doc.GenMarkdownTreeCustom does, so every page it writes has an entry.
func collectPages(cmd *cobra.Command, pages map[string]*cobra.Command) {
	cmd.DisableAutoGenTag = true
	pages[pageName(cmd)] = cmd
	for _, child := range cmd.Commands() {
		if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		collectPages(child, pages)
	}
}

// pageName is the file name of cmd's reference page: its command path with spaces as
// underscores, as cobra/doc names it.
func pageName(cmd *cobra.Command) string {
	return strings.ReplaceAll(cmd.CommandPath(), " ", "_") + markdownExtension
}

// frontmatter is the YAML block that opens cmd's page. Every string value is double-quoted, since
// a Short such as "Print the session briefing: ..." is not valid YAML unquoted.
func frontmatter(cmd *cobra.Command) string {
	order := commandSidebarOrder
	if !cmd.HasParent() {
		order = rootSidebarOrder
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: " + strconv.Quote(cmd.CommandPath()) + "\n")
	b.WriteString("description: " + strconv.Quote(cmd.Short) + "\n")
	b.WriteString("sidebar:\n")
	b.WriteString("  order: " + strconv.Itoa(order) + "\n")
	b.WriteString("---\n\n")
	return b.String()
}
