// practices: [wiki-knowledge-surface, llm-eval-harness]
package main

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/harvest"
)

// llmwikiHarvestAdapter bridges llmwiki.PromoteStage's HarvestPromoter
// interface (sourceDir, destDir, dryRun) to the harvest package's API
// (which takes a *harvest.Catalog). The adapter walks sourceDir using
// harvest.ExtractArtifacts, builds a Catalog with BuildCatalog, then
// delegates to harvest.Promote.
//
// The adapter is intentionally minimal — production tuning of the walk
// (rig labeling, confidence threshold, exclude lists) lives in the
// harvest package and its CLI entry point. PromoteStage uses this
// adapter strictly to honor wiki/.promote-pending.json requests, which
// are operator-supplied and tightly scoped.
type llmwikiHarvestAdapter struct{}

// Promote walks sourceDir for .agents-style artifacts and delegates to
// harvest.Promote. Returns the number of files promoted.
func (llmwikiHarvestAdapter) Promote(sourceDir, destDir string, dryRun bool) (int, error) {
	if sourceDir == "" {
		return 0, fmt.Errorf("llmwiki harvest adapter: sourceDir is required")
	}
	if destDir == "" {
		return 0, fmt.Errorf("llmwiki harvest adapter: destDir is required")
	}

	rig := harvest.RigInfo{
		Path:    sourceDir,
		Project: "llmwiki",
		Crew:    "promote",
		Rig:     "llmwiki-promote",
	}
	opts := harvest.WalkOptions{
		Roots:       []string{sourceDir},
		MaxFileSize: 1048576,
		IncludeDirs: []string{"sources", "concepts", "entities", "synthesis", "knowledge", "reviewed", "learnings", "patterns", "research"},
	}

	artifacts, _ := harvest.ExtractArtifacts(rig, opts)
	catalog := harvest.BuildCatalog(artifacts, 0)
	if catalog == nil {
		return 0, fmt.Errorf("llmwiki harvest adapter: BuildCatalog returned nil")
	}
	catalog.DryRun = dryRun

	count, err := harvest.Promote(catalog, destDir, dryRun)
	if err != nil {
		return count, fmt.Errorf("llmwiki harvest adapter: promote: %w", err)
	}
	return count, nil
}
