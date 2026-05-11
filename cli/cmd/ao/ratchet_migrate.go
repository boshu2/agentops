// practices: [dora-metrics, continuous-delivery]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	migrateSubCmd := &cobra.Command{
		Use:     "migrate",
		GroupID: "management",
		Short:   "Migrate legacy chain",
		Long: `Migrate chain from legacy YAML format to JSONL.

Reads from .agents/provenance/chain.yaml
Writes to .agents/ao/chain.jsonl

Examples:
  ao ratchet migrate
  ao ratchet migrate --dry-run`,
		RunE: runRatchetMigrate,
	}
	ratchetCmd.AddCommand(migrateSubCmd)

	// ol-a46.1.3: Artifact schema version migration
	migrateArtifactsCmd := &cobra.Command{
		Use:     "migrate-artifacts [path]",
		GroupID: "management",
		Short:   "Add schema_version to artifacts (ol-a46.1.3)",
		Long: `Add schema_version: 1 to existing .agents/ artifacts.

Scans markdown files and adds **Schema Version:** 1 if missing.
Non-destructive: only adds the field, doesn't modify existing content.

Examples:
  ao ratchet migrate-artifacts .agents/
  ao ratchet migrate-artifacts .agents/learnings/
  ao ratchet migrate-artifacts --dry-run`,
		RunE: runMigrateArtifacts,
	}
	ratchetCmd.AddCommand(migrateArtifactsCmd)
}

// runRatchetMigrate migrates legacy chain format.
func runRatchetMigrate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if GetDryRun() {
		fmt.Println("Would migrate chain from:")
		fmt.Println("  .agents/provenance/chain.yaml")
		fmt.Println("To:")
		fmt.Println("  .agents/ao/chain.jsonl")
		return nil
	}

	if err := ratchet.MigrateChain(cwd); err != nil {
		return fmt.Errorf("migrate chain: %w", err)
	}

	fmt.Println("Migration complete ✓")
	return nil
}

func runMigrateArtifacts(cmd *cobra.Command, args []string) error {
	path := ".agents"
	if len(args) > 0 {
		path = args[0]
	}

	migrated := 0
	skipped := 0
	errors := 0

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !shouldMigrateFile(p, info) {
			return nil
		}

		result := migrateFile(p, info)
		switch result {
		case migrateResultSuccess:
			migrated++
		case migrateResultSkipped:
			skipped++
		case migrateResultError:
			errors++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("walk path: %w", err)
	}

	fmt.Printf("\nSummary: %d migrated, %d skipped, %d errors\n", migrated, skipped, errors)
	return nil
}

// migrateResult represents the outcome of migrating a single file.
type migrateResult int

const (
	migrateResultSuccess migrateResult = iota
	migrateResultSkipped
	migrateResultError
)

// shouldMigrateFile checks if a file is a markdown file eligible for migration.
func shouldMigrateFile(path string, info os.FileInfo) bool {
	return !info.IsDir() && strings.HasSuffix(path, ".md")
}

// findSchemaInsertPoint locates where to insert schema_version in the file.
// Returns -1 if no suitable insertion point is found.
func findSchemaInsertPoint(lines []string) int {
	insertIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "**Date:**") || strings.HasPrefix(line, "**Epic:**") {
			return i + 1
		}
		if strings.HasPrefix(line, "# ") && insertIdx == -1 {
			insertIdx = i + 1
		}
	}
	if insertIdx >= len(lines) {
		return -1
	}
	return insertIdx
}

// migrateFile reads a file, adds schema_version if missing, and writes it back.
func migrateFile(path string, info os.FileInfo) migrateResult {
	content, err := os.ReadFile(path)
	if err != nil {
		return migrateResultError
	}

	text := string(content)

	// Already has schema version
	if strings.Contains(text, "Schema Version:") || strings.Contains(text, "schema_version:") {
		return migrateResultSkipped
	}

	lines := strings.Split(text, "\n")
	insertIdx := findSchemaInsertPoint(lines)
	if insertIdx == -1 {
		return migrateResultSkipped
	}

	// Insert schema version
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, "**Schema Version:** 1")
	newLines = append(newLines, lines[insertIdx:]...)
	newContent := strings.Join(newLines, "\n")

	if GetDryRun() {
		fmt.Printf("Would add schema_version to: %s\n", path)
		return migrateResultSuccess
	}

	if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
		return migrateResultError
	}
	fmt.Printf("✓ Migrated: %s\n", path)
	return migrateResultSuccess
}
