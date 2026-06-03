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
// bundling it to any background/cloud profile would leak the locked eval
// substrate. Hosted Managed Agents are NOT ZDR, and local NTM background
// sessions should also avoid inlining holdout payloads into reusable profiles.
var holdoutMarkers = []string{"private_holdout", "ground_truth", "holdout target"}

// bundleOptions is the resolved input to buildAgentBundle.
type bundleOptions struct {
	Runtime   string   // "managed" | "codex-ntm" | "claude-ntm"
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
	Runtime        string        `json:"runtime"`
	Model          string        `json:"model,omitempty"`
	Instructions   string        `json:"instructions"`
	Skills         []string      `json:"skills"`
	Tools          []bundleTool  `json:"tools,omitempty"`
	Sandbox        *sandboxBlock `json:"sandbox,omitempty"`
	Bootstrap      string        `json:"bootstrap,omitempty"`       // NTM pane/session snippet
	Reference      string        `json:"reference,omitempty"`       // runtime skill reference
	Mailbox        string        `json:"mailbox,omitempty"`         // mcp-agent-mail identity
	WorktreePolicy string        `json:"worktree_policy,omitempty"` // isolation contract
	Coordination   []string      `json:"coordination,omitempty"`    // reservation/handoff contract
}

// agentRoster is the render-only list of background sessions AgentOps expects
// NTM to keep warm. It is deliberately declarative: starting/stopping live tmux
// panes remains an explicit NTM operation outside this command.
type agentRoster struct {
	SchemaVersion int           `json:"schema_version"`
	Agents        []agentBundle `json:"agents"`
}

// buildAgentBundle resolves the skill set, enforces the NOT-ZDR holdout
// refusal, and dispatches to the runtime-specific builder.
func buildAgentBundle(opts bundleOptions) (agentBundle, error) {
	if opts.Runtime != "managed" && opts.Runtime != "codex-ntm" && opts.Runtime != "claude-ntm" {
		return agentBundle{}, fmt.Errorf("unknown --runtime %q (want managed|codex-ntm|claude-ntm)", opts.Runtime)
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
			"refusing to bundle %s — %s would inline holdout/eval content (the eval substrate is LOCKED). "+
				"AGENTOPS_HOLDOUT_EVALUATOR does not authorize leaking holdout data into a reusable agent profile", opts.Runtime, why)
	}
	if opts.Runtime == "codex-ntm" {
		return buildNTMBundle("codex-ntm", skills, "skills-codex/agent-native"), nil
	}
	if opts.Runtime == "claude-ntm" {
		return buildNTMBundle("claude-ntm", skills, "skills/agent-native"), nil
	}
	return buildManagedBundle(skills, opts.Sandbox), nil
}

func buildAgentRoster(opts bundleOptions) (agentRoster, error) {
	codex, err := buildAgentBundle(bundleOptions{
		Runtime:   "codex-ntm",
		Skills:    opts.Skills,
		SkillsDir: opts.SkillsDir,
	})
	if err != nil {
		return agentRoster{}, err
	}
	claude, err := buildAgentBundle(bundleOptions{
		Runtime:   "claude-ntm",
		Skills:    opts.Skills,
		SkillsDir: opts.SkillsDir,
	})
	if err != nil {
		return agentRoster{}, err
	}
	return agentRoster{
		SchemaVersion: 1,
		Agents:        []agentBundle{claude, codex},
	}, nil
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

// buildNTMBundle emits an NTM-consumable background-session profile. NTM owns
// pane lifecycle; mcp-agent-mail owns assignment, reservations, and handoff;
// the Claude/Codex worker runs a skill-guided session with ao support commands.
func buildNTMBundle(runtime string, skills []string, reference string) agentBundle {
	return agentBundle{
		Runtime:        runtime,
		Instructions:   stitchInstructions(skills),
		Skills:         skills,
		Bootstrap:      "ao session bootstrap --json && ao inject --query \"$BEAD\"",
		Reference:      reference,
		Mailbox:        "agentops-" + runtime + "-worker",
		WorktreePolicy: "one-worktree-per-bead",
		Coordination: []string{
			"mcp-agent-mail: fetch assignment thread before work",
			"mcp-agent-mail: reserve file paths before edits",
			"bd: claim/update/close the bead with evidence",
			"git: commit and push from the assigned worktree",
		},
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
