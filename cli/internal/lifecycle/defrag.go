package lifecycle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefragReport is the top-level output of a defrag run.
type DefragReport struct {
	Timestamp   time.Time          `json:"timestamp"`
	DryRun      bool               `json:"dry_run"`
	Prune       *PruneResult       `json:"prune,omitempty"`
	Dedup       *DefragDedupResult `json:"dedup,omitempty"`
	Oscillation *OscillationResult `json:"oscillation,omitempty"`
}

// PruneResult holds orphan-detection results.
type PruneResult struct {
	TotalLearnings int      `json:"total_learnings"`
	StaleCount     int      `json:"stale_count"`
	Orphans        []string `json:"orphans,omitempty"`
	Deleted        []string `json:"deleted,omitempty"`
}

// DefragDedupResult holds near-duplicate detection results for defrag.
type DefragDedupResult struct {
	Checked        int         `json:"checked"`
	DuplicatePairs [][2]string `json:"duplicate_pairs,omitempty"`
	Deleted        []string    `json:"deleted,omitempty"`
}

// OscillationResult holds oscillating-goal sweep results.
type OscillationResult struct {
	OscillatingGoals []OscillatingGoal `json:"oscillating_goals,omitempty"`
}

// OscillatingGoal describes a goal that alternates improved/fail.
type OscillatingGoal struct {
	Target           string `json:"target"`
	AlternationCount int    `json:"alternation_count"`
	LastCycle        int    `json:"last_cycle"`
}

// ExecutePrune finds orphan learnings and optionally deletes them.
func ExecutePrune(cwd string, isDryRun bool, staleDays int) (*PruneResult, error) {
	result, err := FindOrphanLearnings(cwd, staleDays)
	if err != nil {
		return nil, fmt.Errorf("prune: %w", err)
	}
	if !isDryRun && len(result.Orphans) > 0 {
		for _, orphan := range result.Orphans {
			p := filepath.Join(cwd, orphan)
			if err := os.Remove(p); err != nil {
				return nil, fmt.Errorf("delete orphan %s: %w", orphan, err)
			}
			result.Deleted = append(result.Deleted, orphan)
		}
	}
	return result, nil
}

// ExecuteDedup finds duplicate learnings and optionally removes them.
func ExecuteDedup(cwd string, isDryRun bool) (*DefragDedupResult, error) {
	result, err := FindDuplicateLearnings(cwd)
	if err != nil {
		return nil, fmt.Errorf("dedup: %w", err)
	}
	if !isDryRun {
		for _, pair := range result.DuplicatePairs {
			keep, del := pair[0], pair[1]
			if IsHashNamed(pair[0]) && !IsHashNamed(pair[1]) {
				keep, del = pair[1], pair[0]
			}
			_ = keep
			p := filepath.Join(cwd, ".agents", "learnings", del)
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("dedup remove %s: %w", del, err)
			}
			result.Deleted = append(result.Deleted, del)
		}
		result.DuplicatePairs = nil
	}
	return result, nil
}

// FindOrphanLearnings scans .agents/learnings/ for files older than staleDays
// that are not referenced in any .agents/patterns/ or .agents/research/ file.
func FindOrphanLearnings(cwd string, staleDays int) (*PruneResult, error) {
	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &PruneResult{}, nil
		}
		return nil, fmt.Errorf("read learnings dir: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -staleDays)

	refContent, err := CollectReferenceContent(cwd)
	if err != nil {
		return nil, err
	}

	result := &PruneResult{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		result.TotalLearnings++

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(cutoff) {
			continue
		}
		result.StaleCount++

		if !strings.Contains(refContent, entry.Name()) {
			relPath := filepath.Join(".agents", "learnings", entry.Name())
			result.Orphans = append(result.Orphans, relPath)
		}
	}

	sort.Strings(result.Orphans)
	return result, nil
}

// CollectReferenceContent reads all .md files from .agents/patterns/ and
// .agents/research/ and returns their concatenated content for link checking.
func CollectReferenceContent(cwd string) (string, error) {
	var buf strings.Builder
	for _, sub := range []string{"patterns", "research"} {
		dir := filepath.Join(cwd, ".agents", sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("read %s dir: %w", sub, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			buf.Write(data)
			buf.WriteByte('\n')
		}
	}
	return buf.String(), nil
}

// IsHashNamed returns true if the filename looks like an auto-generated hash name
// (8 hex chars preceded by a date prefix, e.g. "2026-02-23-4556c2b4.md").
func IsHashNamed(name string) bool {
	base := filepath.Base(name)
	stem := strings.TrimSuffix(base, ".md")
	parts := strings.Split(stem, "-")
	if len(parts) < 4 {
		return false
	}
	last := parts[len(parts)-1]
	if len(last) != 8 {
		return false
	}
	for _, c := range last {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// FindDuplicateLearnings reads all .agents/learnings/*.md files and flags
// pairs with >80% trigram overlap as near-duplicates.
func FindDuplicateLearnings(cwd string) (*DefragDedupResult, error) {
	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &DefragDedupResult{}, nil
		}
		return nil, fmt.Errorf("read learnings dir: %w", err)
	}

	type learningFile struct {
		name     string
		trigrams map[string]bool
	}

	var files []learningFile

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(learningsDir, entry.Name()))
		if err != nil {
			continue
		}
		text := strings.ToLower(string(data))
		tg := BuildTrigrams(text)
		files = append(files, learningFile{name: entry.Name(), trigrams: tg})
	}

	result := &DefragDedupResult{Checked: len(files)}

	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			overlap := TrigramOverlap(files[i].trigrams, files[j].trigrams)
			if overlap > 0.80 {
				result.DuplicatePairs = append(result.DuplicatePairs, [2]string{
					files[i].name, files[j].name,
				})
			}
		}
	}

	return result, nil
}

// BuildTrigrams returns the set of character trigrams from text.
func BuildTrigrams(text string) map[string]bool {
	tg := make(map[string]bool)
	runes := []rune(text)
	for i := 0; i+2 < len(runes); i++ {
		tg[string(runes[i:i+3])] = true
	}
	return tg
}

// TrigramOverlap returns the Jaccard similarity of two trigram sets.
func TrigramOverlap(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}

	intersect := 0
	for k := range a {
		if b[k] {
			intersect++
		}
	}

	union := len(a) + len(b) - intersect
	if union == 0 {
		return 0
	}
	return float64(intersect) / float64(union)
}

// CycleRecord represents one line in cycle-history.jsonl.
type CycleRecord struct {
	Cycle  int    `json:"cycle"`
	Target string `json:"target"`
	Result string `json:"result"`
}

// SweepOscillatingGoals parses .agents/evolve/cycle-history.jsonl and finds
// goals whose result alternates between "improved" and non-"improved" >=3 times.
func SweepOscillatingGoals(cwd string) (*OscillationResult, error) {
	histPath := filepath.Join(cwd, ".agents", "evolve", "cycle-history.jsonl")
	targetRecords, err := collectCycleRecordsByTarget(histPath)
	if err != nil {
		return nil, err
	}

	return buildOscillationResult(targetRecords), nil
}

func collectCycleRecordsByTarget(histPath string) (map[string][]CycleRecord, error) {
	f, err := os.Open(histPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]CycleRecord{}, nil
		}
		return nil, fmt.Errorf("open cycle history: %w", err)
	}
	defer func() { _ = f.Close() }()

	targetRecords := make(map[string][]CycleRecord)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		rec, ok := parseCycleRecordLine(scanner.Text())
		if ok {
			targetRecords[rec.Target] = append(targetRecords[rec.Target], rec)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan cycle history: %w", err)
	}
	return targetRecords, nil
}

func parseCycleRecordLine(line string) (CycleRecord, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return CycleRecord{}, false
	}
	var rec CycleRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		return CycleRecord{}, false
	}
	return rec, rec.Target != ""
}

func buildOscillationResult(targetRecords map[string][]CycleRecord) *OscillationResult {
	result := &OscillationResult{}

	for _, target := range sortedCycleTargets(targetRecords) {
		if goal, ok := oscillatingGoalForTarget(target, targetRecords[target]); ok {
			result.OscillatingGoals = append(result.OscillatingGoals, goal)
		}
	}

	return result
}

func sortedCycleTargets(targetRecords map[string][]CycleRecord) []string {
	targets := make([]string, 0, len(targetRecords))
	for t := range targetRecords {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	return targets
}

func oscillatingGoalForTarget(target string, records []CycleRecord) (OscillatingGoal, bool) {
	alternations := CountAlternations(records)
	if alternations < 3 {
		return OscillatingGoal{}, false
	}
	lastCycle := records[len(records)-1].Cycle
	return OscillatingGoal{
		Target:           target,
		AlternationCount: alternations,
		LastCycle:        lastCycle,
	}, true
}

// CountAlternations counts how many times the result alternates between
// "improved" and non-"improved" in a sequence of records.
func CountAlternations(records []CycleRecord) int {
	if len(records) < 2 {
		return 0
	}
	count := 0
	for i := 1; i < len(records); i++ {
		prevImproved := records[i-1].Result == "improved"
		currImproved := records[i].Result == "improved"
		if prevImproved != currImproved {
			count++
		}
	}
	return count
}
