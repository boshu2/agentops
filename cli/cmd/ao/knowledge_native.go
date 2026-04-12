package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	knowledgepkg "github.com/boshu2/agentops/cli/internal/knowledge"
)

type knowledgeTopicDetail = knowledgepkg.TopicDetail
type knowledgePromotedPacketState = knowledgepkg.PromotedPacketState
type knowledgeChunkState = knowledgepkg.ChunkState
type knowledgeChunkBundleState = knowledgepkg.ChunkBundleState
type knowledgeNativeBuildResult = knowledgepkg.NativeBuildResult
type knowledgeBriefEvidence = knowledgepkg.BriefEvidence
type knowledgePlaybookRow = knowledgepkg.PlaybookRow

func runKnowledgeNativeBuilder(workspace, agentsRoot string, step knowledgeBuilderInvocation) (knowledgeBuilderRun, error) {
	run := knowledgeBuilderRun{knowledgeBuilderInvocation: step}

	var (
		result knowledgeNativeBuildResult
		err    error
	)

	switch step.Step {
	case "belief-book":
		result, err = buildKnowledgeBeliefBook(agentsRoot)
	case "playbooks":
		result, err = buildKnowledgePlaybooks(agentsRoot, slicesContain(step.Args, "--include-thin"))
	case "briefing":
		goal := knowledgeBuilderGoal(step.Args)
		if goal == "" {
			return run, fmt.Errorf("briefing builder requires --goal")
		}
		result, err = buildKnowledgeBriefing(agentsRoot, goal)
	default:
		return run, fmt.Errorf("unsupported native knowledge builder step: %s", step.Step)
	}
	if err != nil {
		return run, err
	}

	run.Path = result.OutputPath
	run.Metadata = result.Metadata
	run.Output = strings.TrimSpace(result.Output)
	if run.Output == "" && result.OutputPath != "" {
		run.Output = result.OutputPath
	}
	return run, nil
}

func buildKnowledgeBeliefBook(agentsRoot string) (knowledgeNativeBuildResult, error) {
	topics := loadKnowledgeTopicDetails(agentsRoot)
	if len(topics) == 0 {
		return knowledgeNativeBuildResult{}, fmt.Errorf("knowledge beliefs requires topic packets under %s", filepath.Join(agentsRoot, "topics"))
	}

	coreBeliefs, operatingPrinciples := collectKnowledgeBeliefSections(topics, agentsRoot)
	thinTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		if topic.Health != "healthy" {
			thinTopics = append(thinTopics, topic.Title)
		}
	}

	outputPath := filepath.Join(agentsRoot, "knowledge", "book-of-beliefs.md")
	sources := knowledgeExistingPaths(
		filepath.Join(agentsRoot, "packets", "index.md"),
		filepath.Join(agentsRoot, "packets", "chunks", "index.md"),
		filepath.Join(agentsRoot, "topics", "index.md"),
	)
	sourcePath := ""
	if len(sources) > 0 {
		sourcePath = sources[0]
	}
	content := renderKnowledgeBeliefBook(outputPath, sourcePath, coreBeliefs, operatingPrinciples, thinTopics, sources)
	if err := writeKnowledgeOutput(outputPath, content); err != nil {
		return knowledgeNativeBuildResult{}, err
	}

	return knowledgeNativeBuildResult{
		OutputPath: outputPath,
		Metadata:   map[string]string{"belief_book": outputPath},
		Output:     fmt.Sprintf("belief_book=%s", outputPath),
	}, nil
}

func buildKnowledgePlaybooks(agentsRoot string, includeThin bool) (knowledgeNativeBuildResult, error) {
	topics := loadKnowledgeTopicDetails(agentsRoot)
	if len(topics) == 0 {
		return knowledgeNativeBuildResult{}, fmt.Errorf("knowledge playbooks requires topic packets under %s", filepath.Join(agentsRoot, "topics"))
	}

	selected := make([]knowledgeTopicDetail, 0, len(topics))
	for _, topic := range topics {
		if includeThin || topic.Health == "healthy" {
			selected = append(selected, topic)
		}
	}
	if len(selected) == 0 {
		return knowledgeNativeBuildResult{}, fmt.Errorf("knowledge playbooks found no topics eligible for promotion")
	}

	playbooksDir := filepath.Join(agentsRoot, "playbooks")
	if err := pruneKnowledgeMarkdown(playbooksDir, "index.md", "README.md"); err != nil {
		return knowledgeNativeBuildResult{}, err
	}

	rows := make([]knowledgePlaybookRow, 0, len(selected))
	for _, topic := range selected {
		outputPath := filepath.Join(playbooksDir, topic.ID+".md")
		content := renderKnowledgePlaybook(topic, agentsRoot)
		if err := writeKnowledgeOutput(outputPath, content); err != nil {
			return knowledgeNativeBuildResult{}, err
		}
		rows = append(rows, knowledgePlaybookRow{
			Topic:     topic.Title,
			Path:      filepath.Base(outputPath),
			Health:    topic.Health,
			Canonical: topic.Health == "healthy" && knowledgePathExists(filepath.Join(agentsRoot, "packets", "promoted", topic.ID+".md")),
		})
	}

	indexPath := filepath.Join(playbooksDir, "index.md")
	if err := writeKnowledgeOutput(indexPath, renderKnowledgePlaybooksIndex(rows)); err != nil {
		return knowledgeNativeBuildResult{}, err
	}

	return knowledgeNativeBuildResult{
		OutputPath: indexPath,
		Metadata:   map[string]string{"playbooks": fmt.Sprintf("%d", len(rows))},
		Output:     fmt.Sprintf("playbooks=%d", len(rows)),
	}, nil
}

func buildKnowledgeBriefing(agentsRoot, goal string) (knowledgeNativeBuildResult, error) {
	topics := loadKnowledgeTopicDetails(agentsRoot)
	if len(topics) == 0 {
		return knowledgeNativeBuildResult{}, fmt.Errorf("knowledge brief requires topic packets under %s", filepath.Join(agentsRoot, "topics"))
	}

	selected, suppressedThin := selectKnowledgeBriefTopics(goal, topics, agentsRoot)
	if len(selected) == 0 {
		return knowledgeNativeBuildResult{}, fmt.Errorf("knowledge brief could not select relevant topics for %q", goal)
	}

	coreBeliefs := collectKnowledgeBriefCoreBeliefs(topics, agentsRoot)
	evidence := collectKnowledgeBriefEvidence(agentsRoot, selected, 6)
	warnings := collectKnowledgeBriefWarnings(agentsRoot, suppressedThin)
	sourceSurfaces, playbookPath := collectKnowledgeBriefSources(agentsRoot, selected)
	outputPath := knowledgeBriefingOutputPath(agentsRoot, goal)
	content := renderKnowledgeBriefing(goal, selected, coreBeliefs, evidence, warnings, sourceSurfaces, playbookPath, filepath.Join(agentsRoot, "knowledge", "operator-model.md"))
	if err := writeKnowledgeOutput(outputPath, content); err != nil {
		return knowledgeNativeBuildResult{}, err
	}

	return knowledgeNativeBuildResult{
		OutputPath: outputPath,
		Metadata:   map[string]string{"briefing": outputPath},
		Output:     fmt.Sprintf("briefing=%s", outputPath),
	}, nil
}

func selectKnowledgeBriefTopics(goal string, topics []knowledgeTopicDetail, agentsRoot string) ([]knowledgeTopicDetail, []string) {
	candidates := selectRelevantKnowledgeTopics(goal, topics, agentsRoot, 6)
	selected := make([]knowledgeTopicDetail, 0, 3)
	suppressedThin := make([]string, 0, len(candidates))
	for _, topic := range candidates {
		if topic.Health != "healthy" {
			suppressedThin = append(suppressedThin, topic.ID)
			continue
		}
		selected = append(selected, topic)
		if len(selected) == 3 {
			break
		}
	}
	if len(selected) == 0 {
		selected = appendKnowledgeBriefFallbackTopics(selected, topics, 3)
	}
	return selected, suppressedThin
}

func appendKnowledgeBriefFallbackTopics(selected, topics []knowledgeTopicDetail, limit int) []knowledgeTopicDetail {
	for _, topic := range topics {
		if topic.Health != "healthy" || containsKnowledgeTopic(selected, topic.ID) {
			continue
		}
		selected = append(selected, topic)
		if len(selected) == limit {
			break
		}
	}
	return selected
}

func collectKnowledgeBriefCoreBeliefs(topics []knowledgeTopicDetail, agentsRoot string) []string {
	coreBeliefs, _ := collectKnowledgeBeliefSections(topics, agentsRoot)
	if len(coreBeliefs) > 3 {
		return coreBeliefs[:3]
	}
	return coreBeliefs
}

func collectKnowledgeBriefSources(agentsRoot string, selected []knowledgeTopicDetail) ([]string, string) {
	sourceSurfaces := make([]string, 0, len(selected)*3)
	playbookPath := ""
	for _, topic := range selected {
		sourceSurfaces = append(sourceSurfaces, topic.Path)
		chunksPath := filepath.Join(agentsRoot, "packets", "chunks", topic.ID+".md")
		promotedPath := filepath.Join(agentsRoot, "packets", "promoted", topic.ID+".md")
		playbookCandidate := filepath.Join(agentsRoot, "playbooks", topic.ID+".md")
		if knowledgePathExists(chunksPath) {
			sourceSurfaces = append(sourceSurfaces, chunksPath)
		}
		if knowledgePathExists(promotedPath) {
			sourceSurfaces = append(sourceSurfaces, promotedPath)
		}
		if playbookPath == "" && knowledgePathExists(playbookCandidate) {
			playbookPath = playbookCandidate
			sourceSurfaces = append(sourceSurfaces, playbookCandidate)
		}
	}
	sourceSurfaces = appendKnowledgeBriefKnowledgeSources(agentsRoot, sourceSurfaces)
	return dedupeKnowledgeStrings(sourceSurfaces), playbookPath
}

func appendKnowledgeBriefKnowledgeSources(agentsRoot string, sourceSurfaces []string) []string {
	operatorModel := filepath.Join(agentsRoot, "knowledge", "operator-model.md")
	if knowledgePathExists(operatorModel) {
		sourceSurfaces = append(sourceSurfaces, operatorModel)
	}
	bookOfBeliefs := filepath.Join(agentsRoot, "knowledge", "book-of-beliefs.md")
	if knowledgePathExists(bookOfBeliefs) {
		sourceSurfaces = append(sourceSurfaces, bookOfBeliefs)
	}
	return sourceSurfaces
}

func collectKnowledgeBriefWarnings(agentsRoot string, suppressedThin []string) []string {
	warnings := make([]string, 0, len(suppressedThin)+1)
	for _, topicID := range dedupeKnowledgeStrings(suppressedThin) {
		warnings = append(warnings, fmt.Sprintf("`%s` matched the goal but is still thin; it stays discovery-only and was not injected into the control plane.", topicID))
	}
	if warning := knowledgeSourceManifestWarning(agentsRoot); warning != "" {
		warnings = append(warnings, warning)
	}
	return warnings
}

func knowledgeBriefingOutputPath(agentsRoot, goal string) string {
	slug := slugify(goal)
	if slug == "" {
		slug = "briefing"
	}
	return filepath.Join(agentsRoot, "briefings", fmt.Sprintf("%s-%s.md", time.Now().Format("2006-01-02"), slug))
}

func loadKnowledgeTopicDetails(agentsRoot string) []knowledgeTopicDetail {
	topicsDir := filepath.Join(agentsRoot, "topics")
	entries, err := os.ReadDir(topicsDir)
	if err != nil {
		return nil
	}

	topics := make([]knowledgeTopicDetail, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		if entry.Name() == "index.md" || entry.Name() == "README.md" {
			continue
		}

		path := filepath.Join(topicsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		frontmatter := parseKnowledgeFrontmatter(text)

		topic := knowledgeTopicDetail{
			TopicState: knowledgeTopicState{
				ID:     knowledgeFrontmatterString(frontmatter, "topic_id", strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))),
				Title:  knowledgeFrontmatterString(frontmatter, "title", strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))),
				Health: knowledgeFrontmatterString(frontmatter, "health_state", "thin"),
				Path:   path,
			},
			Summary:          knowledgeSectionText(text, "## Summary"),
			Consumers:        dedupeKnowledgeStrings(append(knowledgeFrontmatterStringList(frontmatter, "consumer_surfaces"), extractKnowledgeBullets(text, "## Consumers")...)),
			Aliases:          knowledgeFrontmatterStringList(frontmatter, "aliases"),
			QuerySeeds:       knowledgeFrontmatterStringList(frontmatter, "query_seeds"),
			KeyDecisions:     extractKnowledgeBullets(text, "## Key Decisions"),
			RepeatedPatterns: extractKnowledgeBullets(text, "## Repeated Patterns"),
			Conversations:    knowledgeFrontmatterNestedInt(frontmatter, "evidence_counts", "conversations"),
			Artifacts:        knowledgeFrontmatterNestedInt(frontmatter, "evidence_counts", "artifacts"),
			VerifiedHits:     knowledgeFrontmatterNestedInt(frontmatter, "evidence_counts", "verified_hits"),
		}
		topic.OpenGaps = filterKnowledgeOpenGaps(extractKnowledgeBullets(text, "## Open Gaps"))
		topics = append(topics, topic)
	}

	sort.Slice(topics, func(i, j int) bool {
		if topics[i].Health != topics[j].Health {
			return knowledgeHealthRank(topics[i].Health) > knowledgeHealthRank(topics[j].Health)
		}
		return topics[i].Title < topics[j].Title
	})
	return topics
}

func collectKnowledgeBeliefSections(topics []knowledgeTopicDetail, agentsRoot string) ([]string, []string) {
	coreBeliefs := []string{
		"Build the environment, not the actor; durable intelligence lives in state, traces, and gates.",
		"Replaceable actors should coordinate through stigmergic traces and provenance, not hidden conversational state.",
		"Promote only what survives explicit selection gates and improves future execution.",
	}
	operatingPrinciples := []string{
		"Startup context is budgeted: a few beliefs, one healthy playbook, concrete warnings, and source links.",
		"Thin topics stay discovery-only until they survive verification and promotion.",
		"When CAS freshness is unhealthy, file-backed artifacts and lexical probes are authoritative.",
	}
	for _, topic := range topics {
		if topic.Health != "healthy" {
			continue
		}

		promoted := loadKnowledgePromotedPacket(agentsRoot, topic.ID)
		for _, claim := range promoted.PrimaryClaims {
			coreBeliefs = appendKnowledgeCandidate(coreBeliefs, claim)
		}
		for _, decision := range topic.KeyDecisions {
			coreBeliefs = appendKnowledgeCandidate(coreBeliefs, decision)
		}
		for _, pattern := range topic.RepeatedPatterns {
			operatingPrinciples = appendKnowledgeCandidate(operatingPrinciples, pattern)
		}
	}

	if len(coreBeliefs) == 0 {
		coreBeliefs = appendKnowledgeCandidate(coreBeliefs, "Promoted topic packets have not surfaced stable beliefs yet.")
	}
	if len(operatingPrinciples) == 0 {
		operatingPrinciples = appendKnowledgeCandidate(operatingPrinciples, "Generated operator surfaces should stay citation-backed and grounded in topic packets.")
	}

	if len(coreBeliefs) > 8 {
		coreBeliefs = coreBeliefs[:8]
	}
	if len(operatingPrinciples) > 7 {
		operatingPrinciples = operatingPrinciples[:7]
	}
	return coreBeliefs, operatingPrinciples
}

func loadKnowledgePromotedPacket(agentsRoot, topicID string) knowledgePromotedPacketState {
	path := filepath.Join(agentsRoot, "packets", "promoted", topicID+".md")
	if !knowledgePathExists(path) {
		return knowledgePromotedPacketState{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return knowledgePromotedPacketState{}
	}
	text := string(data)
	frontmatter := parseKnowledgeFrontmatter(text)
	return knowledgePromotedPacketState{
		TopicID:       knowledgeFrontmatterString(frontmatter, "source_topic", topicID),
		Path:          path,
		PrimaryClaims: extractKnowledgeBullets(text, "## Primary Claims"),
	}
}

func selectRelevantKnowledgeTopics(goal string, topics []knowledgeTopicDetail, agentsRoot string, limit int) []knowledgeTopicDetail {
	type scoredTopic struct {
		topic knowledgeTopicDetail
		score int
	}

	goalLower := strings.ToLower(goal)
	goalTokens := knowledgeTokens(goal)
	scored := make([]scoredTopic, 0, len(topics))
	for _, topic := range topics {
		score := 0
		if strings.Contains(goalLower, strings.ToLower(topic.ID)) || strings.Contains(goalLower, strings.ToLower(topic.Title)) {
			score += 8
		}

		combined := strings.ToLower(strings.Join(append(append([]string{topic.Title, topic.Summary}, topic.Aliases...), topic.QuerySeeds...), " "))
		for _, token := range goalTokens {
			if strings.Contains(combined, token) {
				score += 2
			}
		}
		if topic.Health == "healthy" {
			score += 2
		}
		if knowledgePathExists(filepath.Join(agentsRoot, "packets", "promoted", topic.ID+".md")) {
			score++
		}
		scored = append(scored, scoredTopic{topic: topic, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].topic.Health != scored[j].topic.Health {
			return knowledgeHealthRank(scored[i].topic.Health) > knowledgeHealthRank(scored[j].topic.Health)
		}
		return scored[i].topic.Title < scored[j].topic.Title
	})

	selected := make([]knowledgeTopicDetail, 0, limit)
	for _, candidate := range scored {
		if candidate.score == 0 && len(selected) > 0 {
			break
		}
		selected = append(selected, candidate.topic)
		if len(selected) == limit {
			return selected
		}
	}

	for _, candidate := range scored {
		if len(selected) == limit {
			break
		}
		if containsKnowledgeTopic(selected, candidate.topic.ID) {
			continue
		}
		selected = append(selected, candidate.topic)
	}
	return selected
}

func collectKnowledgeBriefEvidence(agentsRoot string, topics []knowledgeTopicDetail, maxItems int) []knowledgeBriefEvidence {
	evidence := make([]knowledgeBriefEvidence, 0, maxItems)
	for _, topic := range topics {
		if len(evidence) == maxItems {
			break
		}

		bundle := loadKnowledgeChunkBundle(agentsRoot, topic.ID)
		if len(bundle.Chunks) == 0 {
			fallbackID := topic.ID + "-overview"
			evidence = append(evidence, knowledgeBriefEvidence{TopicID: topic.ID, ChunkID: fallbackID, Claim: topic.Summary})
			continue
		}

		chunks := bundle.Chunks
		sort.SliceStable(chunks, func(i, j int) bool {
			if knowledgeChunkRank(chunks[i].Type) != knowledgeChunkRank(chunks[j].Type) {
				return knowledgeChunkRank(chunks[i].Type) > knowledgeChunkRank(chunks[j].Type)
			}
			return chunks[i].ID < chunks[j].ID
		})

		added := 0
		for _, chunk := range chunks {
			if strings.TrimSpace(chunk.Claim) == "" {
				continue
			}
			evidence = append(evidence, knowledgeBriefEvidence{
				TopicID: topic.ID,
				ChunkID: chunk.ID,
				Claim:   chunk.Claim,
			})
			added++
			if len(evidence) == maxItems || added == 2 {
				break
			}
		}
	}
	return evidence
}

func loadKnowledgeChunkBundle(agentsRoot, topicID string) knowledgeChunkBundleState {
	path := filepath.Join(agentsRoot, "packets", "chunks", topicID+".md")
	if !knowledgePathExists(path) {
		return knowledgeChunkBundleState{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return knowledgeChunkBundleState{}
	}
	text := string(data)
	frontmatter := parseKnowledgeFrontmatter(text)
	return knowledgeChunkBundleState{
		TopicID:            knowledgeFrontmatterString(frontmatter, "topic_id", topicID),
		Title:              knowledgeFrontmatterString(frontmatter, "title", topicID),
		Path:               path,
		PromotedPacketPath: knowledgeFrontmatterString(frontmatter, "promoted_packet_path", ""),
		Chunks:             parseKnowledgeChunks(text),
	}
}

func parseKnowledgeChunks(text string) []knowledgeChunkState {
	return knowledgepkg.ParseChunks(text)
}

func renderKnowledgeBeliefBook(outputPath, sourcePath string, coreBeliefs, operatingPrinciples, thinTopics, sourceSurfaces []string) string {
	return knowledgepkg.RenderBeliefBook(outputPath, sourcePath, coreBeliefs, operatingPrinciples, thinTopics, sourceSurfaces)
}

func knowledgeSourceManifestWarning(agentsRoot string) string {
	indexPath := filepath.Join(agentsRoot, "packets", "source-manifests", "index.md")
	if !knowledgePathExists(indexPath) {
		return ""
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return ""
	}
	text := string(data)
	if detail := knowledgeSectionText(text, "## Refresh Warning"); strings.TrimSpace(detail) != "" {
		return "CAS freshness warning: " + detail
	}
	if strings.Contains(text, "CASS stale at run start: `true`") || strings.Contains(text, "CASS refresh succeeded: `false`") {
		return "CAS freshness was unhealthy during packet refresh; treat file-backed artifacts and lexical probes as authoritative."
	}
	return ""
}

func knowledgePrimitiveDescriptions() map[string]string {
	return knowledgepkg.PrimitiveDescriptions()
}

func knowledgePrimitivesForTopic(topic knowledgeTopicDetail) []string {
	return knowledgepkg.PrimitivesForTopic(topic)
}

func renderKnowledgePlaybook(topic knowledgeTopicDetail, agentsRoot string) string {
	var b strings.Builder
	chunksPath := filepath.Join(agentsRoot, "packets", "chunks", topic.ID+".md")
	promotedPath := filepath.Join(agentsRoot, "packets", "promoted", topic.ID+".md")
	rules := make([]string, 0, len(topic.KeyDecisions)+len(topic.RepeatedPatterns))
	rules = append(rules, topic.KeyDecisions...)
	rules = append(rules, topic.RepeatedPatterns...)
	rules = dedupeKnowledgeStrings(rules)
	if len(rules) > 6 {
		rules = rules[:6]
	}
	primitives := knowledgePrimitivesForTopic(topic)
	primitiveDescriptions := knowledgePrimitiveDescriptions()

	fmt.Fprintf(&b, "# Playbook Candidate: %s\n\n", topic.Title)
	b.WriteString("## When To Use\n\n")
	b.WriteString(knowledgeWhenToUse(topic))
	b.WriteString("\n\n## Summary\n\n")
	if topic.Summary != "" {
		b.WriteString(topic.Summary)
	} else {
		fmt.Fprintf(&b, "%s has an eligible topic packet but no summary text yet.", topic.Title)
	}
	b.WriteString("\n\n## Primitives Covered\n\n")
	for _, primitive := range primitives {
		fmt.Fprintf(&b, "- `%s`: %s\n", primitive, primitiveDescriptions[primitive])
	}
	b.WriteString("\n## Operator Policy\n\n")
	if len(rules) == 0 {
		b.WriteString("- No durable operator policy surfaced yet.\n")
	} else {
		for _, rule := range rules {
			fmt.Fprintf(&b, "- %s\n", rule)
		}
	}
	b.WriteString("\n## Stigmergic Traces\n\n")
	fmt.Fprintf(&b, "- Topic packet: `%s`\n", topic.Path)
	fmt.Fprintf(&b, "- Chunk bundle: `%s`\n", chunksPath)
	fmt.Fprintf(&b, "- Promoted packet: `%s`\n", promotedPath)
	b.WriteString("\n## Selection Gates\n\n")
	b.WriteString("- Validate the chosen path against current repo or workspace reality before promotion.\n")
	b.WriteString("- Keep thin topics discovery-only; only healthy topics and promoted packets may define default behavior.\n")
	b.WriteString("- Prefer planning rules, pre-mortem checks, and direct artifact lineage over conversational confidence.\n")
	b.WriteString("\n## Trust Status\n\n")
	fmt.Fprintf(&b, "- Topic health: `%s`\n", topic.Health)
	fmt.Fprintf(&b, "- Promoted packet present: `%s`\n", yesNo(knowledgePathExists(promotedPath)))
	if topic.Health != "healthy" {
		b.WriteString("- Treat this playbook as non-canonical until the topic health improves.\n")
	}
	b.WriteString("\n## Source Surfaces\n\n")
	fmt.Fprintf(&b, "- `%s`\n", topic.Path)
	if knowledgePathExists(chunksPath) {
		fmt.Fprintf(&b, "- `%s`\n", chunksPath)
	}
	if knowledgePathExists(promotedPath) {
		fmt.Fprintf(&b, "- `%s`\n", promotedPath)
	}
	return b.String()
}

func renderKnowledgePlaybooksIndex(rows []knowledgePlaybookRow) string {
	return knowledgepkg.RenderPlaybooksIndex(rows)
}

func renderKnowledgeBriefing(goal string, topics []knowledgeTopicDetail, beliefs []string, evidence []knowledgeBriefEvidence, warnings, sourceSurfaces []string, playbookPath, operatorModelPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Briefing: %s\n\n", goal)
	fmt.Fprintf(&b, "**Date:** %s\n\n", time.Now().Format("2006-01-02"))
	b.WriteString("## Objective + Fitness Gradient\n\n")
	fmt.Fprintf(&b, "- Objective: %s\n", goal)
	b.WriteString("- Fitness gradient: prefer the smallest change that improves the environment for this objective and survives explicit proof surfaces.\n")
	b.WriteString("\n## Environment State\n\n")
	if knowledgePathExists(operatorModelPath) {
		fmt.Fprintf(&b, "- Canonical doctrine: `%s`\n", operatorModelPath)
	}
	for _, belief := range beliefs {
		fmt.Fprintf(&b, "- Core belief: %s\n", belief)
	}
	if len(topics) == 0 {
		b.WriteString("- Healthy topics in scope: none selected from the current taxonomy.\n")
	} else {
		for _, topic := range topics {
			fmt.Fprintf(&b, "- Healthy topic in scope: `%s`\n", topic.ID)
		}
	}
	b.WriteString("- Retrieval mode: file-backed operator surfaces remain authoritative; lexical truth beats semantic drift when freshness is degraded.\n")
	b.WriteString("\n## Actors / Ownership\n\n")
	b.WriteString("- Primary actor: the current Codex session operating on one objective at a time.\n")
	b.WriteString("- Replaceable actors: use the briefing for local state and healthy playbooks for behavior policy instead of conversational memory.\n")
	if strings.TrimSpace(playbookPath) != "" {
		fmt.Fprintf(&b, "- Reusable behavior policy: `%s`\n", playbookPath)
	} else {
		b.WriteString("- Reusable behavior policy: no healthy playbook matched this objective yet.\n")
	}
	b.WriteString("- Surface ownership: briefing=local state, playbook=policy, planning rules=selection pressure, packets/provenance=evidence.\n")
	b.WriteString("\n## Stigmergic Traces + Source Surfaces\n\n")
	for _, item := range evidence {
		fmt.Fprintf(&b, "- Trace `%s` `%s`: %s\n", item.TopicID, item.ChunkID, item.Claim)
	}
	if len(evidence) == 0 {
		b.WriteString("- Trace: no matching chunk claims were found in healthy topics.\n")
	}
	for _, source := range sourceSurfaces {
		fmt.Fprintf(&b, "- Source surface: `%s`\n", source)
	}
	b.WriteString("\n## Selection Gates / Proof Surfaces\n\n")
	b.WriteString("- Gate: planning rules and pre-mortem checks outrank preference when they disagree with the current plan.\n")
	b.WriteString("- Gate: promote only changes backed by direct artifacts, promoted packets, or runnable validation.\n")
	if strings.TrimSpace(playbookPath) != "" {
		fmt.Fprintf(&b, "- Proof surface: healthy playbook at `%s`\n", playbookPath)
	}
	if len(evidence) > 0 {
		b.WriteString("- Proof surface: use the chunk claims above as candidate evidence, then confirm against the linked topic or promoted packet.\n")
	}
	b.WriteString("\n## Promotion / Reuse Warnings\n\n")
	if len(warnings) == 0 {
		b.WriteString("- No promotion warnings surfaced for the selected healthy topics.\n")
	} else {
		for _, warning := range warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	b.WriteString("\n## Refresh Command\n\n")
	fmt.Fprintf(&b, "`ao knowledge brief --goal %q`\n", goal)
	return b.String()
}

func knowledgeWhenToUse(topic knowledgeTopicDetail) string {
	return knowledgepkg.WhenToUse(topic)
}

func pruneKnowledgeMarkdown(dir string, preserve ...string) error {
	if GetDryRun() {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	preserveSet := make(map[string]bool, len(preserve))
	for _, name := range preserve {
		preserveSet[name] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" || preserveSet[entry.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func writeKnowledgeOutput(path, content string) error {
	if GetDryRun() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimRight(content, "\n")+"\n"), 0o644)
}

func knowledgeSectionText(text, heading string) string {
	return knowledgepkg.SectionText(text, heading)
}

func knowledgeFrontmatterStringList(frontmatter map[string]any, key string) []string {
	return knowledgepkg.FrontmatterStringList(frontmatter, key)
}

func knowledgeFrontmatterNestedInt(frontmatter map[string]any, parent, key string) int {
	return knowledgepkg.FrontmatterNestedInt(frontmatter, parent, key)
}

func knowledgeFieldValue(line string) string { return knowledgepkg.FieldValue(line) }

func knowledgeBuilderGoal(args []string) string { return knowledgepkg.BuilderGoal(args) }

func slicesContain(items []string, target string) bool {
	return knowledgepkg.SlicesContain(items, target)
}

func knowledgeTokens(text string) []string { return knowledgepkg.Tokens(text) }

func containsKnowledgeTopic(topics []knowledgeTopicDetail, topicID string) bool {
	return knowledgepkg.ContainsTopic(topics, topicID)
}

func knowledgeHealthRank(health string) int { return knowledgepkg.HealthRank(health) }

func knowledgeChunkRank(chunkType string) int { return knowledgepkg.ChunkRank(chunkType) }

func appendKnowledgeCandidate(items []string, candidate string) []string {
	return knowledgepkg.AppendCandidate(items, candidate)
}

func knowledgeExistingPaths(paths ...string) []string {
	existing := make([]string, 0, len(paths))
	for _, path := range paths {
		if knowledgePathExists(path) {
			existing = append(existing, path)
		}
	}
	return dedupeKnowledgeStrings(existing)
}

func yesNo(value bool) string { return knowledgepkg.YesNo(value) }
