package main

import (
	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/spf13/cobra"
)

var orchestrateToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Probe the orchestration tool matrix",
	Long:  "Probe binaries declared in docs/contracts/orchestration-tools.yaml and emit a verdict.",
	RunE:  runOrchestrateTools,
}

func runOrchestrateTools(cmd *cobra.Command, _ []string) error {
	root, err := orchestrateRepoRoot()
	if err != nil {
		return err
	}
	contract, err := orchestration.LoadToolsContract(root)
	if err != nil {
		return err
	}
	runID := orchestration.NewRunID()
	reports, err := orchestration.ProbeTools(cmd.Context(), contract, orchestrateRunner())
	if err != nil {
		return err
	}
	result := orchestration.BuildToolsResult(reports, runID)
	return finishInstrumentCommand(cmd, result)
}

func initOrchestrateTools() {
	orchestrateToolsCmd.Flags().BoolVar(&orchestrateJSON, "json", false, "Emit JSON instrument result")
	orchestrateCmd.AddCommand(orchestrateToolsCmd)
}
