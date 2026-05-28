// practices: [ai-assisted-dev, distributed-tracing]
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	aocontext "github.com/boshu2/agentops/cli/internal/context"
)

type rankedContextBundle struct {
	CWD            string
	Query          string
	Packet         StigmergicPacket
	Beliefs        []string
	Playbooks      []knowledgeContextPlaybook
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
	return aocontext.NormalizeAssemblePhase(phase)
}

func contextBundleLimit(budget int) int {
	return aocontext.ContextBundleLimit(budget)
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
	nextWorkEntries, _ := loadVisibleNextWorkEntries(cwd, repo)
	nextWork := flattenNextWorkEntries(nextWorkEntries)

	return rankedContextBundle{
		CWD:            cwd,
		Query:          query,
		Packet:         packet,
		Beliefs:        loadKnowledgeBeliefsForContext(cwd, query, limit),
		Playbooks:      loadKnowledgePlaybooksForContext(cwd, query, limit),
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
		CWD:            cwd,
		Query:          query,
		Packet:         packet,
		Beliefs:        loadKnowledgeBeliefsForContext(cwd, query, limit),
		Playbooks:      loadKnowledgePlaybooksForContext(cwd, query, limit),
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
	bundle = rerankContextBundleForPhase(bundle.CWD, bundle.Query, phase, bundle)
	var sb strings.Builder
	for _, section := range rankedIntelSections(bundle, phase) {
		if len(section.Bullets) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "### %s\n", section.Title)
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
	beliefs := beliefBullets(bundle.Beliefs)
	playbooks := playbookBullets(bundle.CWD, bundle.Playbooks)
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
		add("Operating Beliefs", beliefs)
		add("Relevant Playbooks", playbooks)
		add("Relevant Next Work", nextWork)
		add("Findings", findings)
		add("Learnings", learnings)
		add("Patterns", patterns)
		add("Recent Sessions", sessions)
		add("Recent Research", research)
	case "planning":
		add("Planning Rules", planningRules)
		add("Known Risks", knownRisks)
		add("Operating Beliefs", beliefs)
		add("Relevant Playbooks", playbooks)
		add("Findings", findings)
		add("Patterns", patterns)
		add("Learnings", learnings)
		add("Relevant Next Work", nextWork)
		add("Recent Research", research)
	case "pre-mortem":
		add("Known Risks", knownRisks)
		add("Planning Rules", planningRules)
		add("Operating Beliefs", beliefs)
		add("Relevant Playbooks", playbooks)
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
		add("Operating Beliefs", beliefs)
		add("Relevant Playbooks", playbooks)
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

func beliefBullets(beliefs []string) []string {
	return append([]string(nil), beliefs...)
}

func playbookBullets(cwd string, playbooks []knowledgeContextPlaybook) []string {
	bullets := make([]string, 0, len(playbooks))
	for _, playbook := range playbooks {
		summary := strings.TrimSpace(playbook.Summary)
		if summary == "" {
			summary = "Use the generated operator loop for this topic."
		}
		bullets = append(bullets, fmt.Sprintf("%s: %s (`%s`)", playbook.Title, summary, displayKnowledgeContextPath(cwd, playbook.Path)))
	}
	return bullets
}

func prioritizeFindings(findings []knowledgeFinding, preferredIDs []string, limit int) []knowledgeFinding {
	return aocontext.PrioritizeFindings(findings, preferredIDs, limit)
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
	return aocontext.StringBullets(items)
}

func learningBullets(items []learning) []string {
	return aocontext.LearningBullets(items)
}

func patternBullets(items []pattern) []string {
	return aocontext.PatternBullets(items)
}

func findingBullets(items []knowledgeFinding) []string {
	return aocontext.FindingBullets(items)
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
	return aocontext.SessionBullets(items)
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
	return aocontext.JoinBullet(title, details)
}

func compactText(input string) string {
	return aocontext.CompactText(input)
}

func firstNonEmpty(values ...string) string {
	return aocontext.FirstNonEmpty(values...)
}

func limitLearnings(items []learning, limit int) []learning {
	return aocontext.LimitLearnings(items, limit)
}

func limitPatterns(items []pattern, limit int) []pattern {
	return aocontext.LimitPatterns(items, limit)
}

func limitSessions(items []session, limit int) []session {
	return aocontext.LimitSessions(items, limit)
}

func limitResearchRefs(items []codexArtifactRef, limit int) []codexArtifactRef {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]codexArtifactRef(nil), items...)
}
