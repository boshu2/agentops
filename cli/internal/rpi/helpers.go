package rpi

import (
	"cmp"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// SeverityRank returns a numeric rank for severity strings (high=3, medium=2, low=1).
func SeverityRank(s string) int {
	switch s {
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

// FreshnessRank returns 0 for previously-failed items, 1 otherwise.
func FreshnessRank(item NextWorkItem) int {
	if item.FailedAt != nil {
		return 0
	}
	return 1
}

// RepoAffinityRank scores an item's target_repo match against the filter.
func RepoAffinityRank(item NextWorkItem, repoFilter string) int {
	if repoFilter == "" {
		return 0
	}
	switch item.TargetRepo {
	case repoFilter:
		return 3
	case "*":
		return 2
	case "":
		return 1
	default:
		return 0
	}
}

// WorkTypeRank returns a priority rank based on the work item type.
func WorkTypeRank(item NextWorkItem) int {
	switch item.Type {
	case "feature", "improvement", "tech-debt", "pattern-fix", "bug", "task":
		return 2
	case "process-improvement":
		return 1
	default:
		return 0
	}
}

// NormalizeClaimStatus keeps omitted item `claim_status` semantically
// equivalent to available unless the item is already consumed.
func NormalizeClaimStatus(consumed bool, claimStatus string) string {
	switch claimStatus {
	case "available", "in_progress", "consumed":
		if consumed && claimStatus != "in_progress" {
			return "consumed"
		}
		return claimStatus
	default:
		if consumed {
			return "consumed"
		}
		return "available"
	}
}

// IsQueueItemSelectable returns true when an item can be picked from the queue.
func IsQueueItemSelectable(item NextWorkItem) bool {
	if item.Consumed || NormalizeClaimStatus(item.Consumed, item.ClaimStatus) == "consumed" {
		return false
	}
	return NormalizeClaimStatus(item.Consumed, item.ClaimStatus) != "in_progress"
}

// HasQueueItemLifecycleMetadata returns true when an item has any lifecycle fields set.
func HasQueueItemLifecycleMetadata(item NextWorkItem) bool {
	return item.ClaimStatus != "" ||
		item.ClaimedBy != nil ||
		item.ClaimedAt != nil ||
		item.ConsumedBy != nil ||
		item.ConsumedAt != nil ||
		item.FailedAt != nil
}

// ShouldSkipLegacyFailedEntry returns true when a failed entry has proof-backed
// completion evidence indicating the work was completed despite the failure marker.
func ShouldSkipLegacyFailedEntry(entry NextWorkEntry) bool {
	if entry.FailedAt == nil {
		return false
	}
	return entry.CompletionEvidence != ""
}

// HasLegacyFlatNextWorkItem returns true when the entry has legacy flat fields set.
func HasLegacyFlatNextWorkItem(entry NextWorkEntry) bool {
	return strings.TrimSpace(entry.Title) != "" ||
		strings.TrimSpace(entry.Type) != "" ||
		strings.TrimSpace(entry.Severity) != "" ||
		strings.TrimSpace(entry.Description) != "" ||
		strings.TrimSpace(entry.Evidence) != "" ||
		strings.TrimSpace(entry.TargetRepo) != "" ||
		strings.TrimSpace(entry.Source) != ""
}

// NextWorkSearchRoot returns the repo root from a next-work.jsonl path.
func NextWorkSearchRoot(path string) string {
	dir := filepath.Dir(filepath.Clean(path))
	parent := filepath.Dir(dir)
	if filepath.Base(dir) == "rpi" && filepath.Base(parent) == ".agents" {
		return filepath.Dir(parent)
	}
	return dir
}

// SelectHighestSeverityItem returns the title of the highest-severity item.
func SelectHighestSeverityItem(items []NextWorkItem) string {
	if len(items) == 0 {
		return ""
	}

	slices.SortFunc(items, func(a, b NextWorkItem) int {
		return cmp.Compare(SeverityRank(b.Severity), SeverityRank(a.Severity))
	})

	return items[0].Title
}

// IsFullyConsumed returns true when the entry and all its items are consumed.
func IsFullyConsumed(entry *NextWorkEntry) bool {
	if !entry.Consumed && NormalizeClaimStatus(entry.Consumed, entry.ClaimStatus) != "consumed" {
		return false
	}
	for _, item := range entry.Items {
		if !item.Consumed && NormalizeClaimStatus(item.Consumed, item.ClaimStatus) != "consumed" {
			return false
		}
	}
	return true
}

// EntryConsumedTime returns the consumed_at time for the entry, falling back to
// the latest item consumed_at.
func EntryConsumedTime(entry *NextWorkEntry) time.Time {
	if entry.ConsumedAt != nil {
		if t, err := time.Parse(time.RFC3339, *entry.ConsumedAt); err == nil {
			return t
		}
	}
	var latest time.Time
	for _, item := range entry.Items {
		if item.ConsumedAt != nil {
			if t, err := time.Parse(time.RFC3339, *item.ConsumedAt); err == nil {
				if t.After(latest) {
					latest = t
				}
			}
		}
	}
	return latest
}

// EnsureQueueItemClaimable checks whether a queue item can be claimed.
func EnsureQueueItemClaimable(status string, currentClaimedBy *string, claimedBy string) error {
	if status == "consumed" {
		return ErrQueueClaimConflict
	}
	if status == "in_progress" && (currentClaimedBy == nil || *currentClaimedBy != claimedBy) {
		return ErrQueueClaimConflict
	}
	return nil
}

// RequireQueueClaimOwner verifies the current claim owner matches expected.
func RequireQueueClaimOwner(currentClaimedBy *string, expectedClaimedBy string) error {
	if expectedClaimedBy == "" {
		return nil
	}
	if currentClaimedBy == nil || *currentClaimedBy != expectedClaimedBy {
		return ErrQueueClaimConflict
	}
	return nil
}

// RecomputeEntryLifecycle recomputes the entry-level lifecycle fields from its items.
func RecomputeEntryLifecycle(entry *NextWorkEntry) {
	if len(entry.Items) == 0 {
		return
	}

	allConsumed := true
	claimedIndex := -1
	var latestFailed *string
	var finalConsumedBy *string
	var finalConsumedAt *string

	for i := range entry.Items {
		status := NormalizeClaimStatus(entry.Items[i].Consumed, entry.Items[i].ClaimStatus)
		entry.Items[i].ClaimStatus = status

		switch status {
		case "consumed":
			entry.Items[i].Consumed = true
			if entry.Items[i].ConsumedBy != nil {
				finalConsumedBy = entry.Items[i].ConsumedBy
			}
			if entry.Items[i].ConsumedAt != nil {
				finalConsumedAt = entry.Items[i].ConsumedAt
			}
		default:
			allConsumed = false
		}

		if status == "in_progress" && claimedIndex == -1 {
			claimedIndex = i
		}
		if entry.Items[i].FailedAt != nil {
			latestFailed = entry.Items[i].FailedAt
		}
	}

	entry.FailedAt = latestFailed
	if allConsumed {
		entry.Consumed = true
		entry.ClaimStatus = "consumed"
		entry.ClaimedBy = nil
		entry.ClaimedAt = nil
		entry.ConsumedBy = finalConsumedBy
		entry.ConsumedAt = finalConsumedAt
		return
	}

	entry.Consumed = false
	entry.ConsumedBy = nil
	entry.ConsumedAt = nil
	if claimedIndex >= 0 {
		entry.ClaimStatus = "in_progress"
		entry.ClaimedBy = entry.Items[claimedIndex].ClaimedBy
		entry.ClaimedAt = entry.Items[claimedIndex].ClaimedAt
		return
	}
	entry.ClaimStatus = "available"
	entry.ClaimedBy = nil
	entry.ClaimedAt = nil
}

// ParseNextWorkEntryLine parses a single line from next-work.jsonl.
func ParseNextWorkEntryLine(line string) (NextWorkEntry, error) {
	var entry NextWorkEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return NextWorkEntry{}, err
	}

	if entry.Timestamp == "" && entry.CreatedAt != "" {
		entry.Timestamp = entry.CreatedAt
	}

	if len(entry.Items) == 0 && HasLegacyFlatNextWorkItem(entry) {
		entry.Items = []NextWorkItem{{
			ID:          entry.LegacyID,
			Title:       entry.Title,
			Type:        entry.Type,
			Severity:    entry.Severity,
			Source:      entry.Source,
			Description: entry.Description,
			Evidence:    entry.Evidence,
			TargetRepo:  entry.TargetRepo,
			Consumed:    entry.Consumed,
			ClaimStatus: NormalizeClaimStatus(entry.Consumed, entry.ClaimStatus),
			ClaimedBy:   entry.ClaimedBy,
			ClaimedAt:   entry.ClaimedAt,
			ConsumedBy:  entry.ConsumedBy,
			ConsumedAt:  entry.ConsumedAt,
			FailedAt:    entry.FailedAt,
		}}
	}

	return entry, nil
}

// QueueProofTargetIDs extracts candidate target IDs from a queue selection.
func QueueProofTargetIDs(sel *QueueSelection) []string {
	if sel == nil {
		return nil
	}

	texts := []string{
		strings.TrimSpace(sel.SourceEpic),
		strings.TrimSpace(sel.Item.Title),
		strings.TrimSpace(sel.Item.Description),
		strings.TrimSpace(sel.Item.Evidence),
	}

	candidates := make([]string, 0, len(texts))
	seen := make(map[string]struct{}, 8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	add(sel.SourceEpic)
	for _, text := range texts[1:] {
		for _, match := range QueueProofPacketPathPattern.FindAllStringSubmatch(text, -1) {
			if len(match) >= 2 {
				add(match[1])
			}
		}
		for _, match := range QueueProofTargetPattern.FindAllString(text, -1) {
			add(match)
		}
	}

	return candidates
}
