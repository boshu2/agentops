package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/formatter"
	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/storage"
)

var forgeBatchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Process multiple transcripts at once",
	Long: `Find and process pending transcripts in bulk.

Scans standard Claude Code transcript locations, processes each through
the forge extraction pipeline, and deduplicates similar learnings.

Examples:
  ao forge batch                    # Process all pending transcripts
  ao forge batch --dry-run          # List what would be processed
  ao forge batch --dir ~/.claude/projects/my-project
  ao forge batch --max 10           # Process up to 10 transcripts
  ao forge batch --extract          # Trigger extraction after forging`,
	RunE: runForgeBatch,
}

var (
	batchDir     string
	batchExtract bool
	batchMax     int
)

func init() {
	forgeCmd.AddCommand(forgeBatchCmd)
	forgeBatchCmd.Flags().StringVar(&batchDir, "dir", "", "Specific directory to scan (default: all Claude project dirs)")
	forgeBatchCmd.Flags().BoolVar(&batchExtract, "extract", false, "Trigger extraction after forging")
	forgeBatchCmd.Flags().IntVar(&batchMax, "max", 0, "Maximum transcripts to process (0 = all)")
}

// loadAndFilterTranscripts discovers transcripts, filters already-forged ones, and applies the max limit.
func loadAndFilterTranscripts(cwd, specificDir string, maxCount int) ([]transcriptCandidate, int, string, error) {
	transcripts, err := findPendingTranscripts(specificDir)
	if err != nil {
		return nil, 0, "", fmt.Errorf("find transcripts: %w", err)
	}
	if len(transcripts) == 0 {
		return nil, 0, "", nil
	}

	forgedIndexPath := filepath.Join(cwd, storage.DefaultBaseDir, "forged.jsonl")
	forgedSet, err := loadForgedIndex(forgedIndexPath)
	if err != nil {
		return nil, 0, "", fmt.Errorf("load forged index: %w", err)
	}

	unforged, skipped := filterUnforgedTranscripts(transcripts, forgedSet)
	if maxCount > 0 && len(unforged) > maxCount {
		unforged = unforged[:maxCount]
	}
	return unforged, skipped, forgedIndexPath, nil
}

// batchForgeAccumulator collects results across transcript processing.
type batchForgeAccumulator struct {
	processed      int
	failed         int
	totalDecisions int
	totalKnowledge int
	allKnowledge   []string
	allDecisions   []string
	processedPaths []string
}

// accumulate records the result of processing a single transcript.
func (a *batchForgeAccumulator) accumulate(ok bool, decisions, knowledge []string, path string) {
	if !ok {
		a.failed++
		return
	}
	a.processed++
	a.totalDecisions += len(decisions)
	a.totalKnowledge += len(knowledge)
	a.allKnowledge = append(a.allKnowledge, knowledge...)
	a.allDecisions = append(a.allDecisions, decisions...)
	a.processedPaths = append(a.processedPaths, path)
}

func runForgeBatch(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	unforged, skippedCount, forgedIndexPath, err := loadAndFilterTranscripts(cwd, batchDir, batchMax)
	if err != nil {
		return err
	}
	if len(unforged) == 0 {
		fmt.Println("No pending transcripts found.")
		return nil
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would process %d transcript(s) (skipped %d):\n", len(unforged), skippedCount)
		for _, t := range unforged {
			fmt.Printf("  - %s (%s)\n", t.path, humanSize(t.size))
		}
		return nil
	}

	fmt.Printf("Found %d transcript(s) to process (skipped %d already forged).\n", len(unforged), skippedCount)

	baseDir := filepath.Join(cwd, storage.DefaultBaseDir)
	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(formatter.NewMarkdownFormatter(), formatter.NewJSONLFormatter()),
	)
	if err := fs.Init(); err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}

	p := parser.NewParser()
	p.MaxContentLength = 0
	extractor := parser.NewExtractor()

	var acc batchForgeAccumulator
	for i, t := range unforged {
		ok, decisions, knowledge, path := forgeSingleTranscript(i, len(unforged), t, fs, p, extractor, forgedIndexPath)
		acc.accumulate(ok, decisions, knowledge, path)
	}

	dedupedKnowledge := dedupSimilar(acc.allKnowledge)
	dedupedDecisions := dedupSimilar(acc.allDecisions)
	totalDupsRemoved := (len(acc.allKnowledge) - len(dedupedKnowledge)) + (len(acc.allDecisions) - len(dedupedDecisions))

	totalExtracted := runBatchExtractionStep(cwd, acc.processed)

	return outputBatchForgeResult(baseDir, acc.processed, skippedCount, acc.failed, acc.totalDecisions, acc.totalKnowledge, totalDupsRemoved, totalExtracted, dedupedKnowledge, dedupedDecisions, acc.processedPaths)
}

// filterUnforgedTranscripts returns transcripts not yet in the forged index, plus a skip count.
func filterUnforgedTranscripts(transcripts []transcriptCandidate, forgedSet map[string]bool) ([]transcriptCandidate, int) {
	var unforged []transcriptCandidate
	skipped := 0
	for _, t := range transcripts {
		if forgedSet[t.path] {
			skipped++
			VerbosePrintf("Skipping already-forged: %s\n", t.path)
		} else {
			unforged = append(unforged, t)
		}
	}
	return unforged, skipped
}

// forgeSingleTranscript processes one transcript through the forge pipeline.
// Returns (ok, decisions, knowledge, path).
func forgeSingleTranscript(i, total int, t transcriptCandidate, fs *storage.FileStorage, p *parser.Parser, extractor *parser.Extractor, forgedIndexPath string) (bool, []string, []string, string) {
	fmt.Printf("[%d/%d] Processing %s...\n", i+1, total, filepath.Base(t.path))

	session, err := processTranscript(t.path, p, extractor, false, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: skipping %s: %v\n", t.path, err)
		return false, nil, nil, ""
	}

	sessionPath, err := fs.WriteSession(session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: failed to write session for %s: %v\n", t.path, err)
		return false, nil, nil, ""
	}

	indexEntry := &storage.IndexEntry{
		SessionID:   session.ID,
		Date:        session.Date,
		SessionPath: sessionPath,
		Summary:     session.Summary,
	}
	if err := fs.WriteIndex(indexEntry); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: failed to index session: %v\n", err)
	}

	provRecord := &storage.ProvenanceRecord{
		ID:            fmt.Sprintf("prov-%s", session.ID[:7]),
		ArtifactPath:  sessionPath,
		WorkspacePath: workspacePathFromAgentArtifactPath(sessionPath),
		ArtifactType:  "session",
		SourcePath:    t.path,
		SourceType:    "transcript",
		SessionID:     session.ID,
		CreatedAt:     time.Now(),
	}
	if err := fs.WriteProvenance(provRecord); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: failed to write provenance: %v\n", err)
	}

	forgedRecord := ForgedRecord{Path: t.path, ForgedAt: time.Now(), Session: session.ID}
	if err := appendForgedRecord(forgedIndexPath, forgedRecord); err != nil {
		VerbosePrintf("  Warning: failed to record forged transcript: %v\n", err)
	}

	VerbosePrintf("  -> %d decisions, %d learnings\n", len(session.Decisions), len(session.Knowledge))
	return true, session.Decisions, session.Knowledge, t.path
}

// runBatchExtractionStep triggers extraction when --extract is set and transcripts were processed.
func runBatchExtractionStep(cwd string, totalProcessed int) int {
	if !batchExtract || totalProcessed == 0 {
		return 0
	}
	fmt.Printf("\nTriggering extraction for %d session(s)...\n", totalProcessed)
	extractedCount, extractErr := triggerExtraction(cwd)
	if extractErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: extraction failed: %v\n", extractErr)
		return 0
	}
	return extractedCount
}

// outputBatchForgeResult prints or JSON-encodes the batch forge summary.
func outputBatchForgeResult(baseDir string, processed, skipped, failed, decisions, knowledge, dupsRemoved, extracted int, dedupedKnowledge, dedupedDecisions, processedPaths []string) error {
	if GetOutput() == "json" {
		result := BatchForgeResult{
			Forged:    processed,
			Skipped:   skipped,
			Failed:    failed,
			Extracted: extracted,
			Paths:     processedPaths,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal batch forge result: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("\n--- Batch Forge Summary ---\n")
	fmt.Printf("Transcripts processed: %d\n", processed)
	fmt.Printf("Skipped (already):     %d\n", skipped)
	fmt.Printf("Failed:                %d\n", failed)
	fmt.Printf("Decisions extracted:   %d\n", decisions)
	fmt.Printf("Learnings extracted:   %d\n", knowledge)
	fmt.Printf("Duplicates removed:    %d\n", dupsRemoved)
	fmt.Printf("Unique decisions:      %d\n", len(dedupedDecisions))
	fmt.Printf("Unique learnings:      %d\n", len(dedupedKnowledge))
	if extracted > 0 {
		fmt.Printf("Extractions processed: %d\n", extracted)
	}
	fmt.Printf("Output:                %s\n", baseDir)
	return nil
}

// transcriptCandidate represents a discovered transcript file.
type transcriptCandidate struct {
	path    string
	modTime time.Time
	size    int64
}

// ForgedRecord represents a forged transcript entry.
type ForgedRecord struct {
	Path     string    `json:"path"`
	ForgedAt time.Time `json:"forged_at"`
	Session  string    `json:"session,omitempty"`
}

// BatchForgeResult holds results from batch forge operation.
type BatchForgeResult struct {
	Forged    int      `json:"forged"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	Extracted int      `json:"extracted,omitempty"`
	Paths     []string `json:"paths"`
}

// resolveSearchDirs returns directories to scan for transcripts.
func resolveSearchDirs(specificDir string) ([]string, error) {
	if specificDir != "" {
		return []string{specificDir}, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}
	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}
	return []string{projectsDir}, nil
}

// isBatchTranscriptCandidate reports whether a file entry qualifies as a transcript candidate.
func isBatchTranscriptCandidate(info os.FileInfo, path string) bool {
	return !info.IsDir() && filepath.Ext(path) == ".jsonl" && info.Size() > 100
}

// findPendingTranscripts discovers JSONL transcript files in Claude project directories.
func findPendingTranscripts(specificDir string) ([]transcriptCandidate, error) {
	searchDirs, err := resolveSearchDirs(specificDir)
	if err != nil || len(searchDirs) == 0 {
		return nil, err
	}

	var candidates []transcriptCandidate
	for _, dir := range searchDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() && info.Name() == "subagents" {
				return filepath.SkipDir
			}
			if isBatchTranscriptCandidate(info, path) {
				candidates = append(candidates, transcriptCandidate{
					path:    path,
					modTime: info.ModTime(),
					size:    info.Size(),
				})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, err)
		}
	}

	slices.SortFunc(candidates, func(a, b transcriptCandidate) int {
		return a.modTime.Compare(b.modTime)
	})

	return candidates, nil
}

// dedupSimilar removes exact duplicates and near-duplicates from a string slice.
// Near-duplicates are detected by comparing normalized prefixes.
func dedupSimilar(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(items))

	for _, item := range items {
		key := normalizeForDedup(item)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// normalizeForDedup creates a normalized key for deduplication using content hashing.
// Lowercases, collapses whitespace, strips ellipsis, then SHA256 hashes the full
// normalized content. This avoids false positives from naive prefix truncation.
func normalizeForDedup(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "...")
	s = strings.Join(strings.Fields(s), " ")
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// humanSize returns a human-readable file size string.
func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMG"[exp])
}

// loadForgedIndex reads the forged.jsonl index and returns a set of forged paths.
func loadForgedIndex(path string) (map[string]bool, error) {
	forgedSet := make(map[string]bool)

	// If file doesn't exist, return empty set
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return forgedSet, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open forged index: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record ForgedRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			VerbosePrintf("Warning: skipping malformed forged record: %v\n", err)
			continue
		}

		forgedSet[record.Path] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan forged index: %w", err)
	}

	return forgedSet, nil
}

// appendForgedRecord appends a forged record to the index using flock.
func appendForgedRecord(path string, record ForgedRecord) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Open file for append with exclusive lock
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open forged index: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Acquire exclusive lock
	if err := flockLock(f); err != nil {
		return fmt.Errorf("lock forged index: %w", err)
	}
	defer func() {
		_ = flockUnlock(f)
	}()

	// Marshal and write record
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write record: %w", err)
	}

	return nil
}

// triggerExtraction runs extraction for all pending sessions.
func triggerExtraction(cwd string) (int, error) {
	pendingPath := filepath.Join(cwd, storage.DefaultBaseDir, "pending.jsonl")

	// Check if pending file exists
	if _, err := os.Stat(pendingPath); os.IsNotExist(err) {
		return 0, nil // No pending extractions
	}

	// Read pending extractions
	pending, err := readPendingExtractions(pendingPath)
	if err != nil {
		return 0, fmt.Errorf("read pending: %w", err)
	}

	if len(pending) == 0 {
		return 0, nil
	}

	// Call the existing runExtractAll function from extract.go
	if err := runExtractAll(pendingPath, pending, cwd); err != nil {
		return 0, err
	}

	return len(pending), nil
}
