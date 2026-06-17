package main

import (
	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/spf13/cobra"
)

var (
	orchestratePreflightProfile string
)

var orchestratePreflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Pre-run admission gate for an orchestration profile",
	Long: `Check tools, atm version floor, Agent Mail health, and session collisions
before spawning an out-of-session ATM session. Appends orchestration.preflight.v1
to the provenance ledger when possible.`,
	RunE: runOrchestratePreflight,
}

func runOrchestratePreflight(cmd *cobra.Command, _ []string) error {
	root, err := orchestrateRepoRoot()
	if err != nil {
		return err
	}
	if orchestratePreflightProfile == "" {
		return cmd.Help()
	}
	runID := orchestration.NewRunID()
	result, err := orchestration.RunPreflight(cmd.Context(), orchestration.PreflightOptions{
		RepoRoot: root,
		Profile:  orchestratePreflightProfile,
		RunID:    runID,
		Runner:   orchestrateRunner(),
	})
	if err != nil {
		return err
	}
	key := orchestration.IdempotencyKey(orchestration.InstrumentCommandPreflight, result.Profile, "", runID)
	orchestration.WriteInstrumentLedger(orchestration.NewLedgerWriter(root), orchestration.LedgerEventPreflight, key, &result)
	return finishInstrumentCommand(cmd, result)
}

func initOrchestratePreflight() {
	orchestratePreflightCmd.Flags().BoolVar(&orchestrateJSON, "json", false, "Emit JSON instrument result")
	orchestratePreflightCmd.Flags().StringVar(&orchestratePreflightProfile, "profile", "", "Profile id (dual-pane, tri-vendor)")
	_ = orchestratePreflightCmd.MarkFlagRequired("profile")
	orchestrateCmd.AddCommand(orchestratePreflightCmd)
}
