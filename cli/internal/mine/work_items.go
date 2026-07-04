package mine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

// WorkItemEmit is a single work item within a next-work.jsonl entry.
type WorkItemEmit struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"`
	File        string `json:"file,omitempty"`
	Func        string `json:"func,omitempty"`
}

// ComplexityHotspot represents a high-complexity function with recent edits.
type ComplexityHotspot struct {
	File        string `json:"file"`
	Func        string `json:"func"`
	Complexity  int    `json:"complexity"`
	RecentEdits int    `json:"recent_edits"`
}

// OrphanedResearch holds the list of orphaned research file names.
type OrphanedResearch struct {
	Files []string
}

// WorkItemID generates a stable ID from the item's identifying fields.
func WorkItemID(item WorkItemEmit) string {
	h := sha256.New()
	h.Write([]byte(item.Type))
	h.Write([]byte{0})
	if item.File != "" && item.Func != "" {
		h.Write([]byte(item.File))
		h.Write([]byte{0})
		h.Write([]byte(item.Func))
	} else {
		h.Write([]byte(item.Title))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// CollectWorkItemsFromHotspots builds work items from code hotspots.
func CollectWorkItemsFromHotspots(hotspots []ComplexityHotspot) []WorkItemEmit {
	var items []WorkItemEmit
	for _, h := range hotspots {
		item := WorkItemEmit{
			Title:       fmt.Sprintf("Reduce complexity: %s in %s (CC=%d)", h.Func, h.File, h.Complexity),
			Type:        "refactor",
			Severity:    "high",
			Source:      "compile-mine",
			Description: fmt.Sprintf("Function %s in %s has cyclomatic complexity %d with %d recent edits. Extract helpers to reduce CC below 15.", h.Func, h.File, h.Complexity, h.RecentEdits),
			Evidence:    fmt.Sprintf("complexity=%d recent_edits=%d", h.Complexity, h.RecentEdits),
			File:        h.File,
			Func:        h.Func,
		}
		item.ID = WorkItemID(item)
		items = append(items, item)
	}
	return items
}

// CollectWorkItemsFromOrphans builds work items from orphaned research files.
func CollectWorkItemsFromOrphans(orphans []string) []WorkItemEmit {
	var items []WorkItemEmit
	for _, orphan := range orphans {
		item := WorkItemEmit{
			Title:       fmt.Sprintf("Rescue orphan: %s", orphan),
			Type:        "knowledge-gap",
			Severity:    "medium",
			Source:      "compile-mine",
			Description: fmt.Sprintf("Research file %q exists in .agents/research/ but is not referenced in any learning. Extract its key insights into a learning file.", orphan),
			Evidence:    "not referenced in .agents/learnings/",
		}
		item.ID = WorkItemID(item)
		items = append(items, item)
	}
	return items
}

// LoadExistingMineIDs scans a JSONL file for unconsumed compile-mine item IDs.
func LoadExistingMineIDs(path string) (map[string]bool, error) {
	ids := make(map[string]bool)
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ids, nil
		}
		return nil, err
	}
	if len(existing) == 0 {
		return ids, nil
	}
	for _, line := range strings.Split(string(existing), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry struct {
			SourceEpic string         `json:"source_epic"`
			Consumed   bool           `json:"consumed"`
			Items      []WorkItemEmit `json:"items"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil &&
			entry.SourceEpic == "compile-mine" && !entry.Consumed {
			for _, it := range entry.Items {
				if it.ID != "" {
					ids[it.ID] = true
				}
			}
		}
	}
	return ids, nil
}

// WriteWorkItems appends one JSONL line per work item to the given path. The
// append runs under the shared next-work sidecar lock
// (rpi.WithNextWorkFileLock) so a line appended here cannot land inside a
// concurrent RewriteNextWorkFile read-truncate-write window and be silently
// destroyed by the rewrite (age-kbw4).
func WriteWorkItems(path string, items []WorkItemEmit, ts string) error {
	type emitEntry struct {
		SourceEpic  string         `json:"source_epic"`
		Timestamp   string         `json:"timestamp"`
		Items       []WorkItemEmit `json:"items"`
		Consumed    bool           `json:"consumed"`
		ClaimStatus string         `json:"claim_status,omitempty"`
		ClaimedBy   *string        `json:"claimed_by,omitempty"`
		ClaimedAt   *string        `json:"claimed_at,omitempty"`
		ConsumedBy  *string        `json:"consumed_by"`
		ConsumedAt  *string        `json:"consumed_at"`
	}

	return rpi.WithNextWorkFileLock(path, func() error {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return fmt.Errorf("open next-work.jsonl: %w", err)
		}

		for _, item := range items {
			entry := emitEntry{
				SourceEpic:  "compile-mine",
				Timestamp:   ts,
				Items:       []WorkItemEmit{item},
				Consumed:    false,
				ClaimStatus: "available",
			}
			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshal work item entry: %w", err)
			}
			data = append(data, '\n')
			if _, writeErr := f.Write(data); writeErr != nil {
				_ = f.Close()
				return writeErr
			}
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close next-work.jsonl: %w", err)
		}
		return nil
	})
}

// WriteMineReportJSON writes a JSON report to dated and latest files.
func WriteMineReportJSON(dir string, data []byte, dateStr string) error {
	if dir == "" {
		return fmt.Errorf("output directory must not be empty")
	}
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create mine output dir: %w", err)
	}

	datedPath := filepath.Join(dir, dateStr+".json")
	if err := os.WriteFile(datedPath, data, 0o644); err != nil {
		return fmt.Errorf("write dated report: %w", err)
	}

	latestPath := filepath.Join(dir, "latest.json")
	if err := os.WriteFile(latestPath, data, 0o644); err != nil {
		return fmt.Errorf("write latest report: %w", err)
	}

	return nil
}
