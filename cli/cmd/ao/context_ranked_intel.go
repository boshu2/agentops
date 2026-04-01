package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type rankedContextBundle struct {
	Packet         StigmergicPacket
	Learnings      []learning
	Patterns       []pattern
	Findings       []knowledgeFinding
	RecentSessions []session
	NextWork       []nextWorkItem
	Research       []codexArtifactRef
	LegacyIntel    []intelEntry
}

type intelSectionSpec struct {
	Title   string
	Bullets []string
}

func normalizeAssemblePhase(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "", "task":
		return "task"
	case "startup", "start":
		return "startup"
	case "planning", "plan":
		return "planning"
	case "pre-mortem", "premortem", "pre_mortem":
		return "pre-mortem"
	case "validation", "validate":
		return "validation"
	default:
		return "task"
	}
}

func contextBundleLimit(budget int) int {
	switch {
	case budget >= 18000:
		return 6
	case budget >= 12000:
		return 5
	case budget >= 7000:
		return 4
	default:
		return 3
	}
}

func collectRankedContextBundle(cwd, query string, limit int) rankedContextBundle {
	if limit <= 0 {
		limit = defaultStigmergicPacketLimit
	}

	repo := detectRepoName(cwd)
	packet, _ := assembleStigmergicPacket(cwd, StigmergicTarget{
		GoalText: query,
		Repo:     repo,
		Limit:    limit,
	})

	learnings, _ := collectLearnings(cwd, query, limit, "", 0)
	patterns, _ := collectPatterns(cwd, query, limit, "", 0)
	findings, _ := collectFindings(cwd, query, limit, "", 0)
	recentSessions, _ := collectRecentSessions(cwd, query, minInt(limit, MaxSessionsToInject))
	nextWork, _ := readUnconsumedItems(filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl"), repo)

	return rankedContextBundle{
		Packet:         packet,
		Learnings:      limitLearnings(learnings, limit),
		Patterns:       limitPatterns(patterns, limit),
		Findings:       prioritizeFindings(findings, packet.AppliedFindings, limit),
		RecentSessions: limitSessions(recentSessions, limit),
		NextWork:       chooseRankedNextWork(packet, nextWork, limit),
		Research:       limitResearchRefs(collectRecentResearchArtifacts(cwd, query, limit), limit),
		LegacyIntel:    collectLegacyIntelEntries(cwd, query, limit),
	}
}

func buildRankedContextBundle(cwd, query string, limit int, learnings []learning, patterns []pattern, findings []knowledgeFinding, recentSessions []session, nextWork []nextWorkItem, research []codexArtifactRef) rankedContextBundle {
	if limit <= 0 {
		limit = defaultStigmergicPacketLimit
	}

	repo := detectRepoName(cwd)
	packet, _ := assembleStigmergicPacket(cwd, StigmergicTarget{
		GoalText: query,
		Repo:     repo,
		Limit:    limit,
	})

	return rankedContextBundle{
		Packet:         packet,
		Learnings:      limitLearnings(learnings, limit),
		Patterns:       limitPatterns(patterns, limit),
		Findings:       prioritizeFindings(findings, packet.AppliedFindings, limit),
		RecentSessions: limitSessions(recentSessions, limit),
		NextWork:       chooseRankedNextWork(packet, nextWork, limit),
		Research:       limitResearchRefs(research, limit),
	}
}

func renderRankedIntelSection(cwd, query, phase string, budget int) string {
	bundle := collectRankedContextBundle(cwd, query, contextBundleLimit(budget))
	return renderRankedIntelSectionFromBundle(bundle, phase, budget)
}

func renderRankedIntelSectionFromBundle(bundle rankedContextBundle, phase string, budget int) string {
	var sb strings.Builder
	for _, section := range rankedIntelSections(bundle, phase) {
		if len(section.Bullets) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n", section.Title))
		for _, bullet := range section.Bullets {
			sb.WriteString("- ")
			sb.WriteString(bullet)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if sb.Len() == 0 {
		sb.WriteString("_No learnings or patterns found._\n")
	}
	return truncateToCharBudget(sb.String(), budget)
}

func rankedIntelSections(bundle rankedContextBundle, phase string) []intelSectionSpec {
	var specs []intelSectionSpec
	add := func(title string, bullets []string) {
		if len(bullets) == 0 {
			return
		}
		specs = append(specs, intelSectionSpec{Title: title, Bullets: bullets})
	}

	planningRules := stringBullets(bundle.Packet.PlanningRules)
	knownRisks := stringBullets(bundle.Packet.KnownRisks)
	findings := findingBullets(bundle.Findings)
	learnings := learningBullets(bundle.Learnings)
	patterns := patternBullets(bundle.Patterns)
	nextWork := nextWorkBullets(bundle.NextWork)
	sessions := sessionBullets(bundle.RecentSessions)
	research := researchBullets(bundle.Research)
	legacy := legacyIntelBullets(bundle.LegacyIntel)

	switch normalizeAssemblePhase(phase) {
	case "startup":
		add("Planning Rules", planningRules)
		add("Known Risks", knownRisks)
		add("Relevant Next Work", nextWork)
		add("Findings", findings)
		add("Learnings", learnings)
		add("Patterns", patterns)
		add("Recent Sessions", sessions)
		add("Recent Research", research)
	case "planning":
		add("Planning Rules", planningRules)
		add("Known Risks", knownRisks)
		add("Findings", findings)
		add("Patterns", patterns)
		add("Learnings", learnings)
		add("Relevant Next Work", nextWork)
		add("Recent Research", research)
	case "pre-mortem":
		add("Known Risks", knownRisks)
		add("Planning Rules", planningRules)
		add("Findings", findings)
		add("Relevant Next Work", nextWork)
		add("Patterns", patterns)
	case "validation":
		add("Findings", findings)
		add("Known Risks", knownRisks)
		add("Learnings", learnings)
		add("Recent Sessions", sessions)
		add("Recent Research", research)
	default:
		add("Planning Rules", planningRules)
		add("Known Risks", knownRisks)
		add("Findings", findings)
		add("Learnings", learnings)
		add("Patterns", patterns)
		add("Relevant Next Work", nextWork)
		add("Recent Sessions", sessions)
		add("Recent Research", research)
		add("Legacy Signals", legacy)
	}

	return specs
}

func prioritizeFindings(findings []knowledgeFinding, preferredIDs []string, limit int) []knowledgeFinding {
	if len(findings) == 0 {
		return nil
	}

	ordered := make([]knowledgeFinding, 0, len(findings))
	seen := make(map[string]bool, len(findings))
	for _, id := range preferredIDs {
		for _, finding := range findings {
			if finding.ID != id || seen[finding.ID] {
				continue
			}
			ordered = append(ordered, finding)
			seen[finding.ID] = true
			break
		}
	}
	for _, finding := range findings {
		if seen[finding.ID] {
			continue
		}
		ordered = append(ordered, finding)
		seen[finding.ID] = true
	}
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}
	return ordered
}

func chooseRankedNextWork(packet StigmergicPacket, nextWork []nextWorkItem, limit int) []nextWorkItem {
	chosen := nextWork
	if len(packet.PriorFindings) > 0 {
		chosen = packet.PriorFindings
	}
	if limit > 0 && len(chosen) > limit {
		chosen = chosen[:limit]
	}
	return append([]nextWorkItem(nil), chosen...)
}

func collectLegacyIntelEntries(cwd, query string, limit int) []intelEntry {
	entries := collectIntelEntries(cwd)
	if len(entries) == 0 {
		return nil
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(queryLower)
	relevant := make([]intelEntry, 0, limit)
	other := make([]intelEntry, 0, limit)
	for _, entry := range entries {
		if intelEntryMatchesQuery(entry, queryLower, queryWords) {
			relevant = append(relevant, entry)
		} else {
			other = append(other, entry)
		}
	}

	legacy := append(relevant, other...)
	if limit > 0 && len(legacy) > limit {
		legacy = legacy[:limit]
	}
	return legacy
}

func intelEntryMatchesQuery(entry intelEntry, queryLower string, queryWords []string) bool {
	if queryLower == "" {
		return true
	}
	contentLower := strings.ToLower(entry.title + " " + entry.content)
	if strings.Contains(contentLower, queryLower) {
		return true
	}
	for _, word := range queryWords {
		if len(word) > 3 && strings.Contains(contentLower, word) {
			return true
		}
	}
	return false
}

func stringBullets(items []string) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		item = compactText(item)
		if item == "" {
			continue
		}
		bullets = append(bullets, truncateText(item, 220))
	}
	return bullets
}

func learningBullets(items []learning) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := firstNonEmpty(item.Title, item.ID)
		summary := compactText(firstNonEmpty(item.Summary, item.BodyText))
		bullets = append(bullets, joinBullet(title, summary))
	}
	return bullets
}

func patternBullets(items []pattern) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		name := firstNonEmpty(item.Name, filepath.Base(item.FilePath))
		description := compactText(item.Description)
		bullets = append(bullets, joinBullet(name, description))
	}
	return bullets
}

func findingBullets(items []knowledgeFinding) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := firstNonEmpty(item.Title, item.ID)
		summary := compactText(item.Summary)
		bullet := joinBullet(title, summary)
		if sev := strings.TrimSpace(item.Severity); sev != "" {
			bullet = fmt.Sprintf("[%s] %s", strings.ToUpper(sev), bullet)
		}
		bullets = append(bullets, bullet)
	}
	return bullets
}

func nextWorkBullets(items []nextWorkItem) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := firstNonEmpty(item.Title, item.Type)
		description := compactText(firstNonEmpty(item.Description, item.Evidence))
		if sev := strings.TrimSpace(item.Severity); sev != "" {
			title = fmt.Sprintf("[%s] %s", strings.ToUpper(sev), title)
		}
		bullets = append(bullets, joinBullet(title, description))
	}
	return bullets
}

func sessionBullets(items []session) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Date)
		summary := compactText(item.Summary)
		bullets = append(bullets, joinBullet(title, summary))
	}
	return bullets
}

func researchBullets(items []codexArtifactRef) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := firstNonEmpty(item.Title, strings.TrimSuffix(filepath.Base(item.Path), filepath.Ext(item.Path)))
		metadata := compactText(item.ModifiedAt)
		bullets = append(bullets, joinBullet(title, metadata))
	}
	return bullets
}

func legacyIntelBullets(items []intelEntry) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := fmt.Sprintf("%s (%s)", item.title, item.kind)
		description := compactText(item.content)
		bullets = append(bullets, joinBullet(title, description))
	}
	return bullets
}

func joinBullet(title, details string) string {
	title = compactText(title)
	details = compactText(details)
	switch {
	case title == "" && details == "":
		return ""
	case details == "":
		return truncateText(title, 220)
	case title == "":
		return truncateText(details, 220)
	default:
		return truncateText(title+" - "+details, 220)
	}
}

func compactText(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func limitLearnings(items []learning, limit int) []learning {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]learning(nil), items...)
}

func limitPatterns(items []pattern, limit int) []pattern {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]pattern(nil), items...)
}

func limitSessions(items []session, limit int) []session {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]session(nil), items...)
}

func limitResearchRefs(items []codexArtifactRef, limit int) []codexArtifactRef {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]codexArtifactRef(nil), items...)
}
