package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

var (
	rpiMarkProbedID    string
	rpiMarkProbedBy    string
	rpiMarkProbedQueue string
	rpiMarkProbedAt    string
)

func init() {
	markProbedCmd := &cobra.Command{
		Use:   "mark-probed",
		Short: "Stamp a next-work item as probed-stale without consuming it",
		Long: `Set probed_stale_at and probed_by on a next-work item whose tractability
probe (file/symbol/script grep) matched existing tracked work. The item
stays unconsumed (consumption requires real proof), but later nightlies
can read these fields to skip re-probing the same item.

The item is matched by --id (matches item.id, item.bead_id, or
item.title). The first matching item is stamped; if multiple items
share the identifier the gate is the queue's responsibility, not this
command.`,
		RunE: runRPIMarkProbed,
	}
	markProbedCmd.Flags().StringVar(&rpiMarkProbedID, "id", "", "Item identifier (id, bead_id, or title) to stamp (required)")
	markProbedCmd.Flags().StringVar(&rpiMarkProbedBy, "by", "", "Probe author tag, e.g. nightly/2026-04-26-v3 (required)")
	markProbedCmd.Flags().StringVar(&rpiMarkProbedQueue, "queue", ".agents/rpi/next-work.jsonl", "Path to next-work.jsonl")
	markProbedCmd.Flags().StringVar(&rpiMarkProbedAt, "at", "", "Override the probed-at timestamp (RFC3339); defaults to now")
	rpiCmd.AddCommand(markProbedCmd)
}

func runRPIMarkProbed(_ *cobra.Command, _ []string) error {
	if strings.TrimSpace(rpiMarkProbedID) == "" {
		return errors.New("--id is required")
	}
	if strings.TrimSpace(rpiMarkProbedBy) == "" {
		return errors.New("--by is required")
	}

	stamp := rpiMarkProbedAt
	if stamp == "" {
		stamp = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, stamp); err != nil {
		return fmt.Errorf("--at must be RFC3339: %w", err)
	}

	queuePath := rpiMarkProbedQueue
	if !filepath.IsAbs(queuePath) {
		// Resolve relative to repo root if we're in one; otherwise relative to cwd.
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getwd: %w", err)
		}
		queuePath = filepath.Join(cwd, queuePath)
	}

	raw, err := os.ReadFile(queuePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", queuePath, err)
	}

	var outLines []string
	matched := 0
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			outLines = append(outLines, line)
			continue
		}
		var entry cliRPI.NextWorkEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Preserve malformed lines verbatim — schema gate handles them separately.
			outLines = append(outLines, line)
			continue
		}
		changed := false
		for i := range entry.Items {
			it := &entry.Items[i]
			if matchesProbedID(it, rpiMarkProbedID) {
				it.ProbedStaleAt = &stamp
				it.ProbedBy = &rpiMarkProbedBy
				changed = true
				matched++
				break // first matching item per entry; stop here
			}
		}
		if changed {
			b, err := json.Marshal(&entry)
			if err != nil {
				return fmt.Errorf("re-marshalling entry: %w", err)
			}
			outLines = append(outLines, string(b))
		} else {
			outLines = append(outLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning queue: %w", err)
	}
	if matched == 0 {
		return fmt.Errorf("no item matched id=%q in %s", rpiMarkProbedID, queuePath)
	}

	tmp := queuePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(outLines, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, queuePath); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	fmt.Printf("ok: stamped probed_stale_at=%s probed_by=%s on %d item(s) in %s\n",
		stamp, rpiMarkProbedBy, matched, queuePath)
	return nil
}

func matchesProbedID(item *cliRPI.NextWorkItem, id string) bool {
	if item == nil {
		return false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	return item.ID == id || item.BeadID == id || item.Title == id
}
