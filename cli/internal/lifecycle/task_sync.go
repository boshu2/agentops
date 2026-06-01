package lifecycle

import (
	"encoding/json"

	"github.com/boshu2/agentops/cli/internal/domain"
)

// StatusToMaturity maps a task status string to a CASS maturity level.
func StatusToMaturity(status string) domain.Maturity {
	switch status {
	case "completed":
		return domain.MaturityEstablished
	case "in_progress":
		return domain.MaturityCandidate
	default: // "pending"
		return domain.MaturityProvisional
	}
}

// CloneStringAnyMap returns a shallow copy of input, or nil if input is empty.
func CloneStringAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for k, v := range input {
		cloned[k] = v
	}
	return cloned
}

// ExtractContentBlocks navigates data["message"]["content"] and returns only
// tool_use blocks as typed maps.
func ExtractContentBlocks(data map[string]any) []map[string]any {
	message, ok := data["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := message["content"].([]any)
	if !ok {
		return nil
	}
	var blocks []map[string]any
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		if blockType == "tool_use" {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// ParseTaskCreate extracts task fields from a TaskCreate tool input.
// Returns subject, description, metadata map, and activeForm.
// subject is empty when the input is invalid.
func ParseTaskCreate(input map[string]any) (subject, description, activeForm string, metadata map[string]any) {
	subject, _ = input["subject"].(string)
	if subject == "" {
		return "", "", "", nil
	}
	description, _ = input["description"].(string)
	activeForm, _ = input["activeForm"].(string)
	if raw, ok := input["metadata"].(map[string]any); ok {
		metadata = CloneStringAnyMap(raw)
	}
	return subject, description, activeForm, metadata
}

// ApplyTaskUpdate applies TaskUpdate fields to mutable task fields.
// Returns the new status (or empty if unchanged) and updated subject/description/owner.
func ApplyTaskUpdate(input map[string]any) (status, subject, description, owner string) {
	status, _ = input["status"].(string)
	subject, _ = input["subject"].(string)
	description, _ = input["description"].(string)
	owner, _ = input["owner"].(string)
	return status, subject, description, owner
}

// ProcessTranscriptLine parses one JSONL line and returns the (possibly updated)
// session ID. Tool blocks are returned for the caller to dispatch.
func ProcessTranscriptLine(line, filterSession, currentSessionID string) (newSessionID string, blocks []map[string]any) {
	var data map[string]any
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return currentSessionID, nil
	}

	if sid, ok := data["sessionId"].(string); ok && sid != "" {
		currentSessionID = sid
	}

	if filterSession != "" && currentSessionID != filterSession {
		return currentSessionID, nil
	}

	return currentSessionID, ExtractContentBlocks(data)
}

// TaskDistribution holds tallied status and maturity counts from a task list.
type TaskDistribution struct {
	StatusCounts   map[string]int
	MaturityCounts map[domain.Maturity]int
	WithLearnings  int
}

// ComputeTaskDistributions tallies status, maturity, and learning counts.
func ComputeTaskDistributions(statuses []string, maturities []domain.Maturity, learningIDs []string) TaskDistribution {
	d := TaskDistribution{
		StatusCounts:   make(map[string]int),
		MaturityCounts: make(map[domain.Maturity]int),
	}
	for _, s := range statuses {
		d.StatusCounts[s]++
	}
	for _, m := range maturities {
		d.MaturityCounts[m]++
	}
	for _, lid := range learningIDs {
		if lid != "" {
			d.WithLearnings++
		}
	}
	return d
}
