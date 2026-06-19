package rpi

import (
	"bufio"
	"fmt"
	"os"
)

// queue_read.go — next-work queue corpus reading, migrated out of cmd/ao
// (rpi_loop.go) so the keeper context/codex commands stay self-contained after
// the ao rpi command surface is removed (ADR-0009 teardown, age-3pdt). Builds
// on the queue lifecycle/ranking primitives already in this package. Logic is
// preserved from the cmd/ao originals; the malformed-line verbose warning is
// dropped (silent skip) since internal/rpi has no cmd/ao logger.

// EntryHasExplicitItemLifecycle reports whether any item in the entry carries
// explicit per-item lifecycle state (a consumed flag or lifecycle metadata).
func EntryHasExplicitItemLifecycle(entry NextWorkEntry) bool {
	for _, item := range entry.Items {
		if item.Consumed || HasQueueItemLifecycleMetadata(item) {
			return true
		}
	}
	return false
}

// ReadQueueEntries reads next-work.jsonl and returns entries with at least one
// selectable queue item (the 0-based QueueIndex is preserved for later marking).
// Malformed lines are skipped. A missing file returns nil, nil.
func ReadQueueEntries(path string) ([]NextWorkEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer func() { _ = f.Close() }()

	var entries []NextWorkEntry
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	parseableIndex := -1

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		entry, err := ParseNextWorkEntryLine(line)
		if err != nil {
			continue // skip malformed lines
		}
		parseableIndex++
		entry.QueueIndex = parseableIndex

		if len(entry.Items) == 0 {
			continue
		}
		if EntryHasExplicitItemLifecycle(entry) {
			RecomputeEntryLifecycle(&entry)
		}
		// Skip entries that are already consumed. Legacy failed_at remains retry
		// metadata; proof-backed preflight decides whether stale work is satisfied.
		if entry.Consumed || NormalizeClaimStatus(entry.Consumed, entry.ClaimStatus) == "consumed" {
			continue
		}
		// Skip entries where every item is consumed or currently claimed.
		hasSelectableItem := false
		for _, item := range entry.Items {
			if IsQueueItemSelectable(item) {
				hasSelectableItem = true
				break
			}
		}
		if !hasSelectableItem {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// ReadUnconsumedItems returns all selectable items across unconsumed entries,
// filtered to repoFilter when set (empty/"*" target repos always pass).
func ReadUnconsumedItems(path string, repoFilter string) ([]NextWorkItem, error) {
	entries, err := ReadQueueEntries(path)
	if err != nil {
		return nil, err
	}

	var items []NextWorkItem
	for _, entry := range entries {
		for _, item := range entry.Items {
			if !IsQueueItemSelectable(item) {
				continue
			}
			if repoFilter != "" && item.TargetRepo != "" && item.TargetRepo != "*" && item.TargetRepo != repoFilter {
				continue
			}
			items = append(items, item)
		}
	}
	return items, nil
}
