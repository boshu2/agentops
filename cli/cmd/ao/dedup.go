package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/spf13/cobra"
)

var (
	dedupMerge bool
	dedupYes   bool
)

// dedupConfirmThreshold is the number of files-to-archive above which `ao dedup
// --merge` pauses for interactive confirmation. Override via env
// AGENTOPS_DEDUP_CONFIRM_THRESHOLD. The Council post-mortem flagged that
// archiving 14,376 files in one shot, with no inventory, made "reversible"
// claims a fiction. The pause is a backstop on top of the manifest.
const dedupConfirmThreshold = 1000

// dedupConfirmReader is the reader used to read the y/N response. Tests can
// swap this to feed scripted input.
var dedupConfirmReader io.Reader = os.Stdin

// DedupGroup is an alias for the lifecycle type, kept for cmd/ao compatibility.
type DedupGroup = lifecycle.DedupGroup

// DedupResult is an alias for the lifecycle type, kept for cmd/ao compatibility.
type DedupResult = lifecycle.DedupResult

var dedupCmd = &cobra.Command{
	Use:   "dedup",
	Short: "Detect near-duplicate learnings",
	Long: `Scan learnings and patterns for near-duplicates using normalized content hashing.

Reads all files (.md and .jsonl) from .agents/learnings/ and .agents/patterns/,
extracts body content, normalizes (lowercase, collapse whitespace, strip markdown
formatting), and groups by SHA256 hash. Groups with more than one member are
duplicates. The patterns directory is optional — if it does not exist, only
learnings are scanned.

With --merge, automatically resolves each duplicate group by keeping the file
with the highest utility (from YAML frontmatter or JSON) and archiving the
rest to .agents/archive/dedup/. Files without a utility field default to 0.5.
A JSON manifest of the operation is written to
.agents/archive/dedup/<UTC-timestamp>-manifest.json BEFORE any moves so the
operation is fully reversible.

If --merge would archive more than 1000 files, the command pauses for
interactive confirmation. Use --yes to skip the prompt (for hooks/CI).
Override the threshold via AGENTOPS_DEDUP_CONFIRM_THRESHOLD.

Examples:
  ao dedup
  ao dedup --json
  ao dedup --merge
  ao dedup --merge --yes`,
	RunE: runDedup,
}

func init() {
	dedupCmd.Flags().BoolVar(&dedupMerge, "merge", false, "Auto-resolve duplicates: keep highest utility, archive the rest")
	dedupCmd.Flags().BoolVar(&dedupYes, "yes", false, "Skip the interactive confirmation prompt for large merges (for hooks/CI)")
	dedupCmd.GroupID = "core"
	rootCmd.AddCommand(dedupCmd)
}

// Thin wrappers preserved for tests.
func collectDedupFiles(cwd string) ([]string, error) { return lifecycle.CollectDedupFiles(cwd) }
func groupByContentHash(files []string) map[string][]string {
	return lifecycle.GroupByContentHash(files)
}
func mergeDedupGroups(hashToFiles map[string][]string, cwd string, dryRun bool) error {
	return lifecycle.MergeDedupGroups(hashToFiles, cwd, dryRun)
}
func buildDedupResult(hashToFiles map[string][]string, totalFiles int, cwd string) DedupResult {
	return lifecycle.BuildDedupResult(hashToFiles, totalFiles, cwd)
}
func pickHighestUtility(files []string) (string, []string) {
	return lifecycle.PickHighestUtility(files)
}
func readUtilityFromFile(path string) float64 { return lifecycle.ReadUtilityFromFile(path) }
func readUtilityFromFrontmatter(text string, defaultVal float64) float64 {
	return lifecycle.ReadUtilityFromFrontmatter(text, defaultVal)
}
func readUtilityFromJSONL(text string, defaultVal float64) float64 {
	return lifecycle.ReadUtilityFromJSONL(text, defaultVal)
}
func extractLearningBody(path string) string { return lifecycle.ExtractLearningBody(path) }
func extractMarkdownBody(text string) string { return lifecycle.ExtractMarkdownBody(text) }
func extractJSONLBody(text string) string    { return lifecycle.ExtractJSONLBody(text) }
func hashNormalizedContent(body string) string {
	return lifecycle.HashNormalizedContent(body)
}

// resolveDedupConfirmThreshold returns the active threshold, honoring the env
// override AGENTOPS_DEDUP_CONFIRM_THRESHOLD when set to a positive integer.
func resolveDedupConfirmThreshold() int {
	if v := strings.TrimSpace(os.Getenv("AGENTOPS_DEDUP_CONFIRM_THRESHOLD")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return dedupConfirmThreshold
}

// confirmLargeMerge prompts the operator to confirm a merge that would archive
// more than the threshold of files. Returns true if the user typed y/yes,
// false otherwise. Reads from dedupConfirmReader (stdin by default; tests
// substitute a scripted reader).
func confirmLargeMerge(archiveCount int, manifestPath string) bool {
	fmt.Printf("\n*** Large merge detected ***\n")
	fmt.Printf("This operation would archive %d file(s) (>%d threshold).\n", archiveCount, resolveDedupConfirmThreshold())
	fmt.Printf("Manifest will be written to: %s\n", manifestPath)
	fmt.Printf("Continue? [y/N]: ")
	scanner := bufio.NewScanner(dedupConfirmReader)
	if !scanner.Scan() {
		return false
	}
	resp := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return resp == "y" || resp == "yes"
}

func runDedup(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	files, err := lifecycle.CollectDedupFiles(cwd)
	if err != nil {
		return err
	}
	if files == nil {
		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(DedupResult{})
		}
		fmt.Println("No learnings or patterns directory found.")
		return nil
	}
	if len(files) == 0 {
		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(DedupResult{})
		}
		fmt.Println("No learning or pattern files found.")
		return nil
	}

	hashToFiles := lifecycle.GroupByContentHash(files)
	result := lifecycle.BuildDedupResult(hashToFiles, len(files), cwd)

	if dedupMerge && result.DuplicateGroups > 0 {
		archiveCount := lifecycle.CountArchiveCandidates(hashToFiles)
		threshold := resolveDedupConfirmThreshold()
		if !GetDryRun() && !dedupYes && archiveCount > threshold {
			previewPath := relPath(cwd, lifecycle.DedupManifestPath(cwd, time.Now()))
			if !confirmLargeMerge(archiveCount, previewPath) {
				fmt.Println("Aborted.")
				return nil
			}
		}
		return lifecycle.MergeDedupGroups(hashToFiles, cwd, GetDryRun())
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Dedup Scan Results\n")
	fmt.Printf("==================\n")
	fmt.Printf("Total files:       %d\n", result.TotalFiles)
	fmt.Printf("Unique content:    %d\n", result.UniqueContent)
	fmt.Printf("Duplicate groups:  %d\n", result.DuplicateGroups)
	fmt.Printf("Duplicate files:   %d\n", result.DuplicateFiles)

	if result.DuplicateGroups > 0 {
		fmt.Println("\nDuplicate Groups:")
		for _, g := range result.Groups {
			fmt.Printf("\n  Hash: %s (%d files)\n", g.Hash, g.Count)
			for _, f := range g.Files {
				fmt.Printf("    - %s\n", f)
			}
		}
	} else {
		fmt.Println("\nNo duplicates found.")
	}

	return nil
}
