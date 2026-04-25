package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const knowledgeHarvestTopicID = "harvested-praxis"

type knowledgeHarvestCatalog struct {
	Timestamp     string                     `json:"timestamp"`
	RigsScanned   int                        `json:"rigs_scanned"`
	TotalFiles    int                        `json:"total_files"`
	Roots         []string                   `json:"roots,omitempty"`
	PromoteTo     string                     `json:"promote_to,omitempty"`
	MinConfidence float64                    `json:"min_confidence,omitempty"`
	Artifacts     []knowledgeHarvestArtifact `json:"artifacts,omitempty"`
	Promoted      []knowledgeHarvestArtifact `json:"promoted,omitempty"`
	Summary       knowledgeHarvestSummary    `json:"summary"`
}

type knowledgeHarvestSummary struct {
	ArtifactsExtracted  int            `json:"artifacts_extracted"`
	UniqueArtifacts     int            `json:"unique_artifacts"`
	DuplicateGroups     int            `json:"duplicate_groups"`
	DuplicateExcess     int            `json:"duplicate_excess"`
	PromotionCandidates int            `json:"promotion_candidates"`
	PromotionWrites     int            `json:"promotion_writes"`
	WarningCount        int            `json:"warning_count"`
	ArtifactsByType     map[string]int `json:"artifacts_by_type,omitempty"`
}

type knowledgeHarvestArtifact struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Summary    string  `json:"summary,omitempty"`
	Type       string  `json:"type"`
	SourceRig  string  `json:"source_rig"`
	SourcePath string  `json:"source_path"`
	Confidence float64 `json:"confidence"`
	Scope      string  `json:"scope"`
	Date       string  `json:"date"`
}

func knowledgeHarvestCatalogPath(agentsRoot string) string {
	return filepath.Join(agentsRoot, "harvest", "latest.json")
}

func runKnowledgeHarvestCatalogBuilder(workspace, agentsRoot string, step knowledgeBuilderInvocation) (knowledgeBuilderRun, error) {
	_ = workspace
	run := knowledgeBuilderRun{knowledgeBuilderInvocation: step, Path: knowledgeHarvestCatalogPath(agentsRoot)}
	catalog, err := loadKnowledgeHarvestCatalog(agentsRoot)
	if err != nil {
		return run, err
	}

	var outputPath string
	var count int
	switch step.Step {
	case "source-manifests":
		outputPath, count, err = buildKnowledgeHarvestSourceManifest(agentsRoot, catalog)
	case "topic-packets":
		outputPath, count, err = buildKnowledgeHarvestTopicPackets(agentsRoot, catalog)
	case "promoted-packets":
		outputPath, count, err = buildKnowledgeHarvestPromotedPackets(agentsRoot, catalog)
	case "chunk-bundles":
		outputPath, count, err = buildKnowledgeHarvestChunkBundles(agentsRoot, catalog)
	default:
		err = fmt.Errorf("unsupported harvest-catalog knowledge builder step: %s", step.Step)
	}
	if err != nil {
		return run, err
	}

	run.Path = outputPath
	run.Metadata = map[string]string{step.Step: fmt.Sprintf("%d", count)}
	run.Output = fmt.Sprintf("%s=%d", strings.ReplaceAll(step.Step, "-", "_"), count)
	return run, nil
}

func loadKnowledgeHarvestCatalog(agentsRoot string) (knowledgeHarvestCatalog, error) {
	path := knowledgeHarvestCatalogPath(agentsRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return knowledgeHarvestCatalog{}, fmt.Errorf("read harvest catalog %s: %w", path, err)
	}

	var catalog knowledgeHarvestCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return knowledgeHarvestCatalog{}, fmt.Errorf("parse harvest catalog %s: %w", path, err)
	}
	if len(catalog.Promoted) == 0 && len(catalog.Artifacts) == 0 {
		return knowledgeHarvestCatalog{}, fmt.Errorf("harvest catalog %s has no artifacts to activate", path)
	}
	return catalog, nil
}

func buildKnowledgeHarvestSourceManifest(agentsRoot string, catalog knowledgeHarvestCatalog) (string, int, error) {
	outputPath := filepath.Join(agentsRoot, "packets", "source-manifests", "index.md")
	artifacts := selectKnowledgeHarvestArtifacts(catalog)
	var b strings.Builder
	b.WriteString("# Harvest Source Manifest\n\n")
	fmt.Fprintf(&b, "- Source catalog: `%s`\n", knowledgeHarvestCatalogPath(agentsRoot))
	fmt.Fprintf(&b, "- Rigs scanned: `%d`\n", catalog.RigsScanned)
	fmt.Fprintf(&b, "- Artifacts extracted: `%d`\n", firstPositive(catalog.Summary.ArtifactsExtracted, catalog.TotalFiles, len(catalog.Artifacts)))
	fmt.Fprintf(&b, "- Unique artifacts: `%d`\n", catalog.Summary.UniqueArtifacts)
	fmt.Fprintf(&b, "- Promotion candidates: `%d`\n", firstPositive(catalog.Summary.PromotionCandidates, len(catalog.Promoted), len(artifacts)))
	fmt.Fprintf(&b, "- Promotion target: `%s`\n", catalog.PromoteTo)
	fmt.Fprintf(&b, "- Minimum confidence: `%g`\n", catalog.MinConfidence)
	if len(catalog.Roots) > 0 {
		b.WriteString("\n## Roots\n\n")
		for _, root := range catalog.Roots {
			fmt.Fprintf(&b, "- `%s`\n", root)
		}
	}
	b.WriteString("\n## Top Source Artifacts\n\n")
	for _, artifact := range limitKnowledgeHarvestArtifacts(artifacts, 12) {
		fmt.Fprintf(&b, "- `%0.2f` %s: `%s`\n", artifact.Confidence, knowledgeHarvestDisplayTitle(artifact), artifact.SourcePath)
	}
	return outputPath, len(artifacts), writeKnowledgeOutput(outputPath, b.String())
}

func buildKnowledgeHarvestTopicPackets(agentsRoot string, catalog knowledgeHarvestCatalog) (string, int, error) {
	artifacts := selectKnowledgeHarvestArtifacts(catalog)
	if len(artifacts) == 0 {
		return "", 0, fmt.Errorf("harvest catalog has no artifacts eligible for topic packet generation")
	}

	topicsDir := filepath.Join(agentsRoot, "topics")
	topicPath := filepath.Join(topicsDir, knowledgeHarvestTopicID+".md")
	indexPath := filepath.Join(topicsDir, "index.md")
	health := knowledgeHarvestHealth(artifacts)
	openGap := "No open gaps recorded."
	if health != "healthy" {
		openGap = "Harvest catalog has too few high-confidence artifacts for canonical promotion."
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "topic_id: %s\n", knowledgeHarvestTopicID)
	b.WriteString("title: Harvested Operator Praxis\n")
	fmt.Fprintf(&b, "health_state: %s\n", health)
	b.WriteString("aliases:\n")
	b.WriteString("  - harvested knowledge\n")
	b.WriteString("  - actionable praxis\n")
	b.WriteString("  - knowledge activation\n")
	b.WriteString("query_seeds:\n")
	b.WriteString("  - harvested knowledge actionable praxis\n")
	b.WriteString("  - operationalize harvest catalog\n")
	b.WriteString("consumer_surfaces:\n")
	b.WriteString("  - .agents/knowledge/book-of-beliefs.md\n")
	b.WriteString("  - .agents/playbooks/harvested-praxis.md\n")
	b.WriteString("  - .agents/briefings/\n")
	b.WriteString("evidence_counts:\n")
	b.WriteString("  conversations: 0\n")
	fmt.Fprintf(&b, "  artifacts: %d\n", len(artifacts))
	fmt.Fprintf(&b, "  verified_hits: %d\n", countHighConfidenceHarvestArtifacts(artifacts, 0.8))
	b.WriteString("---\n\n")
	b.WriteString("# Topic Packet: Harvested Operator Praxis\n\n")
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "The latest harvest catalog surfaced %d candidate artifact(s). This packet converts the strongest harvested learnings, patterns, and research items into a bounded operator surface for this repo.\n\n", len(artifacts))
	b.WriteString("## Consumers\n\n")
	b.WriteString("- `ao knowledge activate`\n")
	b.WriteString("- `ao knowledge brief --goal \"turn harvested knowledge into actionable praxis for this repo\"`\n")
	b.WriteString("- RPI discovery, planning, pre-mortem, and validation phases\n\n")
	b.WriteString("## Key Decisions\n\n")
	b.WriteString("- Convert high-confidence harvested artifacts into operator surfaces before starting freeform implementation.\n")
	b.WriteString("- Treat harvest confidence, source paths, and runnable validation as selection gates for reusable praxis.\n")
	b.WriteString("- Keep generated praxis bounded to the current repository and confirm each rule against executable code before promoting it into default behavior.\n")
	for _, artifact := range limitKnowledgeHarvestArtifacts(artifacts, 4) {
		fmt.Fprintf(&b, "- Apply harvested signal: %s.\n", trimSentencePunctuation(knowledgeHarvestClaim(artifact)))
	}
	b.WriteString("\n## Repeated Patterns\n\n")
	for _, artifact := range limitKnowledgeHarvestArtifacts(artifacts, 8) {
		fmt.Fprintf(&b, "- %s (`%s`, confidence `%0.2f`).\n", trimSentencePunctuation(knowledgeHarvestDisplayTitle(artifact)), artifact.Type, artifact.Confidence)
	}
	b.WriteString("\n## Open Gaps\n\n")
	fmt.Fprintf(&b, "- %s\n", openGap)
	if err := writeKnowledgeOutput(topicPath, b.String()); err != nil {
		return "", 0, err
	}

	index := "# Topic Index\n\n| Topic | Health | Source |\n|---|---|---|\n" +
		fmt.Sprintf("| [Harvested Operator Praxis](%s.md) | `%s` | `%s` |\n", knowledgeHarvestTopicID, health, knowledgeHarvestCatalogPath(agentsRoot))
	if err := writeKnowledgeOutput(indexPath, index); err != nil {
		return "", 0, err
	}
	return topicPath, 1, nil
}

func buildKnowledgeHarvestPromotedPackets(agentsRoot string, catalog knowledgeHarvestCatalog) (string, int, error) {
	artifacts := selectKnowledgeHarvestArtifacts(catalog)
	outputPath := filepath.Join(agentsRoot, "packets", "promoted", knowledgeHarvestTopicID+".md")
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "source_topic: %s\n", knowledgeHarvestTopicID)
	b.WriteString("---\n\n")
	b.WriteString("# Promoted Pattern Packet: Harvested Operator Praxis\n\n")
	b.WriteString("## Primary Claims\n\n")
	b.WriteString("- Harvest is only useful when promoted into small operator surfaces that future sessions actually consume.\n")
	b.WriteString("- High-confidence recurring findings should become planning rules, contract tests, ratchets, or validation checks instead of remaining passive notes.\n")
	for _, artifact := range limitKnowledgeHarvestArtifacts(artifacts, 6) {
		fmt.Fprintf(&b, "- %s.\n", trimSentencePunctuation(knowledgeHarvestClaim(artifact)))
	}
	b.WriteString("\n## Source Artifacts\n\n")
	for _, artifact := range limitKnowledgeHarvestArtifacts(artifacts, 12) {
		fmt.Fprintf(&b, "- `%s` (%s, `%0.2f`)\n", artifact.SourcePath, artifact.Type, artifact.Confidence)
	}
	return outputPath, 1, writeKnowledgeOutput(outputPath, b.String())
}

func buildKnowledgeHarvestChunkBundles(agentsRoot string, catalog knowledgeHarvestCatalog) (string, int, error) {
	artifacts := selectKnowledgeHarvestArtifacts(catalog)
	chunksDir := filepath.Join(agentsRoot, "packets", "chunks")
	outputPath := filepath.Join(chunksDir, knowledgeHarvestTopicID+".md")
	promotedPath := filepath.Join(agentsRoot, "packets", "promoted", knowledgeHarvestTopicID+".md")
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "topic_id: %s\n", knowledgeHarvestTopicID)
	b.WriteString("title: Harvested Operator Praxis\n")
	fmt.Fprintf(&b, "promoted_packet_path: %s\n", promotedPath)
	b.WriteString("---\n\n")
	b.WriteString("# Historical Chunk Bundle: Harvested Operator Praxis\n\n")
	b.WriteString("## Knowledge Chunks\n\n")
	for idx, artifact := range limitKnowledgeHarvestArtifacts(artifacts, 12) {
		fmt.Fprintf(&b, "### Harvest Artifact %02d\n\n", idx+1)
		fmt.Fprintf(&b, "- Chunk ID: %s-%02d\n", knowledgeHarvestTopicID, idx+1)
		fmt.Fprintf(&b, "- Type: %s\n", knowledgeHarvestChunkType(artifact.Type))
		fmt.Fprintf(&b, "- Confidence: %0.2f\n", artifact.Confidence)
		fmt.Fprintf(&b, "- Claim: %s.\n\n", trimSentencePunctuation(knowledgeHarvestClaim(artifact)))
	}
	if len(artifacts) == 0 {
		b.WriteString("### Harvest Overview\n\n")
		fmt.Fprintf(&b, "- Chunk ID: %s-overview\n", knowledgeHarvestTopicID)
		b.WriteString("- Type: overview\n")
		b.WriteString("- Confidence: catalog\n")
		b.WriteString("- Claim: Harvest catalog existed but no artifact claims were eligible.\n")
	}
	if err := writeKnowledgeOutput(outputPath, b.String()); err != nil {
		return "", 0, err
	}

	indexPath := filepath.Join(chunksDir, "index.md")
	index := "# Chunk Index\n\n| Topic | Chunks |\n|---|---:|\n" +
		fmt.Sprintf("| [%s](%s.md) | %d |\n", "Harvested Operator Praxis", knowledgeHarvestTopicID, len(limitKnowledgeHarvestArtifacts(artifacts, 12)))
	if err := writeKnowledgeOutput(indexPath, index); err != nil {
		return "", 0, err
	}

	packetsIndexPath := filepath.Join(agentsRoot, "packets", "index.md")
	packetsIndex := "# Packet Registry\n\n| Packet | Path |\n|---|---|\n" +
		fmt.Sprintf("| Harvested Operator Praxis | `%s` |\n", outputPath)
	if err := writeKnowledgeOutput(packetsIndexPath, packetsIndex); err != nil {
		return "", 0, err
	}
	return outputPath, len(limitKnowledgeHarvestArtifacts(artifacts, 12)), nil
}

func selectKnowledgeHarvestArtifacts(catalog knowledgeHarvestCatalog) []knowledgeHarvestArtifact {
	artifacts := catalog.Promoted
	if len(artifacts) == 0 {
		artifacts = catalog.Artifacts
	}
	out := make([]knowledgeHarvestArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.Title) == "" && strings.TrimSpace(artifact.Summary) == "" {
			continue
		}
		out = append(out, artifact)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if knowledgeHarvestPraxisScore(out[i]) != knowledgeHarvestPraxisScore(out[j]) {
			return knowledgeHarvestPraxisScore(out[i]) > knowledgeHarvestPraxisScore(out[j])
		}
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return knowledgeHarvestDisplayTitle(out[i]) < knowledgeHarvestDisplayTitle(out[j])
	})
	return out
}

func knowledgeHarvestPraxisScore(artifact knowledgeHarvestArtifact) int {
	text := strings.ToLower(knowledgeHarvestDisplayTitle(artifact) + " " + artifact.Summary)
	score := 0
	for _, keyword := range []string{
		" should ",
		" must ",
		" always ",
		" never ",
		" prefer ",
		" validate ",
		" contract test",
		" acceptance probe",
		" ratchet",
		" gate",
		" pre-mortem",
		" deterministic",
	} {
		if strings.Contains(text, keyword) {
			score += 2
		}
	}
	if strings.TrimSpace(artifact.Summary) != "" {
		score++
	}
	if artifact.Type == "pattern" {
		score++
	}
	for _, noisy := range []string{
		"auto-http",
		"# learning: till",
		"picked them up",
		"waiting on your call",
		" in `.agents/rpi/next-work.jsonl`",
	} {
		if strings.Contains(text, noisy) {
			score -= 4
		}
	}
	return score
}

func limitKnowledgeHarvestArtifacts(artifacts []knowledgeHarvestArtifact, limit int) []knowledgeHarvestArtifact {
	if limit <= 0 || len(artifacts) <= limit {
		return artifacts
	}
	return artifacts[:limit]
}

func knowledgeHarvestHealth(artifacts []knowledgeHarvestArtifact) string {
	if len(artifacts) >= 3 && countHighConfidenceHarvestArtifacts(artifacts, 0.8) >= 2 {
		return "healthy"
	}
	return "thin"
}

func countHighConfidenceHarvestArtifacts(artifacts []knowledgeHarvestArtifact, threshold float64) int {
	count := 0
	for _, artifact := range artifacts {
		if artifact.Confidence >= threshold {
			count++
		}
	}
	return count
}

func knowledgeHarvestDisplayTitle(artifact knowledgeHarvestArtifact) string {
	title := strings.TrimSpace(artifact.Title)
	if title == "" {
		title = strings.TrimSpace(artifact.ID)
	}
	if title == "" {
		title = "untitled harvest artifact"
	}
	return strings.Join(strings.Fields(title), " ")
}

func knowledgeHarvestClaim(artifact knowledgeHarvestArtifact) string {
	if summary := strings.TrimSpace(artifact.Summary); summary != "" {
		return strings.Join(strings.Fields(summary), " ")
	}
	return knowledgeHarvestDisplayTitle(artifact)
}

func knowledgeHarvestChunkType(artifactType string) string {
	switch strings.ToLower(strings.TrimSpace(artifactType)) {
	case "pattern":
		return "pattern"
	case "research":
		return "overview"
	default:
		return "decision"
	}
}

func trimSentencePunctuation(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimRight(text, ".;:")
	if text == "" {
		return "No claim text surfaced"
	}
	return text
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
