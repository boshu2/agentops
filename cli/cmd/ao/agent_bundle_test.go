package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// fixtureSkills writes a minimal skills/ tree: the default set as clean stubs
// plus one holdout-tainted skill, and returns the dir. The holdout scan reads
// SKILL.md bodies, so a tainted body must trip the NOT-ZDR refusal.
func fixtureSkills(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	clean := map[string]string{
		"session-bootstrap": "Bootstrap a session.",
		"standards":         "Coding standards checklist.",
		"validation":        "Run the validation gate.",
		"provenance":        "Record provenance.",
		"agent-native":      "Make out-of-session agents AgentOps-native.",
	}
	for name, desc := range clean {
		writeFixtureSkill(t, dir, name, "---\nname: "+name+"\ndescription: "+desc+"\n---\n# "+name+"\n")
	}
	// A skill whose body would inline holdout ground-truth — must be refused.
	writeFixtureSkill(t, dir, "leaky-eval",
		"---\nname: leaky-eval\ndescription: leaks eval data\n---\n# leaky-eval\nrows: private_holdout ground_truth target=42\n")
	return dir
}

func writeFixtureSkill(t *testing.T, dir, name, body string) {
	t.Helper()
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildAgentBundle_ManagedDefaults(t *testing.T) {
	b, err := buildAgentBundle(bundleOptions{Runtime: "managed", SkillsDir: fixtureSkills(t)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Runtime != "managed" {
		t.Errorf("Runtime = %q, want managed", b.Runtime)
	}
	if b.Model == "" {
		t.Error("Model must be set for a managed Agent definition")
	}
	if got, want := b.Skills, defaultBundleSkills; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("default Skills = %v, want %v", got, want)
	}
	if b.Instructions == "" {
		t.Error("Instructions must be stitched from the selected skills")
	}
	// The ao MCP tool descriptor must be present so the hosted loop can self-check.
	var hasAO bool
	for _, tool := range b.Tools {
		if tool.Server == "ao" {
			hasAO = true
		}
	}
	if !hasAO {
		t.Error("managed bundle must carry an `ao` MCP tool descriptor")
	}
}

func TestBuildAgentBundle_SelfHostedSandbox(t *testing.T) {
	b, err := buildAgentBundle(bundleOptions{Runtime: "managed", Sandbox: "self-hosted", SkillsDir: fixtureSkills(t)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Sandbox == nil {
		t.Fatal("--sandbox self-hosted must emit a sandbox block")
	}
	if b.Sandbox.Kind != "self-hosted" {
		t.Errorf("Sandbox.Kind = %q, want self-hosted", b.Sandbox.Kind)
	}
}

func TestBuildAgentBundle_CodexNTM(t *testing.T) {
	b, err := buildAgentBundle(bundleOptions{Runtime: "codex-ntm", SkillsDir: fixtureSkills(t)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Runtime != "codex-ntm" {
		t.Errorf("Runtime = %q, want codex-ntm", b.Runtime)
	}
	if !strings.Contains(b.Bootstrap, "ao session bootstrap") {
		t.Errorf("codex-ntm Bootstrap must run `ao session bootstrap`, got %q", b.Bootstrap)
	}
	if b.Reference != "skills-codex/agent-native" {
		t.Errorf("Reference = %q, want skills-codex/agent-native", b.Reference)
	}
	if b.Mailbox != "agentops-codex-ntm-worker" {
		t.Errorf("Mailbox = %q, want agentops-codex-ntm-worker", b.Mailbox)
	}
	if b.WorktreePolicy != "one-worktree-per-bead" {
		t.Errorf("WorktreePolicy = %q, want one-worktree-per-bead", b.WorktreePolicy)
	}
	if !agentBundleContainsString(b.Coordination, "mcp-agent-mail: reserve file paths before edits") {
		t.Errorf("Coordination missing file reservation contract: %v", b.Coordination)
	}
	// codex shells ao directly — no MCP descriptor needed.
	if len(b.Tools) != 0 {
		t.Errorf("codex-ntm must not carry MCP tool descriptors, got %d", len(b.Tools))
	}
}

func TestBuildAgentBundle_ClaudeNTM(t *testing.T) {
	b, err := buildAgentBundle(bundleOptions{Runtime: "claude-ntm", SkillsDir: fixtureSkills(t)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Runtime != "claude-ntm" {
		t.Errorf("Runtime = %q, want claude-ntm", b.Runtime)
	}
	if b.Reference != "skills/agent-native" {
		t.Errorf("Reference = %q, want skills/agent-native", b.Reference)
	}
	if b.Mailbox != "agentops-claude-ntm-worker" {
		t.Errorf("Mailbox = %q, want agentops-claude-ntm-worker", b.Mailbox)
	}
	if !strings.Contains(b.Bootstrap, "ao session bootstrap") {
		t.Errorf("claude-ntm Bootstrap must run `ao session bootstrap`, got %q", b.Bootstrap)
	}
	if len(b.Tools) != 0 {
		t.Errorf("claude-ntm must not carry hosted MCP tool descriptors, got %d", len(b.Tools))
	}
}

func TestBuildAgentBundle_HoldoutRefusal(t *testing.T) {
	_, err := buildAgentBundle(bundleOptions{
		Runtime:   "managed",
		Skills:    []string{"standards", "leaky-eval"},
		SkillsDir: fixtureSkills(t),
	})
	if err == nil {
		t.Fatal("bundling a holdout-tainted skill to a cloud agent must be refused (NOT-ZDR)")
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "holdout") && !strings.Contains(low, "zdr") {
		t.Errorf("refusal message must name the holdout/ZDR boundary, got %q", err.Error())
	}
}

func TestBuildAgentBundle_UnknownRuntime(t *testing.T) {
	_, err := buildAgentBundle(bundleOptions{Runtime: "wat", SkillsDir: fixtureSkills(t)})
	if err == nil {
		t.Fatal("unknown --runtime must error")
	}
}

func agentBundleContainsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestBuildAgentBundle_ManagedJSONShape(t *testing.T) {
	b, err := buildAgentBundle(bundleOptions{Runtime: "managed", SkillsDir: fixtureSkills(t)})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"runtime", "model", "instructions", "skills", "tools"} {
		if _, ok := m[key]; !ok {
			t.Errorf("managed bundle JSON missing required key %q", key)
		}
	}
}

func TestBuildAgentRoster_DefaultsToClaudeAndCodexNTM(t *testing.T) {
	roster, err := buildAgentRoster(bundleOptions{SkillsDir: fixtureSkills(t)})
	if err != nil {
		t.Fatalf("build roster: %v", err)
	}
	if roster.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", roster.SchemaVersion)
	}
	if len(roster.Agents) != 2 {
		t.Fatalf("Agents length = %d, want 2", len(roster.Agents))
	}
	want := map[string]string{
		"claude-ntm": "agentops-claude-ntm-worker",
		"codex-ntm":  "agentops-codex-ntm-worker",
	}
	for _, agent := range roster.Agents {
		mailbox, ok := want[agent.Runtime]
		if !ok {
			t.Fatalf("unexpected runtime in roster: %q", agent.Runtime)
		}
		if agent.Mailbox != mailbox {
			t.Errorf("%s Mailbox = %q, want %q", agent.Runtime, agent.Mailbox, mailbox)
		}
		if agent.WorktreePolicy != "one-worktree-per-bead" {
			t.Errorf("%s WorktreePolicy = %q", agent.Runtime, agent.WorktreePolicy)
		}
		delete(want, agent.Runtime)
	}
	if len(want) != 0 {
		t.Fatalf("missing roster runtime(s): %v", want)
	}
}

func TestBuildNTMSpawnArgs_DefaultDryRun(t *testing.T) {
	args, err := buildNTMSpawnArgs("agentops-bg", 1, 1, "/repo", true)
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	want := []string{
		"--robot-spawn=agentops-bg",
		"--spawn-cc=1",
		"--spawn-cod=1",
		"--spawn-dir=/repo",
		"--dry-run",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestBuildManualCodexPaneArgs(t *testing.T) {
	got := buildManualCodexPaneArgs("agentops-bg", 1, "/repo", "gpt-5.5")
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	joined := strings.Join(got[0], " ")
	for _, want := range []string{"split-window", "-t agentops-bg:", "-c /repo", "codex", "-m 'gpt-5.5'"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("manual codex args %q missing %q", joined, want)
		}
	}
}

func TestBuildNTMSpawnArgs_RequiresAtLeastOneAgent(t *testing.T) {
	if _, err := buildNTMSpawnArgs("agentops-bg", 0, 0, ".", true); err == nil {
		t.Fatal("expected error when both agent counts are zero")
	}
}

func TestRunAgentEligible_FileFiltersCandidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ready.json")
	raw := `[
  {"id":"ag-ok","title":"ok","labels":["background-agent-safe"]},
  {"id":"ag-holdout","title":"no","labels":["background-agent-safe","holdout"]},
  {"id":"ag-missing","title":"missing","labels":["docs"]}
]`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	prevFile, prevOnly := agentEligibleFile, agentEligibleEligibleOnly
	t.Cleanup(func() {
		agentEligibleFile = prevFile
		agentEligibleEligibleOnly = prevOnly
	})
	agentEligibleFile = path
	agentEligibleEligibleOnly = false

	cmd, out := agentTestCmd()
	if err := runAgentEligible(cmd, nil); err != nil {
		t.Fatalf("eligible: %v", err)
	}
	var decisions []struct {
		Eligible  bool     `json:"eligible"`
		Reasons   []string `json:"reasons"`
		Candidate struct {
			ID string `json:"id"`
		} `json:"candidate"`
	}
	if err := json.Unmarshal(out.Bytes(), &decisions); err != nil {
		t.Fatalf("parse decisions: %v\n%s", err, out.String())
	}
	if len(decisions) != 3 {
		t.Fatalf("len = %d, want 3", len(decisions))
	}
	if !decisions[0].Eligible || decisions[1].Eligible || decisions[2].Eligible {
		t.Fatalf("decisions = %+v, want only first eligible", decisions)
	}
}

func TestRunAgentEligible_EligibleOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ready.json")
	raw := `[{"id":"ag-ok","labels":["background-agent-safe"]},{"id":"ag-no","labels":["human"]}]`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	prevFile, prevOnly := agentEligibleFile, agentEligibleEligibleOnly
	t.Cleanup(func() {
		agentEligibleFile = prevFile
		agentEligibleEligibleOnly = prevOnly
	})
	agentEligibleFile = path
	agentEligibleEligibleOnly = true

	cmd, out := agentTestCmd()
	if err := runAgentEligible(cmd, nil); err != nil {
		t.Fatalf("eligible: %v", err)
	}
	if strings.Contains(out.String(), "ag-no") {
		t.Fatalf("eligible-only output contains ineligible bead: %s", out.String())
	}
	if !strings.Contains(out.String(), "ag-ok") {
		t.Fatalf("eligible-only output missing eligible bead: %s", out.String())
	}
}

func TestBuildAgentInitPrompt(t *testing.T) {
	got := buildAgentInitPrompt("codex-ntm", "JadeElk")
	for _, want := range []string{
		"Runtime profile: codex-ntm",
		"Expected mcp-agent-mail identity: JadeElk",
		"ao session bootstrap --json",
		"reserve file paths through mcp-agent-mail",
		"do not use deprecated `ao rpi` / `ao evolve` wrappers",
		"READY",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("init prompt missing %q:\n%s", want, got)
		}
	}
}

func TestRunAgentInitPrompt(t *testing.T) {
	prevRuntime, prevMailbox := agentInitRuntime, agentInitMailbox
	t.Cleanup(func() {
		agentInitRuntime = prevRuntime
		agentInitMailbox = prevMailbox
	})
	agentInitRuntime = "claude-ntm"
	agentInitMailbox = "JadeBeacon"
	cmd, out := agentTestCmd()
	if err := runAgentInitPrompt(cmd, nil); err != nil {
		t.Fatalf("init-prompt: %v", err)
	}
	if !strings.Contains(out.String(), "claude-ntm") || !strings.Contains(out.String(), "JadeBeacon") {
		t.Fatalf("init prompt output missing runtime/mailbox: %s", out.String())
	}
}

func TestBuildAgentAssignmentPrompt(t *testing.T) {
	got := buildAgentAssignmentPrompt(
		"ag-demo",
		"cursor/ag-demo-work",
		[]string{"cli/cmd/ao/agent.go", "skills/swarm/SKILL.md"},
		[]string{"swarm", "provenance"},
		"go test ./cmd/ao -run Agent",
	)
	for _, want := range []string{
		"BACKGROUND AGENT ASSIGNMENT",
		"Bead: ag-demo",
		"Branch/worktree: cursor/ag-demo-work",
		"Skills: swarm, provenance",
		"Reserve these file paths/globs",
		"cli/cmd/ao/agent.go",
		"mcp-agent-mail thread",
		"do not run deprecated `ao rpi` / `ao evolve` wrappers",
		"Do not self-merge",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("assignment prompt missing %q:\n%s", want, got)
		}
	}
}

func TestRunAgentAssignPromptRequiresBead(t *testing.T) {
	prevBead := agentAssignBead
	t.Cleanup(func() { agentAssignBead = prevBead })
	agentAssignBead = ""
	cmd, _ := agentTestCmd()
	if err := runAgentAssignPrompt(cmd, nil); err == nil {
		t.Fatal("assign-prompt should require --bead")
	}
}

func TestRunAgentAssignPrompt(t *testing.T) {
	prevBead, prevFiles := agentAssignBead, agentAssignFiles
	t.Cleanup(func() {
		agentAssignBead = prevBead
		agentAssignFiles = prevFiles
		agentAssignBranch = ""
		agentAssignSkills = ""
		agentAssignValidation = ""
	})
	agentAssignBead = "ag-demo"
	agentAssignFiles = "README.md,docs/3.0.md"
	cmd, out := agentTestCmd()
	if err := runAgentAssignPrompt(cmd, nil); err != nil {
		t.Fatalf("assign-prompt: %v", err)
	}
	if !strings.Contains(out.String(), "ag-demo") || !strings.Contains(out.String(), "README.md") {
		t.Fatalf("assignment output missing bead/files: %s", out.String())
	}
}

func TestFilterNTMStatus(t *testing.T) {
	raw := []byte(`{"sessions":[{"name":"other","panes":1,"agents":[]},{"name":"agentops-bg","panes":2,"agents":[{"type":"claude","pane_idx":2,"process_state_name":"sleeping","context_model":"claude-opus"}]}]}`)
	got, err := filterNTMStatus(raw, "agentops-bg")
	if err != nil {
		t.Fatalf("filter status: %v", err)
	}
	if got.Name != "agentops-bg" || got.Panes != 2 || len(got.Agents) != 1 {
		t.Fatalf("filtered status = %+v", got)
	}
	if got.Agents[0].ContextModel != "claude-opus" {
		t.Fatalf("agent status = %+v", got.Agents[0])
	}
}

func TestFilterNTMStatusMissingSession(t *testing.T) {
	if _, err := filterNTMStatus([]byte(`{"sessions":[]}`), "missing"); err == nil {
		t.Fatal("expected missing session error")
	}
}

func TestSplitLabelsCSV(t *testing.T) {
	got := splitLabelsCSV("alpha, beta,,gamma ")
	want := []string{"alpha", "beta", "gamma"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("splitCSV = %v, want %v", got, want)
	}
	if got := splitLabelsCSV(""); len(got) != 0 {
		t.Fatalf("empty splitCSV = %v, want empty", got)
	}
}

func agentTestCmd() (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	return cmd, &out
}
