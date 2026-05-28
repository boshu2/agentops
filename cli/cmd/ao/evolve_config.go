package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/boshu2/agentops/cli/internal/evolve"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	evolveConfigShow bool
	evolveConfigJSON bool
)

var evolveConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show per-repo /evolve preferences",
	Long: `Display the resolved per-repo /evolve preferences.

Reads .agents/evolve/preferences.yaml (gitignored per-repo) on top of the
built-in defaults. A missing file is not an error — defaults are shown. A
malformed file exits 1 with file:line:column context for operator triage.

Resolution order (caller applies step 3):
  1. defaults (built-in Go constants)
  2. .agents/evolve/preferences.yaml
  3. CLI flag overrides

Examples:
  ao evolve config --show          # YAML output (default when --show is set)
  ao evolve config --show --json   # JSON output`,
	RunE: runEvolveConfig,
}

func init() {
	evolveConfigCmd.Flags().BoolVar(&evolveConfigShow, "show", false, "Print the resolved preferences (defaults + preferences.yaml)")
	evolveConfigCmd.Flags().BoolVar(&evolveConfigJSON, "json", false, "Emit JSON instead of YAML")
	evolveCmd.AddCommand(evolveConfigCmd)
}

// runEvolveConfig loads preferences and prints them in YAML or JSON.
func runEvolveConfig(cmd *cobra.Command, _ []string) error {
	if !evolveConfigShow {
		return fmt.Errorf("ao evolve config: pass --show to print preferences")
	}
	prefs, err := evolve.Load(cmd.Context())
	if err != nil {
		return err
	}
	return writeEvolvePrefs(cmd.OutOrStdout(), prefs, evolveConfigJSON)
}

// writeEvolvePrefs serializes prefs to w as YAML or JSON depending on asJSON.
func writeEvolvePrefs(w io.Writer, prefs *evolve.Prefs, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(prefs); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		return nil
	}
	data, err := yaml.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write yaml: %w", err)
	}
	return nil
}
