// practices: [distributed-tracing, adr]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenance"
	"github.com/boshu2/agentops/cli/internal/storage"
)

var (
	traceGraph bool
)

var traceCmd = &cobra.Command{
	Use:   "trace <artifact-path>",
	Short: "Track artifact provenance",
	Long: `Trace the provenance of an artifact back to its source transcript.

Shows the lineage from the session file to the original JSONL transcript
that was processed to create it.

Examples:
  ao trace .agents/ao/sessions/2026-01-20-my-session.md
  ao trace .agents/ao/sessions/*.md --graph
  ao trace session-abc123 --json`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTrace,
}

func init() {
	traceCmd.GroupID = "knowledge"
	rootCmd.AddCommand(traceCmd)
	traceCmd.Flags().BoolVar(&traceGraph, "graph", false, "Show ASCII provenance graph")
}

// traceOneArtifact traces and outputs provenance for a single artifact path.
func traceOneArtifact(graph *provenance.Graph, artifactPath string) error {
	result, err := graph.Trace(artifactPath)
	if err != nil {
		return fmt.Errorf("trace %s: %w", artifactPath, err)
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if len(result.Chain) == 0 {
		fmt.Printf("No provenance found for: %s\n", artifactPath)
		return nil
	}

	if traceGraph {
		printTraceGraph(result)
	} else {
		printTraceTable(result)
	}
	return nil
}

func runTrace(cmd *cobra.Command, args []string) error {
	if GetDryRun() {
		fmt.Printf("[dry-run] Would trace provenance for %d artifact(s)\n", len(args))
		return nil
	}

	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}

	provPath := filepath.Join(cwd, storage.DefaultBaseDir, storage.ProvenanceDir, storage.ProvenanceFile)

	graph, err := provenance.NewGraph(provPath)
	if err != nil {
		return fmt.Errorf("load provenance: %w", err)
	}

	if len(graph.Records) == 0 {
		fmt.Println("No provenance records found.")
		fmt.Println("Run 'ao forge transcript <path>' to generate provenance data.")
		return nil
	}

	for _, artifactPath := range args {
		if err := traceOneArtifact(graph, artifactPath); err != nil {
			return err
		}
	}

	return nil
}

func printTraceTable(result *provenance.TraceResult) {
	fmt.Printf("\nProvenance for: %s\n", result.Artifact)
	fmt.Println("=" + repeatString("=", len(result.Artifact)+16))
	fmt.Println()

	for i, record := range result.Chain {
		fmt.Printf("Record %d:\n", i+1)
		fmt.Printf("  ID:        %s\n", record.ID)
		fmt.Printf("  Type:      %s\n", record.ArtifactType)
		fmt.Printf("  Source:    %s\n", record.SourcePath)
		fmt.Printf("  Session:   %s\n", record.SessionID)
		fmt.Printf("  Created:   %s\n", record.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	if len(result.Sources) > 0 {
		fmt.Println("Original Sources:")
		for _, source := range result.Sources {
			fmt.Printf("  • %s\n", source)
		}
	}
}

func printTraceGraph(result *provenance.TraceResult) {
	fmt.Printf("\nProvenance Graph for: %s\n\n", result.Artifact)

	for i, record := range result.Chain {
		// Print the artifact node
		if i == 0 {
			fmt.Printf("  ┌─ %s\n", filepath.Base(result.Artifact))
			fmt.Printf("  │  [%s]\n", record.ArtifactType)
		}

		// Print the source connection
		fmt.Printf("  │\n")
		fmt.Printf("  │  ← %s\n", record.ID)
		fmt.Printf("  │\n")
		fmt.Printf("  └─ %s\n", filepath.Base(record.SourcePath))
		fmt.Printf("     [%s]\n", record.SourceType)

		if record.SessionID != "" {
			fmt.Printf("     session: %s\n", record.SessionID[:min(12, len(record.SessionID))])
		}
	}

	fmt.Println()
}

func repeatString(s string, n int) string {
	result := ""
	for range n {
		result += s
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
