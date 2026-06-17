// practices: [design-by-contract]
package orchestration

// RouteOptions configures posture routing.
type RouteOptions struct {
	Writers    int
	Unattended bool
	Models     []string
}

// RunRoute recommends a profile for out-of-session orchestration.
func RunRoute(opts RouteOptions) InstrumentResult {
	runID := NewRunID()
	models := normalizeModels(opts.Models)
	rec := RouteRecommendation{
		Shape: ShapeATMOnly,
	}

	hasAGY, hasCodex, hasClaude := false, false, false
	for _, m := range models {
		switch m {
		case "agy", "antigravity":
			hasAGY = true
		case "codex", "gpt", "gpt-5", "gpt-5.5":
			hasCodex = true
		case "opus", "claude", "cc":
			hasClaude = true
		}
	}
	if opts.Writers >= 3 || (hasAGY && hasCodex && hasClaude) {
		rec.Profile = "tri-vendor"
		rec.Rationale = "tri-model out-of-session work-split"
		rec.NextCommand = "ao orchestrate preflight --profile tri-vendor"
	} else if opts.Writers >= 2 || hasCodex || opts.Unattended {
		rec.Profile = "dual-pane"
		rec.Rationale = "dual-lane out-of-session orchestration"
		rec.NextCommand = "ao orchestrate preflight --profile dual-pane"
	} else {
		rec.Shape = ShapeSingleAgent
		rec.Rationale = "stay in-session; no ATM profile recommended"
	}

	status := VerdictStatusPass
	if rec.Profile == "" {
		status = VerdictStatusWarn
	}
	return InstrumentResult{
		SchemaVersion: InstrumentSchemaVersionV1,
		Command:       InstrumentCommandRoute,
		RunID:         runID,
		Verdict:       Verdict{Status: status, Confidence: VerdictConfidenceHigh},
		Route:         &rec,
	}
}
