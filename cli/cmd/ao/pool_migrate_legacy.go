package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	poolMigrateLegacySourceDir  string
	poolMigrateLegacyPendingDir string
)

type legacyMove struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type poolMigrateLegacyResult struct {
	Scanned  int          `json:"scanned"`
	Eligible int          `json:"eligible"`
	Moved    int          `json:"moved"`
	Skipped  int          `json:"skipped"`
	Errors   int          `json:"errors"`
	Moves    []legacyMove `json:"moves,omitempty"`
}

var poolMigrateLegacyCmd = &cobra.Command{
	Use:   "migrate-legacy",
	Short: "Move legacy knowledge captures into pending",
	Long: `Move legacy knowledge captures from .agents/knowledge/*.md into
.agents/knowledge/pending/ so they flow through the same ingest path.

Only markdown files that parse as ingestible learning captures are moved.
Files already in pending/processed are not scanned by this command.

Examples:
  ao pool migrate-legacy
  ao pool migrate-legacy --source-dir .agents/knowledge --pending-dir .agents/knowledge/pending
  ao pool migrate-legacy --dry-run -o json`,
	RunE: runPoolMigrateLegacy,
}

func init() {
	poolCmd.AddCommand(poolMigrateLegacyCmd)
	poolMigrateLegacyCmd.Flags().StringVar(&poolMigrateLegacySourceDir, "source-dir", filepath.Join(".agents", "knowledge"), "Source directory containing legacy markdown captures")
	poolMigrateLegacyCmd.Flags().StringVar(&poolMigrateLegacyPendingDir, "pending-dir", filepath.Join(".agents", "knowledge", "pending"), "Pending directory for migrated captures")
}

func runPoolMigrateLegacy(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	sourceDir := poolMigrateLegacySourceDir
	if !filepath.IsAbs(sourceDir) {
		sourceDir = filepath.Join(cwd, sourceDir)
	}
	pendingDir := poolMigrateLegacyPendingDir
	if !filepath.IsAbs(pendingDir) {
		pendingDir = filepath.Join(cwd, pendingDir)
	}

	result, err := migrateLegacyKnowledgeFiles(sourceDir, pendingDir)
	if err != nil {
		return err
	}
	return outputPoolMigrateLegacyResult(result)
}

func migrateLegacyKnowledgeFiles(sourceDir, pendingDir string) (poolMigrateLegacyResult, error) {
	result := poolMigrateLegacyResult{}

	files, err := filepath.Glob(filepath.Join(sourceDir, "*.md"))
	if err != nil {
		return result, fmt.Errorf("scan legacy markdown files: %w", err)
	}
	sort.Strings(files)
	result.Scanned = len(files)

	if result.Scanned == 0 {
		return result, nil
	}

	if !GetDryRun() {
		if err := os.MkdirAll(pendingDir, 0755); err != nil {
			return result, fmt.Errorf("create pending dir: %w", err)
		}
	}

	for _, src := range files {
		data, readErr := os.ReadFile(src)
		if readErr != nil {
			result.Errors++
			continue
		}
		if len(parseLearningBlocks(string(data))) == 0 {
			result.Skipped++
			continue
		}
		result.Eligible++

		dst, dstErr := nextLegacyDestination(pendingDir, filepath.Base(src))
		if dstErr != nil {
			result.Errors++
			continue
		}

		if GetDryRun() {
			result.Moved++
			result.Moves = append(result.Moves, legacyMove{From: src, To: dst})
			continue
		}

		if err := os.Rename(src, dst); err != nil {
			result.Errors++
			continue
		}
		result.Moved++
		result.Moves = append(result.Moves, legacyMove{From: src, To: dst})
	}

	return result, nil
}

func nextLegacyDestination(pendingDir, baseName string) (string, error) {
	ext := filepath.Ext(baseName)
	name := strings.TrimSuffix(baseName, ext)
	if ext == "" {
		ext = ".md"
	}

	candidate := filepath.Join(pendingDir, baseName)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	}

	for i := 1; i <= 1000; i++ {
		try := filepath.Join(pendingDir, fmt.Sprintf("%s-migrated-%d%s", name, i, ext))
		if _, err := os.Stat(try); os.IsNotExist(err) {
			return try, nil
		}
	}
	return "", fmt.Errorf("could not find available destination for %s", baseName)
}

func outputPoolMigrateLegacyResult(res poolMigrateLegacyResult) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	default:
		fmt.Printf("Legacy migration: moved=%d eligible=%d scanned=%d skipped=%d errors=%d\n",
			res.Moved, res.Eligible, res.Scanned, res.Skipped, res.Errors)
		if GetDryRun() && res.Moved > 0 {
			fmt.Println("[dry-run] Planned moves:")
			for _, m := range res.Moves {
				fmt.Printf("  - %s -> %s\n", filepath.Base(m.From), filepath.Base(m.To))
			}
		}
		return nil
	}
}
