package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/pool"
)

// poolReindexResult is the JSON shape emitted by `ao pool reindex --json`.
type poolReindexResult struct {
	Scanned        int      `json:"scanned"`
	NewEntries     int      `json:"new_entries"`
	ExistingHashes int      `json:"existing_hashes"`
	AlreadyIndexed int      `json:"already_indexed"`
	Skipped        int      `json:"skipped"`
	Errors         []string `json:"errors,omitempty"`
	DryRun         bool     `json:"dry_run"`
	IndexPath      string   `json:"index_path"`
}

var (
	poolReindexDryRun bool
	poolReindexJSON   bool
)

var poolReindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Backfill promoted-index.jsonl from on-disk learnings/patterns",
	Long: `Walk .agents/learnings/*.md and .agents/patterns/*.md, compute the
content hash that Promote would have recorded for each surviving artifact,
and append missing entries to .agents/pool/promoted-index.jsonl.

This rebuilds the dedup sidecar for artifacts that pre-date the index, so
future Promote calls correctly collapse against pre-existing canonical
content (and the post-dedup pass leaves no holes in the sidecar).

Existing index entries are preserved; only missing hashes are appended. Use
--dry-run to preview counts without writing.

Examples:
  ao pool reindex
  ao pool reindex --dry-run
  ao pool reindex --json`,
	RunE: runPoolReindex,
}

func init() {
	poolReindexCmd.Flags().BoolVar(&poolReindexDryRun, "dry-run", false, "Print counts only; do not write to the index")
	poolReindexCmd.Flags().BoolVar(&poolReindexJSON, "json", false, "Emit structured JSON output")
	poolCmd.AddCommand(poolReindexCmd)
}

func runPoolReindex(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	res, err := poolReindexRun(cwd, poolReindexDryRun)
	if err != nil {
		return err
	}
	return writePoolReindexResult(cmd.OutOrStdout(), res, poolReindexJSON)
}

// poolReindexRun is the L2 entry point: walk surviving artifacts under cwd,
// compare against the existing promoted-index, and append missing entries
// (unless dryRun).
func poolReindexRun(cwd string, dryRun bool) (poolReindexResult, error) {
	p := pool.NewPool(cwd)
	res := poolReindexResult{
		DryRun:    dryRun,
		IndexPath: p.PromotedIndexPath(),
	}

	existing, err := loadExistingPromotedHashes(p.PromotedIndexPath())
	if err != nil {
		return res, fmt.Errorf("read existing index: %w", err)
	}
	res.ExistingHashes = len(existing)

	files, err := collectReindexFiles(cwd)
	if err != nil {
		return res, fmt.Errorf("walk artifacts: %w", err)
	}
	res.Scanned = len(files)

	// Track hashes we would add so duplicate bodies under different filenames
	// only produce one new index entry per reindex run.
	addedThisRun := make(map[string]struct{}, len(files))

	for _, f := range files {
		body, ok := extractPromotedArtifactBody(f)
		if !ok {
			res.Skipped++
			continue
		}
		hash := pool.ContentHash(body)
		if _, found := existing[hash]; found {
			res.AlreadyIndexed++
			continue
		}
		if _, found := addedThisRun[hash]; found {
			res.AlreadyIndexed++
			continue
		}

		if !dryRun {
			candidateID := strings.TrimSuffix(filepath.Base(f), ".md")
			if appendErr := p.AppendPromotedIndexEntry(hash, f, candidateID); appendErr != nil {
				res.Errors = append(res.Errors,
					fmt.Sprintf("append %s: %v", filepath.Base(f), appendErr))
				continue
			}
		}
		addedThisRun[hash] = struct{}{}
		res.NewEntries++
	}

	return res, nil
}

// collectReindexFiles walks .agents/learnings and .agents/patterns under cwd,
// returning all *.md files. Sorted for deterministic output.
func collectReindexFiles(cwd string) ([]string, error) {
	dirs := []string{
		filepath.Join(cwd, ".agents", "learnings"),
		filepath.Join(cwd, ".agents", "patterns"),
	}
	var files []string
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	return files, nil
}

// loadExistingPromotedHashes reads the sidecar and returns the set of
// content_hash values already indexed. Missing file → empty set, no error.
// Malformed lines are skipped (matches lookupPromotedHash behavior).
func loadExistingPromotedHashes(indexPath string) (map[string]string, error) {
	hashes := make(map[string]string)
	f, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return hashes, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		var entry pool.PromotedIndexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.ContentHash == "" {
			continue
		}
		hashes[entry.ContentHash] = entry.ArtifactPath
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hashes, nil
}

// extractPromotedArtifactBody pulls the candidate-content body from a
// promoted artifact written by Pool.writeArtifact. The artifact format is:
//
//	---
//	<frontmatter>
//	---
//
//	# <Heading>: <title>
//
//	## What We Learned
//
//	<candidate.Content>
//
//	## Context | ## Source | (EOF)
//
// We extract the text between "## What We Learned" and the next "## "
// heading (or EOF). For artifacts that don't follow this template (older
// hand-authored learnings), we fall back to the post-frontmatter body so
// reindex still records *some* hash — that hash won't collide with a fresh
// Promote, but it preserves the "no holes" property of the sidecar for
// already-on-disk content. Returns (body, true) on success, ("", false) if
// the file is unreadable.
func extractPromotedArtifactBody(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	text := string(data)

	postFrontmatter := stripYAMLFrontmatter(text)
	if section, ok := extractSection(postFrontmatter, "## What We Learned"); ok {
		return section, true
	}
	// Fallback: strip the heading line if present, return the rest.
	body := strings.TrimSpace(postFrontmatter)
	if body == "" {
		return "", false
	}
	return body, true
}

// stripYAMLFrontmatter returns the document body after a leading ---/--- block.
// If the document doesn't open with a frontmatter delimiter, returns text
// unchanged.
func stripYAMLFrontmatter(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return text
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return text
}

// extractSection returns the body under a given "## Heading" line, stopping
// at the next "## " heading or end of document. Returns (body, true) if the
// heading was found.
func extractSection(text, heading string) (string, bool) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return "", false
	}
	end := len(lines)
	for j := start; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if strings.HasPrefix(trimmed, "## ") {
			end = j
			break
		}
	}
	body := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if body == "" {
		return "", false
	}
	return body, true
}

// writePoolReindexResult prints the result either as JSON or human-readable.
func writePoolReindexResult(w io.Writer, res poolReindexResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	mode := "live"
	if res.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(w, "Pool Reindex (%s)\n", mode)
	fmt.Fprintf(w, "==================\n")
	fmt.Fprintf(w, "Index path:        %s\n", res.IndexPath)
	fmt.Fprintf(w, "Existing hashes:   %d\n", res.ExistingHashes)
	fmt.Fprintf(w, "Artifacts scanned: %d\n", res.Scanned)
	fmt.Fprintf(w, "New entries:       %d\n", res.NewEntries)
	fmt.Fprintf(w, "Already indexed:   %d\n", res.AlreadyIndexed)
	fmt.Fprintf(w, "Skipped:           %d\n", res.Skipped)
	if len(res.Errors) > 0 {
		fmt.Fprintf(w, "Errors:            %d\n", len(res.Errors))
		for _, e := range res.Errors {
			fmt.Fprintf(w, "  - %s\n", e)
		}
	}
	if res.DryRun && res.NewEntries > 0 {
		fmt.Fprintln(w, "\n(dry-run: nothing written; rerun without --dry-run to apply)")
	}
	return nil
}

// poolReindexNow exists so tests can stamp deterministic timestamps if
// needed (the sidecar entry includes PromotedAt; we don't expose it in the
// CLI surface).
var poolReindexNow = time.Now //nolint:unused // future use; documents intent
