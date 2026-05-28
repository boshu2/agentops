// practices: [ai-assisted-dev, wiki-knowledge-surface]
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var mindCmd = &cobra.Command{
	Use:   "mind",
	Short: "Knowledge graph operations",
	Long: `Scan, normalize, link, and index .agents/ markdown into an Obsidian knowledge graph.

Wraps the mind Python tool (python3 -m mind). Subcommands:
  scan       Show what needs normalization
  normalize  Add/fix YAML frontmatter
  link       Insert wikilinks between related artifacts
  index      Rebuild the graph index
  all        Run full pipeline (normalize → link → index)
  graph      Show graph statistics

By default, ao mind applies changes (--write). Use --dry-run to preview.`,
}

func init() {
	mindCmd.GroupID = "knowledge"
	rootCmd.AddCommand(mindCmd)

	// Register subcommands
	for _, sub := range []struct {
		use, short string
	}{
		{"scan", "Show what needs normalization"},
		{"normalize", "Add/fix YAML frontmatter on .agents/ markdown"},
		{"link", "Insert wikilinks between related artifacts"},
		{"index", "Rebuild the graph index"},
		{"all", "Run full pipeline (normalize → link → index)"},
		{"graph", "Show graph statistics"},
	} {
		subcmd := sub // capture loop variable
		mindCmd.AddCommand(&cobra.Command{
			Use:   subcmd.use,
			Short: subcmd.short,
			RunE:  mindRunFunc(subcmd.use),
		})
	}
}

// mindRunFunc returns a RunE handler that shells out to python3 -m mind <subcommand>.
func mindRunFunc(subcommand string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		pythonPath, err := exec.LookPath("python3")
		if err != nil {
			return fmt.Errorf("python3 not found: install Python 3 to use ao mind")
		}

		mindArgs := []string{"-m", "mind", subcommand}

		// ao defaults to execute; mind defaults to dry-run.
		// So: ao mind all → mind all --write
		//     ao mind all --dry-run → mind all (no --write)
		if !GetDryRun() {
			mindArgs = append(mindArgs, "--write")
		}

		// Detect vault path from working directory
		cwd, err := os.Getwd()
		if err == nil {
			mindArgs = append(mindArgs, "--vault", cwd)
		}

		if GetVerbose() {
			VerbosePrintf("Running: %s %v\n", pythonPath, mindArgs)
		}

		execCmd := exec.Command(pythonPath, mindArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin

		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("mind %s failed: %w", subcommand, err)
		}

		return nil
	}
}
