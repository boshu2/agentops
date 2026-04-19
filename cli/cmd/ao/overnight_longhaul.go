package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	ovn "github.com/boshu2/agentops/cli/internal/overnight"
)

const (
	dreamLongHaulProbeKnowledgeBriefFallback = "knowledge-brief-fallback"
	dreamLongHaulProbePacketCorroboration    = "packet-corroboration"
	dreamLongHaulProbeCouncil                = "council"
)

func runDreamLongHaul(
	ctx context.Context,
	cwd string,
	log io.Writer,
	summary *overnightSummary,
	settings overnightSettings,
) error {
	probes := plannedDreamLongHaulProbes(*summary, settings)
	decision := ovn.EvaluateLongHaulActivation(ovn.LongHaulControllerOptions{
		Enabled: settings.LongHaulEnabled,
		Budget:  settings.LongHaulBudget,
	}, buildDreamLongHaulSignals(*summary, len(probes)))

	longHaul := ensureOvernightLongHaul(summary)
	longHaul.Enabled = decision.Enabled
	longHaul.Active = decision.Active
	longHaul.TriggerReason = decision.TriggerReason
	longHaul.ExitReason = decision.ExitReason
	if !decision.Active {
		if len(settings.Runners) > 0 {
			markDreamCouncilStepsSkipped(summary, firstNonEmptyTrimmed(longHaul.ExitReason, "long-haul controller skipped council"))
		}
		return nil
	}

	startedAt := time.Now()
	councilRan := false
	councilSkipNote := ""
	for i, probe := range probes {
		if reason := ovn.EvaluateLongHaulExit(startedAt, time.Now(), settings.LongHaulBudget, longHaul.ZeroDeltaProbeStreak, len(probes)-i); reason != "" {
			longHaul.ExitReason = reason
			break
		}
		if probe == dreamLongHaulProbeCouncil && !dreamCouncilStillNeeded(*summary) {
			councilSkipNote = "long-haul packet corroboration already produced a strong first move"
			markDreamCouncilStepsSkipped(summary, councilSkipNote)
			continue
		}

		changed, err := runDreamLongHaulProbe(ctx, cwd, log, summary, settings, probe)
		if probe == dreamLongHaulProbeCouncil {
			councilRan = true
		}
		longHaul.ProbeCount++
		if err != nil {
			summary.Degraded = append(summary.Degraded, fmt.Sprintf("long-haul %s: %v", probe, err))
		}
		if changed {
			longHaul.ZeroDeltaProbeStreak = 0
		} else {
			longHaul.ZeroDeltaProbeStreak++
		}
	}

	longHaul.ExitReason = ovn.EvaluateLongHaulExit(startedAt, time.Now(), settings.LongHaulBudget, longHaul.ZeroDeltaProbeStreak, 0)
	if len(settings.Runners) > 0 && !councilRan {
		markDreamCouncilStepsSkipped(summary, firstNonEmptyTrimmed(councilSkipNote, longHaul.ExitReason, "long-haul controller skipped council"))
	}
	return nil
}

func buildDreamLongHaulSignals(summary overnightSummary, probesAvailable int) ovn.LongHaulSignals {
	signals := ovn.LongHaulSignals{
		PacketCount:             len(summary.MorningPackets),
		QueueBackedWon:          len(summary.MorningPackets) > 0 && summary.MorningPackets[0].QueueBacked,
		TopPacketConfidence:     topDreamPacketConfidence(summary.MorningPackets),
		KnowledgeBriefAvailable: dreamKnowledgeBriefAvailable(summary),
		GoalRequested:           strings.TrimSpace(summary.Goal) != "",
		ProbesAvailable:         probesAvailable,
	}
	if coverage, ok := lookupFloat(summary.RetrievalLive, "coverage"); ok {
		signals.RetrievalCoverage = coverage
		signals.RetrievalCoverageKnown = true
	}
	return signals
}

func plannedDreamLongHaulProbes(summary overnightSummary, settings overnightSettings) []string {
	probes := make([]string, 0, 3)
	if strings.TrimSpace(summary.Goal) != "" && !dreamKnowledgeBriefAvailable(summary) {
		probes = append(probes, dreamLongHaulProbeKnowledgeBriefFallback)
	}
	if len(summary.MorningPackets) > 0 {
		probes = append(probes, dreamLongHaulProbePacketCorroboration)
	}
	if len(settings.Runners) > 0 {
		probes = append(probes, dreamLongHaulProbeCouncil)
	}
	return probes
}

func runDreamLongHaulProbe(
	ctx context.Context,
	cwd string,
	log io.Writer,
	summary *overnightSummary,
	settings overnightSettings,
	probe string,
) (bool, error) {
	switch probe {
	case dreamLongHaulProbeKnowledgeBriefFallback:
		return runDreamLongHaulKnowledgeBriefFallback(summary)
	case dreamLongHaulProbePacketCorroboration:
		resetDreamPacketYieldBaseline(summary)
		changed, err := runDreamLongHaulPacketCorroboration(summary)
		if err != nil {
			return false, err
		}
		if changed {
			executeDreamMorningPacketsFn(cwd, summary)
		}
		return changed, nil
	case dreamLongHaulProbeCouncil:
		resetDreamPacketYieldBaseline(summary)
		if err := runDreamCouncilFn(ctx, cwd, log, summary, settings); err != nil {
			return false, err
		}
		executeDreamMorningPacketsFn(cwd, summary)
		return dreamYieldImproved(summary.Yield), nil
	default:
		return false, fmt.Errorf("unknown long-haul probe %q", probe)
	}
}

func runDreamLongHaulKnowledgeBriefFallback(summary *overnightSummary) (bool, error) {
	if strings.TrimSpace(summary.Goal) == "" {
		return false, nil
	}
	if summary.Artifacts == nil {
		summary.Artifacts = map[string]string{}
	}
	path := strings.TrimSpace(summary.Artifacts["briefing_fallback"])
	if path == "" {
		path = filepath.Join(summary.OutputDir, "briefing-fallback.json")
		summary.Artifacts["briefing_fallback"] = path
	}
	created := summary.Briefing == nil
	payload := map[string]any{
		"mode":         "fallback",
		"goal":         summary.Goal,
		"reason":       "knowledge brief unavailable during the short path",
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"first_move":   deriveDreamNextAction(*summary),
		"packets":      dreamLongHaulPacketPreview(summary.MorningPackets),
		"degraded":     append([]string(nil), summary.Degraded...),
	}
	if err := writeJSONFile(path, payload); err != nil {
		return false, err
	}
	summary.Briefing = payload
	setOvernightStepStatus(summary, "knowledge-brief-fallback", "done", path, "fallback synthesized from goal and current packets")
	return created, nil
}

func runDreamLongHaulPacketCorroboration(summary *overnightSummary) (bool, error) {
	if len(summary.MorningPackets) == 0 {
		return false, nil
	}
	if summary.Artifacts == nil {
		summary.Artifacts = map[string]string{}
	}
	path := strings.TrimSpace(summary.Artifacts["packet_corroboration"])
	if path == "" {
		path = filepath.Join(summary.OutputDir, "packet-corroboration.json")
		summary.Artifacts["packet_corroboration"] = path
	}

	annotations := dreamBuildPacketCorroboration(*summary)
	changed := !dreamPacketCorroborationEqual(summary.packetCorroboration, annotations)
	payload := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"packets":      annotations,
	}
	if err := writeJSONFile(path, payload); err != nil {
		return false, err
	}
	summary.packetCorroboration = annotations
	note := "no packet corroboration available"
	if len(annotations) > 0 {
		note = fmt.Sprintf("%d packet(s) corroborated from Dream evidence", len(annotations))
	}
	setOvernightStepStatus(summary, "packet-corroboration", "done", path, note)
	return changed, nil
}

func dreamBuildPacketCorroboration(summary overnightSummary) map[string]dreamPacketCorroboration {
	annotations := map[string]dreamPacketCorroboration{}
	for _, packet := range summary.MorningPackets {
		packetID := strings.TrimSpace(packet.ID)
		if packetID == "" {
			continue
		}
		note := dreamPacketCorroborationForPacket(summary, packet)
		note.Evidence = dreamPacketEvidence(note.Evidence...)
		note.TargetFiles = dreamPacketEvidence(note.TargetFiles...)
		if len(note.Evidence) == 0 && len(note.TargetFiles) == 0 && strings.TrimSpace(note.Confidence) == "" {
			continue
		}
		annotations[packetID] = note
	}
	return annotations
}

// dreamPacketCorroborationForPacket builds the raw corroboration record for a
// single morning packet. Kept separate from the caller so the overall CC stays
// under the cli/ ceiling.
func dreamPacketCorroborationForPacket(summary overnightSummary, packet overnightMorningPacket) dreamPacketCorroboration {
	switch packet.SourceEpic {
	case "dream-goal":
		return dreamPacketCorroborationGoal(summary, packet)
	case "dream-retrieval-live":
		return dreamPacketCorroborationRetrieval(summary)
	case "dream-metrics-health":
		return dreamPacketCorroborationMetrics(summary)
	}
	return dreamPacketCorroboration{}
}

func dreamPacketCorroborationGoal(summary overnightSummary, packet overnightMorningPacket) dreamPacketCorroboration {
	note := dreamPacketCorroboration{}
	supports := 0
	if summary.Briefing != nil {
		mode := firstNonEmptyTrimmed(stringifyAny(summary.Briefing["mode"]), "briefing")
		note.Evidence = append(note.Evidence, "Briefing available: "+mode)
		if firstMove := strings.TrimSpace(stringifyAny(summary.Briefing["first_move"])); firstMove != "" {
			note.Evidence = append(note.Evidence, "Briefing first move: "+firstMove)
		}
		note.TargetFiles = append(note.TargetFiles, firstNonEmptyTrimmed(summary.Artifacts["briefing"], summary.Artifacts["briefing_fallback"]))
		supports++
	}
	if findingsRouted, ok := lookupFloat(summary.CloseLoop, "findings_routed"); ok && findingsRouted > 0 {
		note.Evidence = append(note.Evidence, fmt.Sprintf("Close-loop routed %.0f finding(s) into next-work", findingsRouted))
		note.TargetFiles = append(note.TargetFiles, summary.Artifacts["close_loop"])
		supports++
	}
	if coverage, ok := lookupFloat(summary.RetrievalLive, "coverage"); ok && coverage >= 0.50 {
		note.Evidence = append(note.Evidence, fmt.Sprintf("Retrieval coverage stayed healthy at %.2f", coverage))
		note.TargetFiles = append(note.TargetFiles, summary.Artifacts["retrieval_live"])
		supports++
	}
	if summary.Council != nil && strings.TrimSpace(summary.Council.RecommendedFirstAction) != "" && packet.Rank == 1 {
		note.Evidence = append(note.Evidence, "Council recommended: "+strings.TrimSpace(summary.Council.RecommendedFirstAction))
		note.TargetFiles = append(note.TargetFiles, summary.Artifacts["council_synthesis"])
		supports++
	}
	if supports >= 2 {
		note.Confidence = "high"
	}
	return note
}

func dreamPacketCorroborationRetrieval(summary overnightSummary) dreamPacketCorroboration {
	note := dreamPacketCorroboration{}
	if coverage, ok := lookupFloat(summary.RetrievalLive, "coverage"); ok {
		note.Evidence = append(note.Evidence,
			fmt.Sprintf("Retrieval coverage measured %.2f", coverage),
			fmt.Sprintf("Queries with hits: %v/%v", lookupPath(summary.RetrievalLive, "queries_with_hits"), lookupPath(summary.RetrievalLive, "queries")),
		)
		note.TargetFiles = append(note.TargetFiles, summary.Artifacts["retrieval_live"])
		note.Confidence = "high"
	}
	return note
}

func dreamPacketCorroborationMetrics(summary overnightSummary) dreamPacketCorroboration {
	note := dreamPacketCorroboration{}
	if escape, ok := lookupBool(summary.MetricsHealth, "escape_velocity"); ok && !escape {
		note.Evidence = append(note.Evidence, "Metrics reported escape_velocity=false")
		if harvest, ok := lookupFloat(summary.CloseLoop, "harvest_promoted"); ok && harvest > 0 {
			note.Evidence = append(note.Evidence, fmt.Sprintf("Close-loop still promoted %.0f artifact(s)", harvest))
		}
		note.TargetFiles = append(note.TargetFiles, summary.Artifacts["metrics_health"], summary.Artifacts["close_loop"])
		note.Confidence = "high"
	}
	return note
}

func dreamPacketCorroborationEqual(left, right map[string]dreamPacketCorroboration) bool {
	if len(left) != len(right) {
		return false
	}
	for key, l := range left {
		r, ok := right[key]
		if !ok {
			return false
		}
		if strings.TrimSpace(l.Confidence) != strings.TrimSpace(r.Confidence) {
			return false
		}
		if !dreamStringSlicesEqual(l.Evidence, r.Evidence) || !dreamStringSlicesEqual(l.TargetFiles, r.TargetFiles) || !dreamStringSlicesEqual(l.LikelyTests, r.LikelyTests) {
			return false
		}
	}
	return true
}

func dreamStringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.TrimSpace(left[i]) != strings.TrimSpace(right[i]) {
			return false
		}
	}
	return true
}

func dreamLongHaulPacketPreview(packets []overnightMorningPacket) []map[string]any {
	preview := make([]map[string]any, 0, len(packets))
	for _, packet := range packets {
		preview = append(preview, map[string]any{
			"rank":       packet.Rank,
			"title":      packet.Title,
			"confidence": packet.Confidence,
			"command":    packet.MorningCommand,
			"bead_id":    packet.BeadID,
		})
	}
	return preview
}

func dreamKnowledgeBriefAvailable(summary overnightSummary) bool {
	if strings.TrimSpace(summary.Goal) == "" {
		return true
	}
	if summary.Briefing != nil {
		return true
	}
	if step := findDreamStep(summary.Steps, "knowledge-brief"); step != nil && step.Status == "done" {
		return true
	}
	for _, key := range []string{"briefing", "briefing_fallback"} {
		path := strings.TrimSpace(summary.Artifacts[key])
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func dreamYieldImproved(yield *ovn.YieldSummary) bool {
	if yield == nil {
		return false
	}
	switch {
	case yield.PacketCountAfter != yield.PacketCountBefore:
		return true
	case dreamConfidenceRank(yield.TopPacketConfidenceAfter) > dreamConfidenceRank(yield.TopPacketConfidenceBefore):
		return true
	case yield.CouncilActionDelta == "refined" || yield.CouncilActionDelta == "new":
		return true
	default:
		return false
	}
}

func dreamConfidenceRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func findDreamStep(steps []overnightStepSummary, name string) *overnightStepSummary {
	for i := range steps {
		if steps[i].Name == name {
			return &steps[i]
		}
	}
	return nil
}

func dreamCouncilStillNeeded(summary overnightSummary) bool {
	if summary.Council != nil {
		return false
	}
	if len(summary.MorningPackets) == 0 {
		return true
	}
	if dreamConfidenceRank(topDreamPacketConfidence(summary.MorningPackets)) < dreamConfidenceRank("high") {
		return true
	}
	if strings.TrimSpace(summary.Goal) != "" && !dreamKnowledgeBriefAvailable(summary) {
		return true
	}
	return false
}

func markDreamCouncilStepsSkipped(summary *overnightSummary, note string) {
	if summary == nil || summary.Council == nil {
		return
	}
	setOvernightStepStatus(summary, "council-packet", "skipped", summary.Artifacts["council_packet"], note)
	for _, runner := range summary.Council.RequestedRunners {
		setOvernightStepStatus(summary, "council-"+runner, "skipped", summary.Artifacts["council_"+runner], note)
	}
	setOvernightStepStatus(summary, "council-synthesis", "skipped", summary.Artifacts["council_synthesis"], note)
}

func stringifyAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}
