//go:build legacy

package main

import (
	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/spf13/cobra"
)

var (
	orchestrateVerifyProfile string
	orchestrateVerifySession string
)

var orchestrateVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Post-spawn windshield for a live session",
	Long: `Verify pane map against an orchestration profile using atm activity, spawn
JSON, or tmux titles (weak tier). Appends orchestration.verify.v1 to the ledger.`,
	RunE: runOrchestrateVerify,
}

func runOrchestrateVerify(cmd *cobra.Command, _ []string) error {
	root, err := orchestrateRepoRoot()
	if err != nil {
		return err
	}
	if orchestrateVerifyProfile == "" || orchestrateVerifySession == "" {
		return cmd.Help()
	}
	runID := orchestration.NewRunID()
	result, err := orchestration.RunVerify(cmd.Context(), orchestration.VerifyOptions{
		RepoRoot: root,
		Profile:  orchestrateVerifyProfile,
		Session:  orchestrateVerifySession,
		RunID:    runID,
		Runner:   orchestrateRunner(),
	})
	if err != nil {
		return err
	}
	key := orchestration.IdempotencyKey(orchestration.InstrumentCommandVerify, result.Profile, result.Session, runID)
	orchestration.WriteInstrumentLedger(orchestration.NewLedgerWriter(root), orchestration.LedgerEventVerify, key, &result)
	return finishInstrumentCommand(cmd, result)
}

func initOrchestrateVerify() {
	orchestrateVerifyCmd.Flags().BoolVar(&orchestrateJSON, "json", false, "Emit JSON instrument result")
	orchestrateVerifyCmd.Flags().StringVar(&orchestrateVerifyProfile, "profile", "", "Profile id")
	orchestrateVerifyCmd.Flags().StringVar(&orchestrateVerifySession, "session", "", "ATM session name")
	_ = orchestrateVerifyCmd.MarkFlagRequired("profile")
	_ = orchestrateVerifyCmd.MarkFlagRequired("session")
	orchestrateCmd.AddCommand(orchestrateVerifyCmd)
}
