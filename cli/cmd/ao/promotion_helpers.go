package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cliConfig "github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func resolveAutoPromoteThreshold(cmd *cobra.Command, flagName, flagValue string) (time.Duration, string, error) {
	resolved := flagValue

	// If caller did not explicitly set the flag, allow config/env to override.
	if !cmd.Flags().Changed(flagName) {
		cfg, err := cliConfig.Load(nil)
		if err != nil {
			VerbosePrintf("Warning: could not load config for auto-promote threshold: %v\n", err)
		} else if v := strings.TrimSpace(cfg.Flywheel.AutoPromoteThreshold); v != "" {
			resolved = v
		}
	}

	threshold, err := time.ParseDuration(resolved)
	if err != nil {
		if cmd.Flags().Changed(flagName) {
			return 0, "", fmt.Errorf("invalid --%s: %w", flagName, err)
		}
		return 0, "", fmt.Errorf("invalid auto-promote threshold %q from config/env: %w", resolved, err)
	}

	return threshold, resolved, nil
}

func loadPromotionGateContext(baseDir string) (map[string]int, map[string]bool) {
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		VerbosePrintf("Warning: could not load citations: %v\n", err)
	}
	citationCounts := buildCitationCounts(citations, baseDir)
	promotedContent := loadPromotedContent(baseDir)
	return citationCounts, promotedContent
}
