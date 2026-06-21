package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/llmwiki"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

// OpenKB source lifecycle commands (age-port-openkb-into-agentops-go-5qw.2):
// `ao wiki add`, `ao wiki remove`, `ao wiki recompile`. They operate on the
// active OpenKB workspace (or --path) resolved via wikiResolveWorkspace, and
// delegate to internal/wiki source.go (containment-guarded) + internal/llmwiki
// (the ingest/compile stage). `watch` is deferred (a separate slice).

var wikiAddCmd = &cobra.Command{
	Use:   "add <file|dir>...",
	Short: "Add source files into the active wiki workspace's raw/ + registry",
	Long: `Copy supported source files (.md/.txt — what the ingest processes) into the
active wiki workspace's raw/ directory and record registry entries atomically. A
directory is walked for supported files; unsupported types are reported and
skipped (PDF/URL conversion are adapter follow-ups). Re-adding a file updates its
entry.

The workspace is the one selected by ao wiki init/use, or --path.`,
	Args:         cobra.MinimumNArgs(1),
	RunE:         runWikiAdd,
	SilenceUsage: true,
}

var wikiRemoveCmd = &cobra.Command{
	Use:   "remove <doc>",
	Short: "Remove a source and its derived artifacts from the workspace",
	Long: `Remove a registered source (by id or raw filename) and its derived wiki
artifacts (sources/summaries/concepts/entities/explorations/reports) plus the
registry entry. Use --dry-run to REPORT every artifact that would be removed
without deleting anything; --keep-raw to leave the raw/ source in place. Every
delete is containment-checked — it can never escape the workspace.`,
	Args:         cobra.ExactArgs(1),
	RunE:         runWikiRemove,
	SilenceUsage: true,
}

var wikiRecompileCmd = &cobra.Command{
	Use:   "recompile",
	Short: "Recompile workspace sources (re-run the ingest/compile stage)",
	Long: `Re-run the ingest/compile stage over the active wiki workspace, distilling
raw/ sources into wiki/sources/. With --dry-run, list the raw sources the real
run would recompile (matches the actual ingest input). --refresh-schema also
rewrites the workspace config/schema.

Recompile is whole-workspace. A per-doc argument is rejected (the ingest is
currently whole-workspace; per-doc recompile is a follow-up under epic 5qw).`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runWikiRecompile,
	SilenceUsage: true,
}

func init() {
	wikiCmd.AddCommand(wikiAddCmd)
	wikiCmd.AddCommand(wikiRemoveCmd)
	wikiCmd.AddCommand(wikiRecompileCmd)

	wikiAddCmd.Flags().String("path", "", "Workspace path (default: the active workspace)")
	wikiRemoveCmd.Flags().String("path", "", "Workspace path (default: the active workspace)")
	wikiRemoveCmd.Flags().Bool("dry-run", false, "Report artifacts that would be removed without deleting")
	wikiRemoveCmd.Flags().Bool("keep-raw", false, "Leave the raw/ source file in place")
	wikiRecompileCmd.Flags().String("path", "", "Workspace path (default: the active workspace)")
	wikiRecompileCmd.Flags().Bool("dry-run", false, "List sources that would be recompiled without writing")
	wikiRecompileCmd.Flags().Bool("all", true, "Recompile the whole workspace")
	wikiRecompileCmd.Flags().Bool("refresh-schema", false, "Also rewrite the workspace config/schema")
}

// resolveWorkspaceFlag resolves the workspace for a source command: --path wins,
// else the active workspace recorded by init/use.
func resolveWorkspaceFlag(cmd *cobra.Command) (string, error) {
	pathFlag, _ := cmd.Flags().GetString("path")
	stateDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return wikiResolveWorkspace(stateDir, pathFlag)
}

func runWikiAdd(cmd *cobra.Command, args []string) error {
	ws, err := resolveWorkspaceFlag(cmd)
	if err != nil {
		return err
	}
	res, err := wiki.AddSources(ws, args, time.Now())
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Added %d source(s) to %s\n", len(res.Added), ws)
	for _, e := range res.Added {
		fmt.Fprintf(out, "  + %s (id=%s)\n", e.RawName, e.ID)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(out, "  skipped: %s\n", s)
	}
	return nil
}

func runWikiRemove(cmd *cobra.Command, args []string) error {
	ws, err := resolveWorkspaceFlag(cmd)
	if err != nil {
		return err
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	keepRaw, _ := cmd.Flags().GetBool("keep-raw")
	res, err := wiki.RemoveSource(ws, args[0], wiki.RemoveOptions{DryRun: dryRun, KeepRaw: keepRaw})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	verb := "Removed"
	if dryRun {
		verb = "Would remove"
	}
	fmt.Fprintf(out, "%s %d artifact(s) for %s:\n", verb, len(res.Artifacts), res.DocID)
	for _, a := range res.Artifacts {
		fmt.Fprintf(out, "  - %s\n", a)
	}
	if dryRun {
		fmt.Fprintln(out, "(dry-run: nothing was deleted)")
	}
	return nil
}

func runWikiRecompile(cmd *cobra.Command, args []string) error {
	ws, err := resolveWorkspaceFlag(cmd)
	if err != nil {
		return err
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	refreshSchema, _ := cmd.Flags().GetBool("refresh-schema")
	out := cmd.OutOrStdout()

	// Per-doc recompile is not yet supported: the underlying ingest is
	// whole-workspace (scans all of raw/), so honoring <doc> would either be a
	// no-op (silent ignore) or leak into compiling other uncompiled docs. Reject
	// the arg explicitly rather than ignore it (cross-family REFUTE); true per-doc
	// recompile needs a per-doc/registry-driven ingest — a sibling slice under 5qw.
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		return fmt.Errorf("per-doc recompile is not yet supported; run `ao wiki recompile` (or --all) to recompile the whole workspace (per-doc ingest is a follow-up under epic 5qw)")
	}

	if dryRun {
		// Report the ACTUAL ingest input (raw/ files), not the registry, so the
		// preview matches the real run — incl. an orphaned --keep-raw file
		// (cross-family REFUTE: dry-run/real divergence).
		candidates, err := wiki.RawCandidates(ws)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Would recompile %d raw source(s) in %s:\n", len(candidates), ws)
		for _, name := range candidates {
			fmt.Fprintf(out, "  - %s\n", name)
		}
		return nil
	}

	if refreshSchema {
		cfg, rerr := wiki.ReadScaffoldConfig(ws)
		if rerr != nil {
			cfg = wiki.DefaultScaffoldConfig("", "")
		}
		if _, werr := wiki.WriteScaffoldConfig(ws, cfg); werr != nil {
			return werr
		}
		fmt.Fprintln(out, "Refreshed workspace config/schema.")
	}

	stage := &llmwiki.IngestStage{}
	result, err := stage.Run(cmd.Context(), ws, 1)
	if err != nil {
		return fmt.Errorf("recompile (ingest): %w", err)
	}
	if result.Skipped {
		fmt.Fprintf(out, "Recompile skipped: %s\n", result.SkipReason)
		return nil
	}
	fmt.Fprintf(out, "Recompiled %d source artifact(s) in %s\n", len(result.ArtifactsPath), ws)
	return nil
}
