package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rpilib "github.com/boshu2/agentops/cli/internal/rpi"
)

// phaseHandoff is the structured artifact that carries context between phases.
// Written by the orchestrator after each phase completes.
// Read by buildPromptForPhase() when constructing the next phase's prompt.
type phaseHandoff struct {
	SchemaVersion int    `json:"schema_version"`
	RunID         string `json:"run_id"`
	Phase         int    `json:"phase"`
	PhaseName     string `json:"phase_name"`
	Status        string `json:"status"` // completed, time_boxed, failed

	// Context for next phase
	Goal     string            `json:"goal"`
	EpicID   string            `json:"epic_id,omitempty"`
	Verdicts map[string]string `json:"verdicts"`

	// Mixed-model provenance
	MixedModeRequested      bool   `json:"mixed_mode_requested,omitempty"`
	MixedModeEffective      bool   `json:"mixed_mode_effective,omitempty"`
	PlannerVendor           string `json:"planner_vendor,omitempty"`
	ReviewerVendor          string `json:"reviewer_vendor,omitempty"`
	MixedModeDegradedReason string `json:"mixed_mode_degraded_reason,omitempty"`

	// What happened
	ArtifactsProduced []string `json:"artifacts_produced"`
	DecisionsMade     []string `json:"decisions_made"`
	OpenRisks         []string `json:"open_risks"`
	AppliedFindings   []string `json:"applied_findings,omitempty"`
	PlanningRules     []string `json:"planning_rules,omitempty"`
	KnownRisks        []string `json:"known_risks,omitempty"`

	// Metrics
	DurationSeconds float64 `json:"duration_seconds"`
	CostUSD         float64 `json:"cost_usd,omitempty"`
	ToolCalls       int     `json:"tool_calls"`

	// Context quality signals
	ContextDegradation bool   `json:"context_degradation"`
	AdHocHandoff       string `json:"ad_hoc_handoff,omitempty"`

	// Narrative (from Claude's phase-N-summary.md, capped)
	Narrative string `json:"narrative,omitempty"`

	// Timestamp
	CompletedAt string `json:"completed_at"`
}

// writePhaseHandoff atomically writes a phase handoff to .agents/rpi/phase-N-handoff.json.
func writePhaseHandoff(cwd string, h *phaseHandoff) error {
	dir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create rpi dir: %w", err)
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handoff: %w", err)
	}

	target := filepath.Join(dir, fmt.Sprintf("phase-%d-handoff.json", h.Phase))
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		// Fallback: direct write if rename fails (cross-device)
		_ = os.Remove(tmp)
		return os.WriteFile(target, data, 0o644)
	}
	return nil
}

// readPhaseHandoff reads a single phase handoff. Falls back to reading
// phase-N-summary.md if the structured handoff doesn't exist (backward compat).
func readPhaseHandoff(cwd string, phaseNum int) (*phaseHandoff, error) {
	dir := filepath.Join(cwd, ".agents", "rpi")
	jsonPath := filepath.Join(dir, fmt.Sprintf("phase-%d-handoff.json", phaseNum))

	data, err := os.ReadFile(jsonPath)
	if err == nil {
		var h phaseHandoff
		if parseErr := json.Unmarshal(data, &h); parseErr != nil {
			return nil, fmt.Errorf("parse handoff: %w", parseErr)
		}
		return &h, nil
	}

	// Fallback: construct minimal handoff from legacy summary
	return readLegacySummaryAsHandoff(cwd, phaseNum)
}

// readLegacySummaryAsHandoff reads a phase-N-summary*.md file and wraps it
// in a minimal phaseHandoff struct for backward compatibility.
func readLegacySummaryAsHandoff(cwd string, phaseNum int) (*phaseHandoff, error) {
	dir := filepath.Join(cwd, ".agents", "rpi")
	// Find phase-N-summary files (may have date suffix)
	pattern := filepath.Join(dir, fmt.Sprintf("phase-%d-summary*.md", phaseNum))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("no handoff or summary found for phase %d", phaseNum)
	}

	// Use the most recent match
	latest := matches[len(matches)-1]
	content, err := os.ReadFile(latest)
	if err != nil {
		return nil, fmt.Errorf("read summary: %w", err)
	}

	phaseNames := map[int]string{1: "discovery", 2: "implementation", 3: "validation"}
	return &phaseHandoff{
		SchemaVersion: 1,
		Phase:         phaseNum,
		PhaseName:     phaseNames[phaseNum],
		Status:        "completed",
		Narrative:     string(content),
	}, nil
}

// readAllHandoffs reads handoffs for phases 1 through upToPhase-1.
// Missing phases are silently skipped.
func readAllHandoffs(cwd string, upToPhase int) ([]*phaseHandoff, error) {
	var handoffs []*phaseHandoff
	for i := 1; i < upToPhase; i++ {
		h, err := readPhaseHandoff(cwd, i)
		if err != nil {
			continue // skip missing phases
		}
		handoffs = append(handoffs, h)
	}
	if len(handoffs) == 0 {
		return nil, fmt.Errorf("no handoffs found for phases 1..%d", upToPhase-1)
	}
	return handoffs, nil
}

func uniqueStringsPreserveOrder(items []string) []string {
	return rpilib.UniqueStringsPreserveOrder(items)
}

func latestMatchingFile(cwd string, patterns ...string) string {
	var latest string
	var latestMod time.Time

	for _, pattern := range patterns {
		glob := pattern
		if cwd != "" {
			glob = filepath.Join(cwd, pattern)
		}
		matches, err := filepath.Glob(glob)
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue
			}
			if latest == "" || info.ModTime().After(latestMod) {
				latest = match
				latestMod = info.ModTime()
			}
		}
	}

	return latest
}

func stripMarkdownFrontmatter(content string) string {
	return rpilib.StripMarkdownFrontmatter(content)
}

func extractFindingIDs(text string) []string {
	return rpilib.ExtractFindingIDs(text)
}

func extractBulletItemsAfterMarker(text, marker string) []string {
	return rpilib.ExtractBulletItemsAfterMarker(text, marker)
}

func extractMarkdownListItemsUnderHeading(text, heading string) []string {
	return rpilib.ExtractMarkdownListItemsUnderHeading(text, heading)
}

func compiledChecklistSummary(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return rpilib.CompiledChecklistSummaryFromContent(id, string(data))
}

func compiledSummariesForFindings(cwd, subdir string, findingIDs []string) []string {
	summaries := make([]string, 0, len(findingIDs))
	for _, id := range uniqueStringsPreserveOrder(findingIDs) {
		path := filepath.Join(cwd, ".agents", subdir, id+".md")
		if summary := compiledChecklistSummary(path); summary != "" {
			summaries = append(summaries, summary)
		}
	}
	return uniqueStringsPreserveOrder(summaries)
}

func discoveryPreventionContext(cwd string) (appliedFindings, planningRules, knownRisks []string) {
	planPath := latestMatchingFile(cwd, ".agents/plans/*.md")
	reportPath := latestMatchingFile(cwd, ".agents/council/*pre-mortem*.md")

	var planApplied []string
	var planRuleFallback []string
	if planPath != "" {
		if data, err := os.ReadFile(planPath); err == nil {
			planRuleFallback = extractBulletItemsAfterMarker(string(data), "Applied findings:")
			planApplied = extractFindingIDs(strings.Join(planRuleFallback, "\n"))
		}
	}

	var reportApplied []string
	var knownRiskFallback []string
	if reportPath != "" {
		if data, err := os.ReadFile(reportPath); err == nil {
			knownRiskFallback = extractMarkdownListItemsUnderHeading(string(data), "## Known Risks Applied")
			reportApplied = extractFindingIDs(strings.Join(knownRiskFallback, "\n"))
		}
	}

	appliedFindings = uniqueStringsPreserveOrder(append(planApplied, reportApplied...))
	planningRules = compiledSummariesForFindings(cwd, "planning-rules", appliedFindings)
	knownRisks = compiledSummariesForFindings(cwd, "pre-mortem-checks", appliedFindings)
	if len(planningRules) == 0 {
		planningRules = uniqueStringsPreserveOrder(planRuleFallback)
	}
	if len(knownRisks) == 0 {
		knownRisks = uniqueStringsPreserveOrder(knownRiskFallback)
	}

	return appliedFindings, planningRules, knownRisks
}

func inheritedPreventionContext(cwd string, phaseNum int) (appliedFindings, planningRules, knownRisks []string) {
	if phaseNum <= 1 {
		return nil, nil, nil
	}

	handoffs, err := readAllHandoffs(cwd, phaseNum)
	if err != nil {
		return nil, nil, nil
	}

	for _, handoff := range handoffs {
		appliedFindings = append(appliedFindings, handoff.AppliedFindings...)
		planningRules = append(planningRules, handoff.PlanningRules...)
		knownRisks = append(knownRisks, handoff.KnownRisks...)
	}

	return uniqueStringsPreserveOrder(appliedFindings),
		uniqueStringsPreserveOrder(planningRules),
		uniqueStringsPreserveOrder(knownRisks)
}

func collectPreventionContext(cwd string, phaseNum int) (appliedFindings, planningRules, knownRisks []string) {
	appliedFindings, planningRules, knownRisks = inheritedPreventionContext(cwd, phaseNum)
	if phaseNum != 1 {
		return appliedFindings, planningRules, knownRisks
	}

	currentApplied, currentPlanning, currentKnown := discoveryPreventionContext(cwd)
	appliedFindings = uniqueStringsPreserveOrder(append(appliedFindings, currentApplied...))
	planningRules = uniqueStringsPreserveOrder(append(planningRules, currentPlanning...))
	knownRisks = uniqueStringsPreserveOrder(append(knownRisks, currentKnown...))
	return appliedFindings, planningRules, knownRisks
}

// fieldAllowed checks whether a field should be included in handoff context.
// Returns true if the manifest has no HandoffFields (backward compat) or the field is listed.
func fieldAllowed(m phaseManifest, field string) bool {
	return rpilib.FieldAllowed(m.HandoffFields, field)
}

// formatVerdicts renders a sorted verdict line from a map.
// Returns empty string if verdicts is nil or empty.
func formatVerdicts(verdicts map[string]string) string {
	return rpilib.FormatVerdicts(verdicts)
}

// renderHandoffField renders a labeled field line.
// For string values: returns "Label: value\n" or "" if empty.
// For []string values: returns "Label: a, b, c\n" or "" if empty.
func renderHandoffField(label string, value interface{}) string {
	return rpilib.RenderHandoffField(label, value)
}

// buildHandoffContext formats handoffs for prompt injection into the next phase.
// The manifest controls which fields are included and narrative truncation length.
func buildHandoffContext(handoffs []*phaseHandoff, manifest phaseManifest) string {
	if len(handoffs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- RPI Context (structured handoffs from prior phases) ---\n")

	// Use goal from latest handoff
	if fieldAllowed(manifest, "goal") {
		for i := len(handoffs) - 1; i >= 0; i-- {
			if handoffs[i].Goal != "" {
				fmt.Fprintf(&sb, "Goal: %s\n\n", handoffs[i].Goal)
				break
			}
		}
	}

	narrativeCap := resolveNarrativeCap(manifest)

	for _, h := range handoffs {
		renderHandoffEntry(&sb, h, manifest, narrativeCap)
	}

	renderDegradationWarnings(&sb, handoffs)

	// Apply token budget if specified in manifest
	if manifest.MaxTokens > 0 {
		result, budgetInfo := applyContextBudget(sb.String(), manifest.MaxTokens)
		if budgetInfo.WasTruncated {
			VerbosePrintf("Context budget applied: %d→%d tokens (-%d)\n",
				budgetInfo.OriginalTokens, budgetInfo.BudgetTokens, budgetInfo.TruncatedTokens)
		}
		return result
	}

	return sb.String()
}

// resolveNarrativeCap returns the narrative character cap from manifest.
// NarrativeCap=0 means "omit narrative" when HandoffFields is set (least-privilege).
// When HandoffFields is empty (no manifest), default to 1000 for backward compat.
func resolveNarrativeCap(manifest phaseManifest) int {
	return rpilib.ResolveNarrativeCap(manifest.NarrativeCap, manifest.HandoffFields)
}

// renderHandoffEntry writes a single phase handoff block to the builder.
// Each field-specific sub-section is delegated to a dedicated renderer so this
// function stays a straight-line composition (see .agents/plans/2026-04-15-context-handoff-extract.md).
func renderHandoffEntry(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest, narrativeCap int) {
	renderHandoffHeader(sb, h)
	renderVerdictsSection(sb, h, manifest)
	renderEpicSection(sb, h, manifest)
	renderArtifactsSection(sb, h, manifest)
	renderAppliedFindingsSection(sb, h, manifest)
	renderPlanningRulesSection(sb, h, manifest)
	renderKnownRisksSection(sb, h, manifest)
	renderDecisionsSection(sb, h, manifest)
	renderOpenRisksSection(sb, h, manifest)
	renderNarrativeSection(sb, h, narrativeCap)
	sb.WriteString("\n")
}

// renderHandoffHeader writes the "[Phase N: name — status (source: ...) in Ns]" line.
func renderHandoffHeader(sb *strings.Builder, h *phaseHandoff) {
	fmt.Fprintf(sb, "[Phase %d: %s — %s (source: phase-%d-handoff.json)", h.Phase, h.PhaseName, h.Status, h.Phase)
	if h.DurationSeconds > 0 {
		fmt.Fprintf(sb, " in %.0fs", h.DurationSeconds)
	}
	sb.WriteString("]\n")
}

// renderVerdictsSection writes the formatted verdicts map if the "verdicts" field is allowed.
func renderVerdictsSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "verdicts") {
		sb.WriteString(formatVerdicts(h.Verdicts))
	}
}

// renderEpicSection writes the "Epic: ..." line if the "epic_id" field is allowed.
func renderEpicSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "epic_id") {
		sb.WriteString(renderHandoffField("Epic", h.EpicID))
	}
}

// renderArtifactsSection writes the "Artifacts: ..." line if the "artifacts_produced" field is allowed.
func renderArtifactsSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "artifacts_produced") {
		sb.WriteString(renderHandoffField("Artifacts", h.ArtifactsProduced))
	}
}

// renderAppliedFindingsSection writes the "Applied findings: ..." line if the "applied_findings" field is allowed.
func renderAppliedFindingsSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "applied_findings") {
		sb.WriteString(renderHandoffField("Applied findings", h.AppliedFindings))
	}
}

// renderPlanningRulesSection writes the "Planning rules: ..." line if the "planning_rules" field is allowed.
func renderPlanningRulesSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "planning_rules") {
		sb.WriteString(renderHandoffField("Planning rules", h.PlanningRules))
	}
}

// renderKnownRisksSection writes the "Known risks: ..." line if the "known_risks" field is allowed.
func renderKnownRisksSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "known_risks") {
		sb.WriteString(renderHandoffField("Known risks", h.KnownRisks))
	}
}

// renderDecisionsSection writes the "Decisions: ..." line if the "decisions_made" field is allowed.
func renderDecisionsSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "decisions_made") {
		sb.WriteString(renderHandoffField("Decisions", h.DecisionsMade))
	}
}

// renderOpenRisksSection writes the "Risks: ..." line if the "open_risks" field is allowed.
func renderOpenRisksSection(sb *strings.Builder, h *phaseHandoff, manifest phaseManifest) {
	if fieldAllowed(manifest, "open_risks") {
		sb.WriteString(renderHandoffField("Risks", h.OpenRisks))
	}
}

// renderNarrativeSection writes the truncated phase narrative if narrativeCap > 0 and content is present.
func renderNarrativeSection(sb *strings.Builder, h *phaseHandoff, narrativeCap int) {
	if narrativeCap > 0 && h.Narrative != "" {
		narrative := h.Narrative
		if len(narrative) > narrativeCap {
			narrative = truncateRunes(narrative, narrativeCap)
		}
		fmt.Fprintf(sb, "Narrative (from phase-%d-summary): %s\n", h.Phase, narrative)
	}
}

// renderDegradationWarnings writes context degradation warnings for handoffs with context loss.
func renderDegradationWarnings(sb *strings.Builder, handoffs []*phaseHandoff) {
	var degradedPhases []int
	for _, h := range handoffs {
		if h.ContextDegradation {
			degradedPhases = append(degradedPhases, h.Phase)
		}
	}
	rpilib.RenderDegradationWarnings(sb, degradedPhases)
}

// buildPhaseHandoffFromState constructs a handoff from existing state + phase result + summary.
func buildPhaseHandoffFromState(state *phasedState, phaseNum int, cwd string) *phaseHandoff {
	phaseNames := map[int]string{1: "discovery", 2: "implementation", 3: "validation"}
	prov := mixedModeProvenanceFromOpts(state.Opts)

	h := &phaseHandoff{
		SchemaVersion:           1,
		RunID:                   state.RunID,
		Phase:                   phaseNum,
		PhaseName:               phaseNames[phaseNum],
		Status:                  "completed",
		Goal:                    state.Goal,
		EpicID:                  state.EpicID,
		Verdicts:                make(map[string]string),
		MixedModeRequested:      prov.Requested,
		MixedModeEffective:      prov.Effective,
		PlannerVendor:           prov.PlannerVendor,
		ReviewerVendor:          prov.ReviewerVendor,
		MixedModeDegradedReason: prov.DegradedReason,
		CompletedAt:             time.Now().UTC().Format(time.RFC3339),
	}

	// Copy accumulated verdicts
	for k, v := range state.Verdicts {
		h.Verdicts[k] = v
	}

	// Read phase result for metrics if available
	resultPath := filepath.Join(cwd, ".agents", "rpi", fmt.Sprintf("phase-%d-result.json", phaseNum))
	if data, err := os.ReadFile(resultPath); err == nil {
		var pr phaseResult
		if json.Unmarshal(data, &pr) == nil {
			h.DurationSeconds = pr.DurationSeconds
			h.Status = pr.Status
			if pr.Status == "" {
				h.Status = "completed"
			}
		}
	}

	// Read narrative from summary file
	summaryDir := filepath.Join(cwd, ".agents", "rpi")
	pattern := filepath.Join(summaryDir, fmt.Sprintf("phase-%d-summary*.md", phaseNum))
	if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
		if content, err := os.ReadFile(matches[len(matches)-1]); err == nil {
			narrative := string(content)
			if len(narrative) > 2000 {
				narrative = truncateRunes(narrative, 2000)
			}
			h.Narrative = narrative
		}
	}

	// Scan for artifacts produced during this phase
	h.ArtifactsProduced = discoverPhaseArtifacts(cwd, phaseNum)
	h.AppliedFindings, h.PlanningRules, h.KnownRisks = collectPreventionContext(cwd, phaseNum)

	// Check if previous phase had a structured handoff (degradation = missing prior handoff)
	if phaseNum > 1 && !handoffDetected(cwd, phaseNum-1) {
		h.ContextDegradation = true
	}

	return h
}

// discoverPhaseArtifacts finds key artifacts produced during a phase.
func discoverPhaseArtifacts(cwd string, phaseNum int) []string {
	var artifacts []string
	rpiDir := filepath.Join(cwd, ".agents", "rpi")

	// Check for phase-specific artifacts
	summaryPattern := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary*.md", phaseNum))
	if matches, _ := filepath.Glob(summaryPattern); len(matches) > 0 {
		for _, m := range matches {
			rel, _ := filepath.Rel(cwd, m)
			if rel != "" {
				artifacts = append(artifacts, rel)
			}
		}
	}

	// Phase-specific artifact discovery
	switch phaseNum {
	case 1: // discovery — look for plans and council reports
		planPattern := filepath.Join(cwd, ".agents", "plans", "*.md")
		if matches, _ := filepath.Glob(planPattern); len(matches) > 0 {
			rel, _ := filepath.Rel(cwd, matches[len(matches)-1])
			if rel != "" {
				artifacts = append(artifacts, rel)
			}
		}
		councilPattern := filepath.Join(cwd, ".agents", "council", "*pre-mortem*.md")
		if matches, _ := filepath.Glob(councilPattern); len(matches) > 0 {
			rel, _ := filepath.Rel(cwd, matches[len(matches)-1])
			if rel != "" {
				artifacts = append(artifacts, rel)
			}
		}
	case 2: // implementation — note crank artifacts
		// Crank artifacts are tracked by beads, not file scanning
	case 3: // validation — look for vibe/post-mortem reports
		vibePattern := filepath.Join(cwd, ".agents", "council", "*vibe*.md")
		if matches, _ := filepath.Glob(vibePattern); len(matches) > 0 {
			rel, _ := filepath.Rel(cwd, matches[len(matches)-1])
			if rel != "" {
				artifacts = append(artifacts, rel)
			}
		}
		postMortemPattern := filepath.Join(cwd, ".agents", "council", "*post-mortem*.md")
		if matches, _ := filepath.Glob(postMortemPattern); len(matches) > 0 {
			rel, _ := filepath.Rel(cwd, matches[len(matches)-1])
			if rel != "" {
				artifacts = append(artifacts, rel)
			}
		}
	}

	return artifacts
}

// truncateRunes truncates s to at most cap runes and appends "..." if truncated.
// Safe for multi-byte UTF-8 characters — avoids slicing mid-codepoint.
func truncateRunes(s string, cap int) string {
	return rpilib.TruncateRunes(s, cap)
}
