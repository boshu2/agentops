package overnight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/boshu2/agentops/cli/internal/mine"
)

const mineFindingsGeneratorName = "mine-findings"

// FindingGeneratorSidecar is the durable per-generator result envelope written
// by Dream read-side generators. Generators write sidecars; the queue writer
// stays serialized elsewhere.
type FindingGeneratorSidecar struct {
	SchemaVersion     string                      `json:"schema_version"`
	RunID             string                      `json:"run_id,omitempty"`
	Generator         string                      `json:"generator"`
	SourceEpic        string                      `json:"source_epic"`
	Status            string                      `json:"status"`
	StartedAt         string                      `json:"started_at"`
	FinishedAt        string                      `json:"finished_at"`
	DurationMillis    int64                       `json:"duration_millis"`
	CandidateCount    int                         `json:"candidate_count"`
	DuplicateCount    int                         `json:"duplicate_count"`
	NewCandidateCount int                         `json:"new_candidate_count"`
	DuplicateRate     float64                     `json:"duplicate_rate"`
	Error             string                      `json:"error,omitempty"`
	Candidates        []FindingGeneratorCandidate `json:"candidates,omitempty"`
}

// FindingGeneratorCandidate is a normalized, read-only candidate emitted into a
// sidecar. It intentionally mirrors only queue-safe fields and dedup metadata.
type FindingGeneratorCandidate struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Severity  string `json:"severity"`
	Source    string `json:"source"`
	Evidence  string `json:"evidence,omitempty"`
	File      string `json:"file,omitempty"`
	Func      string `json:"func,omitempty"`
	DedupKey  string `json:"dedup_key"`
	Duplicate bool   `json:"duplicate"`
}

func buildMineGeneratorSidecar(
	opts RunLoopOptions,
	report *mine.Report,
	startedAt time.Time,
	finishedAt time.Time,
	existingIDs map[string]bool,
) FindingGeneratorSidecar {
	if existingIDs == nil {
		existingIDs = map[string]bool{}
	}
	items := mine.CollectMineWorkItems(report)
	candidates := make([]FindingGeneratorCandidate, 0, len(items))
	for _, item := range items {
		duplicate := item.ID != "" && existingIDs[item.ID]
		candidates = append(candidates, FindingGeneratorCandidate{
			ID:        item.ID,
			Title:     item.Title,
			Type:      item.Type,
			Severity:  item.Severity,
			Source:    item.Source,
			Evidence:  item.Evidence,
			File:      item.File,
			Func:      item.Func,
			DedupKey:  mineGeneratorDedupKey(item),
			Duplicate: duplicate,
		})
	}
	return newFindingGeneratorSidecar(
		opts,
		mineFindingsGeneratorName,
		"compile-mine",
		"completed",
		startedAt,
		finishedAt,
		candidates,
		"",
	)
}

func buildFailedMineGeneratorSidecar(
	opts RunLoopOptions,
	startedAt time.Time,
	finishedAt time.Time,
	err error,
) FindingGeneratorSidecar {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return newFindingGeneratorSidecar(
		opts,
		mineFindingsGeneratorName,
		"compile-mine",
		"soft-fail",
		startedAt,
		finishedAt,
		nil,
		message,
	)
}

func newFindingGeneratorSidecar(
	opts RunLoopOptions,
	generator string,
	sourceEpic string,
	status string,
	startedAt time.Time,
	finishedAt time.Time,
	candidates []FindingGeneratorCandidate,
	errMessage string,
) FindingGeneratorSidecar {
	duplicateCount := 0
	for _, candidate := range candidates {
		if candidate.Duplicate {
			duplicateCount++
		}
	}
	candidateCount := len(candidates)
	duplicateRate := 0.0
	if candidateCount > 0 {
		duplicateRate = float64(duplicateCount) / float64(candidateCount)
	}
	return FindingGeneratorSidecar{
		SchemaVersion:     "finding-generator-sidecar/v1",
		RunID:             opts.RunID,
		Generator:         generator,
		SourceEpic:        sourceEpic,
		Status:            status,
		StartedAt:         startedAt.UTC().Format(time.RFC3339Nano),
		FinishedAt:        finishedAt.UTC().Format(time.RFC3339Nano),
		DurationMillis:    finishedAt.Sub(startedAt).Milliseconds(),
		CandidateCount:    candidateCount,
		DuplicateCount:    duplicateCount,
		NewCandidateCount: candidateCount - duplicateCount,
		DuplicateRate:     duplicateRate,
		Error:             errMessage,
		Candidates:        candidates,
	}
}

func writeFindingGeneratorSidecar(opts RunLoopOptions, sidecar FindingGeneratorSidecar) (string, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return "", fmt.Errorf("missing output dir for finding-generator sidecar")
	}
	name := normalizeGeneratorFilename(sidecar.Generator)
	if name == "" {
		return "", fmt.Errorf("missing finding-generator name")
	}
	dir := filepath.Join(opts.OutputDir, "generator-results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create generator-results dir: %w", err)
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal generator sidecar: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(dir, name+".json")
	tmp, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("create generator sidecar temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write generator sidecar temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close generator sidecar temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("replace generator sidecar: %w", err)
	}
	return path, nil
}

func mineGeneratorDedupKey(item mine.WorkItemEmit) string {
	target := item.File
	if target == "" {
		target = item.Title
	}
	return "finding-generator|mine-findings|" + normalizeDedupComponent(item.Type+"|"+item.Title+"|"+target)
}

func normalizeGeneratorFilename(name string) string {
	return normalizeDedupComponent(name)
}

func normalizeDedupComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
