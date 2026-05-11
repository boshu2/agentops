// practices: [agile-manifesto, dora-metrics]
package main

// phaseManifest declares what context a phase needs from prior handoffs.
// This enables least-privilege loading — each phase gets only what it needs.
type phaseManifest struct {
	Phase         int      `json:"phase"`
	HandoffFields []string `json:"handoff_fields"` // which phaseHandoff fields to include
	NarrativeCap  int      `json:"narrative_cap"`  // 0 = omit narrative entirely
	MaxTokens     int      `json:"max_tokens"`     // total token budget (0 = unlimited)
}

// defaultPhaseManifests returns the built-in manifest for each phase.
// Phase 1 has no prior context. Phase 2 gets decisions/risks. Phase 3 gets artifacts.
var defaultPhaseManifests = map[int]phaseManifest{
	1: {Phase: 1, HandoffFields: nil, NarrativeCap: 0, MaxTokens: 0},
	// Phase 2 (implementation) needs constraints and hazards from discovery
	// Artifacts omitted — crank tracks via beads
	2: {Phase: 2, HandoffFields: []string{"goal", "epic_id", "verdicts", "applied_findings", "planning_rules", "known_risks", "decisions_made", "open_risks"}, NarrativeCap: 1500, MaxTokens: 2500},
	// Phase 3 (validation) validates implementation against requirements
	// Decisions/risks from phase 2 dropped — focus on what was produced plus bounded prevention context
	3: {Phase: 3, HandoffFields: []string{"goal", "epic_id", "verdicts", "applied_findings", "artifacts_produced"}, NarrativeCap: 2000, MaxTokens: 2500},
}
