package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/harvest"
	"github.com/spf13/cobra"
)

var (
	harvestRootsFlag     string
	harvestOutputDir     string
	harvestPromoteTo     string
	harvestMinConfidence float64
	harvestInclude       string
	harvestQuiet         bool
	harvestMaxFileSize   int64
)

var harvestCmd = &cobra.Command{
	Use:   "harvest",
	Short: "Sweep all rigs, extract and deduplicate cross-rig knowledge",
	Long: `Walks all .agents/ directories across the workspace, extracts learnings,
patterns, and research, deduplicates across rigs, and promotes high-value
items to the global knowledge hub (~/.agents/learnings/).`,
	RunE: runHarvest,
}

func init() {
	harvestCmd.GroupID = "knowledge"
	rootCmd.AddCommand(harvestCmd)

	home, _ := os.UserHomeDir()
	defaultRoots := filepath.Join(home, "gt")
	defaultPromote := filepath.Join(home, ".agents", "learnings")

	harvestCmd.Flags().StringVar(&harvestRootsFlag, "roots", defaultRoots,
		"Base directories to scan (comma-separated)")
	harvestCmd.Flags().StringVar(&harvestOutputDir, "output-dir", ".agents/harvest",
		"Directory for harvest catalog output")
	harvestCmd.Flags().StringVar(&harvestPromoteTo, "promote-to", defaultPromote,
		"Promotion destination for high-value artifacts")
	harvestCmd.Flags().Float64Var(&harvestMinConfidence, "min-confidence", 0.5,
		"Minimum confidence for promotion")
	harvestCmd.Flags().StringVar(&harvestInclude, "include", "learnings,patterns,research",
		"Artifact types to include (comma-separated)")
	harvestCmd.Flags().BoolVar(&harvestQuiet, "quiet", false, "Suppress progress output")
	harvestCmd.Flags().Int64Var(&harvestMaxFileSize, "max-file-size", 1048576,
		"Skip files larger than this (bytes)")
}

func runHarvest(cmd *cobra.Command, args []string) error {
	roots := strings.Split(harvestRootsFlag, ",")
	for i := range roots {
		roots[i] = strings.TrimSpace(roots[i])
	}

	includeDirs := strings.Split(harvestInclude, ",")
	for i := range includeDirs {
		includeDirs[i] = strings.TrimSpace(includeDirs[i])
	}

	opts := harvest.WalkOptions{
		Roots:       roots,
		MaxFileSize: harvestMaxFileSize,
		SkipDirs:    harvest.DefaultWalkOptions().SkipDirs,
		IncludeDirs: includeDirs,
	}

	rigs, err := harvest.DiscoverRigs(opts)
	if err != nil {
		return fmt.Errorf("discovering rigs: %w", err)
	}

	if !harvestQuiet {
		fmt.Printf("Discovered %d rigs\n", len(rigs))
	}

	var allArtifacts []harvest.Artifact
	for _, rig := range rigs {
		arts, extractErr := harvest.ExtractArtifacts(rig, opts)
		if extractErr != nil {
			fmt.Fprintf(os.Stderr, "harvest: warning: extracting from %s: %v\n", rig.Rig, extractErr)
			continue
		}
		allArtifacts = append(allArtifacts, arts...)
	}

	catalog := harvest.BuildCatalog(allArtifacts, harvestMinConfidence)
	catalog.RigsScanned = len(rigs)

	totalFiles := 0
	for _, rig := range rigs {
		totalFiles += rig.FileCount
	}
	catalog.TotalFiles = totalFiles
	catalog.Timestamp = time.Now().UTC()

	if !harvestQuiet {
		uniqueCount := len(catalog.Artifacts) - duplicateArtifactCount(catalog)
		fmt.Printf("Extracted %d artifacts (%d unique, %d duplicates, %d promotion candidates)\n",
			len(catalog.Artifacts), uniqueCount, duplicateArtifactCount(catalog), len(catalog.Promoted))
	}

	if GetOutput() == "json" {
		data, marshalErr := json.MarshalIndent(catalog, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("marshaling catalog: %w", marshalErr)
		}
		fmt.Println(string(data))
		return nil
	}

	promoted := 0
	if !GetDryRun() {
		var promoteErr error
		promoted, promoteErr = harvest.Promote(catalog, harvestPromoteTo, false)
		if promoteErr != nil {
			return fmt.Errorf("promoting artifacts: %w", promoteErr)
		}
	}

	outputDir := harvestOutputDir
	if err := harvest.WriteCatalog(outputDir, catalog); err != nil {
		return fmt.Errorf("writing catalog: %w", err)
	}

	if !harvestQuiet {
		fmt.Printf("Catalog written to %s\n", outputDir)
		if !GetDryRun() {
			fmt.Printf("Promoted %d artifacts to %s\n", promoted, harvestPromoteTo)
		} else {
			fmt.Println("Dry run: no artifacts promoted")
		}
	}

	VerbosePrintf("Rigs scanned: %d, Total files: %d\n", catalog.RigsScanned, catalog.TotalFiles)

	return nil
}

// duplicateArtifactCount returns the total number of duplicate artifacts
// (excluding the kept winner from each group).
func duplicateArtifactCount(cat *harvest.Catalog) int {
	count := 0
	for _, dg := range cat.Duplicates {
		count += dg.Count - 1
	}
	return count
}
