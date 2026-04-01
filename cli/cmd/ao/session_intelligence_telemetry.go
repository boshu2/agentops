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
	return event
}

func loadCitationAggregate(baseDir string) citationAggregate {
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		return citationAggregate{ByArtifact: make(map[string]artifactUsageSignal)}
	}
	return buildCitationAggregate(baseDir, citations)
}

func buildCitationAggregate(baseDir string, citations []types.CitationEvent) citationAggregate {
	agg := citationAggregate{
		TotalEvents: len(citations),
		ByArtifact:  make(map[string]artifactUsageSignal),
	}

	type dedupeRecord struct {
		artifactKey string
		event       types.CitationEvent
	}

	deduped := make(map[string]dedupeRecord)
	uniqueSessions := make(map[string]bool)
	uniqueWorkspaces := make(map[string]bool)

	for _, raw := range citations {
		event := normalizeCitationEventForRuntime(baseDir, raw)
		artifactKey := canonicalArtifactKey(baseDir, event.ArtifactPath)
		if artifactKey == "" {
			continue
		}

		if event.SessionID != "" {
			uniqueSessions[event.SessionID] = true
		}
		if event.WorkspacePath != "" {
			uniqueWorkspaces[event.WorkspacePath] = true
		}

		dedupeKey := artifactKey + "|" + event.SessionID + "|" + filepath.ToSlash(event.WorkspacePath) + "|" + event.CitationType
		current, ok := deduped[dedupeKey]
		if !ok || event.CitedAt.After(current.event.CitedAt) {
			deduped[dedupeKey] = dedupeRecord{artifactKey: artifactKey, event: event}
		}
	}

	for _, record := range deduped {
		signal := agg.ByArtifact[record.artifactKey]
		signal.ArtifactPath = record.artifactKey
		signal.Citations++
		if record.event.CitedAt.After(signal.LastCited) {
			signal.LastCited = record.event.CitedAt
		}
		switch record.event.CitationType {
		case "applied":
			signal.AppliedCount++
		case "reference":
			signal.ReferenceCount++
		default:
			signal.RetrievedCount++
		}
		if record.event.FeedbackGiven {
			signal.FeedbackCount++
			signal.MeanReward += record.event.FeedbackReward
		}
		agg.ByArtifact[record.artifactKey] = signal
	}

	for key, signal := range agg.ByArtifact {
		sessionSet := make(map[string]bool)
		workspaceSet := make(map[string]bool)
		for _, record := range deduped {
			if record.artifactKey != key {
				continue
			}
			if record.event.SessionID != "" {
				sessionSet[record.event.SessionID] = true
			}
			if record.event.WorkspacePath != "" {
				workspaceSet[record.event.WorkspacePath] = true
			}
		}
		signal.UniqueSessions = len(sessionSet)
		signal.UniqueWorkspaces = len(workspaceSet)
		for sessionID := range sessionSet {
			signal.sessionKeys = append(signal.sessionKeys, sessionID)
		}
		for workspacePath := range workspaceSet {
			signal.workspaceKeys = append(signal.workspaceKeys, workspacePath)
		}
		if signal.FeedbackCount > 0 {
			signal.MeanReward /= float64(signal.FeedbackCount)
		}
		agg.ByArtifact[key] = signal
	}

	agg.DedupedEvents = len(deduped)
	agg.UniqueArtifacts = len(agg.ByArtifact)
	agg.UniqueSessions = len(uniqueSessions)
	agg.UniqueWorkspaces = len(uniqueWorkspaces)
	return agg
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
