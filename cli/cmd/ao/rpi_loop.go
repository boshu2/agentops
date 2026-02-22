package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

var (
	rpiMaxCycles             int
	rpiRepoFilter            string
	rpiSupervisor            bool
	rpiFailurePolicy         string
	rpiCycleRetries          int
	rpiRetryBackoff          time.Duration
	rpiCycleDelay            time.Duration
	rpiLease                 bool
	rpiLeasePath             string
	rpiLeaseTTL              time.Duration
	rpiDetachedHeal          bool
	rpiDetachedBranchPrefix  string
	rpiAutoClean             bool
	rpiAutoCleanStaleAfter   time.Duration
	rpiEnsureCleanup         bool
	rpiCleanupPruneWorktrees bool
	rpiGatePolicy            string
	rpiValidateFastScript    string
	rpiSecurityGateScript    string
	rpiLandingPolicy         string
	rpiLandingBranch         string
	rpiLandingCommitMessage  string
	rpiBDSyncPolicy          string
	rpiCommandTimeout        time.Duration
)

func init() {
	loopCmd := &cobra.Command{
		Use:   "loop [goal]",
		Short: "Run continuous RPI cycles from next-work queue",
		Long: `Execute RPI cycles in a loop, consuming from next-work.jsonl.

Each cycle drives a queue item through the full phased RPI engine:
  1. Read unconsumed items from .agents/rpi/next-work.jsonl
  2. Pick highest-severity item as goal (or use explicit goal)
  3. Run: ao rpi phased "<goal>" (discovery → implementation → validation)
  4. Mark the consumed queue entry with a timestamp on success (or "failed" on error)
  5. Re-read next-work.jsonl (post-mortem may have harvested new items)
  6. Repeat until queue empty or max-cycles reached

Queue semantics:
  - An entry is only marked consumed after the phased engine completes without error.
  - If the phased engine fails, the entry is marked with failed_at so it is
    skipped on subsequent runs but remains recoverable (set consumed=false to retry).
  - Already-consumed or already-failed entries are skipped (idempotent).

Examples:
  ao rpi loop                          # consume from queue until stable
  ao rpi loop "improve test coverage"  # run one cycle with explicit goal
  ao rpi loop --max-cycles 3           # cap at 3 iterations
  ao rpi loop --repo-filter agentops   # only process items targeting agentops
  ao rpi loop --dry-run                # show what would run`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRPILoop,
	}

	loopCmd.Flags().IntVar(&rpiMaxCycles, "max-cycles", 0, "Maximum cycles (0 = unlimited, stop when queue empty)")
	loopCmd.Flags().StringVar(&rpiRepoFilter, "repo-filter", "", "Only process queue items targeting this repo (empty = all)")
	loopCmd.Flags().BoolVar(&rpiSupervisor, "supervisor", false, "Enable autonomous supervisor mode (lease lock, self-heal, retries, gates, cleanup)")
	loopCmd.Flags().StringVar(&rpiFailurePolicy, "failure-policy", "stop", "Cycle failure policy: stop|continue")
	loopCmd.Flags().IntVar(&rpiCycleRetries, "cycle-retries", 0, "Automatic retry count per cycle after a failed attempt")
	loopCmd.Flags().DurationVar(&rpiRetryBackoff, "retry-backoff", 30*time.Second, "Backoff between cycle retry attempts")
	loopCmd.Flags().DurationVar(&rpiCycleDelay, "cycle-delay", 0, "Delay between completed cycles")
	loopCmd.Flags().BoolVar(&rpiLease, "lease", false, "Acquire a single-flight supervisor lease lock before running")
	loopCmd.Flags().StringVar(&rpiLeasePath, "lease-path", filepath.Join(".agents", "rpi", "supervisor.lock"), "Lease lock file path (absolute or repo-relative)")
	loopCmd.Flags().DurationVar(&rpiLeaseTTL, "lease-ttl", 2*time.Minute, "Lease heartbeat TTL for supervisor lock metadata")
	loopCmd.Flags().BoolVar(&rpiDetachedHeal, "detached-heal", false, "Auto-create/switch to a named branch when HEAD is detached")
	loopCmd.Flags().StringVar(&rpiDetachedBranchPrefix, "detached-branch-prefix", "codex/auto-rpi", "Branch prefix used by detached HEAD self-heal")
	loopCmd.Flags().BoolVar(&rpiAutoClean, "auto-clean", false, "Run stale RPI cleanup before each phased cycle")
	loopCmd.Flags().DurationVar(&rpiAutoCleanStaleAfter, "auto-clean-stale-after", 24*time.Hour, "Only auto-clean runs older than this age")
	loopCmd.Flags().BoolVar(&rpiEnsureCleanup, "ensure-cleanup", false, "Run stale-run cleanup after each cycle (cleanup guarantee)")
	loopCmd.Flags().BoolVar(&rpiCleanupPruneWorktrees, "cleanup-prune-worktrees", true, "Run git worktree prune during supervisor cleanup")
	loopCmd.Flags().StringVar(&rpiGatePolicy, "gate-policy", "off", "Quality/security gate policy: off|best-effort|required")
	loopCmd.Flags().StringVar(&rpiValidateFastScript, "gate-fast-script", filepath.Join("scripts", "validate-go-fast.sh"), "Fast validation gate script path")
	loopCmd.Flags().StringVar(&rpiSecurityGateScript, "gate-security-script", filepath.Join("scripts", "security-gate.sh"), "Security gate script path")
	loopCmd.Flags().StringVar(&rpiLandingPolicy, "landing-policy", "off", "Landing policy after successful cycle: off|commit|sync-push")
	loopCmd.Flags().StringVar(&rpiLandingBranch, "landing-branch", "", "Landing target branch (empty resolves origin/HEAD, then current branch, then main)")
	loopCmd.Flags().StringVar(&rpiLandingCommitMessage, "landing-commit-message", "chore(rpi): autonomous cycle {{cycle}}", "Commit message template for landing policies that commit")
	loopCmd.Flags().StringVar(&rpiBDSyncPolicy, "bd-sync-policy", "auto", "bd sync policy for landing: auto|always|never")
	loopCmd.Flags().DurationVar(&rpiCommandTimeout, "command-timeout", 20*time.Minute, "Timeout for supervisor external commands (git/bd/gate scripts)")

	rpiCmd.AddCommand(loopCmd)
}

// nextWorkEntry represents one line in next-work.jsonl.
type nextWorkEntry struct {
	SourceEpic string         `json:"source_epic"`
	Timestamp  string         `json:"timestamp"`
	Items      []nextWorkItem `json:"items"`
	Consumed   bool           `json:"consumed"`
	ConsumedBy *string        `json:"consumed_by"`
	ConsumedAt *string        `json:"consumed_at"`
	FailedAt   *string        `json:"failed_at,omitempty"`
	QueueIndex int            `json:"-"`
}

// nextWorkItem represents a single harvested work item.
type nextWorkItem struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"`
	TargetRepo  string `json:"target_repo,omitempty"`
}

// queueSelection holds the selected item together with its source entry index
// so the caller can mark the correct entry consumed/failed.
type queueSelection struct {
	Item       nextWorkItem
	EntryIndex int // 0-based index among parseable JSON entries in next-work.jsonl
}

var runRPISupervisedCycleFn = runRPISupervisedCycle

func runRPILoop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cfg, err := resolveLoopSupervisorConfig(cmd, cwd)
	if err != nil {
		return err
	}

	// Parse explicit goal if provided
	explicitGoal := ""
	if len(args) > 0 {
		explicitGoal = args[0]
	}

	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Join(cwd, ".agents", "rpi"), 0755); err != nil {
		return fmt.Errorf("ensure .agents/rpi directory: %w", err)
	}

	var lease *supervisorLease
	if cfg.LeaseEnabled && !GetDryRun() {
		runID := generateRunID()
		lease, err = acquireSupervisorLease(cwd, cfg.LeasePath, cfg.LeaseTTL, runID)
		if err != nil {
			return err
		}
		defer func() {
			if releaseErr := lease.Release(); releaseErr != nil {
				VerbosePrintf("Warning: could not release supervisor lease: %v\n", releaseErr)
			}
		}()
		fmt.Printf("Supervisor lease acquired: %s (run=%s)\n", lease.Path(), runID)
	}

	cycle := 0
	executedCycles := 0
	for {
		cycle++

		if rpiMaxCycles > 0 && cycle > rpiMaxCycles {
			fmt.Printf("\nReached max cycles (%d). Stopping.\n", rpiMaxCycles)
			break
		}
		if cycle > 1 && cfg.CycleDelay > 0 {
			fmt.Printf("\nSleeping %s before next cycle...\n", cfg.CycleDelay.Round(time.Second))
			time.Sleep(cfg.CycleDelay)
		}

		fmt.Printf("\n=== RPI Loop: Cycle %d ===\n", cycle)

		// Determine goal and which queue entry to mark after completion.
		goal := explicitGoal
		var sel *queueSelection

		if goal == "" {
			// Read queue for unconsumed, non-failed entries.
			entries, err := readQueueEntries(nextWorkPath)
			if err != nil {
				VerbosePrintf("Warning: %v\n", err)
			}

			sel = selectHighestSeverityEntry(entries, rpiRepoFilter)
			if sel == nil {
				fmt.Println("No unconsumed work in queue. Flywheel stable.")
				break
			}

			goal = sel.Item.Title
			fmt.Printf("From queue: %s\n", goal)
		}

		if goal == "" {
			fmt.Println("No goal and empty queue. Nothing to do.")
			break
		}

		if GetDryRun() {
			fmt.Printf("[dry-run] Would run phased engine for: %q\n", goal)
			if explicitGoal == "" {
				fmt.Println("[dry-run] Queue not consumed in dry-run. Showing first cycle only.")
			}
			break
		}

		fmt.Printf("Running phased engine for: %q\n", goal)
		executedCycles++
		start := time.Now()
		var cycleErr error
		maxAttempts := cfg.MaxCycleAttempts()
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			cycleErr = runRPISupervisedCycleFn(cwd, goal, cycle, attempt, cfg)
			if cycleErr == nil {
				break
			}
			if attempt >= maxAttempts {
				break
			}
			fmt.Printf("Cycle %d attempt %d/%d failed: %v\n", cycle, attempt, maxAttempts, cycleErr)
			if cfg.RetryBackoff > 0 {
				fmt.Printf("Retrying in %s...\n", cfg.RetryBackoff.Round(time.Second))
				time.Sleep(cfg.RetryBackoff)
			}
		}
		elapsed := time.Since(start).Round(time.Second)

		if cycleErr != nil {
			fmt.Printf("Cycle %d failed after %s: %v\n", cycle, elapsed, cycleErr)

			// Mark only task failures as failed. Transient infra failures remain
			// retryable queue items for a future cycle.
			if sel != nil {
				if shouldMarkQueueEntryFailed(cycleErr) {
					if markErr := markEntryFailed(nextWorkPath, sel.EntryIndex); markErr != nil {
						VerbosePrintf("Warning: could not mark queue entry as failed: %v\n", markErr)
					} else {
						fmt.Printf("Queue entry marked failed (set consumed=false to retry): %q\n", sel.Item.Title)
					}
				} else {
					fmt.Printf("Queue entry left unmodified (transient infra failure): %q\n", sel.Item.Title)
				}
			}

			if cfg.ShouldContinueAfterFailure() && explicitGoal == "" {
				fmt.Printf("Failure policy %q: continuing to next queue item.\n", cfg.FailurePolicy)
				continue
			}

			fmt.Println("Stopping loop due to failure policy.")
			return cycleErr
		}

		fmt.Printf("Cycle %d completed in %s\n", cycle, elapsed)

		// Mark the queue entry consumed after successful completion.
		if sel != nil {
			if markErr := markEntryConsumed(nextWorkPath, sel.EntryIndex, "ao-rpi-loop"); markErr != nil {
				VerbosePrintf("Warning: could not mark queue entry as consumed: %v\n", markErr)
			} else {
				fmt.Printf("Queue entry consumed: %q\n", sel.Item.Title)
			}
		}

		// If explicit goal was provided, only run once.
		if explicitGoal != "" {
			fmt.Println("Explicit goal completed.")
			break
		}
	}

	fmt.Printf("\nRPI loop finished after %d cycle(s).\n", executedCycles)
	return nil
}

// readUnconsumedItems reads next-work.jsonl and returns all unconsumed items
// across all entries, flattened. When repoFilter is non-empty, items with a
// non-empty TargetRepo that is neither "*" nor equal to repoFilter are skipped.
// Items without a TargetRepo (legacy) or with TargetRepo=="*" always pass.
func readUnconsumedItems(path string, repoFilter string) ([]nextWorkItem, error) {
	entries, err := readQueueEntries(path)
	if err != nil {
		return nil, err
	}

	var items []nextWorkItem
	for _, entry := range entries {
		for _, item := range entry.Items {
			if repoFilter != "" && item.TargetRepo != "" && item.TargetRepo != "*" && item.TargetRepo != repoFilter {
				continue
			}
			items = append(items, item)
		}
	}
	return items, nil
}

// readQueueEntries reads next-work.jsonl and returns all unconsumed, non-failed
// entries (with their 0-based index preserved for later marking). Malformed
// lines are skipped with a verbose warning. Missing files return nil, nil.
func readQueueEntries(path string) ([]nextWorkEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer f.Close()

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

		var entry nextWorkEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			VerbosePrintf("Skipping malformed line: %v\n", err)
			continue
		}
		parseableIndex++
		entry.QueueIndex = parseableIndex

		// Skip entries that are already consumed or previously failed.
		if entry.Consumed || entry.FailedAt != nil {
			continue
		}
		if len(entry.Items) == 0 {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// selectHighestSeverityEntry picks the best item across all eligible entries.
// It returns a queueSelection containing the winning item and its source entry
// parseable index in next-work.jsonl. Items filtered out by repoFilter are skipped.
// Returns nil if no eligible items exist.
func selectHighestSeverityEntry(entries []nextWorkEntry, repoFilter string) *queueSelection {
	type candidate struct {
		item       nextWorkItem
		entryIndex int
		rank       int
	}

	var candidates []candidate
	for _, entry := range entries {
		for _, item := range entry.Items {
			if repoFilter != "" && item.TargetRepo != "" && item.TargetRepo != "*" && item.TargetRepo != repoFilter {
				continue
			}
			candidates = append(candidates, candidate{item: item, entryIndex: entry.QueueIndex, rank: severityRank(item.Severity)})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].rank > candidates[j].rank
	})

	best := candidates[0]
	return &queueSelection{Item: best.item, EntryIndex: best.entryIndex}
}

// rewriteNextWorkFile rewrites the JSONL file with updated entries applied via
// the transform function. The transform receives a pointer to each parsed entry
// so it can mutate individual fields. Entries that could not be parsed are
// preserved verbatim. If the file does not exist, rewriteNextWorkFile is a no-op.
func rewriteNextWorkFile(path string, transform func(idx int, entry *nextWorkEntry)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read next-work.jsonl: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var lines []string
	idx := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			lines = append(lines, line)
			continue
		}

		var entry nextWorkEntry
		if jsonErr := json.Unmarshal([]byte(line), &entry); jsonErr != nil {
			// Preserve malformed lines verbatim.
			lines = append(lines, line)
			idx++
			continue
		}

		transform(idx, &entry)
		rewritten, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			lines = append(lines, line)
		} else {
			lines = append(lines, string(rewritten))
		}
		idx++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan next-work.jsonl: %w", err)
	}

	var out bytes.Buffer
	for _, l := range lines {
		out.WriteString(l)
		out.WriteByte('\n')
	}

	return os.WriteFile(path, out.Bytes(), 0644)
}

// markEntryConsumed sets Consumed=true and ConsumedAt on the entry at entryIndex.
// entryIndex is the 0-based index of the entry among parseable JSON entries in
// the file (blank/malformed lines do not receive an index).
//
// Returns an error when the file does not exist so callers can distinguish a
// missing-queue situation from a successful no-op.
func markEntryConsumed(path string, entryIndex int, consumedBy string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("next-work.jsonl not found: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	parseable := -1
	return rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) {
		parseable++
		if parseable != entryIndex {
			return
		}
		entry.Consumed = true
		entry.ConsumedAt = &now
		entry.ConsumedBy = &consumedBy
		entry.FailedAt = nil
	})
}

// markItemConsumed marks the first entry matching sourceEpic in path as consumed.
// Unlike markEntryConsumed (index-based), this function identifies entries by
// source_epic field, making it safe to call with the epic ID from a run.
//
// Semantics:
//   - Missing file: returns an error (caller should verify file exists before calling).
//   - Wrong/no match: no-op (idempotent — safe to call even if already consumed).
//   - Match: sets Consumed=true, ConsumedAt, and ConsumedBy=runID.
//
// markEntryFailed records a FailedAt timestamp on the entry at entryIndex without
// setting Consumed. This leaves the entry recoverable: set consumed=false to retry.
func markEntryFailed(path string, entryIndex int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	parseable := -1
	return rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) {
		parseable++
		if parseable != entryIndex {
			return
		}
		entry.FailedAt = &now
	})
}

// selectHighestSeverityItem returns the title of the highest-severity item.
// Severity order: high > medium > low.
func selectHighestSeverityItem(items []nextWorkItem) string {
	if len(items) == 0 {
		return ""
	}

	sort.Slice(items, func(i, j int) bool {
		return severityRank(items[i].Severity) > severityRank(items[j].Severity)
	})

	return items[0].Title
}

func severityRank(s string) int {
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
