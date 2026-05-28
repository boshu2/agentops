// practices: [ai-assisted-dev, distributed-tracing]
package main

import aocontext "github.com/boshu2/agentops/cli/internal/context"

// Type alias — canonical type lives in internal/context.
type sessionIntelligenceArtifactPolicy = aocontext.SessionIntelligenceArtifactPolicy

// Thin wrappers — delegate to context package, kept for test compatibility.
func sessionIntelligencePolicies() []sessionIntelligenceArtifactPolicy {
	return aocontext.SessionIntelligencePolicies()
}

func sessionIntelligencePolicyFor(class string) (sessionIntelligenceArtifactPolicy, bool) {
	return aocontext.SessionIntelligencePolicyFor(class)
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
