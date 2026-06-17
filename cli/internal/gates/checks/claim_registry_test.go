package checks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestClaimRegistryDriftRegistration(t *testing.T) {
	c, ok := gates.Default.Get("claim.registry-drift")
	if !ok {
		t.Fatal("claim.registry-drift not registered")
	}
	if !c.Blocking {
		t.Error("claim.registry-drift must be Blocking")
	}
	if c.Run == nil {
		t.Error("claim.registry-drift must be a native Run check")
	}
	if !c.Tiers.Has(gates.Fast) {
		t.Error("claim.registry-drift must include Fast tier")
	}
	if !c.Tiers.Has(gates.Full) {
		t.Error("claim.registry-drift must include Full tier")
	}
}

func TestClaimPMFEvidenceRegistration(t *testing.T) {
	c, ok := gates.Default.Get("claim.pmf-evidence")
	if !ok {
		t.Fatal("claim.pmf-evidence not registered")
	}
	if c.Blocking {
		t.Error("claim.pmf-evidence must be non-Blocking (WARN-only)")
	}
	if c.Backing != "check-pmf-evidence.sh" {
		t.Errorf("claim.pmf-evidence Backing = %q, want check-pmf-evidence.sh", c.Backing)
	}
}

func TestClaimTierCitationRegistration(t *testing.T) {
	c, ok := gates.Default.Get("claim.tier-citation")
	if !ok {
		t.Fatal("claim.tier-citation not registered")
	}
	if !c.Blocking {
		t.Error("claim.tier-citation must be Blocking")
	}
	if c.Run == nil {
		t.Error("claim.tier-citation must be a native Run check")
	}
	if !c.Tiers.Has(gates.Fast) {
		t.Error("claim.tier-citation must include Fast tier")
	}
	if c.Tiers.Has(gates.Full) {
		t.Error("claim.tier-citation is intentionally changed-scope only until historical claim debt is retired")
	}
}

func TestRunClaimRegistryDrift_InSync(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "README.md"),
		"# Hello\n<!-- agentops:claim:AOP-CLAIM-ALPHA -->\n")
	writeFile(t, filepath.Join(dir, "PRODUCT.md"),
		"# Product\n<!-- agentops:claim:AOP-CLAIM-BETA -->\n")

	registryYAML := `version: "1"
tiers: {}
claims:
  AOP-CLAIM-ALPHA:
    tier: UNPROVEN
    surfaces: [README.md]
  AOP-CLAIM-BETA:
    tier: UNPROVEN
    surfaces: [PRODUCT.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)

	gitAdd(t, dir)

	v, err := runClaimRegistryDrift(context.Background(), gates.RunContext{RepoRoot: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusPass {
		t.Errorf("expected PASS, got %s: %s\n%s", v.Status, v.Reason, v.LogTail)
	}
}

func TestRunClaimRegistryDrift_MissingRegistryEntry(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "README.md"),
		"<!-- agentops:claim:AOP-CLAIM-ALPHA -->\n<!-- agentops:claim:AOP-CLAIM-ORPHAN -->\n")

	registryYAML := `version: "1"
tiers: {}
claims:
  AOP-CLAIM-ALPHA:
    tier: UNPROVEN
    surfaces: [README.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)

	gitAdd(t, dir)

	v, err := runClaimRegistryDrift(context.Background(), gates.RunContext{RepoRoot: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusFail {
		t.Errorf("expected FAIL for orphan marker, got %s: %s", v.Status, v.Reason)
	}
	if v.LogTail == "" || !strings.Contains(v.LogTail, "AOP-CLAIM-ORPHAN") {
		t.Errorf("expected log to mention AOP-CLAIM-ORPHAN, got: %s", v.LogTail)
	}
}

func TestRunClaimRegistryDrift_OrphanRegistryEntry(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "README.md"),
		"<!-- agentops:claim:AOP-CLAIM-ALPHA -->\n")

	registryYAML := `version: "1"
tiers: {}
claims:
  AOP-CLAIM-ALPHA:
    tier: UNPROVEN
    surfaces: [README.md]
  AOP-CLAIM-GHOST:
    tier: PILOT
    surfaces: [old-doc.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)

	gitAdd(t, dir)

	v, err := runClaimRegistryDrift(context.Background(), gates.RunContext{RepoRoot: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusFail {
		t.Errorf("expected FAIL for orphan registry entry, got %s: %s", v.Status, v.Reason)
	}
	if v.LogTail == "" || !strings.Contains(v.LogTail, "AOP-CLAIM-GHOST") {
		t.Errorf("expected log to mention AOP-CLAIM-GHOST, got: %s", v.LogTail)
	}
}

func TestRunClaimRegistryDrift_NoRegistryFile(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	v, err := runClaimRegistryDrift(context.Background(), gates.RunContext{RepoRoot: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusFail {
		t.Errorf("expected FAIL when registry missing, got %s", v.Status)
	}
}

func TestExtractClaimIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	writeFile(t, path,
		"line1\n<!-- agentops:claim:AOP-CLAIM-FOO -->\ntext\n<!-- agentops:claim:AOP-CLAIM-BAR -->\n<!-- agentops:claim:AOP-CLAIM-FOO -->\n")

	ids, err := extractClaimIDs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 unique IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "AOP-CLAIM-FOO" || ids[1] != "AOP-CLAIM-BAR" {
		t.Errorf("unexpected IDs: %v", ids)
	}
}

func TestClaimMarkerSkipsAgentsDir(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, ".agents", "internal.md"),
		"<!-- agentops:claim:AOP-CLAIM-PRIVATE -->\n")
	writeFile(t, filepath.Join(dir, "README.md"),
		"<!-- agentops:claim:AOP-CLAIM-PUBLIC -->\n")

	registryYAML := `version: "1"
tiers: {}
claims:
  AOP-CLAIM-PUBLIC:
    tier: UNPROVEN
    surfaces: [README.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)

	gitAdd(t, dir)

	v, err := runClaimRegistryDrift(context.Background(), gates.RunContext{RepoRoot: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusPass {
		t.Errorf("expected PASS (.agents/ excluded), got %s: %s\n%s", v.Status, v.Reason, v.LogTail)
	}
}

func TestRunClaimTierCitation_FailsChangedSurfaceAboveTier(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "README.md"),
		"# Readme\n<!-- agentops:claim:AOP-CLAIM-PUBLIC -->\n")
	registryYAML := `version: "1"
tiers:
  UNPROVEN:
    cite_allowed: ["docs/comparisons/**", "docs/wiki-for-agents.md"]
claims:
  AOP-CLAIM-PUBLIC:
    tier: UNPROVEN
    surfaces: [README.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)

	v, err := runClaimTierCitation(context.Background(), gates.RunContext{
		RepoRoot:     dir,
		ChangedFiles: []string{"README.md"},
		Mode:         gates.Fast,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusFail {
		t.Fatalf("expected FAIL, got %s: %s", v.Status, v.Reason)
	}
	if !strings.Contains(v.LogTail, "AOP-CLAIM-PUBLIC") || !strings.Contains(v.LogTail, "README.md") {
		t.Fatalf("log should name claim and surface, got:\n%s", v.LogTail)
	}
}

func TestRunClaimTierCitation_AllowsChangedSurfaceWithinTier(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "docs", "comparisons", "vs-tool.md"),
		"# Comparison\n<!-- agentops:claim:AOP-CLAIM-COMP -->\n")
	registryYAML := `version: "1"
tiers:
  UNPROVEN:
    cite_allowed: ["docs/comparisons/**", "docs/wiki-for-agents.md"]
claims:
  AOP-CLAIM-COMP:
    tier: UNPROVEN
    surfaces: [docs/comparisons/vs-tool.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)

	v, err := runClaimTierCitation(context.Background(), gates.RunContext{
		RepoRoot:     dir,
		ChangedFiles: []string{"docs/comparisons/vs-tool.md"},
		Mode:         gates.Fast,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusPass {
		t.Fatalf("expected PASS, got %s: %s\n%s", v.Status, v.Reason, v.LogTail)
	}
}

func TestRunClaimTierCitation_IgnoresUnchangedHistoricalDebt(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "README.md"),
		"# Readme\n<!-- agentops:claim:AOP-CLAIM-OLD-DEBT -->\n")
	writeFile(t, filepath.Join(dir, "docs", "notes.md"), "# changed note\n")
	registryYAML := `version: "1"
tiers:
  UNPROVEN:
    cite_allowed: ["docs/comparisons/**", "docs/wiki-for-agents.md"]
claims:
  AOP-CLAIM-OLD-DEBT:
    tier: UNPROVEN
    surfaces: [README.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)

	v, err := runClaimTierCitation(context.Background(), gates.RunContext{
		RepoRoot:     dir,
		ChangedFiles: []string{"docs/notes.md"},
		Mode:         gates.Fast,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusPass {
		t.Fatalf("expected PASS, got %s: %s\n%s", v.Status, v.Reason, v.LogTail)
	}
}

func TestRunClaimTierCitation_RegistryPolicyChangeScansAllMarkers(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "README.md"),
		"# Readme\n<!-- agentops:claim:AOP-CLAIM-OLD-DEBT -->\n")
	registryYAML := `version: "1"
tiers:
  UNPROVEN:
    cite_allowed: ["docs/comparisons/**", "docs/wiki-for-agents.md"]
claims:
  AOP-CLAIM-OLD-DEBT:
    tier: UNPROVEN
    surfaces: [README.md]
`
	writeFile(t, filepath.Join(dir, "docs", "contracts", "claim-registry.yaml"), registryYAML)
	gitAdd(t, dir)

	v, err := runClaimTierCitation(context.Background(), gates.RunContext{
		RepoRoot:     dir,
		ChangedFiles: []string{"docs/contracts/claim-registry.yaml"},
		Mode:         gates.Fast,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != ports.GateStatusFail {
		t.Fatalf("expected FAIL after registry policy change scans all markers, got %s: %s", v.Status, v.Reason)
	}
}

// --- test helpers ---

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "init", "-q")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
}

func gitAdd(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "add", "-A")
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
