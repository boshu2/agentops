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

// RewriteNextWorkFile rewrites the JSONL file with updated entries applied via
// the transform function. The full read-modify-write runs under an exclusive
// flock so concurrent queue consumers cannot interleave updates. Entries that
// could not be parsed are preserved verbatim. If the file does not exist,
// RewriteNextWorkFile is a no-op.
func RewriteNextWorkFile(path string, transform func(idx int, entry *NextWorkEntry) error) error {
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
