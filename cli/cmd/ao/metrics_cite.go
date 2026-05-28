// practices: [dora-metrics, sre]
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

	cwd, err := resolveProjectDir()
	if err != nil {
		return err
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
	citeVendor, _ := cmd.Flags().GetString("vendor")

	// Auto-detect session ID if not provided
	if citeSession == "" {
		citeSession = detectSessionID()
	}
	citeSession = resolveSessionID(citeSession)

	// Auto-detect vendor from runtime environment if not provided
	if citeVendor == "" {
		citeVendor = detectModelVendor()
	}

	event := types.CitationEvent{
		ArtifactPath:    artifactPath,
		SessionID:       citeSession,
		CitedAt:         time.Now(),
		CitationType:    citeType,
		ModelVendor:     citeVendor,
		Query:           citeQuery,
		MetricNamespace: defaultCitationMetricNamespace(),
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would record citation:\n")
		fmt.Printf("  Artifact: %s\n", artifactPath)
		fmt.Printf("  Session: %s\n", citeSession)
		fmt.Printf("  Type: %s\n", citeType)
		if citeVendor != "" {
			fmt.Printf("  Vendor: %s\n", citeVendor)
		}
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
	return resolveSessionID("")
}

// detectModelVendor infers the model vendor from the runtime environment.
// Returns "claude", "codex", or "" (unknown).
func detectModelVendor() string {
	// Codex sets CODEX_SESSION or runs as codex CLI
	if os.Getenv("CODEX_SESSION") != "" || os.Getenv("CODEX_SANDBOX_TYPE") != "" {
		return "codex"
	}
	// Claude Code sets CLAUDE_CODE_SESSION or similar
	if os.Getenv("CLAUDE_CODE_SESSION") != "" || os.Getenv("CLAUDE_SESSION_ID") != "" {
		return "claude"
	}
	// Check for parent process hints
	if os.Getenv("OPENAI_API_KEY") != "" && os.Getenv("ANTHROPIC_API_KEY") == "" {
		return "codex"
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" && os.Getenv("OPENAI_API_KEY") == "" {
		return "claude"
	}
	return ""
}
