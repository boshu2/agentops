package main

import "strings"

type sessionIntelligenceArtifactPolicy struct {
	Class                string
	TrustTier            string
	StartupEligible      bool
	PlanningEligible     bool
	PreMortemEligible    bool
	PostMortemEligible   bool
	DefaultSuppression   bool
	SuppressionReason    string
	EligibilityRationale string
}

func sessionIntelligencePolicies() []sessionIntelligenceArtifactPolicy {
	return []sessionIntelligenceArtifactPolicy{
		{
			Class:                "discovery-notes",
			TrustTier:            "discovery-only",
			DefaultSuppression:   true,
			SuppressionReason:    "Discovery outputs stay out of default runtime injection until promoted into higher-trust artifacts.",
			EligibilityRationale: "Useful for research provenance, but too low-trust for default runtime context.",
			PostMortemEligible:   true,
		},
		{
			Class:                "pending-knowledge",
			TrustTier:            "discovery-only",
			DefaultSuppression:   true,
			SuppressionReason:    "Pending knowledge remains extraction-only until it is promoted or curated.",
			EligibilityRationale: "Raw extraction output is not stable enough for default runtime injection.",
			PostMortemEligible:   true,
		},
		{
			Class:                "raw-transcripts",
			TrustTier:            "archive-only",
			DefaultSuppression:   true,
			SuppressionReason:    "Raw transcripts remain lookup-only because they are noisy and too large for default runtime context.",
			EligibilityRationale: "Retained for provenance and recovery, not for automatic startup payloads.",
			PostMortemEligible:   true,
		},
		{
			Class:                "learning",
			TrustTier:            "runtime-eligible",
			StartupEligible:      true,
			PlanningEligible:     true,
			PreMortemEligible:    false,
			PostMortemEligible:   true,
			EligibilityRationale: "Ranked learnings can improve runtime decisions once they pass quality gates.",
		},
		{
			Class:                "pattern",
			TrustTier:            "runtime-eligible",
			StartupEligible:      true,
			PlanningEligible:     true,
			PreMortemEligible:    true,
			PostMortemEligible:   true,
			EligibilityRationale: "Patterns are compact and reusable when they match the current task.",
		},
		{
			Class:                "finding",
			TrustTier:            "canonical",
			StartupEligible:      true,
			PlanningEligible:     true,
			PreMortemEligible:    true,
			PostMortemEligible:   true,
			EligibilityRationale: "Promoted findings are the highest-trust reusable runtime signal.",
		},
		{
			Class:                "planning-rule",
			TrustTier:            "canonical",
			StartupEligible:      true,
			PlanningEligible:     true,
			PreMortemEligible:    true,
			PostMortemEligible:   true,
			EligibilityRationale: "Compiled planning rules are canonical prevention artifacts for future sessions.",
		},
		{
			Class:                "known-risk",
			TrustTier:            "canonical",
			StartupEligible:      true,
			PlanningEligible:     true,
			PreMortemEligible:    true,
			PostMortemEligible:   true,
			EligibilityRationale: "Compiled pre-mortem checks are canonical risk memory for future sessions.",
		},
		{
			Class:                "next-work",
			TrustTier:            "runtime-eligible",
			StartupEligible:      true,
			PlanningEligible:     true,
			PreMortemEligible:    true,
			PostMortemEligible:   true,
			EligibilityRationale: "Ranked next-work gives the next session actionable continuity.",
		},
		{
			Class:                "recent-session",
			TrustTier:            "runtime-eligible",
			StartupEligible:      true,
			PlanningEligible:     false,
			PreMortemEligible:    false,
			PostMortemEligible:   true,
			EligibilityRationale: "Recent session summaries help startup recovery when they match the current thread.",
		},
		{
			Class:                "research",
			TrustTier:            "runtime-eligible",
			StartupEligible:      true,
			PlanningEligible:     true,
			PreMortemEligible:    false,
			PostMortemEligible:   true,
			EligibilityRationale: "Research artifacts can help when they are recent and query-matched, but they do not outrank findings or rules.",
		},
		{
			Class:                "topic-packets",
			TrustTier:            "experimental",
			DefaultSuppression:   true,
			SuppressionReason:    "Topic packets remain experimental and require packet-health review before default startup injection.",
			EligibilityRationale: "Useful as optional lookup surfaces, but not stable enough for default runtime injection yet.",
			PostMortemEligible:   true,
		},
		{
			Class:                "source-manifests",
			TrustTier:            "experimental",
			DefaultSuppression:   true,
			SuppressionReason:    "Source manifests remain experimental and do not enter default runtime payloads.",
			EligibilityRationale: "Useful for provenance, but too verbose and low-signal for default startup context.",
			PostMortemEligible:   true,
		},
		{
			Class:                "promoted-packets",
			TrustTier:            "experimental",
			DefaultSuppression:   true,
			SuppressionReason:    "Promoted packets remain experimental until packet health and rollout metrics stabilize.",
			EligibilityRationale: "Potentially strong runtime artifacts, but still behind health gates in the current rollout.",
			PostMortemEligible:   true,
		},
	}
}

func sessionIntelligencePolicyFor(class string) (sessionIntelligenceArtifactPolicy, bool) {
	class = strings.TrimSpace(class)
	for _, item := range sessionIntelligencePolicies() {
		if item.Class == class {
			return item, true
		}
	}
	return sessionIntelligenceArtifactPolicy{}, false
}

func codexStartupExclusionBullets() []string {
	var bullets []string
	for _, item := range sessionIntelligencePolicies() {
		if !item.DefaultSuppression {
			continue
		}
		bullets = append(bullets, item.SuppressionReason)
	}
	return uniqueStringsPreserveOrder(bullets)
}
