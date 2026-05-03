package lifecycle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectRepoReadinessEmptyRepo(t *testing.T) {
	withReadinessEnv(t, nil)
	root := t.TempDir()

	report, err := InspectRepoReadiness(root, ReadinessOptions{})
	if err != nil {
		t.Fatalf("InspectRepoReadiness: %v", err)
	}
	if report.Ready {
		t.Fatal("empty repo should not be ready")
	}
	if report.Template != "generic" {
		t.Fatalf("Template = %q, want generic", report.Template)
	}
	for _, layer := range []ReadinessLayer{LayerCore, LayerGoals, LayerInstructions, LayerTracking, LayerProduct, LayerProgram, LayerSchedule} {
		if !readinessHasLayer(report, layer) {
			t.Fatalf("expected readiness layer %q in report", layer)
		}
	}
}

func TestApplyRepoSeedIdempotent(t *testing.T) {
	withReadinessEnv(t, nil)
	root := t.TempDir()

	first, err := ApplyRepoSeed(root, ReadinessOptions{Template: "generic"})
	if err != nil {
		t.Fatalf("first ApplyRepoSeed: %v", err)
	}
	if !first.Ready {
		t.Fatalf("first report not ready: %#v", first.Items)
	}

	second, err := ApplyRepoSeed(root, ReadinessOptions{Template: "generic"})
	if err != nil {
		t.Fatalf("second ApplyRepoSeed: %v", err)
	}
	if !second.Ready {
		t.Fatalf("second report not ready: %#v", second.Items)
	}

	claudeData, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if got := strings.Count(string(claudeData), ClaudeMDSeedMarker); got != 1 {
		t.Fatalf("CLAUDE seed marker count = %d, want 1", got)
	}
}

func TestApplyRepoSeedRespectsExistingGoalsAndInstructions(t *testing.T) {
	withReadinessEnv(t, nil)
	root := t.TempDir()
	existingGoals := "# Existing Goals\n"
	existingClaude := "# Existing\n\n" + ClaudeMDSeedSection
	if err := os.WriteFile(filepath.Join(root, "GOALS.md"), []byte(existingGoals), 0o644); err != nil {
		t.Fatalf("write GOALS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(existingClaude), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	if _, err := ApplyRepoSeed(root, ReadinessOptions{}); err != nil {
		t.Fatalf("ApplyRepoSeed: %v", err)
	}
	assertFileEquals(t, filepath.Join(root, "GOALS.md"), existingGoals)
	assertFileEquals(t, filepath.Join(root, "CLAUDE.md"), existingClaude)
}

func TestRepoReadinessRespectsAOAgentsDirOverride(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "repo")
	agents := filepath.Join(tmp, "custom-agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	withReadinessEnv(t, map[string]string{"AO_AGENTS_DIR": agents})

	if _, err := ApplyRepoSeed(root, ReadinessOptions{}); err != nil {
		t.Fatalf("ApplyRepoSeed: %v", err)
	}
	report, err := InspectRepoReadiness(root, ReadinessOptions{})
	if err != nil {
		t.Fatalf("InspectRepoReadiness: %v", err)
	}
	if report.AgentsDir != agents {
		t.Fatalf("AgentsDir = %q, want %q", report.AgentsDir, agents)
	}
	if _, err := os.Stat(filepath.Join(agents, "research")); err != nil {
		t.Fatalf("expected research dir under AO_AGENTS_DIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "research")); err == nil {
		t.Fatal("did not expect default .agents/research when AO_AGENTS_DIR is set")
	}
}

func TestRepoReadinessUsesGitRootFallbackForSubdir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	withReadinessEnv(t, nil)
	root := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	subdir := filepath.Join(root, "nested", "pkg")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	report, err := InspectRepoReadiness(subdir, ReadinessOptions{})
	if err != nil {
		t.Fatalf("InspectRepoReadiness: %v", err)
	}
	want := filepath.Join(root, ".agents")
	if !sameReadinessPath(report.AgentsDir, want) {
		t.Fatalf("AgentsDir = %q, want git root fallback %q", report.AgentsDir, want)
	}
}

func sameReadinessPath(a, b string) bool {
	return canonicalReadinessTestPath(a) == canonicalReadinessTestPath(b)
}

func canonicalReadinessTestPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		return filepath.Join(filepath.Clean(resolved), filepath.Base(path))
	}
	return path
}

func TestPlanRepoSeedDryRunDoesNotWrite(t *testing.T) {
	withReadinessEnv(t, nil)
	root := t.TempDir()

	report, err := PlanRepoSeed(root, ReadinessOptions{})
	if err != nil {
		t.Fatalf("PlanRepoSeed: %v", err)
	}
	if !report.DryRun {
		t.Fatal("expected dry-run report")
	}
	if _, err := os.Stat(filepath.Join(root, ".agents")); err == nil {
		t.Fatal("dry-run should not create .agents")
	}
}

func readinessHasLayer(report *ReadinessReport, layer ReadinessLayer) bool {
	for _, item := range report.Items {
		if item.Layer == layer {
			return true
		}
	}
	return false
}

func assertFileEquals(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, string(data), want)
	}
}

func withReadinessEnv(t *testing.T, env map[string]string) {
	t.Helper()
	keys := []string{
		"AO_HOME", "CLAUDE_PLUGIN_DATA",
		"AO_AGENTS_DIR", "AO_KNOWLEDGE_ROOT", "AO_HOOKS_DIR", "AO_SCOPE_LOCK",
		"AO_RPI_DIR", "AO_FINDINGS_DIR", "AO_PLANS_DIR", "AO_COUNCIL_DIR",
		"AO_LEARNINGS_DIR", "AO_PATTERNS_DIR", "AO_DECISIONS_DIR",
	}
	prev := map[string]string{}
	prevSet := map[string]bool{}
	for _, key := range keys {
		prev[key], prevSet[key] = os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unsetenv %s: %v", key, err)
		}
	}
	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("setenv %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if prevSet[key] {
				_ = os.Setenv(key, prev[key])
			} else {
				_ = os.Unsetenv(key)
			}
		}
	})
}
