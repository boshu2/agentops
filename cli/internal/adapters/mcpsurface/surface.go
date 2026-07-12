// practices: [hexagonal-architecture, design-by-contract]
package mcpsurface

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	return realExecutorWithDependencies(
		name,
		args,
		func() (string, error) { return resolveTrustedExecutable(os.Executable) },
		func(executable string, argv ...string) ([]byte, error) {
			return exec.Command(executable, argv...).CombinedOutput()
		},
	)
}

type executableResolver func() (string, error)
type commandRunner func(executable string, argv ...string) ([]byte, error)

// resolveTrustedExecutable returns the filesystem-real regular file for the
// process currently serving MCP. It never consults PATH.
func resolveTrustedExecutable(executable executableResolver) (string, error) {
	path, err := executable()
	if err != nil {
		return "", fmt.Errorf("locate running executable: %w", err)
	}
	if path == "" {
		return "", fmt.Errorf("locate running executable: empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make running executable absolute: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve running executable symlinks: %w", err)
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("stat running executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("trusted ao candidate %q is not a regular file", real)
	}
	return real, nil
}

func realExecutorWithDependencies(
	name string,
	args map[string]string,
	resolve executableResolver,
	run commandRunner,
) (string, error) {
	argv, err := toolArgv(name, args)
	if err != nil {
		return "", err
	}
	if resolve == nil {
		return "", fmt.Errorf("resolve trusted ao: nil resolver")
	}
	executable, err := resolve()
	if err != nil {
		return "", fmt.Errorf("resolve trusted ao: %w", err)
	}
	if executable == "" {
		return "", fmt.Errorf("resolve trusted ao: empty path")
	}
	if run == nil {
		return "", fmt.Errorf("run trusted ao: nil runner")
	}
	out, err := run(executable, argv...)
	if err != nil {
		return string(out), fmt.Errorf("run trusted ao tool %q: %w", name, err)
	}
	return string(out), nil
}

func toolArgv(name string, args map[string]string) ([]string, error) {
	switch name {
	case "session_bootstrap":
		return []string{"session", "bootstrap"}, nil
	case "inject":
		return []string{"inject", "--query", args["query"]}, nil
	case "corpus_inject":
		return []string{"corpus", "inject", "--query", args["query"]}, nil
	case "validate":
		return []string{"validate", "--gate", "--changes", args["target"]}, nil
	case "standards":
		return []string{"validate", "--gate"}, nil // standards checklist runs via validate
	case "goals_measure":
		return []string{"goals", "measure"}, nil
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}
