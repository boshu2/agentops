package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maxDreamMorningPackets      = 3
	maxDreamPacketProbeResults  = maxDreamMorningPackets * 2
	dreamPacketStaleRatePercent = 50
)

type overnightMorningPacket struct {
	ID             string   `json:"id" yaml:"id"`
	Rank           int      `json:"rank" yaml:"rank"`
	Title          string   `json:"title" yaml:"title"`
	Type           string   `json:"type" yaml:"type"`
	Severity       string   `json:"severity" yaml:"severity"`
	Confidence     string   `json:"confidence,omitempty" yaml:"confidence,omitempty"`
	Source         string   `json:"source,omitempty" yaml:"source,omitempty"`
	SourceEpic     string   `json:"source_epic,omitempty" yaml:"source_epic,omitempty"`
	TargetRepo     string   `json:"target_repo,omitempty" yaml:"target_repo,omitempty"`
	WhyNow         string   `json:"why_now" yaml:"why_now"`
	Evidence       []string `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	TargetFiles    []string `json:"target_files,omitempty" yaml:"target_files,omitempty"`
	LikelyTests    []string `json:"likely_tests,omitempty" yaml:"likely_tests,omitempty"`
	MorningCommand string   `json:"morning_command" yaml:"morning_command"`
	QueueBacked    bool     `json:"queue_backed,omitempty" yaml:"queue_backed,omitempty"`
	BeadID         string   `json:"bead_id,omitempty" yaml:"bead_id,omitempty"`
	ArtifactPath   string   `json:"artifact_path,omitempty" yaml:"artifact_path,omitempty"`
}

type dreamPacketCorroboration struct {
	Confidence  string   `json:"confidence,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
	TargetFiles []string `json:"target_files,omitempty"`
	LikelyTests []string `json:"likely_tests,omitempty"`
}

type dreamMorningPacketPlan struct {
	Packet     overnightMorningPacket
	EntryIndex int
	ItemIndex  int
	Existing   bool
}

type dreamPacketIssueRecord struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
}

// dreamCuratorProbeTimeout caps each tractability probe so a slow grep
// cannot stall packet emission. Per the 2026-04-26 retro the probe is a
// "5-second" check; this constant matches the documented intent.
const dreamCuratorProbeTimeout = 5 * time.Second

// dreamCuratorSuppression records that the curator skipped a packet
// because the tractability probe found the cited surface already
// present. Surfaces in the run summary so operators can see what was
// suppressed instead of getting a silent gap.
type dreamCuratorSuppression struct {
	PacketID string `json:"packet_id"`
	Title    string `json:"title"`
	Source   string `json:"source"`
	Reason   string `json:"reason"`
	Match    string `json:"match,omitempty"`
}

type dreamPacketProbeResult struct {
	ProbedAt string `json:"probed_at"`
	PacketID string `json:"packet_id"`
	Title    string `json:"title"`
	Source   string `json:"source,omitempty"`
	Stale    bool   `json:"stale"`
	Reason   string `json:"reason,omitempty"`
	Match    string `json:"match,omitempty"`
}

func executeDreamMorningPackets(cwd string, summary *overnightSummary) {
	snapshotDreamPacketYield(summary)
	plans, suppressed, probeResults, err := buildDreamMorningPacketPlans(cwd, *summary)
	summary.CuratorSuppressed = append(summary.CuratorSuppressed, suppressed...)
	if len(probeResults) > 0 {
		summary.Artifacts["dream_probe_results"] = dreamPacketProbeResultsPath(cwd)
		if err := writeDreamPacketProbeResults(cwd, probeResults); err != nil {
			summary.Degraded = append(summary.Degraded, fmt.Sprintf("dream-curator-probe-results: %v", err))
		}
		appendDreamCuratorDegradedFinding(summary, probeResults)
	}
	if err != nil {
		setOvernightStepStatus(summary, "morning-packets", "soft-fail", summary.Artifacts["morning_packets_json"], err.Error())
		setOvernightStepStatus(summary, "bead-sync", "soft-fail", "", "packet synthesis aborted")
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("morning-packets: %v", err))
		refreshOvernightTelemetry(summary)
		return
	}

	assignDreamMorningPacketPaths(summary, plans)
	syncDreamMorningPacketsToBeads(cwd, summary, plans)
	if err := writeDreamMorningPacketArtifacts(summary, plans); err != nil {
		setOvernightStepStatus(summary, "morning-packets", "soft-fail", summary.Artifacts["morning_packets_json"], err.Error())
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("morning-packets: %v", err))
		refreshOvernightTelemetry(summary)
		return
	}
	if err := syncDreamMorningPacketsToQueue(cwd, plans); err != nil {
		setOvernightStepStatus(summary, "morning-packets", "soft-fail", summary.Artifacts["morning_packets_json"], err.Error())
		summary.Degraded = append(summary.Degraded, fmt.Sprintf("morning-packets queue sync: %v", err))
		refreshOvernightTelemetry(summary)
		return
	}

	summary.MorningPackets = extractDreamMorningPackets(plans)
	note := "no actionable packets synthesized"
	if len(summary.MorningPackets) > 0 {
		note = fmt.Sprintf("%d actionable packet(s) ready", len(summary.MorningPackets))
	}
	setOvernightStepStatus(summary, "morning-packets", "done", summary.Artifacts["morning_packets_json"], note)
	refreshOvernightTelemetry(summary)
}

func buildDreamMorningPacketPlans(cwd string, summary overnightSummary) ([]dreamMorningPacketPlan, []dreamCuratorSuppression, []dreamPacketProbeResult, error) {
	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	entries, err := readQueueEntries(nextWorkPath)
	if err != nil {
		return nil, nil, nil, err
	}

	repoFilter := detectRepoName(cwd)
	selections := rankDreamMorningQueueSelections(cwd, entries, repoFilter, maxDreamMorningPackets)
	selectedIDs := make(map[string]struct{}, len(selections))
	existingByID := indexDreamExistingQueueItems(entries)

	var suppressed []dreamCuratorSuppression
	probeResults := make([]dreamPacketProbeResult, 0, maxDreamPacketProbeResults)
	plans := make([]dreamMorningPacketPlan, 0, maxDreamMorningPackets)
	for i, sel := range selections {
		if len(probeResults) >= maxDreamPacketProbeResults {
			break
		}
		packet := buildDreamQueuePacket(summary, sel, i+1)
		// Curator-side tractability probe: per the 2026-04-26 retro,
		// the curator emitted the same stale packets ("philosophy doc",
		// "next-work schema v1.4") three nightlies in a row because no
		// gate verified the cited surface wasn't already shipped. If
		// the probe is conclusively stale, suppress and record; if
		// inconclusive, emit normally.
		result := probeDreamPacket(cwd, packet)
		probeResults = append(probeResults, result)
		if result.Stale {
			suppressed = append(suppressed, dreamCuratorSuppression{
				PacketID: packet.ID,
				Title:    strings.TrimSpace(packet.Title),
				Source:   strings.TrimSpace(packet.Source),
				Reason:   result.Reason,
				Match:    result.Match,
			})
			continue
		}
		if packet.ID != "" {
			selectedIDs[packet.ID] = struct{}{}
		}
		plans = append(plans, dreamMorningPacketPlan{
			Packet:     packet,
			EntryIndex: sel.EntryIndex,
			ItemIndex:  sel.ItemIndex,
			Existing:   true,
		})
	}

	for _, packet := range buildDreamFallbackPackets(summary) {
		if len(plans) >= maxDreamMorningPackets {
			break
		}
		if len(probeResults) >= maxDreamPacketProbeResults {
			break
		}
		if _, exists := selectedIDs[packet.ID]; exists {
			continue
		}
		result := probeDreamPacket(cwd, packet)
		probeResults = append(probeResults, result)
		if result.Stale {
			suppressed = append(suppressed, dreamCuratorSuppression{
				PacketID: packet.ID,
				Title:    strings.TrimSpace(packet.Title),
				Source:   strings.TrimSpace(packet.Source),
				Reason:   result.Reason,
				Match:    result.Match,
			})
			continue
		}
		packet.Rank = len(plans) + 1
		plan := dreamMorningPacketPlan{Packet: packet}
		if existing, ok := existingByID[packet.ID]; ok {
			plan.EntryIndex = existing.EntryIndex
			plan.ItemIndex = existing.ItemIndex
			plan.Existing = true
			plan.Packet.QueueBacked = true
		}
		plans = append(plans, plan)
		selectedIDs[packet.ID] = struct{}{}
	}

	return plans, suppressed, probeResults, nil
}

func probeDreamPacket(cwd string, packet overnightMorningPacket) dreamPacketProbeResult {
	reason, match, stale := probeDreamPacketStaleness(cwd, packet)
	return dreamPacketProbeResult{
		ProbedAt: time.Now().UTC().Format(time.RFC3339),
		PacketID: strings.TrimSpace(packet.ID),
		Title:    strings.TrimSpace(packet.Title),
		Source:   strings.TrimSpace(packet.Source),
		Stale:    stale,
		Reason:   reason,
		Match:    match,
	}
}

// probeDreamPacketStaleness runs a 5-second tractability probe against the
// candidate packet. It checks five surfaces:
//  1. TargetFiles entries — if any cited path already exists on disk, the
//     packet is treated as already done.
//  2. scripts/<x>.sh tokens in the morning_command or title that resolve
//     to an existing script.
//  3. schemas/<x>.json tokens in the morning_command or title that resolve
//     to an existing schema file.
//  4. "Decompose skills/<name>/SKILL.md to under N-line limit" claims —
//     if the cited skill exists and is below the actual size-check fail
//     threshold, the line-limit claim is fictional.
//  5. "Add <phrase> to /<skill> skill" claims — if the cited skill's
//     SKILL.md already contains the proposed phrase verbatim (modulo
//     hyphen/case normalization), the addition has already shipped.
//
// On conclusive staleness it returns (reason, match, true). On inconclusive
// signals it returns (_, _, false) and the curator emits the packet normally.
func probeDreamPacketStaleness(cwd string, packet overnightMorningPacket) (string, string, bool) {
	deadline := time.Now().Add(dreamCuratorProbeTimeout)
	if reason, match, ok := probeTargetFilesExist(cwd, packet.TargetFiles, deadline); ok {
		return reason, match, true
	}
	if reason, match, ok := probeRepoRefTokens(cwd, packet, deadline); ok {
		return reason, match, true
	}
	if reason, match, ok := probeSkillLineLimitClaim(cwd, packet.Title, deadline); ok {
		return reason, match, true
	}
	if reason, match, ok := probeAddToSkillClaim(cwd, packet.Title, deadline); ok {
		return reason, match, true
	}
	return "", "", false
}

// probeTargetFilesExist treats a packet as stale when any cited TargetFiles
// path already exists in the repo as a regular file. URLs and absolute
// paths outside the repo are skipped.
func probeTargetFilesExist(cwd string, targets []string, deadline time.Time) (string, string, bool) {
	for _, raw := range targets {
		if time.Now().After(deadline) {
			return "", "", false
		}
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			continue
		}
		candidate := path
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(cwd, candidate)
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return "target file already tracked", path, true
		}
	}
	return "", "", false
}

// probeRepoRefTokens scans the morning command and title for prefix/<name><suffix>
// tokens (scripts/*.sh, schemas/*.json) that resolve to existing repo files.
// This is the curator's classic false-positive surface from the 2026-04-26 retro.
func probeRepoRefTokens(cwd string, packet overnightMorningPacket, deadline time.Time) (string, string, bool) {
	pathProbes := []struct {
		prefix string
		suffix string
		reason string
	}{
		{"scripts/", ".sh", "scripts/ reference already tracked"},
		{"schemas/", ".json", "schemas/ reference already tracked"},
	}
	for _, hay := range []string{packet.MorningCommand, packet.Title} {
		if time.Now().After(deadline) {
			return "", "", false
		}
		for _, probe := range pathProbes {
			token := extractRepoRef(hay, probe.prefix, probe.suffix)
			if token == "" {
				continue
			}
			candidate := filepath.Join(cwd, token)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return probe.reason, token, true
			}
		}
	}
	return "", "", false
}

// probeSkillLineLimitClaim treats "Decompose skills/<name>/SKILL.md to under
// N-line limit" packets as stale when the cited skill exists and is below
// the canonical size-check FAIL threshold (warn>500 fail>800 per
// scripts/check-skill-size.sh). Per the 2026-04-30 dream-curator-degraded
// finding this packet shape was emitted on consecutive nightlies with
// fictional line limits.
func probeSkillLineLimitClaim(cwd, title string, deadline time.Time) (string, string, bool) {
	if !time.Now().Before(deadline) {
		return "", "", false
	}
	path, ok := extractSkillLineLimitClaim(title)
	if !ok {
		return "", "", false
	}
	candidate := filepath.Join(cwd, path)
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return "", "", false
	}
	lines := countFileLines(candidate)
	if lines <= 0 || lines >= skillSizeFailThreshold {
		return "", "", false
	}
	return "skill SKILL.md is below the canonical size-check fail threshold (warn>500 fail>800)", path, true
}

// skillSizeFailThreshold mirrors scripts/check-skill-size.sh: SKILL.md files
// that exceed 800 lines fail; below that warns or passes. Used by the dream
// curator to detect fictional line-limit claims.
const skillSizeFailThreshold = 800

// probeAddToSkillClaim treats "Add <phrase> to /<skill> skill" titles as
// stale when the cited skill's SKILL.md already contains <phrase>
// verbatim (case-insensitive, hyphens/underscores collapsed to whitespace).
// Today's stale dream packet shape: "Add binary-deployment gate to
// /implement skill" cites a council finding from 2026-05-01 even though
// skills/implement/SKILL.md already ships the gate. This probe is
// conservative: the entire phrase between "Add " and " to /<skill> skill"
// must appear verbatim in the SKILL.md, not just one keyword.
func probeAddToSkillClaim(cwd, title string, deadline time.Time) (string, string, bool) {
	if !time.Now().Before(deadline) {
		return "", "", false
	}
	skillPath, phrase, ok := extractAddToSkillClaim(title)
	if !ok {
		return "", "", false
	}
	candidate := filepath.Join(cwd, skillPath)
	data, err := os.ReadFile(candidate)
	if err != nil {
		return "", "", false
	}
	if !skillContainsAddPhrase(string(data), phrase) {
		return "", "", false
	}
	return "skill SKILL.md already implements the proposed addition", skillPath, true
}

// extractAddToSkillClaim parses titles of the shape
//
//	Add <phrase> to /<skillname> skill
//	Add <phrase> to the /<skillname> skill
//
// and returns the skill's SKILL.md path plus the phrase. It is tolerant
// of leading/trailing whitespace and multi-word phrases. Verbs other than
// "Add" (Wire, Implement, Refactor, Fix) are intentionally rejected — the
// "phrase already in SKILL.md" heuristic is only safe for additions.
func extractAddToSkillClaim(title string) (string, string, bool) {
	trimmed := strings.TrimSpace(title)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "add ") {
		return "", "", false
	}
	rest := trimmed[4:]
	lowerRest := strings.ToLower(rest)
	suffixes := []string{
		" to the /",
		" to /",
	}
	var phrase, afterTo string
	for _, sfx := range suffixes {
		if idx := strings.Index(lowerRest, sfx); idx > 0 {
			phrase = strings.TrimSpace(rest[:idx])
			afterTo = rest[idx+len(sfx):]
			break
		}
	}
	if phrase == "" || afterTo == "" {
		return "", "", false
	}
	skill := ""
	for i, r := range afterTo {
		if r == ' ' || r == '\t' {
			skill = afterTo[:i]
			afterTo = afterTo[i:]
			break
		}
	}
	if skill == "" {
		return "", "", false
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(afterTo)), "skill") {
		return "", "", false
	}
	for _, r := range skill {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return "", "", false
		}
	}
	if skill == "" {
		return "", "", false
	}
	return "skills/" + skill + "/SKILL.md", phrase, true
}

// skillContainsAddPhrase normalizes both content and phrase by lowercasing,
// collapsing hyphens/underscores to spaces, and collapsing whitespace, then
// reports whether the phrase is a substring of the content. The
// normalization lets "binary-deployment gate" match "Binary-Deployment
// Gate" and "binary deployment gate" interchangeably.
func skillContainsAddPhrase(content, phrase string) bool {
	norm := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "-", " ")
		s = strings.ReplaceAll(s, "_", " ")
		s = strings.Join(strings.Fields(s), " ")
		return s
	}
	np := norm(phrase)
	if np == "" {
		return false
	}
	return strings.Contains(norm(content), np)
}

// extractScriptsRef pulls the first scripts/<...>.sh occurrence from s.
// Kept as a thin wrapper for test compatibility; new probes should use
// extractRepoRef directly.
func extractScriptsRef(s string) string {
	return extractRepoRef(s, "scripts/", ".sh")
}

// extractRepoRef pulls the first prefix<...>suffix occurrence from s.
// Returns "" when no match is found. The prefix is required at the
// start of the token; whitespace, quotes, parens, and backticks
// terminate the token.
func extractRepoRef(s, prefix, suffix string) string {
	if prefix == "" || suffix == "" {
		return ""
	}
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	rest := s[idx:]
	end := len(rest)
	for i, r := range rest {
		switch {
		case r == ' ', r == '"', r == '\'', r == ')', r == '`', r == '\n':
			end = i
		}
		if end != len(rest) {
			break
		}
	}
	tok := rest[:end]
	if !strings.HasSuffix(tok, suffix) {
		return ""
	}
	return tok
}

// extractSkillLineLimitClaim looks for the curator's "Decompose
// skills/<name>/SKILL.md to under N-line limit" packet shape. Returns the
// skill path and true when both the path token and a numeric "<N>-line"
// phrase appear in the same string. The numeric value itself is not
// returned because the caller compares against the canonical fail
// threshold from check-skill-size.sh, not the (often fictional) claim.
func extractSkillLineLimitClaim(s string) (string, bool) {
	path := extractRepoRef(s, "skills/", "/SKILL.md")
	if path == "" {
		return "", false
	}
	// Require the title to actually claim a line limit; "Add a section to
	// skills/foo/SKILL.md" should not be flagged just because the file
	// exists. The phrase looks like "<digits>-line".
	lower := strings.ToLower(s)
	dashLine := strings.Index(lower, "-line")
	if dashLine <= 0 {
		return "", false
	}
	// At least one digit must precede "-line" within a small window.
	window := lower[:dashLine]
	if len(window) > 6 {
		window = window[len(window)-6:]
	}
	hasDigit := false
	for _, r := range window {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return "", false
	}
	return path, true
}

func rankDreamMorningQueueSelections(cwd string, entries []nextWorkEntry, repoFilter string, limit int) []queueSelection {
	if limit <= 0 || len(entries) == 0 {
		return nil
	}
	working := cloneDreamQueueEntries(entries)
	selections := make([]queueSelection, 0, limit)
	for len(selections) < limit {
		sel := selectHighestSeverityEntry(working, repoFilter)
		if sel == nil {
			break
		}
		markDreamWorkingSelectionConsumed(working, sel.EntryIndex, sel.ItemIndex)
		if proof := classifyNextWorkCompletionProof(cwd, sel.SourceEpic, sel.Item); proof.Complete {
			continue
		}
		if shouldSkipDreamQueueSelection(sel.Item) {
			continue
		}
		selections = append(selections, *sel)
	}
	return selections
}

func cloneDreamQueueEntries(entries []nextWorkEntry) []nextWorkEntry {
	out := make([]nextWorkEntry, len(entries))
	for i, entry := range entries {
		out[i] = entry
		if len(entry.Items) > 0 {
			out[i].Items = append([]nextWorkItem(nil), entry.Items...)
		}
	}
	return out
}

func markDreamWorkingSelectionConsumed(entries []nextWorkEntry, entryIndex, itemIndex int) {
	for i := range entries {
		if entries[i].QueueIndex != entryIndex {
			continue
		}
		if itemIndex < 0 || itemIndex >= len(entries[i].Items) {
			return
		}
		entries[i].Items[itemIndex].Consumed = true
		entries[i].Items[itemIndex].ClaimStatus = "consumed"
		return
	}
}

func indexDreamExistingQueueItems(entries []nextWorkEntry) map[string]dreamMorningPacketPlan {
	index := make(map[string]dreamMorningPacketPlan)
	for _, entry := range entries {
		for itemIndex, item := range entry.Items {
			if item.Consumed || normalizeClaimStatus(item.Consumed, item.ClaimStatus) == "consumed" {
				continue
			}
			id := strings.TrimSpace(item.ID)
			if id == "" {
				continue
			}
			if _, exists := index[id]; exists {
				continue
			}
			index[id] = dreamMorningPacketPlan{
				EntryIndex: entry.QueueIndex,
				ItemIndex:  itemIndex,
				Existing:   true,
			}
		}
	}
	return index
}

func buildDreamQueuePacket(summary overnightSummary, sel queueSelection, rank int) overnightMorningPacket {
	item := sel.Item
	targetFiles := dreamPacketTargetFiles(item)
	likelyTests := append([]string(nil), item.LikelyTests...)
	if len(likelyTests) == 0 {
		likelyTests = dreamPacketLikelyTests(targetFiles)
	}
	severity := dreamNormalizeSeverity(item.Severity)
	confidence := firstNonEmptyTrimmed(item.Confidence, dreamPacketConfidence(item, len(targetFiles) > 0))
	whyNow := firstNonEmptyTrimmed(item.WhyNow, fmt.Sprintf(
		"Dream ranked this `%s`-severity %s from `%s` during the overnight run.",
		severity,
		firstNonEmptyTrimmed(item.Type, "task"),
		firstNonEmptyTrimmed(item.Source, sel.SourceEpic, "next-work"),
	))
	if item.WhyNow == "" && (item.SourcePath != "" || item.File != "") {
		whyNow += " It already points at concrete files, so it can become real morning work instead of a prose-only suggestion."
	}
	packetID := strings.TrimSpace(item.ID)
	if packetID == "" {
		packetID = dreamPacketID(sel.SourceEpic, item.Title, item.Type, item.SourcePath, item.File, item.Func)
	}
	morningCommand := firstNonEmptyTrimmed(item.MorningCmd, fmt.Sprintf("ao rpi phased %q", strings.TrimSpace(item.Title)))

	packet := overnightMorningPacket{
		ID:             packetID,
		Rank:           rank,
		Title:          strings.TrimSpace(item.Title),
		Type:           firstNonEmptyTrimmed(item.Type, "task"),
		Severity:       severity,
		Confidence:     confidence,
		Source:         firstNonEmptyTrimmed(item.Source, "dream-queue"),
		SourceEpic:     strings.TrimSpace(sel.SourceEpic),
		TargetRepo:     strings.TrimSpace(item.TargetRepo),
		WhyNow:         whyNow,
		Evidence:       dreamPacketEvidence(item.Description, item.Evidence, item.SourcePath, item.File, sel.SourceEpic),
		TargetFiles:    targetFiles,
		LikelyTests:    likelyTests,
		MorningCommand: morningCommand,
		QueueBacked:    true,
	}
	applyDreamPacketCorroboration(&packet, summary)
	return packet
}

func buildDreamFallbackPackets(summary overnightSummary) []overnightMorningPacket {
	packets := make([]overnightMorningPacket, 0, 3)

	if goal := strings.TrimSpace(summary.Goal); goal != "" {
		evidence := []string{fmt.Sprintf("Dream goal: %s", goal)}
		if summary.Council != nil && strings.TrimSpace(summary.Council.RecommendedFirstAction) != "" {
			evidence = append(evidence, "Council guidance: "+strings.TrimSpace(summary.Council.RecommendedFirstAction))
		}
		packet := overnightMorningPacket{
			ID:             dreamPacketID("goal", goal),
			Title:          "Advance overnight goal: " + goal,
			Type:           "task",
			Severity:       "high",
			Confidence:     "medium",
			Source:         "dream-goal",
			SourceEpic:     "dream-goal",
			WhyNow:         "Dream finished with an explicit goal but no stronger queue-backed packet outranked it. Carry the run forward as an implementation packet instead of leaving the goal stranded in the report.",
			Evidence:       evidence,
			MorningCommand: fmt.Sprintf("ao rpi phased %q", goal),
		}
		applyDreamPacketCorroboration(&packet, summary)
		packets = append(packets, packet)
	}

	if coverage, ok := lookupFloat(summary.RetrievalLive, "coverage"); ok && coverage < 0.50 {
		packet := overnightMorningPacket{
			ID:             dreamPacketID("retrieval", fmt.Sprintf("%.3f", coverage)),
			Title:          "Repair Dream retrieval coverage",
			Type:           "bug",
			Severity:       "high",
			Confidence:     "high",
			Source:         "dream-retrieval-live",
			SourceEpic:     "dream-retrieval-live",
			WhyNow:         "Retrieval coverage fell below the morning threshold, so Dream should hand off a concrete repair packet instead of a vague warning.",
			Evidence:       dreamPacketEvidence(fmt.Sprintf("retrieval coverage=%.3f", coverage), summary.Artifacts["retrieval_live"]),
			TargetFiles:    []string{summary.Artifacts["retrieval_live"]},
			LikelyTests:    []string{"cli/cmd/ao/retrieval_bench_test.go"},
			MorningCommand: `ao rpi phased "Repair Dream retrieval coverage"`,
		}
		applyDreamPacketCorroboration(&packet, summary)
		packets = append(packets, packet)
	}

	if escape, ok := lookupBool(summary.MetricsHealth, "escape_velocity"); ok && !escape {
		packet := overnightMorningPacket{
			ID:             dreamPacketID("escape-velocity", summary.RunID),
			Title:          "Restore flywheel escape velocity",
			Type:           "task",
			Severity:       "medium",
			Confidence:     "medium",
			Source:         "dream-metrics-health",
			SourceEpic:     "dream-metrics-health",
			WhyNow:         "The overnight metrics say the flywheel is not compounding fast enough. That should become explicit morning work, not a buried health line.",
			Evidence:       dreamPacketEvidence("metrics_health.escape_velocity=false", summary.Artifacts["metrics_health"]),
			TargetFiles:    []string{summary.Artifacts["metrics_health"]},
			MorningCommand: `ao rpi phased "Restore flywheel escape velocity"`,
		}
		applyDreamPacketCorroboration(&packet, summary)
		packets = append(packets, packet)
	}

	for _, degraded := range summary.Degraded {
		degraded = strings.TrimSpace(degraded)
		if degraded == "" || !shouldEscalateDreamDegradation(degraded) {
			continue
		}
		packet := overnightMorningPacket{
			ID:             dreamPacketID("degraded", degraded),
			Title:          "Investigate Dream degradation: " + degraded,
			Type:           "bug",
			Severity:       "high",
			Confidence:     "medium",
			Source:         "dream-degraded",
			SourceEpic:     "dream-degraded",
			WhyNow:         "Dream degraded overnight. The morning handoff should produce a tracked repair packet instead of silently carrying the failure forward.",
			Evidence:       dreamPacketEvidence(degraded, summary.Artifacts["summary_json"]),
			TargetFiles:    []string{summary.Artifacts["summary_json"]},
			MorningCommand: fmt.Sprintf("ao rpi phased %q", "Investigate Dream degradation: "+degraded),
		}
		applyDreamPacketCorroboration(&packet, summary)
		packets = append(packets, packet)
		break
	}

	return packets
}

func applyDreamPacketCorroboration(packet *overnightMorningPacket, summary overnightSummary) {
	if packet == nil || summary.packetCorroboration == nil {
		return
	}
	note, ok := summary.packetCorroboration[strings.TrimSpace(packet.ID)]
	if !ok {
		return
	}
	if dreamConfidenceRank(note.Confidence) > dreamConfidenceRank(packet.Confidence) {
		packet.Confidence = strings.TrimSpace(note.Confidence)
	}
	packet.Evidence = mergeDreamPacketLines(packet.Evidence, note.Evidence)
	packet.TargetFiles = mergeDreamPacketLines(packet.TargetFiles, note.TargetFiles)
	packet.LikelyTests = mergeDreamPacketLines(packet.LikelyTests, note.LikelyTests)
}

func mergeDreamPacketLines(current, extra []string) []string {
	if len(extra) == 0 {
		return current
	}
	merged := append([]string{}, current...)
	for _, value := range extra {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen := false
		for _, existing := range merged {
			if strings.TrimSpace(existing) == value {
				seen = true
				break
			}
		}
		if !seen {
			merged = append(merged, value)
		}
	}
	return merged
}

func shouldEscalateDreamDegradation(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	switch {
	case strings.HasPrefix(lower, "recovery:"):
		return false
	case strings.HasPrefix(lower, "knowledge-brief:") && strings.Contains(lower, "requires topic packets"):
		return false
	case strings.HasPrefix(lower, "keep-awake requested but"):
		return false
	}
	return strings.Contains(lower, "failed") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "error") ||
		strings.Contains(lower, "regression") ||
		strings.Contains(lower, "integrity") ||
		strings.Contains(lower, "rollback") ||
		strings.Contains(lower, "crash") ||
		strings.Contains(lower, "stuck") ||
		strings.Contains(lower, "unreachable")
}

func shouldSkipDreamQueueSelection(item nextWorkItem) bool {
	if strings.TrimSpace(item.Source) != "dream-degraded" {
		return false
	}
	value := strings.TrimSpace(strings.TrimPrefix(item.Title, "Investigate Dream degradation: "))
	if value == "" || value == item.Title {
		value = firstNonEmptyTrimmed(item.Evidence, item.Description)
	}
	return !shouldEscalateDreamDegradation(value)
}

func assignDreamMorningPacketPaths(summary *overnightSummary, plans []dreamMorningPacketPlan) {
	for i := range plans {
		plans[i].Packet.Rank = i + 1
		slug := beadSlugify(plans[i].Packet.Title, 36)
		plans[i].Packet.ArtifactPath = filepath.Join(
			summary.OutputDir,
			"morning-packets",
			fmt.Sprintf("%02d-%s-%s.json", plans[i].Packet.Rank, slug, shortDreamPacketID(plans[i].Packet.ID)),
		)
	}
}

func syncDreamMorningPacketsToBeads(cwd string, summary *overnightSummary, plans []dreamMorningPacketPlan) {
	if len(plans) == 0 {
		setOvernightStepStatus(summary, "bead-sync", "done", "", "no packets to sync")
		return
	}
	if _, err := exec.LookPath("bd"); err != nil {
		setOvernightStepStatus(summary, "bead-sync", "soft-fail", "", "bd not available")
		summary.Degraded = append(summary.Degraded, "bead-sync: bd not available")
		return
	}

	synced := 0
	failures := []string{}
	for i := range plans {
		packet := &plans[i].Packet
		issueID, err := ensureDreamPacketIssue(cwd, *packet, *summary)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", packet.Title, err))
			continue
		}
		packet.BeadID = issueID
		synced++
	}

	status := "done"
	note := fmt.Sprintf("%d/%d packet(s) synced", synced, len(plans))
	if len(failures) > 0 {
		status = "soft-fail"
		note = failures[0]
		summary.Degraded = append(summary.Degraded, "bead-sync: "+strings.Join(failures, "; "))
	}
	setOvernightStepStatus(summary, "bead-sync", status, "", note)
}

func ensureDreamPacketIssue(cwd string, packet overnightMorningPacket, summary overnightSummary) (string, error) {
	issues, err := lookupDreamPacketIssues(cwd, packet.ID)
	if err != nil {
		return "", err
	}
	for _, issue := range issues {
		if strings.EqualFold(issue.Status, "closed") {
			continue
		}
		if err := updateDreamPacketIssue(cwd, issue.ID, packet, summary); err != nil {
			return "", err
		}
		return issue.ID, nil
	}
	return createDreamPacketIssue(cwd, packet, summary)
}

func lookupDreamPacketIssues(cwd, packetID string) ([]dreamPacketIssueRecord, error) {
	cmd := exec.Command("bd", "list", "--metadata-field", "dream_packet_id="+packetID, "--all", "--limit", "10", "--json")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lookup packet bead: %w", err)
	}
	var issues []dreamPacketIssueRecord
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse packet bead lookup: %w", err)
	}
	return issues, nil
}

func createDreamPacketIssue(cwd string, packet overnightMorningPacket, summary overnightSummary) (string, error) {
	metadata := map[string]string{
		"dream_packet_id":   packet.ID,
		"dream_run_id":      summary.RunID,
		"dream_packet_path": packet.ArtifactPath,
		"dream_source_epic": packet.SourceEpic,
	}
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal packet metadata: %w", err)
	}
	args := []string{
		"create",
		packet.Title,
		"--type", dreamPacketIssueType(packet.Type),
		"--priority", strconv.Itoa(dreamPacketPriority(packet.Severity)),
		"--description", renderDreamPacketIssueDescription(packet, summary),
		"--labels", "dream,morning-packet",
		"--metadata", string(rawMetadata),
		"--json",
	}
	cmd := exec.Command("bd", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("create packet bead: %w", err)
	}
	issues, err := parseDreamPacketIssueMutation(out)
	if err != nil {
		return "", err
	}
	if len(issues) == 0 || strings.TrimSpace(issues[0].ID) == "" {
		return "", fmt.Errorf("create packet bead returned no id")
	}
	return issues[0].ID, nil
}

func updateDreamPacketIssue(cwd, issueID string, packet overnightMorningPacket, summary overnightSummary) error {
	args := []string{
		"update",
		issueID,
		"--priority", strconv.Itoa(dreamPacketPriority(packet.Severity)),
		"--description", renderDreamPacketIssueDescription(packet, summary),
		"--add-label", "dream",
		"--add-label", "morning-packet",
		"--set-metadata", "dream_packet_id=" + packet.ID,
		"--set-metadata", "dream_run_id=" + summary.RunID,
		"--set-metadata", "dream_packet_path=" + packet.ArtifactPath,
		"--set-metadata", "dream_source_epic=" + packet.SourceEpic,
		"--json",
	}
	cmd := exec.Command("bd", args...)
	cmd.Dir = cwd
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("update packet bead: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func parseDreamPacketIssueMutation(raw []byte) ([]dreamPacketIssueRecord, error) {
	var issues []dreamPacketIssueRecord
	if err := json.Unmarshal(raw, &issues); err == nil {
		return issues, nil
	}
	var issue dreamPacketIssueRecord
	if err := json.Unmarshal(raw, &issue); err == nil && issue.ID != "" {
		return []dreamPacketIssueRecord{issue}, nil
	}
	return nil, fmt.Errorf("parse packet bead mutation output")
}

func renderDreamPacketIssueDescription(packet overnightMorningPacket, summary overnightSummary) string {
	var b strings.Builder
	b.WriteString("Dream morning packet\n\n")
	fmt.Fprintf(&b, "Why now: %s\n\n", packet.WhyNow)
	fmt.Fprintf(&b, "Morning command: `%s`\n", packet.MorningCommand)
	fmt.Fprintf(&b, "Dream run: `%s`\n", summary.RunID)
	if packet.ArtifactPath != "" {
		fmt.Fprintf(&b, "Packet artifact: `%s`\n", packet.ArtifactPath)
	}
	if packet.SourceEpic != "" {
		fmt.Fprintf(&b, "Source epic: `%s`\n", packet.SourceEpic)
	}
	if len(packet.Evidence) > 0 {
		b.WriteString("\nEvidence:\n")
		for _, line := range packet.Evidence {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	if len(packet.TargetFiles) > 0 {
		b.WriteString("\nTarget files:\n")
		for _, file := range packet.TargetFiles {
			fmt.Fprintf(&b, "- `%s`\n", file)
		}
	}
	if len(packet.LikelyTests) > 0 {
		b.WriteString("\nLikely tests:\n")
		for _, file := range packet.LikelyTests {
			fmt.Fprintf(&b, "- `%s`\n", file)
		}
	}
	return strings.TrimSpace(b.String())
}

func writeDreamMorningPacketArtifacts(summary *overnightSummary, plans []dreamMorningPacketPlan) error {
	packets := extractDreamMorningPackets(plans)
	indexPayload := map[string]any{
		"run_id":       summary.RunID,
		"repo_root":    summary.RepoRoot,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"packets":      packets,
	}
	if err := writeJSONFile(summary.Artifacts["morning_packets_json"], indexPayload); err != nil {
		return err
	}
	if err := os.WriteFile(summary.Artifacts["morning_packets_markdown"], []byte(renderDreamMorningPacketsMarkdown(packets)), 0o644); err != nil {
		return err
	}
	for _, packet := range packets {
		if packet.ArtifactPath == "" {
			continue
		}
		if err := writeJSONFile(packet.ArtifactPath, packet); err != nil {
			return err
		}
	}
	return nil
}

func dreamPacketProbeResultsPath(cwd string) string {
	return filepath.Join(cwd, ".agents", "dream", "probe-results.jsonl")
}

func writeDreamPacketProbeResults(cwd string, results []dreamPacketProbeResult) error {
	if len(results) == 0 {
		return nil
	}
	path := dreamPacketProbeResultsPath(cwd)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("ensure dream probe dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open dream probe results: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	for _, result := range results {
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("write dream probe result: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync dream probe results: %w", err)
	}
	return nil
}

func appendDreamCuratorDegradedFinding(summary *overnightSummary, results []dreamPacketProbeResult) {
	if summary == nil || len(results) == 0 {
		return
	}
	stale := 0
	for _, result := range results {
		if result.Stale {
			stale++
		}
	}
	if stale == 0 || stale*100 < len(results)*dreamPacketStaleRatePercent {
		return
	}
	for _, degraded := range summary.Degraded {
		if strings.Contains(degraded, "dream-curator-degraded:") {
			return
		}
	}
	summary.Degraded = append(summary.Degraded, fmt.Sprintf(
		"dream-curator-degraded: stale packet rate %d/%d failed threshold %d%%; probe results in %s",
		stale,
		len(results),
		dreamPacketStaleRatePercent,
		summary.Artifacts["dream_probe_results"],
	))
}

func syncDreamMorningPacketsToQueue(cwd string, plans []dreamMorningPacketPlan) error {
	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o750); err != nil {
		return fmt.Errorf("ensure next-work dir: %w", err)
	}

	if err := rewriteNextWorkFile(nextWorkPath, func(idx int, entry *nextWorkEntry) error {
		for _, plan := range plans {
			if !plan.Existing || plan.EntryIndex != idx {
				continue
			}
			if plan.ItemIndex < 0 || plan.ItemIndex >= len(entry.Items) {
				continue
			}
			applyDreamPacketQueueFields(&entry.Items[plan.ItemIndex], plan.Packet)
		}
		return nil
	}); err != nil {
		return err
	}

	synthetic := make([]nextWorkEntry, 0, len(plans))
	for _, plan := range plans {
		if plan.Existing {
			continue
		}
		synthetic = append(synthetic, nextWorkEntry{
			SourceEpic:  firstNonEmptyTrimmed(plan.Packet.SourceEpic, "dream-morning-packets"),
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Items:       []nextWorkItem{dreamPacketToQueueItem(plan.Packet)},
			Consumed:    false,
			ClaimStatus: "available",
		})
	}
	if len(synthetic) == 0 {
		return nil
	}
	return appendDreamMorningQueueEntries(nextWorkPath, synthetic)
}

func applyDreamPacketQueueFields(item *nextWorkItem, packet overnightMorningPacket) {
	item.ID = packet.ID
	item.Confidence = packet.Confidence
	item.WhyNow = packet.WhyNow
	item.TargetFiles = append([]string(nil), packet.TargetFiles...)
	item.LikelyTests = append([]string(nil), packet.LikelyTests...)
	item.MorningCmd = packet.MorningCommand
	item.PacketPath = packet.ArtifactPath
	if packet.BeadID != "" {
		item.BeadID = packet.BeadID
	}
	if item.TargetRepo == "" {
		item.TargetRepo = packet.TargetRepo
	}
	if item.Source == "" {
		item.Source = packet.Source
	}
	if item.Type == "" {
		item.Type = packet.Type
	}
	if item.Severity == "" {
		item.Severity = packet.Severity
	}
	if item.Description == "" {
		item.Description = packet.WhyNow
	}
	if item.Evidence == "" {
		item.Evidence = strings.Join(packet.Evidence, " | ")
	}
}

func dreamPacketToQueueItem(packet overnightMorningPacket) nextWorkItem {
	return nextWorkItem{
		ID:          packet.ID,
		Title:       packet.Title,
		Type:        packet.Type,
		Severity:    packet.Severity,
		Source:      packet.Source,
		Description: packet.WhyNow,
		Evidence:    strings.Join(packet.Evidence, " | "),
		TargetRepo:  packet.TargetRepo,
		Confidence:  packet.Confidence,
		WhyNow:      packet.WhyNow,
		TargetFiles: append([]string(nil), packet.TargetFiles...),
		LikelyTests: append([]string(nil), packet.LikelyTests...),
		MorningCmd:  packet.MorningCommand,
		PacketPath:  packet.ArtifactPath,
		BeadID:      packet.BeadID,
		Consumed:    false,
		ClaimStatus: "available",
	}
}

func appendDreamMorningQueueEntries(path string, entries []nextWorkEntry) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer func() { _ = f.Close() }()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal morning packet queue entry: %w", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write next-work.jsonl: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync next-work.jsonl: %w", err)
	}
	return nil
}

func extractDreamMorningPackets(plans []dreamMorningPacketPlan) []overnightMorningPacket {
	packets := make([]overnightMorningPacket, 0, len(plans))
	for _, plan := range plans {
		packets = append(packets, plan.Packet)
	}
	return packets
}

func renderDreamMorningPacketsMarkdown(packets []overnightMorningPacket) string {
	var b strings.Builder
	b.WriteString("# Dream Morning Packets\n")
	if len(packets) == 0 {
		b.WriteString("\nNo actionable packets synthesized.\n")
		return b.String()
	}
	for _, packet := range packets {
		fmt.Fprintf(&b, "\n## %d. %s\n\n", packet.Rank, packet.Title)
		fmt.Fprintf(&b, "- Severity: `%s`\n", packet.Severity)
		if packet.Confidence != "" {
			fmt.Fprintf(&b, "- Confidence: `%s`\n", packet.Confidence)
		}
		if packet.BeadID != "" {
			fmt.Fprintf(&b, "- Bead: `%s`\n", packet.BeadID)
		}
		fmt.Fprintf(&b, "- Command: `%s`\n", packet.MorningCommand)
		fmt.Fprintf(&b, "- Why now: %s\n", packet.WhyNow)
		for _, evidence := range packet.Evidence {
			fmt.Fprintf(&b, "- Evidence: %s\n", evidence)
		}
		for _, file := range packet.TargetFiles {
			fmt.Fprintf(&b, "- Target file: `%s`\n", file)
		}
		for _, file := range packet.LikelyTests {
			fmt.Fprintf(&b, "- Likely test: `%s`\n", file)
		}
	}
	return b.String()
}

func appendDreamMorningPacketsSection(b *strings.Builder, packets []overnightMorningPacket) {
	if len(packets) == 0 {
		return
	}
	b.WriteString("\n## Morning Packets\n")
	for _, packet := range packets {
		fmt.Fprintf(b, "\n### %d. %s\n\n", packet.Rank, packet.Title)
		fmt.Fprintf(b, "- Severity: `%s`\n", packet.Severity)
		if packet.Confidence != "" {
			fmt.Fprintf(b, "- Confidence: `%s`\n", packet.Confidence)
		}
		if packet.BeadID != "" {
			fmt.Fprintf(b, "- Bead: `%s`\n", packet.BeadID)
		}
		fmt.Fprintf(b, "- Command: `%s`\n", packet.MorningCommand)
		fmt.Fprintf(b, "- Why now: %s\n", packet.WhyNow)
		for _, evidence := range packet.Evidence {
			fmt.Fprintf(b, "- Evidence: %s\n", evidence)
		}
		for _, file := range packet.TargetFiles {
			fmt.Fprintf(b, "- Target file: `%s`\n", file)
		}
		for _, file := range packet.LikelyTests {
			fmt.Fprintf(b, "- Likely test: `%s`\n", file)
		}
	}
}

func dreamPacketTargetFiles(item nextWorkItem) []string {
	files := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range files {
			if existing == value {
				return
			}
		}
		files = append(files, value)
	}
	add(item.SourcePath)
	add(item.File)
	for _, value := range item.TargetFiles {
		add(value)
	}
	return files
}

func dreamPacketLikelyTests(targetFiles []string) []string {
	tests := make([]string, 0, len(targetFiles))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range tests {
			if existing == value {
				return
			}
		}
		tests = append(tests, value)
	}
	for _, file := range targetFiles {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		add(strings.TrimSuffix(file, ".go") + "_test.go")
	}
	return tests
}

func dreamPacketEvidence(values ...string) []string {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		for _, existing := range lines {
			if existing == value {
				value = ""
				break
			}
		}
		if value != "" {
			lines = append(lines, value)
		}
	}
	return lines
}

func dreamPacketConfidence(item nextWorkItem, hasTargetFiles bool) string {
	switch dreamNormalizeSeverity(item.Severity) {
	case "critical", "high":
		if hasTargetFiles {
			return "high"
		}
		return "medium"
	case "medium":
		if hasTargetFiles {
			return "medium"
		}
		return "low"
	default:
		return "low"
	}
}

func dreamNormalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func dreamPacketIssueType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bug":
		return "bug"
	case "feature":
		return "feature"
	default:
		return "task"
	}
}

func dreamPacketPriority(severity string) int {
	switch dreamNormalizeSeverity(severity) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

func dreamPacketID(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if sum == "" {
		return "dream-packet"
	}
	return "dream-" + sum[:16]
}

func shortDreamPacketID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
