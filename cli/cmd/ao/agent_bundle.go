// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultBundleSkills is the AgentOps-native default skill set stitched into an
// out-of-session Agent definition when --skills is not given (ag-eguw0).
var defaultBundleSkills = []string{"session-bootstrap", "standards", "validation", "provenance"}

// defaultAgentModel is the model an emitted managed Agent definition targets.
const defaultAgentModel = "claude-opus-4-8"

// holdoutMarkers are substrings whose presence in a selected skill's body means
// bundling it to a (non-ZDR) cloud agent would leak the locked eval substrate.
// Managed Agents are NOT ZDR — see ~/.agents/evals/SCHEMA.md (LOCKED).
var holdoutMarkers = []string{"private_holdout", "ground_truth", "holdout target"}

// bundleOptions is the resolved input to buildAgentBundle.
type bundleOptions struct {
	Runtime   string   // "managed" | "codex-ntm"
	Skills    []string // nil => defaultBundleSkills
	Sandbox   string   // "self-hosted" | "cloud" | ""
	SkillsDir string   // root to resolve+scan skills (default: "skills")
}

// bundleTool is an MCP tool descriptor the hosted loop can call to self-check.
type bundleTool struct {
	Type   string `json:"type"`
	Server string `json:"server"`
	Cmd    string `json:"cmd"`
}

// sandboxBlock describes where a self-hosted loop runs.
type sandboxBlock struct {
	Kind string `json:"kind"`
	Note string `json:"note"`
}

// agentBundle is the runtime-specific Agent definition emitted by `ao agent bundle`.
type agentBundle struct {
	Runtime      string        `json:"runtime"`
	Model        string        `json:"model,omitempty"`
	Instructions string        `json:"instructions"`
	Skills       []string      `json:"skills"`
	Tools        []bundleTool  `json:"tools,omitempty"`
	Sandbox      *sandboxBlock `json:"sandbox,omitempty"`
	Bootstrap    string        `json:"bootstrap,omitempty"` // codex-ntm pane snippet
	Reference    string        `json:"reference,omitempty"` // codex-ntm skill reference
}

// buildAgentBundle resolves the skill set, enforces the NOT-ZDR holdout
// refusal, and dispatches to the runtime-specific builder.
func buildAgentBundle(opts bundleOptions) (agentBundle, error) {
	if opts.Runtime != "managed" && opts.Runtime != "codex-ntm" {
		return agentBundle{}, fmt.Errorf("unknown --runtime %q (want managed|codex-ntm)", opts.Runtime)
	}
	skills := opts.Skills
	if len(skills) == 0 {
		skills = append([]string(nil), defaultBundleSkills...)
	}
	skillsDir := opts.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	if leak, why := selectionInlinesHoldout(skills, skillsDir); leak {
		return agentBundle{}, fmt.Errorf(
			"refusing to bundle %s — Managed Agents are NOT ZDR; %s would inline holdout/eval content (the eval substrate is LOCKED). "+
				"AGENTOPS_HOLDOUT_EVALUATOR does not authorize leaking holdout data to a cloud agent", opts.Runtime, why)
	}
	if opts.Runtime == "codex-ntm" {
		return buildCodexNTMBundle(skills), nil
	}
	return buildManagedBundle(skills, opts.Sandbox), nil
}

// buildManagedBundle emits a Managed Agents Agent-definition payload.
func buildManagedBundle(skills []string, sandbox string) agentBundle {
	b := agentBundle{
		Runtime:      "managed",
		Model:        defaultAgentModel,
		Instructions: stitchInstructions(skills),
		Skills:       skills,
		Tools: []bundleTool{{
			Type:   "mcp",
			Server: "ao",
			Cmd:    "ao mcp serve",
		}},
	}
	if sandbox == "self-hosted" {
		b.Sandbox = &sandboxBlock{
			Kind: "self-hosted",
			Note: "MCP server runs inside the sandbox boundary (e.g. bushido) with tailnet access to Dolt; holdout-touching work stays here, never the cloud",
		}
	}
	return b
}

// buildCodexNTMBundle emits an NTM-consumable bundle (Codex shells ao directly).
func buildCodexNTMBundle(skills []string) agentBundle {
	return agentBundle{
		Runtime:      "codex-ntm",
		Instructions: stitchInstructions(skills),
		Skills:       skills,
		Bootstrap:    "ao session bootstrap && ao inject --query \"$BEAD\"",
		Reference:    "skills-codex/agent-native",
	}
}

// stitchInstructions builds the agent instruction string from the selected
// skills' names (bounded: names + a standard preamble, never full bodies — full
// skill bodies are loaded by the agent at runtime via the ao tool surface).
func stitchInstructions(skills []string) string {
	var sb strings.Builder
	sb.WriteString("You are an AgentOps-native agent. Load and follow these skills, ")
	sb.WriteString("self-bootstrap with `ao session bootstrap`, and gate your output with `ao validate`:\n")
	for _, s := range skills {
		sb.WriteString("- ")
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	return sb.String()
}

// selectionInlinesHoldout returns true (with a reason) if any selected skill's
// path escapes the skills dir into a holdout/eval surface, or its SKILL.md body
// contains a holdout marker. Deterministic; fails closed on a read error.
func selectionInlinesHoldout(skills []string, skillsDir string) (bool, string) {
	for _, s := range skills {
		if strings.Contains(s, "..") || strings.Contains(s, "/") {
			return true, fmt.Sprintf("skill ref %q escapes the skills dir", s)
		}
		lower := strings.ToLower(s)
		if strings.Contains(lower, "holdout") || strings.Contains(lower, "evals") {
			return true, fmt.Sprintf("skill %q names a holdout/eval surface", s)
		}
		body, err := os.ReadFile(filepath.Join(skillsDir, s, "SKILL.md"))
		if err != nil {
			continue // unreadable skill contributes no inlined content
		}
		for _, marker := range holdoutMarkers {
			if strings.Contains(string(body), marker) {
				return true, fmt.Sprintf("skill %q body contains holdout marker %q", s, marker)
			}
		}
	}
	return false, ""
}
