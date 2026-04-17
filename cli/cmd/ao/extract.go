package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// PendingExtraction represents a session queued for learning extraction.
type PendingExtraction struct {
	SessionID      string    `json:"session_id"`
	SessionPath    string    `json:"session_path"`
	TranscriptPath string    `json:"transcript_path"`
	Summary        string    `json:"summary"`
	Decisions      []string  `json:"decisions,omitempty"`
	Knowledge      []string  `json:"knowledge,omitempty"`
	QueuedAt       time.Time `json:"queued_at"`
	BeadID         string    `json:"bead_id,omitempty"` // Bead being worked on when session was queued
}

// ExtractBatchResult holds results from batch extraction.
type ExtractBatchResult struct {
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	Remaining int      `json:"remaining"`
	Entries   []string `json:"entries"` // session IDs processed
}

var (
	extractMaxContent int
	extractClear      bool
	extractAll        bool
	extractBead       string
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Process pending learning extractions",
	Long: `Check for pending session extractions and output a prompt for Claude to process.

This command is designed to be called from a SessionStart hook. If there are
pending sessions (queued by 'ao forge --queue'), it outputs a structured prompt
that asks Claude to extract learnings and write them to .agents/learnings/.

The prompt includes:
  - Session summary and context
  - Key decisions and knowledge snippets
  - Clear instructions for Claude to extract 1-3 learnings
  - File path where learnings should be written

If no pending extractions exist, outputs nothing (silent).

Examples:
  ao extract                    # Process most recent pending extraction
  ao extract --all              # Process all pending extractions
  ao extract --all --dry-run    # Preview what would be processed
  ao extract --all --json      # Process all with JSON output
  ao extract --clear            # Clear pending queue without processing
  ao extract --max-content 4000 # Limit content size`,
	RunE: runExtract,
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().IntVar(&extractMaxContent, "max-content", 3000, "Maximum characters of session content to include")
	extractCmd.Flags().BoolVar(&extractClear, "clear", false, "Clear pending queue without processing")
	extractCmd.Flags().BoolVar(&extractAll, "all", false, "Process all pending entries")
	extractCmd.Flags().StringVar(&extractBead, "bead", "", "Bead ID to tag extracted learnings with")
}

func runExtract(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	pendingPath := filepath.Join(cwd, storage.DefaultBaseDir, "pending.jsonl")

	pending, err := loadPendingOrNil(pendingPath)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	if extractClear {
		return clearPendingFile(pendingPath, len(pending))
	}

	if extractAll {
		return runExtractAll(pendingPath, pending, cwd)
	}

	return extractMostRecent(pendingPath, pending, cwd)
}

// loadPendingOrNil reads the pending file if it exists. Returns nil slice
// (no error) when the file is missing or empty.
func loadPendingOrNil(pendingPath string) ([]PendingExtraction, error) {
	if _, err := os.Stat(pendingPath); os.IsNotExist(err) {
		return nil, nil
	}
	pending, err := readPendingExtractions(pendingPath)
	if err != nil {
		return nil, fmt.Errorf("read pending: %w", err)
	}
	return pending, nil
}

// clearPendingFile removes the pending file and reports how many entries were cleared.
func clearPendingFile(pendingPath string, count int) error {
	if err := os.Remove(pendingPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear pending: %w", err)
	}
	fmt.Printf("Cleared %d pending extraction(s)\n", count)
	return nil
}

// extractMostRecent processes only the last pending entry and removes it.
func extractMostRecent(pendingPath string, pending []PendingExtraction, cwd string) error {
	extraction := pending[len(pending)-1]
	if extractBead != "" && extraction.BeadID == "" {
		extraction.BeadID = extractBead
	}
	outputExtractionPrompt(extraction, cwd, extractMaxContent)
	if err := removePendingEntry(pendingPath, pending, len(pending)-1); err != nil {
		VerbosePrintf("Warning: failed to update pending file: %v\n", err)
	}
	return nil
}

// runExtractAll processes all pending extractions.
func runExtractAll(pendingPath string, pending []PendingExtraction, cwd string) error {
	if GetDryRun() {
		return outputExtractDryRun(pending)
	}

	processed := processAllExtractions(pending, cwd)

	remaining := filterUnprocessed(pending, processed)

	if err := rewritePendingFile(pendingPath, remaining); err != nil {
		return fmt.Errorf("update pending file: %w", err)
	}

	return outputExtractBatchResult(processed, 0, remaining)
}

// outputExtractDryRun prints what would be processed without doing work.
func outputExtractDryRun(pending []PendingExtraction) error {
	if GetOutput() == "json" {
		result := ExtractBatchResult{
			Processed: 0,
			Failed:    0,
			Remaining: len(pending),
			Entries:   make([]string, len(pending)),
		}
		for i, p := range pending {
			result.Entries[i] = p.SessionID
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal extract result: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Would process %d pending extraction(s):\n", len(pending))
	for _, p := range pending {
		fmt.Printf("  - %s (%s)\n", p.SessionID, p.Summary)
	}
	return nil
}

// processAllExtractions outputs the extraction prompt for every pending entry
// and returns the list of session IDs that were processed.
func processAllExtractions(pending []PendingExtraction, cwd string) []string {
	var processed []string
	for i, extraction := range pending {
		VerbosePrintf("Processing %d/%d: %s\n", i+1, len(pending), extraction.SessionID)
		if extractBead != "" && extraction.BeadID == "" {
			extraction.BeadID = extractBead
		}
		outputExtractionPrompt(extraction, cwd, extractMaxContent)
		processed = append(processed, extraction.SessionID)
	}
	return processed
}

// filterUnprocessed returns pending entries whose SessionID is not in processed.
func filterUnprocessed(pending []PendingExtraction, processed []string) []PendingExtraction {
	processedSet := make(map[string]bool, len(processed))
	for _, sid := range processed {
		processedSet[sid] = true
	}
	var remaining []PendingExtraction
	for _, entry := range pending {
		if !processedSet[entry.SessionID] {
			remaining = append(remaining, entry)
		}
	}
	return remaining
}

// outputExtractBatchResult prints the batch extraction summary as JSON or text.
func outputExtractBatchResult(processed []string, failed int, remaining []PendingExtraction) error {
	if GetOutput() == "json" {
		result := ExtractBatchResult{
			Processed: len(processed),
			Failed:    failed,
			Remaining: len(remaining),
			Entries:   processed,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal extract batch result: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Processed: %d, Failed: %d, Remaining: %d\n", len(processed), failed, len(remaining))
	return nil
}

// removePendingEntry removes a single entry from pending file using flock.
func removePendingEntry(pendingPath string, pending []PendingExtraction, indexToRemove int) error {
	// Build new list without the removed entry
	remaining := make([]PendingExtraction, 0, len(pending)-1)
	for i, entry := range pending {
		if i != indexToRemove {
			remaining = append(remaining, entry)
		}
	}

	return rewritePendingFile(pendingPath, remaining)
}

// rewritePendingFile rewrites the pending file with the given entries using flock.
func rewritePendingFile(pendingPath string, entries []PendingExtraction) error {
	// Ensure directory exists
	dir := filepath.Dir(pendingPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Open file WITHOUT truncation — truncate after acquiring lock
	f, err := os.OpenFile(pendingPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open pending file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Acquire exclusive lock
	if err := flockLock(f); err != nil {
		return fmt.Errorf("lock pending file: %w", err)
	}
	defer func() {
		_ = flockUnlock(f)
	}()

	// NOW truncate after we hold the lock
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncate pending file: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("seek pending file: %w", err)
	}

	// Write each entry as JSONL
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal entry: %w", err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write entry: %w", err)
		}
	}

	return nil
}

func readPendingExtractions(path string) (pending []PendingExtraction, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var p PendingExtraction
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			continue // Skip malformed lines
		}
		pending = append(pending, p)
	}

	return pending, scanner.Err()
}

func outputExtractionPrompt(extraction PendingExtraction, cwd string, maxContent int) {
	// Generate output file path
	date := extraction.QueuedAt.Format("2006-01-02")
	shortID := extraction.SessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	outputPath := filepath.Join(cwd, ".agents", "learnings", fmt.Sprintf("%s-%s.md", date, shortID))

	fmt.Println("---")
	fmt.Println("# Knowledge Extraction Request")
	fmt.Println()
	fmt.Println("A previous session has been queued for learning extraction. Please process it.")
	fmt.Println()
	fmt.Println("## Session Context")
	fmt.Println()
	fmt.Printf("- **Session ID**: %s\n", extraction.SessionID)
	fmt.Printf("- **Date**: %s\n", extraction.QueuedAt.Format("2006-01-02 15:04"))
	fmt.Printf("- **Summary**: %s\n", extraction.Summary)
	fmt.Println()

	// Include decisions if present
	if len(extraction.Decisions) > 0 {
		fmt.Println("## Key Decisions")
		fmt.Println()
		charCount := 0
		for i, d := range extraction.Decisions {
			if charCount > maxContent/2 {
				fmt.Printf("- ... and %d more\n", len(extraction.Decisions)-i)
				break
			}
			fmt.Printf("- %s\n", truncateForPrompt(d, 200))
			charCount += len(d)
		}
		fmt.Println()
	}

	// Include knowledge snippets if present
	if len(extraction.Knowledge) > 0 {
		fmt.Println("## Knowledge Snippets")
		fmt.Println()
		charCount := 0
		for i, k := range extraction.Knowledge {
			if charCount > maxContent/2 {
				fmt.Printf("- ... and %d more\n", len(extraction.Knowledge)-i)
				break
			}
			fmt.Printf("- %s\n", truncateForPrompt(k, 200))
			charCount += len(k)
		}
		fmt.Println()
	}

	fmt.Println("## Your Task")
	fmt.Println()
	fmt.Println("Extract **1-3 actionable learnings** from this session and write them to:")
	fmt.Println()
	fmt.Printf("```\n%s\n```\n", outputPath)
	fmt.Println()
	fmt.Println("### Learning Format")
	fmt.Println()
	fmt.Println("Use this markdown format for each learning:")
	fmt.Println()
	fmt.Println("```markdown")
	fmt.Println("---")
	fmt.Printf("id: learn-%s-[unique-suffix]\n", time.Now().Format("2006-01-02"))
	fmt.Println("type: learning           # learning | failure | decision | solution | reference")
	fmt.Printf("created_at: \"%s\"\n", time.Now().Format(time.RFC3339))
	fmt.Println("category: architecture   # architecture | debugging | process | testing | security")
	fmt.Println("confidence: high         # high | medium | low")
	fmt.Printf("source_session: \"%s\"\n", extraction.SessionID)
	if extraction.BeadID != "" {
		fmt.Printf("source_bead: \"%s\"\n", extraction.BeadID)
	} else {
		fmt.Println("source_bead: \"\"           # bead ID if working on a tracked issue")
	}
	fmt.Println("source_phase: \"\"          # research | plan | implement | validate")
	fmt.Println("---")
	fmt.Println()
	fmt.Println("# Learning: [Short Title]")
	fmt.Println()
	fmt.Println("## What We Learned")
	fmt.Println()
	fmt.Println("[1-2 sentences describing the insight]")
	fmt.Println()
	fmt.Println("## Why It Matters")
	fmt.Println()
	fmt.Println("[1 sentence on impact/value]")
	fmt.Println("```")
	fmt.Println()
	fmt.Println("### Guidelines")
	fmt.Println()
	fmt.Println("- Only extract learnings that would help **future sessions**")
	fmt.Println("- Skip trivial or context-specific details")
	fmt.Println("- Focus on: debugging insights, architectural decisions, process improvements")
	fmt.Println("- If nothing worth extracting, do NOT create any file. Simply skip extraction entirely.")
	fmt.Println()
	fmt.Println("**After writing the file, continue with your normal work.**")
	fmt.Println("---")
}

func truncateForPrompt(s string, maxLen int) string {
	if maxLen < 4 {
		return s
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ") // Normalize whitespace
	// Fast path: byte length is an upper bound on rune count.
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
