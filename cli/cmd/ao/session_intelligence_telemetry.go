package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

type artifactUsageSignal struct {
	ArtifactPath     string    `json:"artifact_path,omitempty"`
	Citations        int       `json:"citations"`
	UniqueSessions   int       `json:"unique_sessions"`
	UniqueWorkspaces int       `json:"unique_workspaces"`
	RetrievedCount   int       `json:"retrieved_count"`
	ReferenceCount   int       `json:"reference_count"`
	AppliedCount     int       `json:"applied_count"`
	FeedbackCount    int       `json:"feedback_count"`
	MeanReward       float64   `json:"mean_reward"`
	LastCited        time.Time `json:"last_cited,omitempty"`
	sessionKeys      []string
	workspaceKeys    []string
}

type citationAggregate struct {
	TotalEvents      int                            `json:"total_events"`
	DedupedEvents    int                            `json:"deduped_events"`
	UniqueArtifacts  int                            `json:"unique_artifacts"`
	UniqueSessions   int                            `json:"unique_sessions"`
	UniqueWorkspaces int                            `json:"unique_workspaces"`
	ByArtifact       map[string]artifactUsageSignal `json:"by_artifact,omitempty"`
}

func canonicalWorkspacePath(baseDir, workspacePath string) string {
	return ratchet.CanonicalWorkspacePath(baseDir, workspacePath)
}

func normalizeCitationEventForRuntime(baseDir string, event types.CitationEvent) types.CitationEvent {
	event.ArtifactPath = canonicalArtifactPath(baseDir, event.ArtifactPath)
	event.WorkspacePath = canonicalWorkspacePath(baseDir, event.WorkspacePath)
	event.SessionID = canonicalSessionID(event.SessionID)
	event.CitationType = canonicalCitationType(event.CitationType)
	event.MetricNamespace = canonicalMetricNamespace(event.MetricNamespace)
	return event
}

func loadCitationAggregate(baseDir string) citationAggregate {
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		return citationAggregate{ByArtifact: make(map[string]artifactUsageSignal)}
	}
	return buildCitationAggregate(baseDir, citations)
}

type dedupedCitationEvent struct {
	artifactKey string
	event       types.CitationEvent
}

type citationAggregateBuilder struct {
	agg              citationAggregate
	deduped          map[string]dedupedCitationEvent
	uniqueSessions   map[string]struct{}
	uniqueWorkspaces map[string]struct{}
}

type artifactSignalBuilder struct {
	signal        artifactUsageSignal
	sessionKeys   map[string]struct{}
	workspaceKeys map[string]struct{}
}

func newCitationAggregateBuilder(totalEvents int) *citationAggregateBuilder {
	return &citationAggregateBuilder{
		agg: citationAggregate{
			TotalEvents: totalEvents,
			ByArtifact:  make(map[string]artifactUsageSignal),
		},
		deduped:          make(map[string]dedupedCitationEvent),
		uniqueSessions:   make(map[string]struct{}),
		uniqueWorkspaces: make(map[string]struct{}),
	}
}

func buildCitationAggregate(baseDir string, citations []types.CitationEvent) citationAggregate {
	builder := newCitationAggregateBuilder(len(citations))
	for _, raw := range citations {
		builder.ingest(baseDir, raw)
	}

	return builder.finish()
}

func (b *citationAggregateBuilder) ingest(baseDir string, raw types.CitationEvent) {
	event := normalizeCitationEventForRuntime(baseDir, raw)
	artifactKey := canonicalArtifactKey(baseDir, event.ArtifactPath)
	if artifactKey == "" {
		return
	}

	b.recordUniqueSignals(event)
	b.recordDedupedEvent(artifactKey, event)
}

func (b *citationAggregateBuilder) recordUniqueSignals(event types.CitationEvent) {
	if event.SessionID != "" {
		b.uniqueSessions[event.SessionID] = struct{}{}
	}
	if event.WorkspacePath != "" {
		b.uniqueWorkspaces[event.WorkspacePath] = struct{}{}
	}
}

func (b *citationAggregateBuilder) recordDedupedEvent(artifactKey string, event types.CitationEvent) {
	dedupeKey := artifactKey + "|" + event.SessionID + "|" + filepath.ToSlash(event.WorkspacePath) + "|" + event.CitationType
	current, ok := b.deduped[dedupeKey]
	if !ok || event.CitedAt.After(current.event.CitedAt) {
		b.deduped[dedupeKey] = dedupedCitationEvent{artifactKey: artifactKey, event: event}
	}
}

func (b *citationAggregateBuilder) finish() citationAggregate {
	artifactSignals := make(map[string]*artifactSignalBuilder)
	for _, record := range b.deduped {
		builder := artifactSignals[record.artifactKey]
		if builder == nil {
			builder = &artifactSignalBuilder{}
			artifactSignals[record.artifactKey] = builder
		}
		builder.accumulate(record.event)
	}

	for key, builder := range artifactSignals {
		b.agg.ByArtifact[key] = builder.finalize(key)
	}

	b.agg.DedupedEvents = len(b.deduped)
	b.agg.UniqueArtifacts = len(b.agg.ByArtifact)
	b.agg.UniqueSessions = len(b.uniqueSessions)
	b.agg.UniqueWorkspaces = len(b.uniqueWorkspaces)
	return b.agg
}

func (b *artifactSignalBuilder) accumulate(event types.CitationEvent) {
	b.signal.Citations++
	if event.CitedAt.After(b.signal.LastCited) {
		b.signal.LastCited = event.CitedAt
	}
	switch event.CitationType {
	case "applied":
		b.signal.AppliedCount++
	case "reference":
		b.signal.ReferenceCount++
	default:
		b.signal.RetrievedCount++
	}
	if event.FeedbackGiven {
		b.signal.FeedbackCount++
		b.signal.MeanReward += event.FeedbackReward
	}
	if event.SessionID != "" {
		if b.sessionKeys == nil {
			b.sessionKeys = make(map[string]struct{})
		}
		b.sessionKeys[event.SessionID] = struct{}{}
	}
	if event.WorkspacePath != "" {
		if b.workspaceKeys == nil {
			b.workspaceKeys = make(map[string]struct{})
		}
		b.workspaceKeys[event.WorkspacePath] = struct{}{}
	}
}

func (b *artifactSignalBuilder) finalize(artifactKey string) artifactUsageSignal {
	b.signal.ArtifactPath = artifactKey
	b.signal.UniqueSessions = len(b.sessionKeys)
	b.signal.UniqueWorkspaces = len(b.workspaceKeys)
	for sessionID := range b.sessionKeys {
		b.signal.sessionKeys = append(b.signal.sessionKeys, sessionID)
	}
	for workspacePath := range b.workspaceKeys {
		b.signal.workspaceKeys = append(b.signal.workspaceKeys, workspacePath)
	}
	if b.signal.FeedbackCount > 0 {
		b.signal.MeanReward /= float64(b.signal.FeedbackCount)
	}
	return b.signal
}

func usageSignalForArtifact(baseDir, artifactPath string, agg citationAggregate) artifactUsageSignal {
	return agg.ByArtifact[canonicalArtifactKey(baseDir, artifactPath)]
}

func workspacePathFromAgentArtifactPath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	parts := strings.SplitN(clean, "/.agents/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return ""
	}
	return filepath.Clean(parts[0])
}

func annotateCitationMatch(event types.CitationEvent, confidence float64, provenance string) types.CitationEvent {
	event.MatchConfidence = normalizeCitationMatchConfidence(confidence)
	event.MatchProvenance = strings.TrimSpace(provenance)
	return event
}

const matchConfidenceHighThreshold = 0.7

func normalizeCitationMatchConfidence(confidence float64) float64 {
	switch confidence {
	case 0, 0.5, 0.7, 0.9:
		return confidence
	}
	switch {
	case confidence >= matchConfidenceHighThreshold:
		return 0.9
	case confidence >= 0.5:
		return 0.7
	case confidence > 0:
		return 0.5
	default:
		return 0
	}
}
