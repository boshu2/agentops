package overnight

import (
	"fmt"
	"strings"
	"time"
)

// LongHaulControllerOptions holds the opt-in controller knobs for Dream's
// post-short-path extension lane.
type LongHaulControllerOptions struct {
	Enabled bool
	Budget  time.Duration
}

// LongHaulSignals captures the short-path evidence the controller uses to decide
// whether more runtime is justified.
type LongHaulSignals struct {
	PacketCount             int
	QueueBackedWon          bool
	TopPacketConfidence     string
	RetrievalCoverage       float64
	RetrievalCoverageKnown  bool
	KnowledgeBriefAvailable bool
	GoalRequested           bool
	ProbesAvailable         int
}

// EvaluateLongHaulActivation decides whether the opt-in long-haul lane should
// run after the default short Dream path.
func EvaluateLongHaulActivation(opts LongHaulControllerOptions, signals LongHaulSignals) LongHaulSummary {
	summary := LongHaulSummary{
		Enabled: opts.Enabled,
		Active:  false,
	}
	if !opts.Enabled {
		return summary
	}
	if signals.ProbesAvailable <= 0 {
		summary.ExitReason = "no runtime-ready long-haul probes configured"
		return summary
	}
	switch {
	case signals.PacketCount == 0:
		summary.Active = true
		summary.TriggerReason = "no morning packet synthesized"
	case normalizeLongHaulConfidence(signals.TopPacketConfidence) != "high" && !signals.QueueBackedWon:
		summary.Active = true
		summary.TriggerReason = "no strong queue-backed packet won"
	case normalizeLongHaulConfidence(signals.TopPacketConfidence) != "high":
		summary.Active = true
		summary.TriggerReason = "top packet confidence below high"
	case signals.GoalRequested && !signals.KnowledgeBriefAvailable:
		summary.Active = true
		summary.TriggerReason = "knowledge brief unavailable"
	case signals.RetrievalCoverageKnown && signals.RetrievalCoverage < 0.50:
		summary.Active = true
		summary.TriggerReason = fmt.Sprintf("retrieval coverage below %.2f", 0.50)
	default:
		summary.ExitReason = "trigger threshold not met"
	}
	return summary
}

// EvaluateLongHaulExit returns the first stop reason that applies after a
// long-haul probe finishes.
func EvaluateLongHaulExit(startedAt, now time.Time, budget time.Duration, zeroDeltaProbeStreak int, probesRemaining int) string {
	switch {
	case budget > 0 && !startedAt.IsZero() && !now.IsZero() && !now.Before(startedAt) && now.Sub(startedAt) >= budget:
		return fmt.Sprintf("budget exhausted after %s", budget.Round(time.Second))
	case zeroDeltaProbeStreak >= 2:
		return "zero_delta_probe_streak >= 2"
	case probesRemaining <= 0:
		return "no additional long-haul probes available"
	default:
		return ""
	}
}

func normalizeLongHaulConfidence(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
