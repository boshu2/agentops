package rpi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ForEachParseableNextWorkEntry walks successfully-parsed JSONL entries in data,
// assigning parseable indices with the same rules as RewriteNextWorkFile: blank
// lines and malformed JSON receive no index.
//
// This intentionally does not normalize legacy flat entries into items[].
// Callers that need the legacy-normalizing reader should use
// ParseNextWorkEntryLine instead.
func ForEachParseableNextWorkEntry(data []byte, fn func(idx int, entry NextWorkEntry) error) error {
	parseableIndex := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry NextWorkEntry
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		if err := fn(parseableIndex, entry); err != nil {
			return err
		}
		parseableIndex++
	}
	return nil
}

// WithNextWorkFileLock runs fn while holding a BLOCKING exclusive advisory
// lock on the sidecar lock file <path>.lock, serializing every cooperating
// next-work.jsonl read-modify-write critical section (age-kbw4). The sidecar
// carries the lock rather than the data file itself so the lock exists before
// the data file does and stays valid across writers that replace the data
// file's inode. Exported so append-path producers (e.g. mine.WriteWorkItems)
// can share the same critical section.
func WithNextWorkFileLock(path string, fn func() error) error {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open next-work lock file: %w", err)
	}
	defer func() {
		_ = lock.Close()
	}()
	if err := lockFile(lock); err != nil {
		return fmt.Errorf("lock next-work lock file: %w", err)
	}
	defer func() {
		_ = unlockFile(lock)
	}()
	return fn()
}

// mergeConsumedState enforces the consumed ratchet after a transform ran: a
// caller whose in-memory model predates another lane's write (the stale
// read-modify-write hazard, age-kbw4) may replace an entry wholesale and
// thereby revert consumed markings that are already persisted. disk is the
// entry as freshly parsed from the file inside the lock; updated is the entry
// after the transform. Consumed is never downgraded true->false at either the
// batch or the item level, and when the disk side is the consumed one its
// consumed_note / consumed_ref / consumed_by / consumed_at (and, when set, the
// claim_status stamped alongside them) are restored. When the updated side
// itself has consumed=true, its markers win untouched.
func mergeConsumedState(disk NextWorkEntry, updated *NextWorkEntry) {
	if disk.Consumed && !updated.Consumed {
		updated.Consumed = true
		updated.ConsumedNote = disk.ConsumedNote
		updated.ConsumedRef = disk.ConsumedRef
		updated.ConsumedBy = disk.ConsumedBy
		updated.ConsumedAt = disk.ConsumedAt
		if disk.ClaimStatus != "" {
			updated.ClaimStatus = disk.ClaimStatus
		}
	}

	idIndex := make(map[string]int, len(disk.Items))
	for i, item := range disk.Items {
		if item.ID != "" {
			idIndex[item.ID] = i
		}
	}
	for i := range updated.Items {
		item := &updated.Items[i]
		diskIdx := -1
		if item.ID != "" {
			// Keyed items merge only against their identity match; a keyed
			// item absent from disk is genuinely new and merges nothing.
			if j, ok := idIndex[item.ID]; ok {
				diskIdx = j
			}
		} else if i < len(disk.Items) {
			diskIdx = i
		}
		if diskIdx == -1 {
			continue
		}
		d := disk.Items[diskIdx]
		if d.Consumed && !item.Consumed {
			item.Consumed = true
			item.ConsumedNote = d.ConsumedNote
			item.ConsumedRef = d.ConsumedRef
			item.ConsumedBy = d.ConsumedBy
			item.ConsumedAt = d.ConsumedAt
			if d.ClaimStatus != "" {
				item.ClaimStatus = d.ClaimStatus
			}
		}
	}
}

// snapshotEntryConsumedState copies the parts of a freshly-parsed entry that
// mergeConsumedState compares against, so an in-place transform cannot mutate
// the pre-transform view through the shared Items backing array.
func snapshotEntryConsumedState(e NextWorkEntry) NextWorkEntry {
	cp := e
	cp.Items = append([]NextWorkItem(nil), e.Items...)
	return cp
}

// RewriteNextWorkFile rewrites the JSONL file with updated entries applied via
// the transform function. The full read-modify-write runs under the sidecar
// next-work lock (WithNextWorkFileLock) plus the legacy flock on the data file
// itself, so concurrent queue consumers cannot interleave updates; the file is
// re-read fresh INSIDE the lock and, after the transform, consumed state is
// merged so a transform working from a stale snapshot can never downgrade
// consumed=true persisted by a concurrent lane (mergeConsumedState, age-kbw4).
// Entries that could not be parsed are preserved verbatim. If the file does
// not exist, RewriteNextWorkFile is a no-op.
func RewriteNextWorkFile(path string, transform func(idx int, entry *NextWorkEntry) error) error {
	return WithNextWorkFileLock(path, func() error {
		return rewriteNextWorkFileLocked(path, transform)
	})
}

// rewriteNextWorkFileLocked is the RewriteNextWorkFile critical section; the
// caller must hold the sidecar next-work lock.
func rewriteNextWorkFileLocked(path string, transform func(idx int, entry *NextWorkEntry) error) error {
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
	if err := lockFile(f); err != nil {
		return fmt.Errorf("lock next-work.jsonl: %w", err)
	}
	defer func() {
		_ = unlockFile(f)
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

		var entry NextWorkEntry
		if jsonErr := json.Unmarshal([]byte(line), &entry); jsonErr != nil {
			// Preserve malformed lines verbatim.
			lines = append(lines, line)
			continue
		}

		diskState := snapshotEntryConsumedState(entry)
		if err := transform(parseableIndex, &entry); err != nil {
			return err
		}
		mergeConsumedState(diskState, &entry)
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
