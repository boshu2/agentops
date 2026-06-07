// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"github.com/boshu2/agentops/cli/internal/adapters/mcpsurface"
	"github.com/spf13/cobra"
)

// --- cobra wiring ---

// mcpCmd is the `ao mcp` noun: expose the ao tool surface to hosted/SDK Claude
// loops over MCP (Claude-only — Codex shells `ao` directly).
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Expose the ao tool surface over MCP for hosted/SDK Claude loops",
	Long: `Model Context Protocol surface for out-of-session Claude loops (Managed
Agents / Agent SDK). Exposes a curated, read-mostly, deterministic set of ao
tools so a hosted loop can orient and self-check.

Claude-only: Codex/NTM shells the ao CLI directly and does not need MCP.`,
}

var mcpServePrintTools bool
var mcpServeJSON bool

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve (or print) the curated ao MCP tool surface",
	Long: `Emit the curated MCP tool surface (--print-tools) consumable by
` + "`ao agent bundle --runtime managed`" + `, or run the live MCP JSON-RPC
transport.

  ao mcp serve --print-tools --json    # list the tool schemas

The server refuses any tool call whose args would surface holdout/eval corpus
(Managed Agents are NOT ZDR; the eval substrate is LOCKED).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if mcpServePrintTools {
			return mcpsurface.PrintTools(cmd.OutOrStdout())
		}
		return mcpsurface.Run(mcpsurface.Options{PrintTools: false, Out: cmd.OutOrStdout()})
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
	mcpServeCmd.Flags().BoolVar(&mcpServePrintTools, "print-tools", false, "Emit the curated tool surface as JSON and exit")
	mcpServeCmd.Flags().BoolVar(&mcpServeJSON, "json", false, "Machine-readable JSON (always JSON for --print-tools; reserved for parity)")
}
