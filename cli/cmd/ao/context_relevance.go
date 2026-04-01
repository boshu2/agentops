package main

import (
	"cmp"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var runtimeSessionSearchFn = searchUpstreamCASS

type runtimeRelevanceContext struct {
	cwd           string
	repo          string
	phase         string
	needles       []string
	citations     citationAggregate
	cassHitByPath map[string]float64
	scorecard     stigmergicScorecard
}

func rerankContextBundleForPhase(cwd, query, phase string, bundle rankedContextBundle) rankedContextBundle {
	ctx := runtimeRelevanceContext{
		cwd:           cwd,
		repo:          detectRepoName(cwd),
		phase:         normalizeAssemblePhase(phase),
		needles:       tokenizeStigmergicText(query),
		citations:     loadCitationAggregate(cwd),
		cassHitByPath: runtimeCassHitScores(cwd, query, 8),
		scorecard:     bundle.Packet.Scorecard,
	}

	ranked := bundle
	ranked.Learnings = rankBundleLearnings(ctx, bundle.Learnings)
	ranked.Patterns = rankBundlePatterns(ctx, bundle.Patterns)
	ranked.Findings = rankBundleFindings(ctx, bundle.Findings, bundle.Packet.AppliedFindings)
	ranked.RecentSessions = rankBundleSessions(ctx, bundle.RecentSessions)
	ranked.NextWork = rankBundleNextWork(ctx, bundle.NextWork)
	ranked.Research = rankBundleResearch(ctx, bundle.Research)
	ranked.LegacyIntel = rankBundleLegacyIntel(ctx, bundle.LegacyIntel)
	return ranked
}

func runtimeCassHitScores(cwd, query string, limit int) map[string]float64 {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil
	}
	results, err := runtimeSessionSearchFn(query, limit)
	if err != nil {
		return nil
	}
	hits := make(map[string]float64, len(results))
	for _, result := range results {
		hits[canonicalArtifactKey(cwd, result.Path)] = result.Score
	}
	return hits
}

func rankBundleLearnings(ctx runtimeRelevanceContext, items []learning) []learning {
	ranked := append([]learning(nil), items...)
	slices.SortFunc(ranked, func(a, b learning) int {
		if diff := cmp.Compare(scoreLearningForRuntime(ctx, b), scoreLearningForRuntime(ctx, a)); diff != 0 {
			return diff
		}
		return cmp.Compare(b.CompositeScore, a.CompositeScore)
	})
	return ranked
}

func rankBundlePatterns(ctx runtimeRelevanceContext, items []pattern) []pattern {
	ranked := append([]pattern(nil), items...)
	slices.SortFunc(ranked, func(a, b pattern) int {
		if diff := cmp.Compare(scorePatternForRuntime(ctx, b), scorePatternForRuntime(ctx, a)); diff != 0 {
			return diff
		}
		return cmp.Compare(b.CompositeScore, a.CompositeScore)
	})
	return ranked
}

func rankBundleFindings(ctx runtimeRelevanceContext, items []knowledgeFinding, appliedIDs []string) []knowledgeFinding {
	ranked := append([]knowledgeFinding(nil), items...)
	slices.SortFunc(ranked, func(a, b knowledgeFinding) int {
		if diff := cmp.Compare(scoreFindingForRuntime(ctx, b, appliedIDs), scoreFindingForRuntime(ctx, a, appliedIDs)); diff != 0 {
			return diff
		}
		return cmp.Compare(b.CompositeScore, a.CompositeScore)
	})
	return ranked
}

func rankBundleSessions(ctx runtimeRelevanceContext, items []session) []session {
	ranked := append([]session(nil), items...)
	slices.SortFunc(ranked, func(a, b session) int {
		return cmp.Compare(scoreSessionForRuntime(ctx, b), scoreSessionForRuntime(ctx, a))
	})
	return ranked
}

func rankBundleNextWork(ctx runtimeRelevanceContext, items []nextWorkItem) []nextWorkItem {
	ranked := append([]nextWorkItem(nil), items...)
	slices.SortFunc(ranked, func(a, b nextWorkItem) int {
		return cmp.Compare(scoreNextWorkForRuntime(ctx, b), scoreNextWorkForRuntime(ctx, a))
	})
	return ranked
}

func rankBundleResearch(ctx runtimeRelevanceContext, items []codexArtifactRef) []codexArtifactRef {
	ranked := append([]codexArtifactRef(nil), items...)
	slices.SortFunc(ranked, func(a, b codexArtifactRef) int {
		return cmp.Compare(scoreResearchForRuntime(ctx, b), scoreResearchForRuntime(ctx, a))
	})
	return ranked
}

func rankBundleLegacyIntel(ctx runtimeRelevanceContext, items []intelEntry) []intelEntry {
	ranked := append([]intelEntry(nil), items...)
	slices.SortFunc(ranked, func(a, b intelEntry) int {
		return cmp.Compare(scoreLegacyIntelForRuntime(ctx, b), scoreLegacyIntelForRuntime(ctx, a))
	})
	return ranked
}

func scoreLearningForRuntime(ctx runtimeRelevanceContext, item learning) int {
	signal := usageSignalForArtifact(ctx.cwd, item.Source, ctx.citations)
	return trustTierWeight("learning") +
		phaseFitWeight("learning", ctx.phase) +
		lexicalSignalWeight(ctx.needles, item.ID, item.Title, item.Summary, item.BodyText) +
		repoPathWeight(ctx.cwd, item.Source) +
		freshnessWeight(item.AgeWeeks) +
		compositeWeight(item.CompositeScore) +
		usageSignalWeight(signal)
}

func scorePatternForRuntime(ctx runtimeRelevanceContext, item pattern) int {
	signal := usageSignalForArtifact(ctx.cwd, item.FilePath, ctx.citations)
	return trustTierWeight("pattern") +
		phaseFitWeight("pattern", ctx.phase) +
		lexicalSignalWeight(ctx.needles, item.Name, item.Description) +
		repoPathWeight(ctx.cwd, item.FilePath) +
		freshnessWeight(item.AgeWeeks) +
		compositeWeight(item.CompositeScore) +
		usageSignalWeight(signal)
}

func scoreFindingForRuntime(ctx runtimeRelevanceContext, item knowledgeFinding, appliedIDs []string) int {
	score := trustTierWeight("finding") +
		phaseFitWeight("finding", ctx.phase) +
		lexicalSignalWeight(ctx.needles, item.ID, item.Title, item.Summary, item.SourceSkill, strings.Join(item.ScopeTags, " "), strings.Join(item.ApplicableWhen, " ")) +
		repoPathWeight(ctx.cwd, item.Source) +
		freshnessWeight(item.AgeWeeks) +
		compositeWeight(item.CompositeScore) +
		usageSignalWeight(usageSignalForArtifact(ctx.cwd, item.Source, ctx.citations))

	if stringSliceContainsFold(appliedIDs, item.ID) {
		score += 8
	}
	if findingStatusActiveForRetrieval(item.Status) {
		score += 2
	}
	if ctx.scorecard.PromotedFindings >= 3 {
		score += 2
	}
	return score
}

func scoreSessionForRuntime(ctx runtimeRelevanceContext, item session) int {
	score := trustTierWeight("recent-session") +
		phaseFitWeight("recent-session", ctx.phase) +
		lexicalSignalWeight(ctx.needles, item.Summary)
	score += cassHitWeight(ctx.cassHitByPath, item.Path)
	score += repoPathWeight(ctx.cwd, item.Path)
	return score
}

func scoreNextWorkForRuntime(ctx runtimeRelevanceContext, item nextWorkItem) int {
	score := trustTierWeight("next-work") +
		phaseFitWeight("next-work", ctx.phase) +
		lexicalSignalWeight(ctx.needles, item.Title, item.Description, item.Evidence, item.Source)
	score += repoAffinityRank(item, ctx.repo) * 4
	score += severityRank(item.Severity)
	if strings.EqualFold(strings.TrimSpace(item.ClaimStatus), "available") {
		score++
	}
	return score
}

func scoreResearchForRuntime(ctx runtimeRelevanceContext, item codexArtifactRef) int {
	score := trustTierWeight("research") +
		phaseFitWeight("research", ctx.phase) +
		lexicalSignalWeight(ctx.needles, item.Title, item.Path)
	score += repoPathWeight(ctx.cwd, item.Path)
	score += cassHitWeight(ctx.cassHitByPath, item.Path)
	score += modTimeWeight(item.ModifiedAt)
	return score
}

func scoreLegacyIntelForRuntime(ctx runtimeRelevanceContext, item intelEntry) int {
	return trustTierWeight("discovery-notes") +
		lexicalSignalWeight(ctx.needles, item.title, item.content, item.sourcePath) +
		repoPathWeight(ctx.cwd, item.sourcePath)
}

func trustTierWeight(class string) int {
	policy, ok := sessionIntelligencePolicyFor(class)
	if !ok {
		return 0
	}
	switch policy.TrustTier {
	case "canonical":
		return 18
	case "runtime-eligible":
		return 10
	case "experimental":
		return -4
	case "discovery-only":
		return -8
	case "archive-only":
		return -10
	default:
		return 0
	}
}

func phaseFitWeight(class, phase string) int {
	policy, ok := sessionIntelligencePolicyFor(class)
	if !ok {
		return 0
	}
	switch normalizeAssemblePhase(phase) {
	case "startup":
		if policy.StartupEligible {
			return 6
		}
	case "planning":
		if policy.PlanningEligible {
			return 6
		}
	case "pre-mortem":
		if policy.PreMortemEligible {
			return 6
		}
	case "validation":
		if policy.PostMortemEligible {
			return 6
		}
	default:
		if policy.StartupEligible || policy.PlanningEligible || policy.PreMortemEligible || policy.PostMortemEligible {
			return 4
		}
	}
	return -2
}

func lexicalSignalWeight(needles []string, fields ...string) int {
	if len(needles) == 0 {
		return 0
	}
	return overlapScore(strings.Join(fields, " "), needles) * 2
}

func repoPathWeight(cwd, path string) int {
	if path == "" {
		return 0
	}
	clean := canonicalArtifactPath(cwd, path)
	root := canonicalWorkspacePath(cwd, cwd)
	if strings.HasPrefix(filepath.ToSlash(clean), filepath.ToSlash(root)+"/") {
		return 3
	}
	return 0
}

func freshnessWeight(ageWeeks float64) int {
	switch {
	case ageWeeks <= 1:
		return 4
	case ageWeeks <= 4:
		return 3
	case ageWeeks <= 12:
		return 1
	default:
		return 0
	}
}

func compositeWeight(score float64) int {
	return int(math.Round(score * 6))
}

func usageSignalWeight(signal artifactUsageSignal) int {
	score := minInt(signal.UniqueSessions, 4)*2 + minInt(signal.UniqueWorkspaces, 3)*2
	score += minInt(signal.AppliedCount, 2) * 2
	score += minInt(signal.ReferenceCount, 2)
	if signal.FeedbackCount > 0 {
		score += int(math.Round(signal.MeanReward * 3))
	}
	return score
}

func cassHitWeight(hitByPath map[string]float64, path string) int {
	if len(hitByPath) == 0 || path == "" {
		return 0
	}
	return int(math.Round(hitByPath[canonicalArtifactKey("", path)] * 4))
}

func modTimeWeight(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0
	}
	ageDays := time.Since(parsed).Hours() / 24
	switch {
	case ageDays <= 7:
		return 3
	case ageDays <= 30:
		return 1
	default:
		return 0
	}
}
