package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const defaultStartupPayloadTarget = 6

var contextPacketStatusFlags struct {
	task  string
	phase string
	limit int
}

type packetStatusResult struct {
	Query    string                      `json:"query"`
	Phase    string                      `json:"phase"`
	Payload  contextExplainPayloadHealth `json:"payload"`
	Rollout  packetRolloutStatus         `json:"rollout"`
	Families []packetFamilyStatus        `json:"families"`
	Metrics  packetRolloutMetrics        `json:"metrics"`
}

type packetRolloutStatus struct {
	Stage    string `json:"stage"`
	Guidance string `json:"guidance"`
}

type packetFamilyStatus struct {
	Family          string `json:"family"`
	Count           int    `json:"count"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	ReusedArtifacts int    `json:"reused_artifacts"`
	ReuseSessions   int    `json:"reuse_sessions"`
	ReuseWorkspaces int    `json:"reuse_workspaces"`
}

type packetRolloutMetrics struct {
	StartupFillRate       float64 `json:"startup_fill_rate"`
	ThinFamilies          int     `json:"thin_families"`
	TotalFamilies         int     `json:"total_families"`
	ThinRatio             float64 `json:"thin_ratio"`
	PacketReuseArtifacts  int     `json:"packet_reuse_artifacts"`
	PacketReuseSessions   int     `json:"packet_reuse_sessions"`
	PacketReuseWorkspaces int     `json:"packet_reuse_workspaces"`
}

func init() {
	packetStatusCmd := &cobra.Command{
		Use:   "packet-status",
		Short: "Show packet rollout health, reuse, and startup fill metrics",
		Long: `Inspect experimental packet families and the rollout gates that keep
packetization out of default startup injection until health improves.`,
		RunE: runContextPacketStatus,
	}

	packetStatusCmd.Flags().StringVar(&contextPacketStatusFlags.task, "task", "", "task or query to evaluate")
	packetStatusCmd.Flags().StringVar(&contextPacketStatusFlags.phase, "phase", "startup", "Context phase to evaluate")
	packetStatusCmd.Flags().IntVar(&contextPacketStatusFlags.limit, "limit", defaultStigmergicPacketLimit, "max items per class")
	contextCmd.AddCommand(packetStatusCmd)
}

func runContextPacketStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	query := strings.TrimSpace(contextPacketStatusFlags.task)
	phase := normalizeAssemblePhase(contextPacketStatusFlags.phase)
	bundle := collectRankedContextBundle(cwd, query, contextPacketStatusFlags.limit)
	result := buildPacketStatusResult(cwd, query, phase, bundle)

	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printPacketStatusHuman(cmd, result)
	return nil
}

func buildPacketStatusResult(cwd, query, phase string, bundle rankedContextBundle) packetStatusResult {
	explain := buildContextExplainResult(cwd, detectRepoName(cwd), query, phase, bundle)
	families := collectPacketFamilyStatuses(cwd)
	metrics := summarizePacketRolloutMetrics(explain.Payload, families)
	return packetStatusResult{
		Query:    query,
		Phase:    phase,
		Payload:  explain.Payload,
		Rollout:  packetRolloutStage(metrics),
		Families: families,
		Metrics:  metrics,
	}
}

func collectPacketFamilyStatuses(cwd string) []packetFamilyStatus {
	aggregate := loadCitationAggregate(cwd)
	packetDirs := []struct {
		family string
		dir    string
	}{
		{family: "topic-packets", dir: filepath.Join(cwd, ".agents", "topics")},
		{family: "source-manifests", dir: filepath.Join(cwd, ".agents", "packets", "source-manifests")},
		{family: "promoted-packets", dir: filepath.Join(cwd, ".agents", "packets", "promoted")},
	}

	statuses := make([]packetFamilyStatus, 0, len(packetDirs))
	for _, item := range packetDirs {
		count := countKnowledgeArtifacts(item.dir)
		health := describeContextFamily(item.family, count, true)
		stats := packetReuseStatsForDir(item.dir, aggregate)
		statuses = append(statuses, packetFamilyStatus{
			Family:          item.family,
			Count:           count,
			Status:          health.Status,
			Reason:          health.Reason,
			ReusedArtifacts: stats.UniqueArtifacts,
			ReuseSessions:   stats.UniqueSessions,
			ReuseWorkspaces: stats.UniqueWorkspaces,
		})
	}
	return statuses
}

type packetReuseStats struct {
	UniqueArtifacts  int
	UniqueSessions   int
	UniqueWorkspaces int
}

func packetReuseStatsForDir(dir string, aggregate citationAggregate) packetReuseStats {
	stats := packetReuseStats{}
	sessionSet := make(map[string]bool)
	workspaceSet := make(map[string]bool)
	root := filepath.ToSlash(filepath.Clean(dir))
	for path, signal := range aggregate.ByArtifact {
		if !strings.HasPrefix(path, root+"/") {
			continue
		}
		stats.UniqueArtifacts++
		for _, sessionID := range signal.sessionKeys {
			sessionSet[sessionID] = true
		}
		for _, workspacePath := range signal.workspaceKeys {
			workspaceSet[workspacePath] = true
		}
	}
	stats.UniqueSessions = len(sessionSet)
	stats.UniqueWorkspaces = len(workspaceSet)
	return stats
}

func summarizePacketRolloutMetrics(payload contextExplainPayloadHealth, families []packetFamilyStatus) packetRolloutMetrics {
	metrics := packetRolloutMetrics{
		StartupFillRate: float64(payload.SelectedCount) / float64(defaultStartupPayloadTarget),
		TotalFamilies:   len(families),
	}
	if metrics.StartupFillRate > 1 {
		metrics.StartupFillRate = 1
	}
	for _, family := range families {
		if family.Status == "missing" || family.Status == "thin" || family.Status == "manual_review" {
			metrics.ThinFamilies++
		}
		metrics.PacketReuseArtifacts += family.ReusedArtifacts
		metrics.PacketReuseSessions += family.ReuseSessions
		metrics.PacketReuseWorkspaces += family.ReuseWorkspaces
	}
	if metrics.TotalFamilies > 0 {
		metrics.ThinRatio = float64(metrics.ThinFamilies) / float64(metrics.TotalFamilies)
	}
	return metrics
}

func packetRolloutStage(metrics packetRolloutMetrics) packetRolloutStatus {
	switch {
	case metrics.ThinRatio == 0 && metrics.PacketReuseArtifacts >= 5 && metrics.StartupFillRate >= 0.75:
		return packetRolloutStatus{
			Stage:    "recommended",
			Guidance: "Packet families are healthy enough for opt-in enablement, but trust policy should still gate default startup injection until operators explicitly enable it.",
		}
	case metrics.ThinRatio <= 0.34 && metrics.PacketReuseArtifacts >= 3 && metrics.StartupFillRate >= 0.5:
		return packetRolloutStatus{
			Stage:    "opt_in",
			Guidance: "Packet families have enough health for advanced users to opt in, but they should remain suppressed from default startup payloads.",
		}
	default:
		return packetRolloutStatus{
			Stage:    "experimental",
			Guidance: "Keep packet families behind health gates. Default startup context should continue to prefer canonical findings, rules, risks, and ranked next work.",
		}
	}
}

func printPacketStatusHuman(cmd *cobra.Command, result packetStatusResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "## Packet Status")
	fmt.Fprintf(w, "- Query: %s\n", firstNonEmpty(result.Query, "(none)"))
	fmt.Fprintf(w, "- Phase: %s\n", result.Phase)
	fmt.Fprintf(w, "- Rollout: %s\n", strings.ToUpper(result.Rollout.Stage))
	fmt.Fprintf(w, "  %s\n\n", result.Rollout.Guidance)

	fmt.Fprintln(w, "## Startup Fill")
	fmt.Fprintf(w, "- Payload: %s (%d selected)\n", strings.ToUpper(result.Payload.Status), result.Payload.SelectedCount)
	fmt.Fprintf(w, "- Fill rate: %.0f%%\n", result.Metrics.StartupFillRate*100)
	fmt.Fprintf(w, "  %s\n\n", result.Payload.Reason)

	fmt.Fprintln(w, "## Families")
	for _, family := range result.Families {
		fmt.Fprintf(w, "- %s: %s (%d) reuse=%d artifacts/%d sessions/%d workspaces\n",
			family.Family,
			strings.ToUpper(family.Status),
			family.Count,
			family.ReusedArtifacts,
			family.ReuseSessions,
			family.ReuseWorkspaces,
		)
		fmt.Fprintf(w, "  %s\n", family.Reason)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Metrics")
	fmt.Fprintf(w, "- Thin families: %d/%d (%.0f%%)\n", result.Metrics.ThinFamilies, result.Metrics.TotalFamilies, result.Metrics.ThinRatio*100)
	fmt.Fprintf(w, "- Packet reuse: %d artifacts, %d sessions, %d workspaces\n",
		result.Metrics.PacketReuseArtifacts,
		result.Metrics.PacketReuseSessions,
		result.Metrics.PacketReuseWorkspaces,
	)
}
