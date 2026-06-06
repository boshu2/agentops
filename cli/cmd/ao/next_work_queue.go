package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

// Type aliases for the next-work queue data model. Canonical definitions
// live in internal/rpi; these aliases let the rest of cmd/ao use short names.
type (
	nextWorkEntry         = rpi.NextWorkEntry
	nextWorkProofRef      = rpi.NextWorkProofRef
	nextWorkItem          = rpi.NextWorkItem
	nextWorkProofDecision = rpi.NextWorkProofDecision
)

// parseNextWorkEntryLine delegates to internal/rpi.
func parseNextWorkEntryLine(line string) (nextWorkEntry, error) {
	return rpi.ParseNextWorkEntryLine(line)
}

// normalizeClaimStatus delegates to internal/rpi.
func normalizeClaimStatus(consumed bool, claimStatus string) string {
	return rpi.NormalizeClaimStatus(consumed, claimStatus)
}

// isQueueItemSelectable delegates to internal/rpi.
func isQueueItemSelectable(item nextWorkItem) bool {
	return rpi.IsQueueItemSelectable(item)
}

// hasQueueItemLifecycleMetadata delegates to internal/rpi.
func hasQueueItemLifecycleMetadata(item nextWorkItem) bool {
	return rpi.HasQueueItemLifecycleMetadata(item)
}

// recomputeEntryLifecycle delegates to internal/rpi.
func recomputeEntryLifecycle(entry *nextWorkEntry) { rpi.RecomputeEntryLifecycle(entry) }

// isQueueItemHeldForReview delegates to internal/rpi.
func isQueueItemHeldForReview(item nextWorkItem) bool {
	return rpi.IsQueueItemHeldForReview(item)
}

func entryHasExplicitItemLifecycle(entry nextWorkEntry) bool {
	for _, item := range entry.Items {
		if item.Consumed || hasQueueItemLifecycleMetadata(item) {
			return true
		}
	}
	return false
}

// readQueueEntries reads next-work.jsonl and returns entries with at least one
// selectable queue item. Malformed lines are skipped. Missing files return nil.
func readQueueEntries(path string) ([]nextWorkEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer func() { _ = f.Close() }()

	var entries []nextWorkEntry
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	parseableIndex := -1

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		entry, err := parseNextWorkEntryLine(line)
		if err != nil {
			VerbosePrintf("Skipping malformed line: %v\n", err)
			continue
		}
		parseableIndex++
		entry.QueueIndex = parseableIndex

		if len(entry.Items) == 0 {
			continue
		}
		if entryHasExplicitItemLifecycle(entry) {
			recomputeEntryLifecycle(&entry)
		}
		if entry.Consumed || normalizeClaimStatus(entry.Consumed, entry.ClaimStatus) == "consumed" {
			continue
		}
		hasSelectableItem := false
		for _, item := range entry.Items {
			if isQueueItemSelectable(item) {
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

// readUnconsumedItems reads the next-work queue and returns individual selectable items.
func readUnconsumedItems(path string, repoFilter string) ([]nextWorkItem, error) {
	entries, err := readQueueEntries(path)
	if err != nil {
		return nil, err
	}

	var items []nextWorkItem
	for _, entry := range entries {
		for _, item := range entry.Items {
			if !isQueueItemSelectable(item) {
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

// rewriteNextWorkFile rewrites the JSONL file with updated entries applied via
// the transform function. The full read-modify-write runs under an exclusive
// flock so concurrent queue consumers cannot interleave updates.
func rewriteNextWorkFile(path string, transform func(idx int, entry *nextWorkEntry) error) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if err := flockLock(f); err != nil {
		return fmt.Errorf("lock next-work.jsonl: %w", err)
	}
	defer func() {
		_ = flockUnlock(f)
	}()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek next-work.jsonl: %w", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read next-work.jsonl: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var lines []string
	parseableIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			lines = append(lines, line)
			continue
		}

		var entry nextWorkEntry
		if jsonErr := json.Unmarshal([]byte(line), &entry); jsonErr != nil {
			lines = append(lines, line)
			continue
		}

		if err := transform(parseableIndex, &entry); err != nil {
			return err
		}
		rewritten, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			lines = append(lines, line)
		} else {
			lines = append(lines, string(rewritten))
		}
		parseableIndex++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan next-work.jsonl: %w", err)
	}

	var out bytes.Buffer
	for _, l := range lines {
		out.WriteString(l)
		out.WriteByte('\n')
	}

	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncate next-work.jsonl: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek next-work.jsonl for write: %w", err)
	}
	if _, err := f.Write(out.Bytes()); err != nil {
		return fmt.Errorf("write next-work.jsonl: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync next-work.jsonl: %w", err)
	}
	return nil
}
