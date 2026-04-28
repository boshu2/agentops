package overnight

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/boshu2/agentops/cli/internal/mine"
)

const mineFindingsGeneratorName = "mine-findings"
const findingGeneratorSidecarSchemaVersion = "finding-generator-sidecar/v1"
const dreamGeneratorAggregatorSourceEpic = "dream-generator-aggregator"

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
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Source      string   `json:"source"`
	Description string   `json:"description"`
	Evidence    string   `json:"evidence,omitempty"`
	File        string   `json:"file,omitempty"`
	Func        string   `json:"func,omitempty"`
	TargetRepo  string   `json:"target_repo,omitempty"`
	DedupKey    string   `json:"dedup_key"`
	Duplicate   bool     `json:"duplicate"`
	Status      string   `json:"status,omitempty"`
	Requires    []string `json:"requires,omitempty"`
}

// FindingGeneratorAggregateResult summarizes the single-writer merge of all
// completed generator sidecars into one next-work batch.
type FindingGeneratorAggregateResult struct {
	SidecarsRead      int
	SidecarsSoftFail  int
	CandidatesSeen    int
	DuplicatesSkipped int
	ItemsWritten      int
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
			ID:          item.ID,
			Title:       item.Title,
			Type:        normalizeGeneratorCandidateType(item.Type),
			Severity:    normalizeGeneratorCandidateSeverity(item.Severity),
			Source:      "evolve-generator",
			Description: item.Description,
			Evidence:    item.Evidence,
			File:        item.File,
			Func:        item.Func,
			DedupKey:    mineGeneratorDedupKey(item),
			Duplicate:   duplicate,
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
	return buildFailedFindingGeneratorSidecar(opts, mineFindingsGeneratorName, "compile-mine", startedAt, finishedAt, err)
}

func buildFailedFindingGeneratorSidecar(
	opts RunLoopOptions,
	generator string,
	sourceEpic string,
	startedAt time.Time,
	finishedAt time.Time,
	err error,
) FindingGeneratorSidecar {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if sourceEpic == "" {
		sourceEpic = generator
	}
	return newFindingGeneratorSidecar(
		opts,
		generator,
		sourceEpic,
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
		SchemaVersion:     findingGeneratorSidecarSchemaVersion,
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

// AggregateFindingGeneratorSidecars reads completed generator sidecars from
// outputDir, reconciles duplicates, and appends one next-work batch under cwd.
// It is the only queue writer for read-side generators.
func AggregateFindingGeneratorSidecars(cwd, outputDir string) (FindingGeneratorAggregateResult, []string, error) {
	var result FindingGeneratorAggregateResult
	var degraded []string
	sidecarDir := filepath.Join(outputDir, "generator-results")
	entries, err := os.ReadDir(sidecarDir)
	if os.IsNotExist(err) {
		return result, []string{"generator-results dir missing"}, nil
	}
	if err != nil {
		return result, nil, fmt.Errorf("read generator-results dir: %w", err)
	}

	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	existing, err := loadGeneratorNextWorkDedupState(nextWorkPath)
	if err != nil {
		return result, degraded, fmt.Errorf("load generator next-work dedup state: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	// selected is populated through applyGeneratorSidecarCandidates before use.
	selected := map[string]FindingGeneratorCandidate{} // nosemgrep: trailofbits.go.iterate-over-empty-map.iterate-over-empty-map
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		sidecar, readErr := readFindingGeneratorSidecar(filepath.Join(sidecarDir, entry.Name()))
		if readErr != nil {
			degraded = append(degraded, fmt.Sprintf("%s: %v", entry.Name(), readErr))
			continue
		}
		result.SidecarsRead++
		if sidecar.Status != "completed" {
			result.SidecarsSoftFail++
			degraded = append(degraded, fmt.Sprintf("%s: %s", sidecar.Generator, sidecar.Status))
			continue
		}
		degraded = applyGeneratorSidecarCandidates(sidecar, existing, selected, &result, degraded)
	}

	if len(selected) == 0 {
		return result, degraded, nil
	}

	items := make([]generatorNextWorkItem, 0, len(selected))
	keys := make([]string, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		items = append(items, generatorCandidateNextWorkItem(selected[key]))
	}
	if err := appendGeneratorNextWorkBatch(nextWorkPath, items, time.Now().UTC()); err != nil {
		return result, degraded, err
	}
	result.ItemsWritten = len(items)
	return result, degraded, nil
}

// applyGeneratorSidecarCandidates merges a completed sidecar's candidates into
// the cumulative selected map, updating result counters and appending any
// degraded notes. Extracted from AggregateFindingGeneratorSidecars to keep
// the parent's cyclomatic complexity below the cli/internal/ ceiling.
func applyGeneratorSidecarCandidates(
	sidecar FindingGeneratorSidecar,
	existing generatorNextWorkDedupState,
	selected map[string]FindingGeneratorCandidate,
	result *FindingGeneratorAggregateResult,
	degraded []string,
) []string {
	for _, candidate := range sidecar.Candidates {
		result.CandidatesSeen++
		candidate = normalizeGeneratorCandidate(candidate)
		key := candidate.DedupKey
		if key == "" {
			result.DuplicatesSkipped++
			degraded = append(degraded, fmt.Sprintf("%s: candidate %q missing dedup key", sidecar.Generator, candidate.Title))
			continue
		}
		if candidate.Duplicate || existing.has(candidate.ID, key) {
			result.DuplicatesSkipped++
			continue
		}
		if previous, ok := selected[key]; ok {
			result.DuplicatesSkipped++
			if preferGeneratorCandidate(candidate, previous) {
				selected[key] = candidate
			}
			continue
		}
		selected[key] = candidate
	}
	return degraded
}

func mineGeneratorDedupKey(item mine.WorkItemEmit) string {
	target := item.File
	if target == "" {
		target = item.Title
	}
	return "finding-generator|mine-findings|" + normalizeDedupComponent(item.Type+"|"+item.Title+"|"+target)
}

type generatorNextWorkDedupState struct {
	ids       map[string]bool
	dedupKeys map[string]bool
}

func (s generatorNextWorkDedupState) has(id, dedupKey string) bool {
	if id != "" && s.ids[id] {
		return true
	}
	return dedupKey != "" && s.dedupKeys[dedupKey]
}

type generatorNextWorkItem struct {
	ID          string   `json:"id,omitempty"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Source      string   `json:"source"`
	Description string   `json:"description"`
	Evidence    string   `json:"evidence,omitempty"`
	TargetRepo  string   `json:"target_repo,omitempty"`
	File        string   `json:"file,omitempty"`
	Func        string   `json:"func,omitempty"`
	DedupKey    string   `json:"dedup_key,omitempty"`
	Status      string   `json:"status,omitempty"`
	Requires    []string `json:"requires,omitempty"`
}

type generatorNextWorkLine struct {
	SourceEpic  string                  `json:"source_epic"`
	Timestamp   string                  `json:"timestamp"`
	Items       []generatorNextWorkItem `json:"items"`
	Consumed    bool                    `json:"consumed"`
	ClaimStatus string                  `json:"claim_status"`
	ClaimedBy   *string                 `json:"claimed_by"`
	ClaimedAt   *string                 `json:"claimed_at"`
	ConsumedBy  *string                 `json:"consumed_by"`
	ConsumedAt  *string                 `json:"consumed_at"`
}

func readFindingGeneratorSidecar(path string) (FindingGeneratorSidecar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FindingGeneratorSidecar{}, err
	}
	var sidecar FindingGeneratorSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return FindingGeneratorSidecar{}, fmt.Errorf("decode sidecar: %w", err)
	}
	if sidecar.SchemaVersion != findingGeneratorSidecarSchemaVersion {
		return FindingGeneratorSidecar{}, fmt.Errorf("unsupported sidecar schema %q", sidecar.SchemaVersion)
	}
	if sidecar.Generator == "" {
		return FindingGeneratorSidecar{}, fmt.Errorf("missing generator")
	}
	return sidecar, nil
}

func loadGeneratorNextWorkDedupState(path string) (generatorNextWorkDedupState, error) {
	state := generatorNextWorkDedupState{
		ids:       map[string]bool{},
		dedupKeys: map[string]bool{},
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry struct {
			Items []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Title    string `json:"title"`
				File     string `json:"file"`
				DedupKey string `json:"dedup_key"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		for _, item := range entry.Items {
			if item.ID != "" {
				state.ids[item.ID] = true
			}
			key := item.DedupKey
			if key == "" {
				target := item.File
				if target == "" {
					target = item.Title
				}
				key = "finding-generator|mine-findings|" + normalizeDedupComponent(item.Type+"|"+item.Title+"|"+target)
			}
			if key != "" {
				state.dedupKeys[key] = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return state, err
	}
	return state, nil
}

func appendGeneratorNextWorkBatch(path string, items []generatorNextWorkItem, now time.Time) error {
	if len(items) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir next-work dir: %w", err)
	}
	line := generatorNextWorkLine{
		SourceEpic:  dreamGeneratorAggregatorSourceEpic,
		Timestamp:   now.UTC().Format(time.RFC3339),
		Items:       items,
		Consumed:    false,
		ClaimStatus: "available",
		ClaimedBy:   nil,
		ClaimedAt:   nil,
		ConsumedBy:  nil,
		ConsumedAt:  nil,
	}
	data, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("marshal generator next-work line: %w", err)
	}
	data = append(data, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write next-work.jsonl: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync next-work.jsonl: %w", err)
	}
	return nil
}

func normalizeGeneratorCandidate(candidate FindingGeneratorCandidate) FindingGeneratorCandidate {
	candidate.Type = normalizeGeneratorCandidateType(candidate.Type)
	candidate.Severity = normalizeGeneratorCandidateSeverity(candidate.Severity)
	candidate.Source = "evolve-generator"
	if candidate.Description == "" {
		candidate.Description = candidate.Evidence
	}
	if candidate.Description == "" {
		candidate.Description = candidate.Title
	}
	candidate.DedupKey = strings.ToLower(strings.TrimSpace(candidate.DedupKey))
	hasKnownPrefix := strings.HasPrefix(candidate.DedupKey, "finding-generator|") ||
		strings.HasPrefix(candidate.DedupKey, "external-watchlist|")
	if candidate.DedupKey != "" && !hasKnownPrefix {
		candidate.DedupKey = "finding-generator|" + candidate.DedupKey
	}
	if candidate.DedupKey == "" {
		target := candidate.File
		if target == "" {
			target = candidate.Title
		}
		candidate.DedupKey = "finding-generator|" + normalizeDedupComponent(candidate.Type+"|"+candidate.Title+"|"+target)
	}
	return candidate
}

func generatorCandidateNextWorkItem(candidate FindingGeneratorCandidate) generatorNextWorkItem {
	candidate = normalizeGeneratorCandidate(candidate)
	return generatorNextWorkItem{
		ID:          candidate.ID,
		Title:       candidate.Title,
		Type:        candidate.Type,
		Severity:    candidate.Severity,
		Source:      candidate.Source,
		Description: candidate.Description,
		Evidence:    candidate.Evidence,
		TargetRepo:  candidate.TargetRepo,
		File:        candidate.File,
		Func:        candidate.Func,
		DedupKey:    candidate.DedupKey,
		Status:      candidate.Status,
		Requires:    candidate.Requires,
	}
}

func preferGeneratorCandidate(candidate, previous FindingGeneratorCandidate) bool {
	cRank := severityRank(candidate.Severity)
	pRank := severityRank(previous.Severity)
	if cRank != pRank {
		return cRank > pRank
	}
	if candidate.Title != previous.Title {
		return candidate.Title < previous.Title
	}
	return candidate.ID < previous.ID
}

func severityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func normalizeGeneratorCandidateType(value string) string {
	trimmed := strings.TrimSpace(value)
	switch trimmed {
	case "tech-debt", "improvement", "pattern-fix", "process-improvement", "feature", "bug", "task":
		return trimmed
	case "refactor":
		return "tech-debt"
	case "knowledge-gap":
		return "task"
	default:
		return "task"
	}
}

func normalizeGeneratorCandidateSeverity(value string) string {
	trimmed := strings.TrimSpace(value)
	switch trimmed {
	case "high", "medium", "low":
		return trimmed
	default:
		return "medium"
	}
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
