package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

// wiki init / wiki use (age-port-openkb-into-agentops-go-5qw.1): the OpenKB-style
// scaffold + workspace-selection surface, ported to Go so a usable wiki stands
// up without OpenKB Python. Accretive — the existing wiki subcommands are
// unchanged.

var wikiInitCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Scaffold an OpenKB-style wiki workspace (layout + config + schema)",
	Long: `Create an OpenKB-style wiki workspace at [path] (default: current directory).

Lays down the workspace directory structure (raw/, wiki/{sources,summaries,
concepts,entities,explorations,reports}, output/{skills,decks}), seeds
wiki/index.md, wiki/log.md, and a wiki/AGENTS.md schema, and writes the
config/state file (wiki/config.yaml) with the model, language, entity types, and
thresholds. Idempotent: existing dirs/files are preserved; the config is
(re)written to reflect --model/--language.

The workspace is self-contained and does NOT write into the private .agents/
corpus or the gold .ao/wiki view — the raw/private and gold/public boundaries
are preserved.`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runWikiInit,
	SilenceUsage: true,
}

var wikiUseCmd = &cobra.Command{
	Use:   "use [path]",
	Short: "Select (or show) the active wiki workspace for subsequent commands",
	Long: `With <path>: record it as the active wiki workspace. With no argument:
print the currently active workspace.

The selection is stored repo-locally (.ao/wiki/active-workspace) so subsequent
ao wiki commands resolve the workspace without re-specifying it (see
wikiResolveWorkspace). ao wiki init selects the workspace it creates. The path
must be an existing directory (typically one created by ao wiki init).`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runWikiUse,
	SilenceUsage: true,
}

func init() {
	wikiCmd.AddCommand(wikiInitCmd)
	wikiCmd.AddCommand(wikiUseCmd)

	wikiInitCmd.Flags().String("model", "", "Model the wiki is initialized for (recorded in config)")
	wikiInitCmd.Flags().String("language", "en", "Primary language for the wiki (recorded in config)")
}

func runWikiInit(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		root = args[0]
	}
	model, _ := cmd.Flags().GetString("model")
	language, _ := cmd.Flags().GetString("language")

	cfg := wiki.DefaultScaffoldConfig(model, language)
	cfgPath, err := wiki.Scaffold(root, cfg)
	if err != nil {
		return err
	}
	// Select the freshly-initialized workspace so init -> use round-trips and
	// subsequent commands resolve it (the bead's `use` scenario). Best-effort:
	// a state-write failure must not fail the scaffold itself.
	active := root
	if abs, serr := wiki.SetActiveWorkspace(".", root); serr == nil {
		active = abs
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Initialized OpenKB-style wiki workspace at %s\n", root)
	fmt.Fprintf(out, "  model:    %s\n", orNone(cfg.Model))
	fmt.Fprintf(out, "  language: %s\n", cfg.Language)
	fmt.Fprintf(out, "  config:   %s\n", cfgPath)
	fmt.Fprintf(out, "  active:   %s\n", active)
	return nil
}

func runWikiUse(cmd *cobra.Command, args []string) error {
	stateDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	out := cmd.OutOrStdout()
	// No argument: READ and report the active workspace (proves resolution; the
	// same path wikiResolveWorkspace returns to workspace-operating commands).
	if len(args) == 0 {
		active, err := wiki.ActiveWorkspace(stateDir)
		if err != nil {
			return err
		}
		if active == "" {
			fmt.Fprintln(out, "No active wiki workspace (run `ao wiki init` or `ao wiki use <path>`).")
			return nil
		}
		fmt.Fprintf(out, "Active wiki workspace: %s\n", active)
		return nil
	}
	abs, err := wiki.SetActiveWorkspace(stateDir, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Active wiki workspace: %s\n", abs)
	return nil
}

// wikiResolveWorkspace resolves the OpenKB workspace a command should operate on:
// an explicit path argument wins; otherwise the active workspace recorded by
// `ao wiki init`/`ao wiki use` (read from stateDir). Returns an error when no
// workspace is selected. This is the resolver workspace-operating subcommands
// (source ingestion, compilation — sibling beads under the epic) consume so the
// `use` selection is honored without re-specifying the path.
func wikiResolveWorkspace(stateDir, pathArg string) (string, error) {
	if strings.TrimSpace(pathArg) != "" {
		abs, err := filepath.Abs(pathArg)
		if err != nil {
			return "", fmt.Errorf("resolve workspace path: %w", err)
		}
		return abs, nil
	}
	active, err := wiki.ActiveWorkspace(stateDir)
	if err != nil {
		return "", err
	}
	if active == "" {
		return "", fmt.Errorf("no wiki workspace selected — run `ao wiki init` or `ao wiki use <path>`, or pass a path")
	}
	return active, nil
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(unset)"
	}
	return s
}
