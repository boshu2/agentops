package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

// runMetricsCite records a citation event.
func runMetricsCite(cmd *cobra.Command, args []string) error {
	artifactPath := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Make path absolute if needed
	artifactPath = canonicalArtifactPath(cwd, artifactPath)

	// Verify artifact exists
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		return fmt.Errorf("artifact not found: %s", artifactPath)
	}

	// Get flags
	citeType, _ := cmd.Flags().GetString("type")
	citeSession, _ := cmd.Flags().GetString("session")
	citeQuery, _ := cmd.Flags().GetString("query")

	// Auto-detect session ID if not provided
	if citeSession == "" {
		citeSession = detectSessionID()
	}
	citeSession = canonicalSessionID(citeSession)

	event := types.CitationEvent{
		ArtifactPath: artifactPath,
		SessionID:    citeSession,
		CitedAt:      time.Now(),
		CitationType: citeType,
		Query:        citeQuery,
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would record citation:\n")
		fmt.Printf("  Artifact: %s\n", artifactPath)
		fmt.Printf("  Session: %s\n", citeSession)
		fmt.Printf("  Type: %s\n", citeType)
		return nil
	}

	if err := ratchet.RecordCitation(cwd, event); err != nil {
		return fmt.Errorf("record citation: %w", err)
	}

	fmt.Printf("Citation recorded: %s\n", filepath.Base(artifactPath))
	return nil
}

// detectSessionID tries to detect the current session ID.
func detectSessionID() string {
	// Check CLAUDE_SESSION_ID env var
	if id := os.Getenv("CLAUDE_SESSION_ID"); id != "" {
		return canonicalSessionID(id)
	}

	// Check for session file in current dir
	// This is a fallback - real session ID should come from Claude
	return canonicalSessionID("")
}
