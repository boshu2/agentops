//go:build legacy

package main

import (
	"strings"

	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/spf13/cobra"
)

var (
	orchestrateRouteWriters    int
	orchestrateRouteUnattended bool
	orchestrateRouteModels     string
)

var orchestrateRouteCmd = &cobra.Command{
	Use:   "route",
	Short: "Recommend orchestration profile for out-of-session work",
	Long:  "Map posture inputs to a profile and next preflight command.",
	RunE:  runOrchestrateRoute,
}

func runOrchestrateRoute(cmd *cobra.Command, _ []string) error {
	var models []string
	if orchestrateRouteModels != "" {
		for _, m := range strings.Split(orchestrateRouteModels, ",") {
			models = append(models, strings.TrimSpace(m))
		}
	}
	result := orchestration.RunRoute(orchestration.RouteOptions{
		Writers:    orchestrateRouteWriters,
		Unattended: orchestrateRouteUnattended,
		Models:     models,
	})
	return finishInstrumentCommand(cmd, result)
}

func initOrchestrateRoute() {
	orchestrateRouteCmd.Flags().BoolVar(&orchestrateJSON, "json", false, "Emit JSON instrument result")
	orchestrateRouteCmd.Flags().IntVar(&orchestrateRouteWriters, "writers", 0, "Parallel writer count")
	orchestrateRouteCmd.Flags().BoolVar(&orchestrateRouteUnattended, "unattended", false, "Durability axis: unattended out-of-session")
	orchestrateRouteCmd.Flags().StringVar(&orchestrateRouteModels, "models", "", "Comma-separated models (opus,codex,agy)")
	orchestrateCmd.AddCommand(orchestrateRouteCmd)
}
