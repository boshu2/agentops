// practices: [wiki-knowledge-surface, distributed-tracing]
package main

import (
	"os"
	"strings"

	"github.com/boshu2/agentops/cli/internal/types"
)

const primaryMetricNamespace = "primary"

func canonicalMetricNamespace(raw string) string {
	namespace := strings.ToLower(strings.TrimSpace(raw))
	if namespace == "" {
		return primaryMetricNamespace
	}
	return namespace
}

func defaultCitationMetricNamespace() string {
	return canonicalMetricNamespace(os.Getenv("AO_CITATION_NAMESPACE"))
}

func isPrimaryMetricNamespace(raw string) bool {
	return canonicalMetricNamespace(raw) == primaryMetricNamespace
}

func filterCitationsByMetricNamespace(citations []types.CitationEvent, namespace string) []types.CitationEvent {
	canonicalNamespace := canonicalMetricNamespace(namespace)
	filtered := make([]types.CitationEvent, 0, len(citations))
	for _, citation := range citations {
		citation.MetricNamespace = canonicalMetricNamespace(citation.MetricNamespace)
		if citation.MetricNamespace != canonicalNamespace {
			continue
		}
		filtered = append(filtered, citation)
	}
	return filtered
}

func citationFeedbackNamespaceKey(baseDir, artifactPath, namespace string) string {
	return canonicalArtifactKey(baseDir, artifactPath) + "|" + canonicalMetricNamespace(namespace)
}
