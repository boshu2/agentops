package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	// Exact Bootstrap value — guards the codex-ntm struct literal against
	// silent edits/drift (the field was the site of an unformatted-source fix).
	if want := `ao session bootstrap && ao inject --bead "$BEAD"`; b.Bootstrap != want {
		t.Errorf("codex-ntm Bootstrap = %q, want exactly %q", b.Bootstrap, want)
	}
	if !strings.Contains(b.Bootstrap, "ao session bootstrap") {
		t.Errorf("codex-ntm Bootstrap must run `ao session bootstrap`, got %q", b.Bootstrap)
	}
	if !strings.Contains(b.Bootstrap, `ao inject --bead "$BEAD"`) {
		t.Errorf("codex-ntm Bootstrap must request bead-scoped injection, got %q", b.Bootstrap)
	}
	if strings.Contains(b.Bootstrap, "ao inject --query") {
		t.Errorf("codex-ntm Bootstrap must not use invalid `ao inject --query`, got %q", b.Bootstrap)
	}
	if b.Reference != "skills-codex/agent-native" {
		t.Errorf("Reference = %q, want skills-codex/agent-native", b.Reference)
	}
	// codex shells ao directly — no MCP descriptor needed.
	if len(b.Tools) != 0 {
		t.Errorf("codex-ntm must not carry MCP tool descriptors, got %d", len(b.Tools))
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
