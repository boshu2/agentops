// practices: [hexagonal-architecture, design-by-contract]
package mcpsurface

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/boshu2/agentops/cli/internal/adapters/mcptransport"
)

// ToolDescriptor is one curated tool the MCP surface exposes to a hosted / SDK
// Claude loop.
type ToolDescriptor = mcptransport.ToolDescriptor

// ToolExecutor runs a curated tool by name and returns its structured text
// output. The production executor shells the wrapped `ao` subcommand; tests
// inject a deterministic fake.
type ToolExecutor = mcptransport.ToolExecutor

// Options is the resolved input to Run.
type Options struct {
	PrintTools bool
	In         io.Reader    // live transport input (default os.Stdin)
	Out        io.Writer    // print-tools / live transport output (default os.Stdout)
	Exec       ToolExecutor // tool executor (default RealExecutor)
}

// HoldoutMarkers are substrings in a tool-call arg that would surface the
// LOCKED eval/holdout substrate to a cloud agent (Managed Agents are NOT ZDR).
var HoldoutMarkers = []string{"holdout", "ground_truth", ".agents/evals", "evals/"}

// ToolDescriptors returns the curated, read-mostly, deterministic tool surface.
// Each wraps an existing `ao` subcommand and returns structured JSON.
func ToolDescriptors() []ToolDescriptor {
	strProp := func(name, desc string) map[string]any {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{name: map[string]any{"type": "string", "description": desc}},
		}
	}
	empty := map[string]any{"type": "object", "properties": map[string]any{}}
	return []ToolDescriptor{
		{Name: "session_bootstrap", Description: "Run `ao session bootstrap` — the universal orientation report.", InputSchema: empty},
		{Name: "inject", Description: "Run `ao inject` — decay-ranked prior context for a query.", InputSchema: strProp("query", "Topic to retrieve context for"), HoldoutSensitive: true},
		{Name: "corpus_inject", Description: "Run `ao corpus inject --query` — typed BC1 corpus retrieval.", InputSchema: strProp("query", "Corpus query"), HoldoutSensitive: true},
		{Name: "standards", Description: "Load the standards checklist for the given filetypes.", InputSchema: strProp("filetypes", "Comma-separated filetypes")},
		{Name: "validate", Description: "Run `ao validate --gate` over the target artifact.", InputSchema: strProp("target", "File or bead to gate"), HoldoutSensitive: true},
		{Name: "goals_measure", Description: "Run `ao goals measure` — fitness gate snapshot.", InputSchema: empty},
	}
}

// ToolDenied reports whether a proposed tool call would surface holdout/eval
// content to the cloud MCP surface. Deterministic; fails closed. Managed Agents
// are NOT ZDR, so AGENTOPS_HOLDOUT_EVALUATOR does NOT unlock this for the cloud
// surface — the eval substrate stays LOCKED.
func ToolDenied(name string, args map[string]string) (bool, string) {
	for key, val := range args {
		low := strings.ToLower(val)
		for _, marker := range HoldoutMarkers {
			if strings.Contains(low, marker) {
				return true, fmt.Sprintf(
					"refusing tool %q: arg %q=%q would surface holdout/eval corpus — the MCP surface is cloud-facing and Managed Agents are NOT ZDR (eval substrate is LOCKED)",
					name, key, val)
			}
		}
	}
	return false, ""
}

// PrintTools writes the curated tool surface as JSON consumable by
// `ao agent bundle --runtime managed`.
func PrintTools(w io.Writer) error {
	doc := struct {
		Tools []ToolDescriptor `json:"tools"`
	}{Tools: ToolDescriptors()}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// Run serves the MCP surface, including --print-tools and the live JSON-RPC
// stdio transport.
func Run(opts Options) error {
	if opts.PrintTools {
		out := opts.Out
		if out == nil {
			return fmt.Errorf("mcpsurface: print-tools requires a non-nil writer")
		}
		return PrintTools(out)
	}
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	execFn := opts.Exec
	if execFn == nil {
		execFn = RealExecutor
	}
	return mcptransport.Serve(in, out, mcptransport.Options{
		ToolDescriptors: ToolDescriptors,
		Deny:            ToolDenied,
		Exec:            execFn,
	})
}

// RealExecutor shells the curated tool's underlying `ao` subcommand. Each tool
// is read-mostly and deterministic. `standards` has no standalone command: the
// standards checklist is enforced through `ao validate`, so it routes there.
// Only reachable for non-denied calls (ToolDenied runs first).
func RealExecutor(name string, args map[string]string) (string, error) {
	var argv []string
	switch name {
	case "session_bootstrap":
		argv = []string{"session", "bootstrap"}
	case "inject":
		argv = []string{"inject", "--query", args["query"]}
	case "corpus_inject":
		argv = []string{"corpus", "inject", "--query", args["query"]}
	case "validate":
		argv = []string{"validate", "--gate", "--changes", args["target"]}
	case "standards":
		argv = []string{"validate", "--gate"} // standards checklist runs via validate
	case "goals_measure":
		argv = []string{"goals", "measure"}
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
	out, err := exec.Command("ao", argv...).CombinedOutput()
	return string(out), err
}
