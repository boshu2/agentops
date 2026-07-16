// Package evidence owns append-only, generic observation records. These
// records never control validation, phase sequencing, delivery, or closure.
package evidence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

const CitationsFilePath = ".agents/ao/citations.jsonl"

func CanonicalArtifactPath(baseDir, artifactPath string) string {
	path := strings.TrimSpace(artifactPath)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		if strings.TrimSpace(baseDir) == "" {
			baseDir = "."
		}
		path = filepath.Join(baseDir, path)
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}

func CanonicalWorkspacePath(baseDir, workspacePath string) string {
	path := strings.TrimSpace(workspacePath)
	if path == "" {
		path = strings.TrimSpace(baseDir)
	}
	if path == "" {
		return ""
	}
	return CanonicalArtifactPath(baseDir, path)
}

func RecordCitation(baseDir string, event types.CitationEvent) error {
	if event.CitedAt.IsZero() {
		event.CitedAt = time.Now().UTC()
	}
	event.ArtifactPath = CanonicalArtifactPath(baseDir, event.ArtifactPath)
	event.WorkspacePath = CanonicalWorkspacePath(baseDir, event.WorkspacePath)
	path := filepath.Join(baseDir, CitationsFilePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create citation directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open citation ledger: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal citation: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append citation: %w", err)
	}
	return nil
}

func LoadCitations(baseDir string) ([]types.CitationEvent, error) {
	file, err := os.Open(filepath.Join(baseDir, CitationsFilePath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open citation ledger: %w", err)
	}
	defer func() { _ = file.Close() }()
	var citations []types.CitationEvent
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if event, ok := parseCitation(scanner.Bytes(), baseDir); ok {
			citations = append(citations, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return citations, fmt.Errorf("scan citation ledger: %w", err)
	}
	return citations, nil
}

func parseCitation(line []byte, baseDir string) (types.CitationEvent, bool) {
	var raw map[string]any
	if len(line) == 0 || json.Unmarshal(line, &raw) != nil {
		return types.CitationEvent{}, false
	}
	event := types.CitationEvent{
		ArtifactPath:       firstString(raw, "artifact_path", "learning_file"),
		WorkspacePath:      firstString(raw, "workspace_path", "workspace"),
		SessionID:          firstString(raw, "session_id", "session", "cited_by"),
		CitationType:       firstString(raw, "citation_type", "type"),
		ModelVendor:        firstString(raw, "model_vendor", "vendor"),
		ArtifactAuthorID:   firstString(raw, "artifact_author_id", "author_id", "artifact_author"),
		CitedByAgentID:     firstString(raw, "cited_by_agent_id", "cited_by_agent", "reviewer_id", "judge_id"),
		CitedByModelFamily: firstString(raw, "cited_by_model_family", "model_family", "reviewer_model_family"),
		Query:              firstString(raw, "query"),
		MetricNamespace:    firstString(raw, "metric_namespace"),
		MatchConfidence:    number(raw, "match_confidence"),
		MatchProvenance:    firstString(raw, "match_provenance"),
		SectionHeading:     firstString(raw, "section_heading"),
		SectionLocator:     firstString(raw, "section_locator"),
		FeedbackGiven:      boolean(raw, "feedback_given"),
		FeedbackReward:     number(raw, "feedback_reward"),
		UtilityBefore:      number(raw, "utility_before"),
		UtilityAfter:       number(raw, "utility_after"),
	}
	if value, ok := timestamp(raw, "cited_at", "timestamp"); ok {
		event.CitedAt = value
	}
	if value, ok := timestamp(raw, "feedback_at"); ok {
		event.FeedbackAt = value
	}
	event.ArtifactPath = CanonicalArtifactPath(baseDir, event.ArtifactPath)
	event.WorkspacePath = CanonicalWorkspacePath(baseDir, event.WorkspacePath)
	return event, true
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func number(raw map[string]any, key string) float64 {
	value, _ := raw[key].(float64)
	return value
}

func boolean(raw map[string]any, key string) bool {
	value, _ := raw[key].(bool)
	return value
}

func timestamp(raw map[string]any, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		value := firstString(raw, key)
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}
