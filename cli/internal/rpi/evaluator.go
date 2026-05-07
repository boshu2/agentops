package rpi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Finding represents a single evaluator finding with description, fix, and reference.
type Finding struct {
	Description string `json:"description"`
	Fix         string `json:"fix,omitempty"`
	Ref         string `json:"ref,omitempty"`
}

// PhaseNameForNumber maps a 1-based phase number to its canonical name.
func PhaseNameForNumber(phaseNum int) string {
	switch phaseNum {
	case 1:
		return "discovery"
	case 2:
		return "implementation"
	case 3:
		return "validation"
	default:
		return fmt.Sprintf("phase-%d", phaseNum)
	}
}

// gateVerdictIsStructurallyPassing reports whether the gate verdict represents
// a positive structural signal — the orchestrator-level check (e.g. crank
// completion, validation report parse) explicitly succeeded. When this is true,
// the agent transcript heuristic must NOT be used to downgrade the result.
//
// Historical bug (soc-3mtv): the transcript regex set was narrow (looked for
// "PASSED" / "tests passed" / "✓") and missed legitimate green test output that
// reported in different shapes, producing reward < 0.55 → WARN even when the
// orchestrator confirmed the cycle's work-bead was closed and validation had
// passed. The fix: trust the structural gate verdict over the transcript
// heuristic whenever the structural signal is positive.
func gateVerdictIsStructurallyPassing(gateVerdict string) bool {
	switch strings.ToUpper(strings.TrimSpace(gateVerdict)) {
	case "PASS", "DONE":
		return true
	}
	return false
}

// PhaseEvaluatorVerdict computes the evaluator verdict from gate verdict,
// transcript reward, tracker mode, and phase number.
//
// Decision order:
//  1. FAIL/BLOCKED gate is authoritative → FAIL.
//  2. PASS/DONE gate is authoritative → PASS (transcript heuristic does NOT
//     downgrade; see gateVerdictIsStructurallyPassing for rationale).
//  3. WARN/PARTIAL/SKIP gate → WARN.
//  4. No structural signal: fall back to transcript reward heuristic.
func PhaseEvaluatorVerdict(phaseNum int, trackerMode, gateVerdict string, hasTranscript bool, reward float64) string {
	normalized := strings.ToUpper(strings.TrimSpace(gateVerdict))
	switch normalized {
	case "FAIL", "BLOCKED":
		return "FAIL"
	}
	if gateVerdictIsStructurallyPassing(normalized) {
		return "PASS"
	}
	if normalized == "WARN" || normalized == "PARTIAL" || normalized == "SKIP" {
		return "WARN"
	}
	// No structural signal — fall back to transcript heuristic.
	if hasTranscript && reward < 0.25 {
		return "FAIL"
	}
	if phaseNum == 1 && trackerMode == "tasklist" {
		return "WARN"
	}
	if hasTranscript && reward < 0.55 {
		return "WARN"
	}
	return "PASS"
}

// PhaseEvaluatorSummary builds a human-readable summary line for a phase evaluator.
func PhaseEvaluatorSummary(phaseNum int, trackerMode, gateVerdict string, hasTranscript bool, reward float64, findingCount int) string {
	verdict := PhaseEvaluatorVerdict(phaseNum, trackerMode, gateVerdict, hasTranscript, reward)
	parts := []string{
		fmt.Sprintf("%s evaluator marked the phase %s", PhaseNameForNumber(phaseNum), verdict),
	}
	if gate := strings.ToUpper(strings.TrimSpace(gateVerdict)); gate != "" {
		parts = append(parts, fmt.Sprintf("gate=%s", gate))
	}
	if hasTranscript {
		parts = append(parts, fmt.Sprintf("reward=%.2f", reward))
	}
	if findingCount > 0 {
		parts = append(parts, fmt.Sprintf("findings=%d", findingCount))
	}
	if phaseNum == 1 && trackerMode == "tasklist" {
		parts = append(parts, "tracker degraded -> tasklist fallback")
	}
	return strings.Join(parts, " · ")
}

// DefaultEvaluatorFindings produces the standard findings based on gate verdict,
// tracker mode, and transcript reward.
func DefaultEvaluatorFindings(phaseNum int, trackerMode, gateVerdict string, hasTranscript bool, reward float64, transcriptPath string, evidenceRef string) []Finding {
	var findings []Finding

	switch strings.ToUpper(strings.TrimSpace(gateVerdict)) {
	case "BLOCKED":
		findings = append(findings, Finding{
			Description: "Implementation phase ended blocked",
			Fix:         "Unblock the remaining execution path before advancing validation.",
			Ref:         evidenceRef,
		})
	case "PARTIAL":
		findings = append(findings, Finding{
			Description: "Implementation phase ended partial",
			Fix:         "Complete the remaining execution work before validation claims success.",
			Ref:         evidenceRef,
		})
	case "FAIL":
		findings = append(findings, Finding{
			Description: fmt.Sprintf("%s gate returned FAIL", PhaseNameForNumber(phaseNum)),
			Fix:         "Resolve the failing report findings and rerun the phase gate.",
			Ref:         evidenceRef,
		})
	}

	if phaseNum == 1 && trackerMode == "tasklist" {
		findings = append(findings, Finding{
			Description: "Tracker degraded during discovery",
			Fix:         "Use the execution packet and plan artifact as the objective spine until tracker health is restored.",
			Ref:         evidenceRef,
		})
	}

	// Suppress transcript-reward findings when the gate is structurally passing
	// (soc-3mtv). The transcript regex heuristic is too narrow to be authoritative;
	// it produced false-positive "weak completion" findings on green cycles whose
	// test output didn't match the regex set. When the orchestrator-level gate
	// says PASS/DONE, that's the authoritative signal.
	if hasTranscript && !gateVerdictIsStructurallyPassing(gateVerdict) {
		switch {
		case reward < 0.25:
			findings = append(findings, Finding{
				Description: fmt.Sprintf("Transcript-derived reward %.2f indicates a failing session outcome", reward),
				Fix:         "Inspect the transcript signals and resolve the failing test/error/push conditions before retrying.",
				Ref:         transcriptPath,
			})
		case reward < 0.55:
			findings = append(findings, Finding{
				Description: fmt.Sprintf("Transcript-derived reward %.2f indicates weak completion quality", reward),
				Fix:         "Tighten verification and closeout before treating the phase as complete.",
				Ref:         transcriptPath,
			})
		}
	}

	return findings
}

// UniqueFindings deduplicates findings by (Description, Fix, Ref), dropping empty entries.
func UniqueFindings(items []Finding) []Finding {
	seen := make(map[string]struct{}, len(items))
	out := make([]Finding, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Description) + "\x00" + strings.TrimSpace(item.Fix) + "\x00" + strings.TrimSpace(item.Ref)
		if strings.TrimSpace(item.Description) == "" && strings.TrimSpace(item.Fix) == "" && strings.TrimSpace(item.Ref) == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

// SessionIDFromEventDetails extracts a session_id from a JSON details blob.
func SessionIDFromEventDetails(details json.RawMessage) string {
	if len(details) == 0 {
		return ""
	}
	var d map[string]any
	if err := json.Unmarshal(details, &d); err != nil {
		return ""
	}
	if raw, ok := d["session_id"].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}
