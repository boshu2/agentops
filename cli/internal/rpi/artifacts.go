package rpi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PhaseArtifactNumberPattern matches phase-N in artifact filenames.
var PhaseArtifactNumberPattern = regexp.MustCompile(`phase-(\d+)`)

// GasCityPhaseEvidenceFileFmt is the filename pattern for per-phase GasCity
// session evidence captured after a terminal provider event.
const GasCityPhaseEvidenceFileFmt = "phase-%d-gascity-evidence.json"

// ArtifactRef is a reference to an RPI artifact on disk.
type ArtifactRef struct {
	Path      string `json:"path"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Phase     int    `json:"phase,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// ArtifactContent holds the body of a read artifact.
type ArtifactContent struct {
	Path        string `json:"path"`
	Label       string `json:"label,omitempty"`
	Kind        string `json:"kind,omitempty"`
	ContentType string `json:"content_type"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	Body        string `json:"body"`
	Truncated   bool   `json:"truncated,omitempty"`
}

// GasCityPhaseEvidence records provider correlation and transcript metadata
// for a GasCity-backed RPI phase. The transcript body may remain in GasCity;
// this file is the durable AgentOps registry projection.
type GasCityPhaseEvidence struct {
	SchemaVersion        int                         `json:"schema_version"`
	RunID                string                      `json:"run_id"`
	Phase                int                         `json:"phase"`
	PhaseName            string                      `json:"phase_name,omitempty"`
	CityName             string                      `json:"city_name"`
	SessionID            string                      `json:"session_id"`
	SessionAlias         string                      `json:"session_alias,omitempty"`
	Status               string                      `json:"status"`
	EventCursor          string                      `json:"event_cursor,omitempty"`
	RequestIDs           map[string]string           `json:"request_ids,omitempty"`
	TranscriptID         string                      `json:"transcript_id,omitempty"`
	TranscriptFormat     string                      `json:"transcript_format,omitempty"`
	TranscriptTurnCount  int                         `json:"transcript_turn_count,omitempty"`
	TranscriptMsgCount   int                         `json:"transcript_message_count,omitempty"`
	TranscriptArtifacts  []GasCityTranscriptArtifact `json:"transcript_artifacts,omitempty"`
	TranscriptCapturedAt string                      `json:"transcript_captured_at,omitempty"`
	RecordedAt           string                      `json:"recorded_at"`
}

// GasCityTranscriptArtifact is one artifact reference returned by GasCity's
// transcript/result evidence endpoint.
type GasCityTranscriptArtifact struct {
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"`
}

// PathClean normalises a relative path to forward-slash form.
func PathClean(rel string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel))))
}

// IsSafeArtifactRelPath returns true when rel is a safe relative artifact path.
func IsSafeArtifactRelPath(rel string) bool {
	rel = PathClean(rel)
	if rel == "." || rel == "" {
		return false
	}
	if strings.HasPrefix(rel, "../") || rel == ".." || strings.HasPrefix(rel, "/") {
		return false
	}
	return true
}

// GasCityPhaseEvidencePath returns the registry path for one phase's GasCity
// evidence. When a run ID is available, evidence is stored under the per-run
// registry; otherwise it falls back to the legacy .agents/rpi directory.
func GasCityPhaseEvidencePath(cwd, runID string, phase int) string {
	file := fmt.Sprintf(GasCityPhaseEvidenceFileFmt, phase)
	if runDir := RPIRunRegistryDir(cwd, runID); runDir != "" {
		return filepath.Join(runDir, file)
	}
	return filepath.Join(cwd, ".agents", "rpi", file)
}

// WriteGasCityPhaseEvidence writes GasCity phase evidence atomically and returns
// the absolute path that was written.
func WriteGasCityPhaseEvidence(cwd string, evidence GasCityPhaseEvidence) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return "", fmt.Errorf("cwd is required")
	}
	if evidence.Phase <= 0 {
		return "", fmt.Errorf("phase must be positive")
	}
	if evidence.SchemaVersion == 0 {
		evidence.SchemaVersion = 1
	}
	if evidence.RecordedAt == "" {
		evidence.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	path := GasCityPhaseEvidencePath(cwd, evidence.RunID, evidence.Phase)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("create evidence dir: %w", err)
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal gascity phase evidence: %w", err)
	}
	data = append(data, '\n')
	if err := WritePhasedStateAtomic(path, data); err != nil {
		return "", fmt.Errorf("write gascity phase evidence: %w", err)
	}
	return path, nil
}

// ClassifyRPIArtifact returns (kind, label, phase) for a relative artifact path.
// phasedStateFile and c2EventsFileName are passed in to avoid coupling to cmd/ao constants.
func ClassifyRPIArtifact(rel, phasedStateFile, c2EventsFileName string) (kind, label string, phase int) {
	base := filepath.Base(rel)
	phase = ArtifactPhaseNumber(base)

	switch {
	case strings.HasSuffix(rel, "execution-packet.json"):
		return "execution_packet", "Execution packet", 0
	case strings.HasSuffix(rel, filepath.ToSlash(filepath.Join(".agents", "rpi", phasedStateFile))):
		return "phased_state", "Phased state", 0
	case strings.HasSuffix(rel, c2EventsFileName):
		return "run_events", "Run events", 0
	case strings.HasSuffix(rel, "heartbeat.txt"):
		return "run_heartbeat", "Heartbeat", 0
	case strings.Contains(base, "-result.json"):
		return "phase_result", fmt.Sprintf("Phase %d result", phase), phase
	case strings.Contains(base, "-handoff.json"):
		return "phase_handoff", fmt.Sprintf("Phase %d handoff", phase), phase
	case strings.Contains(base, "-summary") && strings.HasSuffix(base, ".md"):
		return "phase_summary", fmt.Sprintf("Phase %d summary", phase), phase
	case strings.Contains(base, "-evaluator.json"):
		return "phase_evaluator", fmt.Sprintf("Phase %d evaluator", phase), phase
	case strings.Contains(base, "-gascity-evidence.json"):
		return "phase_gascity_evidence", fmt.Sprintf("Phase %d GasCity evidence", phase), phase
	case strings.Contains(rel, "/plans/"):
		return "plan", "Plan", 0
	case strings.Contains(rel, "/research/"):
		return "research", "Research", 0
	}
	if k, l, ok := classifyCouncilArtifact(rel, base); ok {
		return k, l, 0
	}
	return "artifact", base, phase
}

// classifyCouncilArtifact returns the council kind/label for /council/-rooted
// artifacts (pre-mortem, post-mortem, vibe). ok is false for non-council
// paths or unknown council variants.
func classifyCouncilArtifact(rel, base string) (kind, label string, ok bool) {
	if !strings.Contains(rel, "/council/") {
		return "", "", false
	}
	lower := strings.ToLower(base)
	switch {
	case strings.Contains(lower, "pre-mortem"):
		return "council_pre_mortem", "Pre-mortem report", true
	case strings.Contains(lower, "post-mortem"):
		return "council_post_mortem", "Post-mortem report", true
	case strings.Contains(lower, "vibe"):
		return "council_vibe", "Vibe report", true
	}
	return "", "", false
}

// ArtifactPhaseNumber extracts the phase number from a filename like "phase-2-result.json".
func ArtifactPhaseNumber(name string) int {
	matches := PhaseArtifactNumberPattern.FindStringSubmatch(name)
	if len(matches) != 2 {
		return 0
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return n
}

// ArtifactContentType returns the MIME type for an artifact path.
func ArtifactContentType(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".json", ".jsonl":
		return "application/json"
	case ".md", ".mdx":
		return "text/markdown"
	default:
		return "text/plain"
	}
}

// LaneResult captures a per-lane PASS/FAIL signal extracted from a packet's
// validation_lanes_results field. Status is the raw status string (typically
// "PASS" or "FAIL") and Passed is its case-insensitive interpretation.
type LaneResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

// ParseValidationLanesResults reads the execution packet at packetPath and
// returns the lane results from the packet's validation_lanes_results field.
// Returns an empty slice (no error) when the field is absent or the packet
// has no results yet. Returns nil (no error) when the file does not exist;
// a parse error is returned when the JSON is malformed.
func ParseValidationLanesResults(packetPath string) ([]LaneResult, error) {
	if strings.TrimSpace(packetPath) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(packetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read execution packet: %w", err)
	}
	var packet struct {
		Results []struct {
			Name   string `json:"name"`
			Status string `json:"status,omitempty"`
			Passed *bool  `json:"passed,omitempty"`
			Result string `json:"result,omitempty"`
		} `json:"validation_lanes_results"`
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		return nil, fmt.Errorf("parse validation_lanes_results: %w", err)
	}
	if len(packet.Results) == 0 {
		return []LaneResult{}, nil
	}
	out := make([]LaneResult, 0, len(packet.Results))
	for _, r := range packet.Results {
		lane := LaneResult{Name: r.Name}
		switch {
		case r.Passed != nil:
			lane.Passed = *r.Passed
		default:
			marker := strings.ToUpper(strings.TrimSpace(r.Status))
			if marker == "" {
				marker = strings.ToUpper(strings.TrimSpace(r.Result))
			}
			lane.Passed = marker == "PASS" || marker == "PASSED" || marker == "OK"
		}
		out = append(out, lane)
	}
	return out, nil
}

// AnyLaneResultPassed returns true when at least one lane reports passed.
// Convenience helper for evaluators that gate on "any-PASS" reductions.
func AnyLaneResultPassed(results []LaneResult) bool {
	for _, r := range results {
		if r.Passed {
			return true
		}
	}
	return false
}

// SortArtifactRefs sorts artifact refs by UpdatedAt (descending) then Path (ascending).
func SortArtifactRefs(refs []ArtifactRef) {
	for i := 0; i < len(refs); i++ {
		for j := i + 1; j < len(refs); j++ {
			if refs[j].UpdatedAt > refs[i].UpdatedAt ||
				(refs[j].UpdatedAt == refs[i].UpdatedAt && refs[j].Path < refs[i].Path) {
				refs[i], refs[j] = refs[j], refs[i]
			}
		}
	}
}
