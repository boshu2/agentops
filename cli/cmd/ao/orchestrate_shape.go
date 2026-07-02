//go:build legacy

package main

import (
	"github.com/spf13/cobra"
)

var (
	orchestrateShapePacket     string
	orchestrateShapeProject    string
	orchestrateShapeProposed   string
	orchestrateShapeUnattended bool
	orchestrateShapeNoAM       bool
)

var orchestrateShapeCmd = &cobra.Command{
	Use:   "shape",
	Short: "Stamp orchestration shape onto the execution packet",
	Long:  "Validate and stamp chosen_shape on .agents/rpi/execution-packet.json.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return executeStampShape(stampShapeOptions{
			PacketPath: orchestrateShapePacket,
			Project:    orchestrateShapeProject,
			Proposed:   orchestrateShapeProposed,
			Unattended: orchestrateShapeUnattended,
			NoAM:       orchestrateShapeNoAM,
			Out:        cmd.OutOrStdout(),
		})
	},
}

func initOrchestrateShape() {
	orchestrateShapeCmd.Flags().StringVar(&orchestrateShapePacket, "packet", "", "Execution packet path")
	orchestrateShapeCmd.Flags().StringVar(&orchestrateShapeProject, "project", "", "Agent Mail project key")
	orchestrateShapeCmd.Flags().StringVar(&orchestrateShapeProposed, "proposed", "", "Proposed shape")
	orchestrateShapeCmd.Flags().BoolVar(&orchestrateShapeUnattended, "unattended", false, "Durability axis")
	orchestrateShapeCmd.Flags().BoolVar(&orchestrateShapeNoAM, "no-am", false, "Skip Agent Mail gathering")
	orchestrateCmd.AddCommand(orchestrateShapeCmd)
}

func initOrchestrateInstrumentLane() {
	initOrchestrateTools()
	initOrchestratePreflight()
	initOrchestrateVerify()
	initOrchestrateRoute()
	initOrchestrateStatus()
	initOrchestrateShape()
}
