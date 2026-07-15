package search

import (
	"fmt"
	"time"

	ratchet "github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ArtifactCanonicalizer converts a raw source path into the form stored in citation events.
type ArtifactCanonicalizer func(baseDir, source string) string

// RecordLearningCitations writes retrieved-citation events for each learning.
// canonicalSession and canonicalNamespace must already be canonicalized by the caller.
func RecordLearningCitations(baseDir string, learnings []Learning, canonicalSession, query, canonicalNamespace string, canonicalizeArtifact ArtifactCanonicalizer) error {
	for _, l := range learnings {
		event := types.CitationEvent{
			ArtifactPath:    canonicalizeArtifact(baseDir, l.Source),
			SessionID:       canonicalSession,
			CitedAt:         time.Now(),
			CitationType:    "retrieved",
			Query:           query,
			MetricNamespace: canonicalNamespace,
			MatchConfidence: l.MatchConfidence,
			MatchProvenance: l.MatchProvenance,
			SectionHeading:  l.SectionHeading,
			SectionLocator:  l.SectionLocator,
		}
		if err := ratchet.RecordCitation(baseDir, event); err != nil {
			return fmt.Errorf("record citation for %s: %w", l.ID, err)
		}
	}
	return nil
}

// RecordPatternCitations writes retrieved-citation events for each pattern with a non-empty FilePath.
func RecordPatternCitations(baseDir string, patterns []Pattern, canonicalSession, query, canonicalNamespace string, canonicalizeArtifact ArtifactCanonicalizer) error {
	for _, p := range patterns {
		if p.FilePath == "" {
			continue
		}
		event := types.CitationEvent{
			ArtifactPath:    canonicalizeArtifact(baseDir, p.FilePath),
			SessionID:       canonicalSession,
			CitedAt:         time.Now(),
			CitationType:    "retrieved",
			Query:           query,
			MetricNamespace: canonicalNamespace,
		}
		if err := ratchet.RecordCitation(baseDir, event); err != nil {
			return fmt.Errorf("record citation for pattern %s: %w", p.Name, err)
		}
	}
	return nil
}
