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

const maxDreamMorningPackets = 3

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

func executeDreamMorningPackets(cwd string, summary *overnightSummary) {
	snapshotDreamPacketYield(summary)
	plans, err := buildDreamMorningPacketPlans(cwd, *summary)
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

func buildDreamMorningPacketPlans(cwd string, summary overnightSummary) ([]dreamMorningPacketPlan, error) {
	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	entries, err := readQueueEntries(nextWorkPath)
	if err != nil {
		return nil, err
	}

	repoFilter := detectRepoName(cwd)
	selections := rankDreamMorningQueueSelections(cwd, entries, repoFilter, maxDreamMorningPackets)
	selectedIDs := make(map[string]struct{}, len(selections))
	existingByID := indexDreamExistingQueueItems(entries)

	plans := make([]dreamMorningPacketPlan, 0, maxDreamMorningPackets)
	for i, sel := range selections {
		packet := buildDreamQueuePacket(summary, sel, i+1)
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
		if _, exists := selectedIDs[packet.ID]; exists {
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

	return plans, nil
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
