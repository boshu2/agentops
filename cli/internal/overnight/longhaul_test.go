package overnight

import (
	"testing"
	"time"
)

func TestEvaluateLongHaulActivationDefaultsOff(t *testing.T) {
	got := EvaluateLongHaulActivation(LongHaulControllerOptions{}, LongHaulSignals{
		PacketCount:             1,
		QueueBackedWon:          false,
		TopPacketConfidence:     "medium",
		KnowledgeBriefAvailable: false,
		GoalRequested:           true,
		ProbesAvailable:         2,
	})

	if got.Enabled {
		t.Fatalf("Enabled = true, want false")
	}
	if got.Active {
		t.Fatalf("Active = true, want false")
	}
	if got.TriggerReason != "" {
		t.Fatalf("TriggerReason = %q, want empty", got.TriggerReason)
	}
}

func TestEvaluateLongHaulActivationWeakSignals(t *testing.T) {
	got := EvaluateLongHaulActivation(LongHaulControllerOptions{Enabled: true}, LongHaulSignals{
		PacketCount:             1,
		QueueBackedWon:          false,
		TopPacketConfidence:     "medium",
		KnowledgeBriefAvailable: true,
		GoalRequested:           true,
		ProbesAvailable:         1,
	})

	if !got.Enabled || !got.Active {
		t.Fatalf("summary = %#v, want enabled+active", got)
	}
	if got.TriggerReason != "no strong queue-backed packet won" {
		t.Fatalf("TriggerReason = %q, want no strong queue-backed packet won", got.TriggerReason)
	}
}

func TestEvaluateLongHaulActivationSkipsStrongBaseline(t *testing.T) {
	got := EvaluateLongHaulActivation(LongHaulControllerOptions{Enabled: true}, LongHaulSignals{
		PacketCount:             1,
		QueueBackedWon:          true,
		TopPacketConfidence:     "high",
		RetrievalCoverage:       0.82,
		RetrievalCoverageKnown:  true,
		KnowledgeBriefAvailable: true,
		GoalRequested:           true,
		ProbesAvailable:         2,
	})

	if got.Active {
		t.Fatalf("Active = true, want false")
	}
	if got.ExitReason != "trigger threshold not met" {
		t.Fatalf("ExitReason = %q, want trigger threshold not met", got.ExitReason)
	}
}

func TestEvaluateLongHaulActivationIgnoresSyntheticHighConfidenceWinner(t *testing.T) {
	got := EvaluateLongHaulActivation(LongHaulControllerOptions{Enabled: true}, LongHaulSignals{
		PacketCount:             1,
		QueueBackedWon:          false,
		TopPacketConfidence:     "high",
		RetrievalCoverage:       0.91,
		RetrievalCoverageKnown:  true,
		KnowledgeBriefAvailable: true,
		GoalRequested:           true,
		ProbesAvailable:         2,
	})

	if got.Active {
		t.Fatalf("Active = true, want false (%+v)", got)
	}
	if got.ExitReason != "trigger threshold not met" {
		t.Fatalf("ExitReason = %q, want trigger threshold not met", got.ExitReason)
	}
}

func TestEvaluateLongHaulExit(t *testing.T) {
	start := time.Date(2026, 4, 15, 2, 0, 0, 0, time.UTC)
	if got := EvaluateLongHaulExit(start, start.Add(30*time.Second), time.Hour, 2, 1); got != "zero_delta_probe_streak >= 2" {
		t.Fatalf("EvaluateLongHaulExit zero-delta = %q", got)
	}
	if got := EvaluateLongHaulExit(start, start.Add(2*time.Hour), time.Hour, 0, 1); got != "budget exhausted after 1h0m0s" {
		t.Fatalf("EvaluateLongHaulExit budget = %q", got)
	}
	if got := EvaluateLongHaulExit(start, start.Add(30*time.Second), time.Hour, 0, 0); got != "no additional long-haul probes available" {
		t.Fatalf("EvaluateLongHaulExit probes = %q", got)
	}
}
