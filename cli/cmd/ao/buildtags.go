package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// archiveBuildTags lists the ADR-0012 archive build tags this binary was
// compiled with. It is EMPTY in the default build (the spine). Tag-gated files
// append to it from their init():
//
//	cmd/ao/buildtags_flywheel.go  (//go:build flywheel) -> "flywheel"
//	cmd/ao/buildtags_legacy.go    (//go:build legacy)   -> "legacy"
//
// The mechanism: a command archived behind one of these tags lives in a file
// carrying the matching `//go:build <tag>` constraint, so its init() (which
// registers the cobra command) is only compiled when that tag is passed. The
// default `go build ./...` omits the file entirely — the command is neither
// compiled nor registered. `make build-flywheel` (tags: flywheel legacy) or
// `AGENTOPS_LEGACY=1 make build` (tag: legacy) restore them. ADR-0012 requires
// archive-not-delete, so the code stays buildable for the revival conditions.
var archiveBuildTags []string

// buildtagsCmd is a hidden introspection surface: it reports which archive build
// tags the running binary was compiled with, so an operator (or a test) can tell
// a spine build from a restored-satellite build. Hidden so the default command
// surface is unchanged.
var buildtagsCmd = &cobra.Command{
	Use:    "buildtags",
	Short:  "Report which ADR-0012 archive build tags this binary was compiled with",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if len(archiveBuildTags) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "spine (no archive build tags; corpus/flywheel + RPI/factory omitted)")
			return nil
		}
		for _, t := range archiveBuildTags {
			fmt.Fprintln(cmd.OutOrStdout(), t)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildtagsCmd)
}
