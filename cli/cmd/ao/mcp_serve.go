// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// mcpToolDescriptor is one curated tool the MCP surface exposes to a hosted /
// SDK Claude loop. Slice 1 (ag-h1mk) ships the descriptors + the holdout
// refusal policy; the live JSON-RPC transport is the ag-3ucpd follow-up.
type mcpToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	// HoldoutSensitive marks a tool whose args can reference corpus paths and
	// therefore must be run through mcpToolDenied before a real call.
	HoldoutSensitive bool `json:"holdout_sensitive"`
}

// mcpHoldoutMarkers are substrings in a tool-call arg that would surface the
// LOCKED eval/holdout substrate to a cloud agent (Managed Agents are NOT ZDR).
var mcpHoldoutMarkers = []string{"holdout", "ground_truth", ".agents/evals", "evals/"}

// mcpToolDescriptors returns the curated, read-mostly, deterministic tool
// surface. Each wraps an existing `ao` subcommand and returns structured JSON.
func mcpToolDescriptors() []mcpToolDescriptor {
	strProp := func(name, desc string) map[string]any {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{name: map[string]any{"type": "string", "description": desc}},
		}
	}
	empty := map[string]any{"type": "object", "properties": map[string]any{}}
	return []mcpToolDescriptor{
		{Name: "session_bootstrap", Description: "Run `ao session bootstrap` — the universal orientation report.", InputSchema: empty},
		{Name: "inject", Description: "Run `ao inject` — decay-ranked prior context for a query.", InputSchema: strProp("query", "Topic to retrieve context for"), HoldoutSensitive: true},
		{Name: "corpus_inject", Description: "Run `ao corpus inject --query` — typed BC1 corpus retrieval.", InputSchema: strProp("query", "Corpus query"), HoldoutSensitive: true},
		{Name: "standards", Description: "Load the standards checklist for the given filetypes.", InputSchema: strProp("filetypes", "Comma-separated filetypes")},
		{Name: "validate", Description: "Run `ao validate --gate` over the target artifact.", InputSchema: strProp("target", "File or bead to gate"), HoldoutSensitive: true},
		{Name: "goals_measure", Description: "Run `ao goals measure` — fitness gate snapshot.", InputSchema: empty},
	}
}

// mcpToolDenied reports whether a proposed tool call would surface holdout/eval
// content to the cloud MCP surface. Deterministic; fails closed. Managed Agents
// are NOT ZDR, so AGENTOPS_HOLDOUT_EVALUATOR does NOT unlock this for the cloud
// surface — the eval substrate stays LOCKED.
func mcpToolDenied(name string, args map[string]string) (bool, string) {
	for key, val := range args {
		low := strings.ToLower(val)
		for _, marker := range mcpHoldoutMarkers {
			if strings.Contains(low, marker) {
				return true, fmt.Sprintf(
					"refusing tool %q: arg %q=%q would surface holdout/eval corpus — the MCP surface is cloud-facing and Managed Agents are NOT ZDR (eval substrate is LOCKED)",
					name, key, val)
			}
		}
	}
	return false, ""
}

// runMCPServePrintTools writes the curated tool surface as JSON consumable by
// `ao agent bundle --runtime managed`.
func runMCPServePrintTools(w io.Writer) error {
	doc := struct {
		Tools []mcpToolDescriptor `json:"tools"`
	}{Tools: mcpToolDescriptors()}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// mcpServeOptions is the resolved input to runMCPServe.
type mcpServeOptions struct {
	PrintTools bool
	Out        io.Writer
}

// runMCPServe handles `ao mcp serve`. Slice 1 supports --print-tools only; the
// live JSON-RPC transport is the ag-3ucpd follow-up and errors loudly until then.
func runMCPServe(opts mcpServeOptions) error {
	if !opts.PrintTools {
		return fmt.Errorf("live MCP transport is not yet implemented (follow-up ag-3ucpd); use `ao mcp serve --print-tools` to emit the tool surface")
	}
	out := opts.Out
	if out == nil {
		return fmt.Errorf("runMCPServe: print-tools requires a non-nil writer")
	}
	return runMCPServePrintTools(out)
}

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
` + "`ao agent bundle --runtime managed`" + `, or (future) run the live MCP
JSON-RPC transport.

  ao mcp serve --print-tools --json    # list the tool schemas

The server refuses any tool call whose args would surface holdout/eval corpus
(Managed Agents are NOT ZDR; the eval substrate is LOCKED).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if mcpServePrintTools {
			return runMCPServePrintTools(cmd.OutOrStdout())
		}
		return runMCPServe(mcpServeOptions{PrintTools: false, Out: cmd.OutOrStdout()})
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
	mcpServeCmd.Flags().BoolVar(&mcpServePrintTools, "print-tools", false, "Emit the curated tool surface as JSON and exit")
	mcpServeCmd.Flags().BoolVar(&mcpServeJSON, "json", false, "Machine-readable JSON (always JSON for --print-tools; reserved for parity)")
}
