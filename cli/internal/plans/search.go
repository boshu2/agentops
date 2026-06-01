package plans

import (
	"strings"

	"github.com/boshu2/agentops/cli/internal/domain"
)

// SearchPlans returns entries whose name, project, or beads ID contain the query (case-insensitive).
func SearchPlans(entries []domain.PlanManifestEntry, query string) []domain.PlanManifestEntry {
	q := strings.ToLower(query)
	var matches []domain.PlanManifestEntry
	for _, e := range entries {
		searchText := strings.ToLower(e.PlanName + " " + e.ProjectPath + " " + e.BeadsID)
		if strings.Contains(searchText, q) {
			matches = append(matches, e)
		}
	}
	return matches
}

// DriftEntry represents a single drift detection between manifest and beads.
type DriftEntry struct {
	Type     string
	PlanName string
	BeadsID  string
	Manifest string
	Beads    string
}

// DetectStatusDrifts finds status mismatches between manifest entries and beads.
func DetectStatusDrifts(byBeadsID map[string]*domain.PlanManifestEntry, beadsIndex map[string]string) []DriftEntry {
	var drifts []DriftEntry
	for beadsID, entry := range byBeadsID {
		beadsStatus, exists := beadsIndex[beadsID]
		if !exists {
			drifts = append(drifts, DriftEntry{
				Type: "missing_beads", PlanName: entry.PlanName,
				BeadsID: beadsID, Manifest: string(entry.Status), Beads: "(not found)",
			})
			continue
		}
		manifestClosed := entry.Status == domain.PlanStatusCompleted
		beadsClosed := beadsStatus == "closed"
		if manifestClosed != beadsClosed {
			drifts = append(drifts, DriftEntry{
				Type: "status_mismatch", PlanName: entry.PlanName,
				BeadsID: beadsID, Manifest: string(entry.Status), Beads: beadsStatus,
			})
		}
	}
	return drifts
}

// DetectOrphanedEntries finds manifest entries without beads linkage.
func DetectOrphanedEntries(entries []domain.PlanManifestEntry) []DriftEntry {
	var drifts []DriftEntry
	for _, e := range entries {
		if e.BeadsID == "" {
			drifts = append(drifts, DriftEntry{
				Type: "orphaned", PlanName: e.PlanName,
				BeadsID: "(none)", Manifest: string(e.Status), Beads: "n/a",
			})
		}
	}
	return drifts
}

// CountUnlinkedEntries counts entries without beads linkage.
// Returns the count and a slice of plan names that are unlinked.
func CountUnlinkedEntries(entries []domain.PlanManifestEntry) (int, []string) {
	count := 0
	var names []string
	for _, e := range entries {
		if e.BeadsID == "" {
			count++
			names = append(names, e.PlanName)
		}
	}
	return count, names
}

// UpsertEntry updates an existing entry or appends a new one.
// Returns true if an existing entry was updated.
func UpsertEntry(manifestPath string, existing []domain.PlanManifestEntry, entry domain.PlanManifestEntry) (bool, error) {
	for i, e := range existing {
		if e.Path == entry.Path {
			existing[i] = entry
			return true, SaveManifest(manifestPath, existing)
		}
	}
	return false, AppendManifestEntry(manifestPath, entry)
}
