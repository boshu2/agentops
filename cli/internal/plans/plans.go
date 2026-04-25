// Package plans provides pure helpers for plan manifest management.
package plans

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

const (
	// ManifestFileName is the name of the plan manifest file.
	ManifestFileName = "manifest.jsonl"
	// PlansDir is the subdirectory under .agents for plan manifests.
	PlansDir = "plans"
)

// ComputePlanChecksum returns first 8 bytes of SHA256 as hex.
func ComputePlanChecksum(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	checksum := sha256.Sum256(content)
	return hex.EncodeToString(checksum[:8]), nil
}

// CreatePlanEntry builds a manifest entry from path and metadata.
func CreatePlanEntry(absPath string, modTime time.Time, projectPath, name, beadsID, checksum string) types.PlanManifestEntry {
	return types.PlanManifestEntry{
		Path:        absPath,
		CreatedAt:   modTime,
		ProjectPath: projectPath,
		PlanName:    name,
		Status:      types.PlanStatusActive,
		BeadsID:     beadsID,
		UpdatedAt:   time.Now(),
		Checksum:    checksum,
	}
}

// AppendManifestEntry appends an entry to the manifest file.
func AppendManifestEntry(manifestPath string, entry types.PlanManifestEntry) error {
	f, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.WriteString(string(data) + "\n")
	return err
}

// ResolvePlanName returns the explicit name or derives one from the file path.
func ResolvePlanName(explicit, planPath string) string {
	if explicit != "" {
		return explicit
	}
	return strings.TrimSuffix(filepath.Base(planPath), filepath.Ext(planPath))
}

// FindAgentsDir looks for .agents directory walking up to rig root.
//
// Skips the OS tempdir (os.TempDir()) when traversing — a stale
// /tmp/.agents/ left over from a prior test run or scratch session
// is not a real rig root and must not shadow legitimate startDirs that
// happen to live below /tmp.
func FindAgentsDir(startDir string) string {
	tmpDir := filepath.Clean(os.TempDir())
	dir := startDir
	for {
		if filepath.Clean(dir) != tmpDir {
			candidate := filepath.Join(dir, ".agents")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			markers := []string{".beads", "crew", "polecats"}
			for _, marker := range markers {
				if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
					return filepath.Join(dir, ".agents")
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// LoadManifest reads all entries from the manifest file.
func LoadManifest(path string) ([]types.PlanManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var entries []types.PlanManifestEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry types.PlanManifestEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// SaveManifest writes all entries to the manifest file.
func SaveManifest(path string, entries []types.PlanManifestEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		if _, err := f.WriteString(string(data) + "\n"); err != nil {
			return err
		}
	}
	return nil
}

// DetectProjectPath attempts to find the project path for a plan file.
func DetectProjectPath(planPath string) string {
	if strings.Contains(planPath, ".claude/plans/") {
		content, err := os.ReadFile(planPath)
		if err != nil {
			return ""
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Project:") || strings.Contains(line, "Working directory:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

// FilterPlans returns entries matching the project and status filters.
func FilterPlans(entries []types.PlanManifestEntry, project, status string) []types.PlanManifestEntry {
	var out []types.PlanManifestEntry
	for _, e := range entries {
		if project != "" && !strings.Contains(e.ProjectPath, project) {
			continue
		}
		if status != "" && string(e.Status) != status {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ApplyPlanUpdates applies status and beadsID updates to the matching entry.
func ApplyPlanUpdates(entries []types.PlanManifestEntry, absPath, status, beadsID string) bool {
	for i, e := range entries {
		if e.Path == absPath {
			if status != "" {
				entries[i].Status = types.PlanStatus(status)
			}
			if beadsID != "" {
				entries[i].BeadsID = beadsID
			}
			entries[i].UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// BuildBeadsIDIndex creates a map of beadsID -> slice index.
func BuildBeadsIDIndex(entries []types.PlanManifestEntry) map[string]int {
	index := make(map[string]int)
	for i, e := range entries {
		if e.BeadsID != "" {
			index[e.BeadsID] = i
		}
	}
	return index
}

// SyncEpicStatus syncs a single epic status and returns true if changed.
func SyncEpicStatus(entries []types.PlanManifestEntry, idx int, beadsStatus string) bool {
	newStatus := types.PlanStatusActive
	if beadsStatus == "closed" {
		newStatus = types.PlanStatusCompleted
	}
	if entries[idx].Status != newStatus {
		entries[idx].Status = newStatus
		entries[idx].UpdatedAt = time.Now()
		return true
	}
	return false
}

// BuildBeadsStatusIndex creates a map of epic ID -> status from beads epics.
func BuildBeadsStatusIndex(epics []BeadsEpic) map[string]string {
	index := make(map[string]string)
	for _, e := range epics {
		index[e.ID] = e.Status
	}
	return index
}

// BeadsEpic represents a beads epic for sync.
type BeadsEpic struct {
	ID     string
	Status string
}

// ParseBeadsEpicJSONL parses JSONL output from bd list into BeadsEpic slices.
func ParseBeadsEpicJSONL(data []byte) []BeadsEpic {
	var epics []BeadsEpic
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		var obj map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
			continue
		}
		id, _ := obj["id"].(string)
		status, _ := obj["status"].(string)
		if id != "" {
			epics = append(epics, BeadsEpic{ID: id, Status: status})
		}
	}
	return epics
}
